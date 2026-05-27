// Package labelsample analyses a bead store and proposes a per-work
// `bead_filter` clause for a given codename. It is the standalone library that
// `kerf bootstrap-filters` (Plan 019 / kerf-a7t) consumes — pure functions, no
// filesystem, no CLI surface.
//
// The sampler is convention-aware: it recognises both prefixed labels
// (e.g. `subsystem:bridge`, `codename:bridge`) and bare labels (e.g. `bridge`)
// and proposes whichever pattern dominates for the work in question. The
// distinction matters because the harmonik feedback that motivated Plan 019
// noted that real bead corpora mix the two conventions — a project-wide
// detector cannot pick one shape for every work without misfiring.
//
// The output shape is the canonical `bead_filter` schema documented in
// specs/architecture.md (the `project.yaml` example block beginning at the
// "Project-wide bead attachment filter for `kerf next` ..." comment): a single
// leaf clause when one candidate dominates, or an `any:` union when several
// candidates carry non-trivial counts. The library returns the existing
// internal/beads.Filter struct directly so callers can serialise it without
// translation.
package labelsample

import (
	"sort"

	"github.com/gregberns/kerf/internal/beads"
)

// Tunables for the sampler. Kept as named constants so the spec change has one
// place to update. The dominance threshold (80%) and the absolute floor (3
// matches) follow specs/commands.md §"kerf bootstrap-filters" step 3.3.
const (
	// dominanceFraction is the share of total candidate matches one clause
	// must clear to be proposed alone (≥ 80%).
	dominanceFraction = 0.80
	// minAbsoluteFloor is the minimum bead count a clause must reach before
	// the sampler will propose it. Below this floor the signal is too weak
	// regardless of share.
	minAbsoluteFloor = 3
	// minUnionMemberFloor is the floor a clause must reach to join an `any:`
	// union when no single candidate dominates. Lower than the
	// single-clause floor because the union represents a softer commitment.
	minUnionMemberFloor = 2
)

// Reason classifies why the sampler returned the proposal it did. It is part
// of the public API so `kerf bootstrap-filters` can render a precise summary
// line per work without re-deriving the verdict.
type Reason int

const (
	// ReasonDominant — one candidate cleared the dominance threshold and the
	// absolute floor. Proposal is a single-leaf Filter.
	ReasonDominant Reason = iota
	// ReasonUnion — no single candidate dominated, but two or more candidates
	// cleared the union floor. Proposal is an `any:` Filter.
	ReasonUnion
	// ReasonNoMatch — no candidate label matched any bead in the store. No
	// proposal; the work stays `unwired`.
	ReasonNoMatch
	// ReasonBelowFloor — at least one candidate matched, but no candidate
	// reached the absolute floor. No proposal; the signal is too weak.
	ReasonBelowFloor
)

// String renders the Reason for diagnostics.
func (r Reason) String() string {
	switch r {
	case ReasonDominant:
		return "dominant"
	case ReasonUnion:
		return "union"
	case ReasonNoMatch:
		return "no-match"
	case ReasonBelowFloor:
		return "below-floor"
	default:
		return "unknown"
	}
}

// Candidate captures one label shape considered for a work, plus the number
// of beads it matched in the supplied store. Exposed so callers can render a
// detail view of the sampler's decision (which shapes were considered, which
// counts each carried) — useful for the dry-run preview text.
type Candidate struct {
	// Clause is the bead_filter clause this candidate would produce.
	Clause beads.Filter
	// Label is the literal label string the candidate looks for (the value
	// that would be substituted into Clause.Label at match time).
	Label string
	// Count is how many beads in the supplied slice carry the Label.
	Count int
}

// Proposal is the sampler's verdict for one work. When Filter is nil there is
// no proposal — Reason explains why (ReasonNoMatch or ReasonBelowFloor) and
// Candidates is empty or holds the sub-threshold counts for diagnostics.
type Proposal struct {
	// Filter is the proposed bead_filter clause for the work, or nil when
	// the sampler had nothing confident to suggest.
	Filter *beads.Filter
	// Reason explains the verdict (see Reason constants).
	Reason Reason
	// MatchCount is the number of beads the proposed Filter would match
	// (sum of union members for ReasonUnion; the single count for
	// ReasonDominant; 0 otherwise).
	MatchCount int
	// Candidates is the full set of candidate shapes considered, with their
	// per-label match counts. Ordered by descending Count, with alphabetical
	// tiebreak. Always populated when at least one candidate matched.
	Candidates []Candidate
}

// candidateShapes returns the ordered list of (clause, label) pairs the
// sampler considers for a given codename. The shapes follow the bead body's
// acceptance criteria: codename, codename combined with the standard
// prefixes (`codename:`, `subsystem:`, `area:`, `kind:`), and the bare slug.
//
// The bare-slug shape and the `codename:<slug>` shape are distinct candidates
// rather than aliases — the harmonik feedback noted that some projects use
// bare labels and others use the prefixed form. The sampler counts them
// separately and lets the dominance / union rules pick.
func candidateShapes(codename string) []Candidate {
	if codename == "" {
		return nil
	}
	prefixes := []string{"codename", "subsystem", "area", "kind"}
	out := make([]Candidate, 0, len(prefixes)+1)
	for _, p := range prefixes {
		lbl := p + ":" + codename
		out = append(out, Candidate{
			Clause: beads.Filter{Label: lbl},
			Label:  lbl,
		})
	}
	// Bare codename slug — the "no prefix at all" convention.
	out = append(out, Candidate{
		Clause: beads.Filter{Label: codename},
		Label:  codename,
	})
	return out
}

