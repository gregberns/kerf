package labelsample

import (
	"testing"

	"github.com/gberns/kerf/internal/beads"
)

func mkBead(id string, labels ...string) beads.Bead {
	return beads.Bead{ID: id, Labels: labels}
}

// TestProposeFilter_DominantPrefix exercises tier-1 prefix dominance: a
// codename whose beads almost all carry "subsystem:<codename>" should yield a
// single-leaf clause proposal pointed at that label.
func TestProposeFilter_DominantPrefix(t *testing.T) {
	all := []beads.Bead{
		mkBead("1", "subsystem:bridge"),
		mkBead("2", "subsystem:bridge"),
		mkBead("3", "subsystem:bridge"),
		mkBead("4", "subsystem:bridge"),
		mkBead("5", "subsystem:bridge"),
		mkBead("6", "kind:bridge"), // one outlier, well below 20%
		mkBead("7", "unrelated"),
	}
	p := ProposeFilter(all, "bridge")
	if p.Reason != ReasonDominant {
		t.Fatalf("expected ReasonDominant, got %s (filter=%+v)", p.Reason, p.Filter)
	}
	if p.Filter == nil || p.Filter.Label != "subsystem:bridge" {
		t.Fatalf("expected leaf label 'subsystem:bridge', got %+v", p.Filter)
	}
	if p.MatchCount != 5 {
		t.Errorf("expected MatchCount=5, got %d", p.MatchCount)
	}
	if len(p.Filter.Any) != 0 {
		t.Errorf("expected no Any union on dominant leaf, got %+v", p.Filter.Any)
	}
}

// TestProposeFilter_UnionFallback exercises tier-2: when two conventions
// carry non-trivial counts and neither dominates, the sampler should propose
// an `any:` union of the qualifying clauses.
func TestProposeFilter_UnionFallback(t *testing.T) {
	all := []beads.Bead{
		// 3 beads tagged with the prefixed convention.
		mkBead("1", "codename:phase-3"),
		mkBead("2", "codename:phase-3"),
		mkBead("3", "codename:phase-3"),
		// 3 beads tagged with the bare convention.
		mkBead("4", "phase-3"),
		mkBead("5", "phase-3"),
		mkBead("6", "phase-3"),
	}
	p := ProposeFilter(all, "phase-3")
	if p.Reason != ReasonUnion {
		t.Fatalf("expected ReasonUnion, got %s (filter=%+v)", p.Reason, p.Filter)
	}
	if p.Filter == nil || len(p.Filter.Any) < 2 {
		t.Fatalf("expected any: union with ≥2 members, got %+v", p.Filter)
	}
	// Both conventions should appear among union members.
	gotLabels := map[string]bool{}
	for _, m := range p.Filter.Any {
		gotLabels[m.Label] = true
	}
	if !gotLabels["codename:phase-3"] || !gotLabels["phase-3"] {
		t.Errorf("union missing expected members; got %+v", p.Filter.Any)
	}
	if p.MatchCount != 6 {
		t.Errorf("expected MatchCount=6 (3+3), got %d", p.MatchCount)
	}
}

// TestProposeFilter_NoConfidentPrefix_ReturnsEmpty exercises the "nothing to
// suggest" path: a bead store with zero labels resembling the codename should
// return no proposal and the ReasonNoMatch verdict.
func TestProposeFilter_NoConfidentPrefix_ReturnsEmpty(t *testing.T) {
	all := []beads.Bead{
		mkBead("1", "subsystem:auth"),
		mkBead("2", "subsystem:db"),
		mkBead("3", "kind:bug"),
		mkBead("4", "area:storage"),
	}
	p := ProposeFilter(all, "scratch")
	if p.Filter != nil {
		t.Fatalf("expected nil Filter, got %+v", p.Filter)
	}
	if p.Reason != ReasonNoMatch {
		t.Errorf("expected ReasonNoMatch, got %s", p.Reason)
	}
	if p.MatchCount != 0 {
		t.Errorf("expected MatchCount=0, got %d", p.MatchCount)
	}
	if len(p.Candidates) != 0 {
		t.Errorf("expected empty Candidates list on no-match, got %+v", p.Candidates)
	}
}

// TestProposeFilterWithFloor_LowerFloorPromotesBelowFloorMatches exercises
// the kerf-fx5 path: when the caller supplies a lower floor (the kerf next
// near-match advisor passes 2), a 2-bead dominant signal that the default
// floor (3) rejects as ReasonBelowFloor is promoted to ReasonDominant. The
// default ProposeFilter keeps the strict floor — locked by TestProposeFilter_BelowFloor
// immediately below.
func TestProposeFilterWithFloor_LowerFloorPromotesBelowFloorMatches(t *testing.T) {
	all := []beads.Bead{
		mkBead("1", "codename:gama"),
		mkBead("2", "codename:gama"),
		mkBead("3", "subsystem:other"),
	}
	if p := ProposeFilter(all, "gama"); p.Reason != ReasonBelowFloor {
		t.Fatalf("default floor expected ReasonBelowFloor, got %s", p.Reason)
	}
	p := ProposeFilterWithFloor(all, "gama", 2)
	if p.Reason != ReasonDominant {
		t.Fatalf("floor=2 expected ReasonDominant, got %s", p.Reason)
	}
	if p.Filter == nil || p.Filter.Label != "codename:gama" {
		t.Fatalf("expected leaf 'codename:gama', got %+v", p.Filter)
	}
	if p.MatchCount != 2 {
		t.Errorf("expected MatchCount=2, got %d", p.MatchCount)
	}
}

