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
//
// BeadToWork maps bead ID -> the list of work codenames whose resolved
// bead_filter matches that bead (per specs/coordination.md §"Bead
// Attachment"). The caller (cmd/next.go) builds this map by running the
// resolved per-work filter over the bead store via
// beads.ForWorkWithFilter and inverting the result. A bead that matches
// no work is absent from the map; a bead that matches N works appears
// with a slice of length N. BeadSource consults this map (NOT the
// bead's Epic field) to attach beads to works — see Plan 008 / Bead 3
// for rationale (the Epic field is not populated by the bd→br migration
// and discarded the resolved filter).
type Input struct {
	Works               []*spec.SpecYAML
	AllBeads            []beads.Bead
	QueueEntries        []queue.Entry
	ProjectID           string
	// ProjectFilter is the project-wide bead_filter from project.yaml.
	// Per-work filters live on each spec.SpecYAML.BeadFilter. Resolution
	// follows beads.Resolve(perWork, project) — see specs/coordination.md
	// §"Resolution order". Used by warning detectors (B5) and any future
	// detectors that need the effective per-work filter set.
	ProjectFilter       *beads.Filter
	WorkCreated         map[string]time.Time
	BlockedWorks        map[string]bool
	ArchivedOrFinalized map[string]bool
	// BeadToWork is the bead.ID -> matching-work-codenames join, computed
	// by the caller from each work's resolved bead_filter. See type-level
	// doc above and BeadSource for multi-match emission semantics.
	BeadToWork map[string][]string

	// CorruptSpecs lists per-work spec.yaml files that failed to parse
	// during feed assembly. Populated by cmd/next.go's work-loading loop
	// (replacing the legacy silent-skip). The `corrupt_spec` warning
	// detector emits one warning per entry. Per specs/commands.md §"kerf
	// next" §"Warning kinds" → `corrupt_spec`.
	CorruptSpecs []CorruptSpec

	// NoProjectYAML is true when the project resolves but project.yaml is
	// absent from both local-storage and bench paths. Populated by
	// cmd/next.go. The `no_project_yaml` warning detector emits a single
	// fatal warning; the caller is responsible for suppressing feed
	// rendering and setting a non-zero exit per specs/commands.md
	// §"Warning kinds" → `no_project_yaml`.
	NoProjectYAML bool
}

// CorruptSpec records a per-work spec.yaml that failed to parse.
// Codename is the directory-derived codename (since the spec itself could
// not be parsed); ParseError is the underlying error string surfaced in
// the warning's reason field.
type CorruptSpec struct {
	Codename   string
	ParseError string
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
// Attachment join: BeadSource consults in.BeadToWork (built by the caller
// from each work's resolved bead_filter) — NOT the bead's Epic field.
// Multi-match semantics: a bead matching N works produces N items, one
// per match, each carrying a distinct WorkCodename and the score of that
// specific work. This is consistent with beads.Match semantics and lets
// agents see the same bead under each work it might satisfy. A bead
// absent from BeadToWork matches no work and yields no item here; such
// beads are surfaced separately as the "unmatched" count in the
// `kerf next` header.
//
// Emission order: BeadSource iterates in.AllBeads in input order, and for
// each bead emits its matches in the order they appear in
// BeadToWork[bead.ID]. Final ordering is established by Assemble (Score
// desc). Stability across runs is the caller's responsibility (build
// BeadToWork deterministically).
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
		matches := in.BeadToWork[b.ID]
		if len(matches) == 0 {
			// Unattached bead: no item emitted. Surfaced elsewhere
			// (unmatched header count). See Plan 008 / Bead 3.
			continue
		}
		id := b.ID
		for _, work := range matches {
			wc := work
			workPtr := &wc
			out = append(out, Item{
				Kind:         KindBead,
				Score:        scoreByWork[work],
				Title:        b.Title,
				Action:       "", // populated by renderer based on jig + status
				WorkCodename: workPtr,
				BeadID:       &id,
				Reason:       "",
			})
		}
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
