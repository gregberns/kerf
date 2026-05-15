package feed

import (
	"sort"
	"time"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/queue"
	"github.com/gberns/kerf/internal/spec"
)

// Input is the pre-loaded state a Detector consumes. Callers (cmd/next.go)
// own the I/O of populating it; the feed package is pure over Input.
//
// WorkCreated maps work codename -> created timestamp, used as the
// cleanup tie-break per specs/coordination.md.
//
// BlockedWorks contains codenames of works whose must-complete-first
// dependencies are unmet. Bead items targeting these works are excluded
// by Assemble; cleanup items targeting them are NOT excluded (cleanup
// runs on blocked works too — see specs/commands.md §"kerf next").
//
// ArchivedOrFinalized contains codenames of works in archived/finalized
// state — both bead and cleanup items targeting these are excluded.
type Input struct {
	Works               []*spec.SpecYAML
	AllBeads            []beads.Bead
	QueueEntries        []queue.Entry
	ProjectID           string
	WorkCreated         map[string]time.Time
	BlockedWorks        map[string]bool
	ArchivedOrFinalized map[string]bool
}

// Detector produces a slice of Items from project state. Detectors are pure.
// Bead B4 and B5 supply concrete implementations (cleanup, warnings).
type Detector interface {
	Detect(in Input) []Item
}

// DetectorFunc adapts a plain function to the Detector interface.
type DetectorFunc func(in Input) []Item

func (f DetectorFunc) Detect(in Input) []Item { return f(in) }

// BeadSource emits one Item{Kind: KindBead} per (ready bead, matching work)
// pair. A bead is ready when its status is not blocked, in-progress, or
// complete. The bead's score is taken from its parent work's queue.Entry.
//
// Excluded automatically: beads whose target work is blocked, archived,
// or finalized (Assemble enforces this via Exclude as well; BeadSource is
// the canonical producer).
func BeadSource(in Input) []Item {
	if len(in.AllBeads) == 0 {
		return nil
	}
	scoreByWork := make(map[string]float64, len(in.QueueEntries))
	for _, e := range in.QueueEntries {
		scoreByWork[e.Codename] = e.Score
	}
	out := make([]Item, 0, len(in.AllBeads))
	for _, b := range in.AllBeads {
		if !isReady(b) {
			continue
		}
		work := b.Epic
		score := scoreByWork[work]
		id := b.ID
		wc := work
		var workPtr *string
		if work != "" {
			workPtr = &wc
		}
		out = append(out, Item{
			Kind:         KindBead,
			Score:        score,
			Title:        b.Title,
			Action:       "", // populated by renderer based on jig + status
			WorkCodename: workPtr,
			BeadID:       &id,
			Reason:       "",
		})
	}
	return out
}

func isReady(b beads.Bead) bool {
	switch b.Status {
	case "blocked", "in_progress", "in-progress", "closed", "complete", "completed", "done":
		return false
	}
	return true
}

// AssembleOpts carries optional lookup data for Assemble. WorkCreated is
// the tie-break source for cleanup ordering. If nil, Assemble falls back to
// Input.WorkCreated.
type AssembleOpts struct {
	WorkCreated map[string]time.Time
}

// Assemble composes the final ordered Item slice from already-classified
// inputs. Beads come first (sorted by Score desc); cleanups follow (sorted
// by Score desc, tie-broken by parent-work created ascending). Warnings are
// NOT interleaved — they are returned separately by AssembleWithWarnings
// for the renderer to draw as a header block.
//
// Exclusion rules are applied here as a safety net:
//   - Bead items: excluded if WorkCodename is in BlockedWorks OR
//     ArchivedOrFinalized.
//   - Cleanup items: excluded ONLY if WorkCodename is in ArchivedOrFinalized
//     (blocked works still get cleanup).
//   - Warning items: never excluded.
func Assemble(bds, cleanups, warnings []Item, opts AssembleOpts) []Item {
	state := Input{WorkCreated: opts.WorkCreated}
	main, _ := AssembleWithWarnings(bds, cleanups, warnings, state)
	return main
}

