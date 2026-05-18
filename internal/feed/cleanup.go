package feed

import (
	"fmt"
	"strings"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/queue"
	"github.com/gberns/kerf/internal/spec"
)

// NewCleanupDetectors returns the v1 cleanup detectors as Detector-interface
// values. The supplied projectFilter is the project-wide bead_filter (may be
// nil), used as the second tier of resolution after per-work overrides.
//
// Detector pair (per specs/commands.md §"Behavior" step 3):
//
//   - work_no_attached_beads — fires when a work's resolved bead_filter
//     matches zero beads in the store (attached_count == 0).
//   - work_beads_done_status_open — fires when attached_count > 0 AND every
//     attached bead's status is terminal (closed/done/complete) AND the work's
//     jig status is not its terminal status.
//
// The two detectors are mutually exclusive by construction: the attached-count
// guard on the second detector ensures zero-bead works are reported only by
// the first.
//
// Items targeting archived/finalized works are suppressed here (matching the
// downstream Exclude rule in feed.Assemble — defense in depth).
func NewCleanupDetectors(projectFilter *beads.Filter) []Detector {
	return []Detector{
		DetectorFunc(func(in Input) []Item {
			return detectWorkNoAttachedBeads(in, projectFilter)
		}),
		DetectorFunc(func(in Input) []Item {
			return detectWorkBeadsDoneStatusOpen(in, projectFilter)
		}),
	}
}

// detectWorkNoAttachedBeads emits a cleanup item per non-archived/non-finalized
// work whose resolved bead_filter matches zero beads in the store.
func detectWorkNoAttachedBeads(in Input, projectFilter *beads.Filter) []Item {
	scoreByWork := scoreIndex(in.QueueEntries)
	out := make([]Item, 0)
	for _, w := range in.Works {
		if w == nil {
			continue
		}
		codename := w.Codename
		if in.ArchivedOrFinalized[codename] {
			continue
		}
		resolved := beads.Resolve(w.BeadFilter, projectFilter)
		matched := beads.ForWorkWithFilter(in.AllBeads, codename, resolved)
		if len(matched) != 0 {
			continue
		}
		wc := codename
		// Rank-label classification per specs/coordination.md §"Rank Labels
		// for Zero-Match Works" (Plan 019 / B2):
		//   - unwired: no per-work bead_filter key authored on spec.yaml.
		//   - empty:   per-work bead_filter present and parsed, but resolves
		//              to zero matches.
		//   - broken:  filter present but malformed. spec.Read rejects
		//              malformed filters at parse time, so this state is not
		//              observable here today; per Plan 019 open question 5
		//              it collapses into "empty" until parser support lands.
		rankLabel := "empty"
		reason := "resolved bead_filter matches zero beads in the store"
		action := "edit spec.yaml bead_filter or check the project filter"
		if w.BeadFilter == nil {
			rankLabel = "unwired"
			reason = "no bead_filter declared on spec.yaml"
			action = "kerf work edit " + codename + " --bead-filter-add '<clause>'"
		}
		out = append(out, Item{
			Kind:         KindCleanup,
			Score:        scoreByWork[codename],
			Title:        "no attached beads",
			Action:       action,
			WorkCodename: &wc,
			BeadID:       nil,
			Reason:       reason,
			RankLabel:    rankLabel,
		})
	}
	return out
}

// detectWorkBeadsDoneStatusOpen emits a cleanup item per non-archived/
// non-finalized work whose attached beads are all closed but whose jig status
// is not terminal.
func detectWorkBeadsDoneStatusOpen(in Input, projectFilter *beads.Filter) []Item {
	scoreByWork := scoreIndex(in.QueueEntries)
	out := make([]Item, 0)
	for _, w := range in.Works {
		if w == nil {
			continue
		}
		codename := w.Codename
		if in.ArchivedOrFinalized[codename] {
			continue
		}
		resolved := beads.Resolve(w.BeadFilter, projectFilter)
		matched := beads.ForWorkWithFilter(in.AllBeads, codename, resolved)
		if len(matched) == 0 {
			continue // mutual exclusion: handled by detectWorkNoAttachedBeads
		}
		if !allClosed(matched) {
			continue
		}
		if isJigTerminal(w) {
			continue
		}
		wc := codename
		out = append(out, Item{
			Kind:         KindCleanup,
			Score:        scoreByWork[codename],
			Title:        "beads done, jig walk owed",
			Action:       fmt.Sprintf("kerf status %s <next-stage> or kerf shelve %s", codename, codename),
			WorkCodename: &wc,
			BeadID:       nil,
			Reason:       fmt.Sprintf("%d attached beads closed; status: %s", len(matched), w.Status),
		})
	}
	return out
}

func scoreIndex(entries []queue.Entry) map[string]float64 {
	out := make(map[string]float64, len(entries))
	for _, e := range entries {
		out[e.Codename] = e.Score
	}
	return out
}

// allClosed reports whether every bead in the slice has a terminal status
// (closed/done/complete, case-insensitive). An empty slice returns false —
// callers gate on attached_count > 0 first.
func allClosed(bds []beads.Bead) bool {
	if len(bds) == 0 {
		return false
	}
	for _, b := range bds {
		if !isClosedStatus(b.Status) {
			return false
		}
	}
	return true
}

func isClosedStatus(s string) bool {
	switch strings.ToLower(s) {
	case "closed", "done", "complete", "completed":
		return true
	}
	return false
}

// isJigTerminal reports whether the work's current Status equals the last
// value in its StatusValues (the jig's terminal stage). If StatusValues is
// empty, the work is treated as non-terminal — the detector still fires so
// the misconfiguration surfaces via the cleanup item rather than being
// silently swallowed.
func isJigTerminal(w *spec.SpecYAML) bool {
	if len(w.StatusValues) == 0 {
		return false
	}
	return w.Status == w.StatusValues[len(w.StatusValues)-1]
}