// TestProposeFilterWithFloor_ClampsZeroFloor confirms a floor < 1 is
// clamped to 1 — zero matches can never be "dominant" so the function
// must not degrade into proposing on no signal.
func TestProposeFilterWithFloor_ClampsZeroFloor(t *testing.T) {
	all := []beads.Bead{mkBead("1", "unrelated")}
	p := ProposeFilterWithFloor(all, "gama", 0)
	if p.Filter != nil {
		t.Fatalf("floor=0 must clamp; expected nil filter, got %+v", p.Filter)
	}
	if p.Reason != ReasonNoMatch {
		t.Errorf("expected ReasonNoMatch on empty signal, got %s", p.Reason)
	}
}

// TestProposeFilter_BelowFloor exercises the "matched but too weak" path: a
// single label hit with only 2 beads is below the absolute floor of 3, and
// has no second candidate to form a union with — should return ReasonBelowFloor.
func TestProposeFilter_BelowFloor(t *testing.T) {
	all := []beads.Bead{
		mkBead("1", "subsystem:bridge"),
		mkBead("2", "subsystem:bridge"),
		// No other "bridge"-shaped labels at all.
		mkBead("3", "subsystem:auth"),
	}
	p := ProposeFilter(all, "bridge")
	if p.Filter != nil {
		t.Fatalf("expected nil Filter (below floor), got %+v", p.Filter)
	}
	if p.Reason != ReasonBelowFloor {
		t.Errorf("expected ReasonBelowFloor, got %s", p.Reason)
	}
	// Diagnostic candidates should surface the 2-bead hit so the caller
	// can render "matched 2 — below floor".
	if len(p.Candidates) == 0 {
		t.Errorf("expected non-empty Candidates list for below-floor diagnostic")
	}
}

// TestProposeFilter_PrefixedVsBareReconciliation exercises the harmonik
// drift case: one bead carries the prefixed form and the rest carry the
// bare form. The bare convention dominates, so the sampler should propose
// the bare-label clause — not the prefixed one and not a union (the prefix
// hit is only 1, below the union member floor).
func TestProposeFilter_PrefixedVsBareReconciliation(t *testing.T) {
	all := []beads.Bead{
		mkBead("1", "harmonik"),
		mkBead("2", "harmonik"),
		mkBead("3", "harmonik"),
		mkBead("4", "harmonik"),
		mkBead("5", "harmonik"),
		mkBead("6", "codename:harmonik"), // single prefixed outlier
	}
	p := ProposeFilter(all, "harmonik")
	if p.Reason != ReasonDominant {
		t.Fatalf("expected ReasonDominant (bare wins), got %s", p.Reason)
	}
	if p.Filter == nil || p.Filter.Label != "harmonik" {
		t.Fatalf("expected leaf label 'harmonik', got %+v", p.Filter)
	}
	if p.MatchCount != 5 {
		t.Errorf("expected MatchCount=5, got %d", p.MatchCount)
	}
}

// TestProposeFilter_EmptyStore confirms the no-bead corner: should return
// ReasonNoMatch with no proposal.
func TestProposeFilter_EmptyStore(t *testing.T) {
	p := ProposeFilter(nil, "anything")
	if p.Filter != nil || p.Reason != ReasonNoMatch {
		t.Errorf("expected nil filter + ReasonNoMatch on empty store, got %+v / %s", p.Filter, p.Reason)
	}
}

// TestProposeFilter_DedupesWithinBead confirms a bead carrying the same
// candidate label twice only counts once.
func TestProposeFilter_DedupesWithinBead(t *testing.T) {
	all := []beads.Bead{
		// Same label repeated on one bead.
		{ID: "1", Labels: []string{"subsystem:bridge", "subsystem:bridge"}},
		{ID: "2", Labels: []string{"subsystem:bridge"}},
		{ID: "3", Labels: []string{"subsystem:bridge"}},
	}
	p := ProposeFilter(all, "bridge")
	if p.Reason != ReasonDominant {
		t.Fatalf("expected ReasonDominant, got %s", p.Reason)
	}
	if p.MatchCount != 3 {
		t.Errorf("expected dedup MatchCount=3, got %d", p.MatchCount)
	}
}