// ProposeFilter is the sampler's public entry point. Given the full bead
// slice (typically open beads from the store) and a work codename, it returns
// a Proposal describing the dominant label convention — or no proposal when
// the signal is too weak.
//
// Algorithm (see specs/commands.md §"kerf bootstrap-filters" step 3):
//  1. Build candidate shapes for the codename.
//  2. Count exact-label matches per candidate. (Matching is case-sensitive,
//     same as beads.Filter.Match.)
//  3. If exactly one candidate dominates (≥ 80% of total matches across
//     candidates AND its count ≥ minAbsoluteFloor), propose that single clause.
//  4. Else, if two or more candidates clear minUnionMemberFloor, propose an
//     `any:` union of those clauses. Order: descending count, alphabetical
//     tiebreak.
//  5. Else, if total matches > 0 but no candidate cleared the floor, return
//     ReasonBelowFloor with no proposal.
//  6. Else, return ReasonNoMatch.
//
// The function is pure: it does not read from disk, the network, or any
// global state. Safe to call concurrently.
func ProposeFilter(all []beads.Bead, codename string) Proposal {
	return ProposeFilterWithFloor(all, codename, minAbsoluteFloor)
}

// ProposeFilterWithFloor is identical to ProposeFilter but lets the caller
// override the absolute minimum-match floor for a dominant proposal. The
// bootstrap-filters path uses the default (3) — it writes filters in bulk
// and wants a confident signal before doing so. The `kerf next` near-match
// advisor (cmd/next.go computeNearMatchHints, kerf-fx5) lowers it to 2:
// it surfaces an inline hint a human / agent reads before applying, so a
// softer signal is acceptable and avoids the dogfood-2026-05-18 miss where
// a 2-bead `codename:gama` corpus produced no advisor output because the
// stricter floor blocked the dominant proposal.
//
// A floor < 1 is clamped to 1 (zero matches can never be "dominant").
func ProposeFilterWithFloor(all []beads.Bead, codename string, floor int) Proposal {
	if floor < 1 {
		floor = 1
	}
	candidates := candidateShapes(codename)
	if len(candidates) == 0 {
		return Proposal{Reason: ReasonNoMatch}
	}

	// Build a label -> index map so we can update counts in one pass over
	// the bead labels rather than O(beads × candidates).
	labelIdx := make(map[string]int, len(candidates))
	for i, c := range candidates {
		labelIdx[c.Label] = i
	}

	// For each bead, increment the count of each candidate label it carries.
	// A bead with both "subsystem:foo" and "codename:foo" contributes to
	// both counts — this is intentional: we want to know how many beads
	// each convention reaches independently.
	for _, b := range all {
		// Dedup labels within a bead so duplicates do not inflate counts.
		seen := make(map[int]bool, len(b.Labels))
		for _, lbl := range b.Labels {
			if idx, ok := labelIdx[lbl]; ok && !seen[idx] {
				candidates[idx].Count++
				seen[idx] = true
			}
		}
	}

	// Sort candidates by descending Count, alphabetical tiebreak on Label.
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Count != candidates[j].Count {
			return candidates[i].Count > candidates[j].Count
		}
		return candidates[i].Label < candidates[j].Label
	})

	// Total matches across all candidates — denominator for dominance.
	total := 0
	for _, c := range candidates {
		total += c.Count
	}

	// Drop zero-count candidates from the diagnostic list so callers see
	// only what actually matched. Keep at least the top entry so the caller
	// can distinguish "nothing matched" from "matched but below floor" —
	// the Reason already encodes this, but the candidate list makes it
	// human-readable.
	nonzero := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Count > 0 {
			nonzero = append(nonzero, c)
		}
	}

	if total == 0 {
		return Proposal{Reason: ReasonNoMatch}
	}

	top := nonzero[0]

	// Dominance: top count is ≥ 80% of total AND clears the caller-supplied
	// absolute floor (ProposeFilter passes minAbsoluteFloor; the kerf-next
	// advisor passes a lower value — see ProposeFilterWithFloor).
	if top.Count >= floor && float64(top.Count) >= dominanceFraction*float64(total) {
		clause := top.Clause
		return Proposal{
			Filter:     &clause,
			Reason:     ReasonDominant,
			MatchCount: top.Count,
			Candidates: nonzero,
		}
	}

	// Union: gather candidates that clear the union member floor. Need at
	// least two members to form an `any:` clause — a single member that
	// missed the dominance floor would be ReasonBelowFloor instead.
	var members []beads.Filter
	unionCount := 0
	for _, c := range nonzero {
		if c.Count >= minUnionMemberFloor {
			members = append(members, c.Clause)
			unionCount += c.Count
		}
	}
	if len(members) >= 2 {
		f := beads.Filter{Any: members}
		return Proposal{
			Filter:     &f,
			Reason:     ReasonUnion,
			MatchCount: unionCount,
			Candidates: nonzero,
		}
	}

	// Something matched but no clause cleared the floors.
	return Proposal{
		Reason:     ReasonBelowFloor,
		Candidates: nonzero,
	}
}
