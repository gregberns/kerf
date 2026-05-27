// Graph-signal decorator: read-only, append-only enhancement of an
// already-computed []Entry with the T=0 static analyzer's signals
// (critical-path length, fan-out width, area-overlap density). Surfaced via
// the existing `reason` field on `kerf next --format=json` output — no new
// top-level fields, no new command.
//
// Spec reference: specs/coordination.md §"Graph signals (T=0 static
// analyzer)".
//
// Plan 014 / Bead B2 (kerf-n9vq). The decorator is intentionally narrow:
// it does not change scoring, ordering, or the set of entries. The next
// bead in the plan (B3) owns the bigger refactor of internal/queue.

package queue

import (
	"fmt"

	"github.com/gregberns/kerf/internal/spec"
	"github.com/gregberns/kerf/internal/static"
)

// DecorateGraphSignals computes graph-shape signals on the work graph and
// appends matching reason strings onto each Entry.Reasons slice. The
// function returns a map of codename -> the signal strings appended for
// that entry (the freshly added portion only), so callers that surface
// per-item reason text (e.g. cmd/next.go feeding feed.Item.Reason) can
// pick up the new lines without diffing the slice.
//
// Signals are appended when they are informative on the work graph:
//
//   - critical-path: appended to every entry whose codename appears on the
//     graph's longest weighted chain. Chain length is reported as node
//     count (Estimate is set to 1 per work today; later betting will pull
//     real duration estimates).
//   - fan-out: appended when the entry has at least one direct dependent.
//   - area-overlap: appended once to every entry, naming the density across
//     the currently actionable candidate set (the entries supplied here),
//     when there are at least two entries with declared areas.
//
// The function is pure-with-respect-to-its-inputs: it does not consult I/O
// or shared state. It mutates entries in place (appending to Reasons) but
// does not reorder them.
func DecorateGraphSignals(entries []Entry, works []*spec.SpecYAML) map[string][]string {
	if len(entries) == 0 || len(works) == 0 {
		return nil
	}
	g := buildGraph(works)
	chain, chainLen := static.CriticalPath(g)
	onCritical := make(map[string]bool, len(chain))
	for _, id := range chain {
		onCritical[id] = true
	}

	// Area-overlap density across the candidate set (the actionable
	// entries supplied by the caller). One number; surface only when
	// meaningful (>=2 entries; at least one area declared).
	candidateIDs := make([]string, 0, len(entries))
	areaCount := 0
	for _, e := range entries {
		candidateIDs = append(candidateIDs, e.Codename)
		if len(e.Areas) > 0 {
			areaCount++
		}
	}
	var overlap float64
	overlapMeaningful := len(candidateIDs) >= 2 && areaCount >= 2
	if overlapMeaningful {
		overlap = static.AreaOverlap(g, candidateIDs)
	}

	added := make(map[string][]string, len(entries))
	for i := range entries {
		cn := entries[i].Codename
		var fresh []string
		if onCritical[cn] {
			fresh = append(fresh,
				fmt.Sprintf("on critical path (length %.0f)", chainLen))
		}
		if fo := static.FanOut(g, cn); fo > 0 {
			fresh = append(fresh, fmt.Sprintf("graph fan-out %d", fo))
		}
		if overlapMeaningful {
			fresh = append(fresh,
				fmt.Sprintf("area-overlap density %.2f", overlap))
		}
		if len(fresh) == 0 {
			continue
		}
		entries[i].Reasons = append(entries[i].Reasons, fresh...)
		added[cn] = fresh
	}
	return added
}

// buildGraph adapts kerf work specs into a static.Graph. Edges follow
// spec.SpecYAML.DependsOn with relationship "must-complete-first" (the same
// edge semantics queue.Compute uses for fan-out and gating). Estimate is
// fixed at 1.0 per work today; later beads in plan 014 may swap in real
// duration estimates.
func buildGraph(works []*spec.SpecYAML) static.Graph {
	nodes := make([]static.Node, 0, len(works))
	for _, w := range works {
		var deps []string
		for _, d := range w.DependsOn {
			if d.Relationship != "must-complete-first" {
				continue
			}
			deps = append(deps, d.Codename)
		}
		nodes = append(nodes, static.Node{
			ID:        w.Codename,
			Estimate:  1.0,
			Areas:     append([]string(nil), w.Areas...),
			DependsOn: deps,
		})
	}
	return static.NewGraph(nodes)
}