// AssembleWithWarnings returns (mainItems, warnings) where mainItems is
// beads-then-cleanups in render order, and warnings is the unfiltered
// warning slice (header block).
func AssembleWithWarnings(bds, cleanups, warnings []Item, state Input) ([]Item, []Item) {
	filteredBeads := excludeBeads(bds, state)
	filteredCleanups := excludeCleanups(cleanups, state)

	sortBeads(filteredBeads)
	sortCleanups(filteredCleanups, state.WorkCreated)

	out := make([]Item, 0, len(filteredBeads)+len(filteredCleanups))
	out = append(out, filteredBeads...)
	out = append(out, filteredCleanups...)

	// Warnings: insertion order preserved; never filtered.
	w := make([]Item, len(warnings))
	copy(w, warnings)
	return out, w
}

func excludeBeads(items []Item, state Input) []Item {
	if len(state.BlockedWorks) == 0 && len(state.ArchivedOrFinalized) == 0 {
		return append([]Item(nil), items...)
	}
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.WorkCodename != nil {
			wc := *it.WorkCodename
			if state.BlockedWorks[wc] || state.ArchivedOrFinalized[wc] {
				continue
			}
		}
		out = append(out, it)
	}
	return out
}

func excludeCleanups(items []Item, state Input) []Item {
	if len(state.ArchivedOrFinalized) == 0 {
		return append([]Item(nil), items...)
	}
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.WorkCodename != nil && state.ArchivedOrFinalized[*it.WorkCodename] {
			continue
		}
		out = append(out, it)
	}
	return out
}

// Exclude applies the spec's exclusion rules to a mixed slice in place.
// Kept as a stand-alone helper for callers that already have a pre-merged
// list (tests, alternative pipelines).
func Exclude(items []Item, state Input) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		switch it.Kind {
		case KindBead:
			if it.WorkCodename != nil {
				wc := *it.WorkCodename
				if state.BlockedWorks[wc] || state.ArchivedOrFinalized[wc] {
					continue
				}
			}
		case KindCleanup:
			if it.WorkCodename != nil && state.ArchivedOrFinalized[*it.WorkCodename] {
				continue
			}
		case KindWarning:
			// never excluded
		}
		out = append(out, it)
	}
	return out
}

// Sort sorts a flat slice of feed items per the cross-kind rule:
// beads ranked first (by Score desc), then cleanups (by Score desc, tied
// on parent-work created ascending), then warnings (insertion order).
// `workCreated` may be nil — ties then resort to codename ascending for
// determinism.
func Sort(items []Item, workCreated map[string]time.Time) {
	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := kindRank(items[i].Kind), kindRank(items[j].Kind)
		if ri != rj {
			return ri < rj
		}
		// Same kind.
		switch items[i].Kind {
		case KindBead:
			return items[i].Score > items[j].Score
		case KindCleanup:
			if items[i].Score != items[j].Score {
				return items[i].Score > items[j].Score
			}
			return cleanupTieBreak(items[i], items[j], workCreated)
		default:
			return false // warnings: insertion order
		}
	})
}

func kindRank(k Kind) int {
	switch k {
	case KindBead:
		return 0
	case KindCleanup:
		return 1
	case KindWarning:
		return 2
	}
	return 3
}

func sortBeads(items []Item) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
}

func sortCleanups(items []Item, workCreated map[string]time.Time) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return cleanupTieBreak(items[i], items[j], workCreated)
	})
}

func cleanupTieBreak(a, b Item, workCreated map[string]time.Time) bool {
	var ac, bc string
	if a.WorkCodename != nil {
		ac = *a.WorkCodename
	}
	if b.WorkCodename != nil {
		bc = *b.WorkCodename
	}
	if workCreated != nil {
		at, aok := workCreated[ac]
		bt, bok := workCreated[bc]
		if aok && bok && !at.Equal(bt) {
			return at.Before(bt)
		}
	}
	// Deterministic fallback when no timestamps available.
	return ac < bc
}
