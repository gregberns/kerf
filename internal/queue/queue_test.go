package queue

import (
	"testing"
	"time"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/spec"
)

func strPtr(s string) *string { return &s }

func makeWork(codename string, status string, statusValues []string, created time.Time) *spec.SpecYAML {
	return &spec.SpecYAML{
		Codename:     codename,
		Status:       status,
		StatusValues: statusValues,
		Created:      created,
	}
}

var baseTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestEmptyInput(t *testing.T) {
	result := Compute(nil, nil)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d entries", len(result))
	}

	result = Compute([]*spec.SpecYAML{}, nil)
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %d entries", len(result))
	}
}

func TestSingleWork(t *testing.T) {
	w := makeWork("alpha", "research", []string{"problem-space", "research", "ready"}, baseTime)
	w.Title = strPtr("Alpha Work")
	w.Areas = []string{"cli"}

	result := Compute([]*spec.SpecYAML{w}, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Codename != "alpha" {
		t.Errorf("expected codename 'alpha', got %q", result[0].Codename)
	}
	if result[0].Title != "Alpha Work" {
		t.Errorf("expected title 'Alpha Work', got %q", result[0].Title)
	}
	if result[0].Status != "research" {
		t.Errorf("expected status 'research', got %q", result[0].Status)
	}
	if len(result[0].Areas) != 1 || result[0].Areas[0] != "cli" {
		t.Errorf("expected areas [cli], got %v", result[0].Areas)
	}
}

func TestTerminalStatusFiltered(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"at terminal (ready)", "ready"},
		{"past terminal (finalized)", "finalized"},
		{"past terminal (implementing)", "implementing"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := makeWork("done-work", tc.status,
				[]string{"problem-space", "research", "ready"}, baseTime)

			result := Compute([]*spec.SpecYAML{w}, nil)
			if len(result) != 0 {
				t.Errorf("expected work with status %q to be filtered out, got %d entries", tc.status, len(result))
			}
		})
	}
}

func TestShelvedFiltered(t *testing.T) {
	w := makeWork("shelved-work", "shelved",
		[]string{"problem-space", "research", "shelved", "ready"}, baseTime)

	result := Compute([]*spec.SpecYAML{w}, nil)
	if len(result) != 0 {
		t.Errorf("expected shelved work to be filtered out, got %d entries", len(result))
	}
}

func TestUnmetDependenciesFiltered(t *testing.T) {
	upstream := makeWork("upstream", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)
	downstream := makeWork("downstream", "problem-space",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(time.Hour))
	downstream.DependsOn = []spec.Dependency{
		{Codename: "upstream", Relationship: "must-complete-first"},
	}

	result := Compute([]*spec.SpecYAML{upstream, downstream}, nil)

	// downstream should be filtered out because upstream is not terminal.
	// upstream should remain.
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Codename != "upstream" {
		t.Errorf("expected 'upstream' to remain, got %q", result[0].Codename)
	}
}

func TestMetDependenciesNotFiltered(t *testing.T) {
	upstream := makeWork("upstream", "ready",
		[]string{"problem-space", "research", "ready"}, baseTime)
	downstream := makeWork("downstream", "problem-space",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(time.Hour))
	downstream.DependsOn = []spec.Dependency{
		{Codename: "upstream", Relationship: "must-complete-first"},
	}

	result := Compute([]*spec.SpecYAML{upstream, downstream}, nil)

	// upstream is terminal so filtered. downstream's dep is met so it stays.
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Codename != "downstream" {
		t.Errorf("expected 'downstream', got %q", result[0].Codename)
	}
}

func TestUnresolvableDependencyDoesNotBlock(t *testing.T) {
	w := makeWork("lonely", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)
	w.DependsOn = []spec.Dependency{
		{Codename: "nonexistent", Relationship: "must-complete-first"},
	}

	result := Compute([]*spec.SpecYAML{w}, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry (unresolvable dep should not block), got %d", len(result))
	}
}

