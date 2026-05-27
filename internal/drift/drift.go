package drift

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/gregberns/kerf/internal/beads"
)

// BeadRecord is the per-bead entry in a Snapshot. It carries the fields
// kerf consumes for drift detection plus the deterministic content hash.
// See specs/coordination.md §"Snapshot shape" for the canonical JSON
// shape.
type BeadRecord struct {
	Status string   `json:"status"`
	Labels []string `json:"labels"`
	Title  string   `json:"title"`
	Deps   []string `json:"deps"`
	Hash   string   `json:"hash"`
}

// Snapshot is the on-disk drift baseline. Written to
// `.kerf/sync-cache.json` by `kerf triage --ack`; read on every
// invocation that touches the bead store. A zero-value Snapshot (or a
// missing cache file) is the empty baseline — every current bead reads
// as "new since baseline".
type Snapshot struct {
	SnapshotID        string                `json:"snapshot_id"`
	CapturedAt        time.Time             `json:"captured_at"`
	Beads             map[string]BeadRecord `json:"beads"`
	FilterAssignments map[string][]string   `json:"filter_assignments"`
}

// Capture builds a Snapshot from the current bead store and the resolved
// filter assignments (bead ID -> work codenames). Labels and deps in
// each BeadRecord are normalized (lowercased+dedup+sorted for labels;
// sorted for deps) so that the snapshot is byte-stable across runs even
// when the bead store returns the same data in different orders. The
// snapshot's CapturedAt is set to time.Now() in UTC.
//
// SnapshotID is sha256 (lowercase hex) over the per-bead hashes joined
// in sorted-by-id order as `<id>:<hash>\n` lines, per
// specs/coordination.md §"Snapshot shape".
func Capture(all []beads.Bead, assignments map[string][]string) Snapshot {
	snap := Snapshot{
		CapturedAt:        time.Now().UTC(),
		Beads:             make(map[string]BeadRecord, len(all)),
		FilterAssignments: make(map[string][]string, len(assignments)),
	}

	ids := make([]string, 0, len(all))
	for _, b := range all {
		labels := normalizeLabels(b.Labels)
		deps := normalizeDeps(b.DependsOn)
		snap.Beads[b.ID] = BeadRecord{
			Status: strings.ToLower(b.Status),
			Labels: labels,
			Title:  b.Title,
			Deps:   deps,
			Hash:   HashBead(b),
		}
		ids = append(ids, b.ID)
	}
	sort.Strings(ids)

	// Copy assignments with sorted codename slices for byte-stable output.
	for bid, works := range assignments {
		cp := make([]string, len(works))
		copy(cp, works)
		sort.Strings(cp)
		snap.FilterAssignments[bid] = cp
	}

	var sb strings.Builder
	for _, id := range ids {
		sb.WriteString(id)
		sb.WriteByte(':')
		sb.WriteString(snap.Beads[id].Hash)
		sb.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(sb.String()))
	snap.SnapshotID = hex.EncodeToString(sum[:])

	return snap
}

// Diff is the categorized result of comparing two Snapshots. Each field
// is a sorted slice of bead IDs. See specs/coordination.md §"Drift
// categories" for category definitions.
//
// - New: present at current, absent at baseline.
// - Deleted: present at baseline, absent at current.
// - ClosedExternally: present at both; status moved from non-closed to closed.
// - ReopenedExternally: present at both; status moved from closed to non-closed.
// - Changed: present at both, same closed/open polarity, hashes differ
//            (relabel, retitle, dependency change, status change within
//            the same closed/open class).
type Diff struct {
	New                []string
	Deleted            []string
	ClosedExternally   []string
	ReopenedExternally []string
	Changed            []string
}

// Compute returns the categorized diff between a baseline snapshot and
// the current snapshot. closedStatuses names the bead-tool statuses
// considered terminal; the caller owns this set because terminal-status
// vocabulary is a property of the bead tool, not of drift.
//
// Empty baseline (zero-value Snapshot, or one with no Beads) → every
// current bead is classified as New.
func Compute(baseline, current Snapshot, closedStatuses map[string]bool) Diff {
	var d Diff

	// Iterate baseline → categorize Deleted / Closed / Reopened / Changed.
	for id, baseRec := range baseline.Beads {
		curRec, present := current.Beads[id]
		if !present {
			d.Deleted = append(d.Deleted, id)
			continue
		}
		baseClosed := closedStatuses[strings.ToLower(baseRec.Status)]
		curClosed := closedStatuses[strings.ToLower(curRec.Status)]
		switch {
		case !baseClosed && curClosed:
			d.ClosedExternally = append(d.ClosedExternally, id)
		case baseClosed && !curClosed:
			d.ReopenedExternally = append(d.ReopenedExternally, id)
		case baseRec.Hash != curRec.Hash:
			d.Changed = append(d.Changed, id)
		}
	}

	// Iterate current → categorize New (anything absent from baseline).
	for id := range current.Beads {
		if _, present := baseline.Beads[id]; !present {
			d.New = append(d.New, id)
		}
	}

	sort.Strings(d.New)
	sort.Strings(d.Deleted)
	sort.Strings(d.ClosedExternally)
	sort.Strings(d.ReopenedExternally)
	sort.Strings(d.Changed)

	return d
}

// Empty reports whether the diff has no entries in any category.
func (d Diff) Empty() bool {
	return len(d.New) == 0 &&
		len(d.Deleted) == 0 &&
		len(d.ClosedExternally) == 0 &&
		len(d.ReopenedExternally) == 0 &&
		len(d.Changed) == 0
}
