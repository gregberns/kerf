package feed

// Pin layer: applies `spec.yaml.pinned_beads` overrides on top of the
// filter-resolved BeadToWork join. Per specs/coordination.md §"Pin Layer":
//
//   1. Filters resolve into a candidate set of (bead, work) pairs.
//   2. If a bead ID appears in any work's pinned_beads, its attachment is
//      restricted to that single pinning work — it is removed from every
//      other work's filter-resolved match.
//   3. A bead pinned to a work it would not otherwise match attaches to
//      that work regardless of filter outcome.
//
// The pin layer is single-owner: a bead ID appears in at most one work's
// pinned_beads list across the project (specs/coordination.md §"Single-
// owner invariant"). The caller (cmd/next.go, cmd/triage.go) is
// responsible for detecting two-owner conflicts when it scans every
// active work's spec.yaml; ResolvePins receives an already-collapsed
// PinAssignments map (bead ID → owning work codename) and therefore
// cannot directly observe a conflict. cmd/pin.go (B9) is the primary
// enforcement boundary; the caller's two-owner detection is defense-in-
// depth against a manual edit to spec.yaml.
//
// Pin-layer placement (load-bearing). ResolvePins MUST run BEFORE
// BeadSource emits. BeadSource still reads in.BeadToWork unchanged; the
// caller composes:
//
//     beadToWork := buildBeadToWork(works, beads, ...)
//     beadToWork = feed.ResolvePins(beadToWork, pinAssignments)
//     in := feed.Input{BeadToWork: beadToWork, PinAssignments: pinAssignments, ...}
//     items := feed.BeadSource(in)
//
// Mutating BeadToWork inside Assemble is too late — items are already
// emitted. See Plan 009 / Bead 5.

// ResolvePins returns a new BeadToWork map with the pin layer applied on
// top of the filter-resolved join `beadToWork`. The input map is not
// mutated; ResolvePins is a pure helper.
//
// Semantics:
//   - For each bead ID present in pinAssignments, the returned slice is
//     replaced with [owningWork], regardless of what the filter resolved.
//     Any other works that matched the bead via filter are dropped.
//   - A bead pinned to a work but absent from `beadToWork` (i.e. the
//     filter matched nothing) is added with [owningWork]. This is the
//     whole point of pins: surface a bead that would not otherwise
//     attach.
//   - Beads not present in pinAssignments pass through unchanged
//     (their original slice is copied so the caller cannot mutate the
//     input map through the result).
//
// Returns a non-nil map even when both inputs are empty.
func ResolvePins(beadToWork map[string][]string, pinAssignments map[string]string) map[string][]string {
	out := make(map[string][]string, len(beadToWork)+len(pinAssignments))

	// Copy filter-resolved entries that are NOT pinned. Pinned entries
	// are filled below to ensure they override.
	for bid, works := range beadToWork {
		if _, pinned := pinAssignments[bid]; pinned {
			continue
		}
		cp := make([]string, len(works))
		copy(cp, works)
		out[bid] = cp
	}

	// Apply pins: each pinned bead attaches solely to its owning work.
	// This both overrides a multi-match filter result and surfaces a
	// bead that the filter did not match at all.
	for bid, owner := range pinAssignments {
		if owner == "" {
			// Defensive: an empty owner is meaningless. Skip rather
			// than emit a bogus empty-codename attachment.
			continue
		}
		out[bid] = []string{owner}
	}

	return out
}