func TestFanOutScoring(t *testing.T) {
	// "base" unblocks 3 works; "leaf" unblocks 0.
	base := makeWork("base", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)
	a := makeWork("a", "problem-space",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(time.Hour))
	a.DependsOn = []spec.Dependency{{Codename: "base", Relationship: "must-complete-first"}}
	b := makeWork("b", "problem-space",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(2*time.Hour))
	b.DependsOn = []spec.Dependency{{Codename: "base", Relationship: "must-complete-first"}}
	c := makeWork("c", "problem-space",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(3*time.Hour))
	c.DependsOn = []spec.Dependency{{Codename: "base", Relationship: "must-complete-first"}}
	leaf := makeWork("leaf", "research",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(4*time.Hour))

	// a, b, c are blocked (base not terminal), so only base and leaf are actionable.
	result := Compute([]*spec.SpecYAML{base, a, b, c, leaf}, nil)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].Codename != "base" {
		t.Errorf("expected 'base' to rank first (fan-out=3), got %q", result[0].Codename)
	}
	if result[1].Codename != "leaf" {
		t.Errorf("expected 'leaf' to rank second (fan-out=0), got %q", result[1].Codename)
	}
	if result[0].Score <= result[1].Score {
		t.Errorf("base score (%.2f) should exceed leaf score (%.2f)", result[0].Score, result[1].Score)
	}
}

func TestTransitiveFanOut(t *testing.T) {
	// Chain: root -> mid -> leaf. Root transitively unblocks 2 works.
	root := makeWork("root", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)
	mid := makeWork("mid", "problem-space",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(time.Hour))
	mid.DependsOn = []spec.Dependency{{Codename: "root", Relationship: "must-complete-first"}}
	leaf := makeWork("leaf", "problem-space",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(2*time.Hour))
	leaf.DependsOn = []spec.Dependency{{Codename: "mid", Relationship: "must-complete-first"}}

	// Only root is actionable (mid and leaf are blocked).
	result := Compute([]*spec.SpecYAML{root, mid, leaf}, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Codename != "root" {
		t.Errorf("expected 'root', got %q", result[0].Codename)
	}
	// Root should have fan-out of 2 (mid + leaf transitively).
	expectedFanOutScore := 2 * WeightFanOut
	if result[0].Score < expectedFanOutScore {
		t.Errorf("expected score >= %.1f (fan-out 2), got %.2f", expectedFanOutScore, result[0].Score)
	}
}

func TestMomentumScoring(t *testing.T) {
	// Two works with same creation time, no deps. Differ only in bead momentum.
	almostDone := makeWork("almost-done", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)
	justStarted := makeWork("just-started", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)

	beadsByWork := map[string]beads.EpicSummary{
		"almost-done": {Total: 10, Complete: 8},
		"just-started": {Total: 10, Complete: 1},
	}

	result := Compute([]*spec.SpecYAML{almostDone, justStarted}, beadsByWork)
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].Codename != "almost-done" {
		t.Errorf("expected 'almost-done' to rank first (8/10 > 1/10), got %q", result[0].Codename)
	}
	if result[0].Score <= result[1].Score {
		t.Errorf("almost-done score (%.2f) should exceed just-started score (%.2f)",
			result[0].Score, result[1].Score)
	}
}

func TestCreationOrderTiebreaker(t *testing.T) {
	// Two works with identical momentum and no deps. Older should win.
	older := makeWork("older", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)
	newer := makeWork("newer", "research",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(24*time.Hour))

	result := Compute([]*spec.SpecYAML{newer, older}, nil) // input order reversed
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].Codename != "older" {
		t.Errorf("expected 'older' to rank first as tiebreaker, got %q", result[0].Codename)
	}
	if result[0].Score <= result[1].Score {
		t.Errorf("older score (%.2f) should exceed newer score (%.2f)",
			result[0].Score, result[1].Score)
	}
}

func TestCombinedScoring(t *testing.T) {
	// Work A: fan-out 1, momentum 5/10, oldest
	// Work B: fan-out 0, momentum 9/10, newest
	// Work C: fan-out 0, momentum 0/10, middle age
	//
	// A gets: 10 (fan-out) + 2.5 (momentum) + 0.2 (creation) = 12.7
	// B gets: 0 (fan-out) + 4.5 (momentum) + 0.0 (creation) = 4.5
	// C gets: 0 (fan-out) + 0.0 (momentum) + 0.1 (creation) = 0.1

	a := makeWork("a", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)
	b := makeWork("b", "research",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(2*time.Hour))
	c := makeWork("c", "research",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(time.Hour))

	// d depends on a but is blocked (a not terminal).
	d := makeWork("d", "problem-space",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(3*time.Hour))
	d.DependsOn = []spec.Dependency{{Codename: "a", Relationship: "must-complete-first"}}

	beadsByWork := map[string]beads.EpicSummary{
		"a": {Total: 10, Complete: 5},
		"b": {Total: 10, Complete: 9},
		"c": {Total: 10, Complete: 0},
	}

	result := Compute([]*spec.SpecYAML{a, b, c, d}, beadsByWork)

	// d is blocked, so only a, b, c should appear.
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}

	// a should be first (fan-out dominates).
	if result[0].Codename != "a" {
		t.Errorf("expected 'a' first (fan-out dominates), got %q", result[0].Codename)
	}
	// b should be second (high momentum).
	if result[1].Codename != "b" {
		t.Errorf("expected 'b' second (momentum), got %q", result[1].Codename)
	}
	// c should be last (no momentum, newest).
	if result[2].Codename != "c" {
		t.Errorf("expected 'c' last, got %q", result[2].Codename)
	}
}

func TestReasonsPopulated(t *testing.T) {
	base := makeWork("base", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)
	dep := makeWork("dep", "problem-space",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(time.Hour))
	dep.DependsOn = []spec.Dependency{{Codename: "base", Relationship: "must-complete-first"}}

	beadsByWork := map[string]beads.EpicSummary{
		"base": {Total: 5, Complete: 3},
	}

	result := Compute([]*spec.SpecYAML{base, dep}, beadsByWork)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}

	reasons := result[0].Reasons
	if len(reasons) == 0 {
		t.Fatal("expected non-empty reasons")
	}

	// Should have fan-out reason and momentum reason.
	hasUnblocks := false
	hasCompletion := false
	for _, r := range reasons {
		if len(r) > 8 && r[:8] == "unblocks" {
			hasUnblocks = true
		}
		if len(r) > 10 && r[:10] == "completion" {
			hasCompletion = true
		}
	}
	if !hasUnblocks {
		t.Errorf("expected an 'unblocks' reason, got %v", reasons)
	}
	if !hasCompletion {
		t.Errorf("expected a 'completion' reason, got %v", reasons)
	}
}

func TestNonBlockingRelationshipIgnored(t *testing.T) {
	upstream := makeWork("upstream", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)
	downstream := makeWork("downstream", "problem-space",
		[]string{"problem-space", "research", "ready"}, baseTime.Add(time.Hour))
	downstream.DependsOn = []spec.Dependency{
		{Codename: "upstream", Relationship: "informs"},
	}

	result := Compute([]*spec.SpecYAML{upstream, downstream}, nil)
	// "informs" is not "must-complete-first", so downstream should not be filtered.
	if len(result) != 2 {
		t.Fatalf("expected 2 entries (informs doesn't block), got %d", len(result))
	}
}

func TestNoBeadsData(t *testing.T) {
	w := makeWork("no-beads", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)

	// nil beads map — should not panic, momentum is just zero.
	result := Compute([]*spec.SpecYAML{w}, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
}

func TestZeroTotalBeads(t *testing.T) {
	w := makeWork("zero-beads", "research",
		[]string{"problem-space", "research", "ready"}, baseTime)

	beadsByWork := map[string]beads.EpicSummary{
		"zero-beads": {Total: 0, Complete: 0},
	}

	result := Compute([]*spec.SpecYAML{w}, beadsByWork)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	// Should not panic on divide-by-zero.
}
