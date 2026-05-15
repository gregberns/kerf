package store

import (
	"testing"
	"time"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/queue"
	"github.com/gberns/kerf/internal/spec"
)

// makeWork is a tiny constructor for a *spec.SpecYAML used as test fixture.
func makeWork(codename string, created time.Time) *spec.SpecYAML {
	return &spec.SpecYAML{
		Codename: codename,
		Type:     "feature",
		Status:   "in-progress",
		StatusValues: []string{
			"design", "in-progress", "review", "complete",
		},
		Created: created,
		Updated: created,
		Areas:   []string{"area-" + codename},
	}
}

// makeBead builds a beads.Bead tagged for a particular work codename.
func makeBead(id, workCode string, labels ...string) beads.Bead {
	all := append([]string{"work:" + workCode}, labels...)
	return beads.Bead{
		ID:     id,
		Title:  id,
		Status: "open",
		Epic:   workCode,
		Labels: all,
	}
}

// newWorld is a small GeneratedWorld used across tests.
func newWorld() *GeneratedWorld {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &GeneratedWorld{
		Works: []*spec.SpecYAML{
			makeWork("alpha", t0),
			makeWork("beta", t0.Add(time.Hour)),
		},
		InitialBeads: []beads.Bead{
			makeBead("a1", "alpha"),
			makeBead("a2", "alpha"),
			makeBead("b1", "beta", "finding:alpha"), // rework bead
		},
	}
}

func TestFrom_LoadsWorldShape(t *testing.T) {
	s := From(newWorld())
	if got, want := len(s.Works()), 2; got != want {
		t.Fatalf("Works() len = %d, want %d", got, want)
	}
	if s.Works()[0].Codename != "alpha" || s.Works()[1].Codename != "beta" {
		t.Fatalf("Works() canonical order broken: %v", []string{s.Works()[0].Codename, s.Works()[1].Codename})
	}
	if got, want := len(s.Beads()), 3; got != want {
		t.Fatalf("Beads() len = %d, want %d", got, want)
	}
	sum := s.SummaryByWork()
	if sum["alpha"].Total != 2 {
		t.Errorf("alpha.Total = %d, want 2", sum["alpha"].Total)
	}
	if sum["beta"].Total != 1 {
		t.Errorf("beta.Total = %d, want 1", sum["beta"].Total)
	}
	if sum["beta"].Rework != 1 {
		t.Errorf("beta.Rework = %d, want 1 (finding: label)", sum["beta"].Rework)
	}
}

func TestFrom_NilWorld(t *testing.T) {
	s := From(nil)
	if s == nil {
		t.Fatal("From(nil) returned nil store")
	}
	if len(s.Works()) != 0 || len(s.Beads()) != 0 {
		t.Errorf("From(nil) returned non-empty store")
	}
}

func TestFrom_Isolation(t *testing.T) {
	world := newWorld()
	a := From(world)
	b := From(world)

	a.Dispatch("a1", 7, 10)
	a.Complete("a1", 25)

	// b must be unaffected.
	if got := b.Lookup("a1").Status; got != StatusOpen {
		t.Errorf("isolation broken: b.a1.Status = %q, want %q", got, StatusOpen)
	}
	if got := a.Lookup("a1").Status; got != StatusClosed {
		t.Errorf("a.a1.Status = %q, want %q", got, StatusClosed)
	}

	// Arrival on one store must not leak to the other.
	a.Arrive(makeBead("a3", "alpha"), 30)
	if b.Lookup("a3") != nil {
		t.Errorf("isolation broken: a3 leaked into b")
	}
	if a.Lookup("a3") == nil {
		t.Errorf("a3 not present in a after Arrive")
	}
}

func TestAdapter_FeedsQueueCompute(t *testing.T) {
	s := From(newWorld())

	// The whole point of the adapter is that the outputs feed queue.Compute
	// directly with no transformation.
	entries := queue.Compute(s.Works(), s.SummaryByWork(), queue.DefaultWeights())

	if len(entries) == 0 {
		t.Fatal("queue.Compute returned no entries for non-empty store")
	}
	// Both works are actionable; both must appear.
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.Codename] = true
	}
	for _, code := range []string{"alpha", "beta"} {
		if !seen[code] {
			t.Errorf("queue.Compute missing entry for %q", code)
		}
	}
}

func TestDispatchComplete_Transitions(t *testing.T) {
	s := From(newWorld())

	s.Dispatch("a1", 1, 5)
	if got := s.Lookup("a1").Status; got != StatusInProgress {
		t.Errorf("after Dispatch: status = %q, want %q", got, StatusInProgress)
	}
	if got := s.Lookup("a1").DispatchAt; got != 5 {
		t.Errorf("DispatchAt = %d, want 5", got)
	}
	if got := s.Lookup("a1").AgentID; got != 1 {
		t.Errorf("AgentID = %d, want 1", got)
	}
	if got := s.SummaryByWork()["alpha"].InProgress; got != 1 {
		t.Errorf("alpha.InProgress = %d, want 1", got)
	}

	s.Complete("a1", 12)
	if got := s.Lookup("a1").Status; got != StatusClosed {
		t.Errorf("after Complete: status = %q, want %q", got, StatusClosed)
	}
	if got := s.Lookup("a1").CompleteAt; got != 12 {
		t.Errorf("CompleteAt = %d, want 12", got)
	}
	sum := s.SummaryByWork()["alpha"]
	if sum.InProgress != 0 {
		t.Errorf("after Complete: alpha.InProgress = %d, want 0", sum.InProgress)
	}
	if sum.Complete != 1 {
		t.Errorf("after Complete: alpha.Complete = %d, want 1", sum.Complete)
	}
}

func TestDispatchComplete_UnknownBead(t *testing.T) {
	s := From(newWorld())
	// Must not panic on unknown bead.
	s.Dispatch("does-not-exist", 1, 5)
	s.Complete("does-not-exist", 6)
}

func TestArrive_AddsAndIsVisibleInSummary(t *testing.T) {
	s := From(newWorld())
	before := s.SummaryByWork()["alpha"].Total

	s.Arrive(makeBead("a3", "alpha"), 42)

	state := s.Lookup("a3")
	if state == nil {
		t.Fatal("a3 not found after Arrive")
	}
	if state.ArrivedAt != 42 {
		t.Errorf("ArrivedAt = %d, want 42", state.ArrivedAt)
	}
	if state.Status != StatusOpen {
		t.Errorf("new bead status = %q, want %q", state.Status, StatusOpen)
	}
	if state.WorkCode != "alpha" {
		t.Errorf("WorkCode = %q, want alpha", state.WorkCode)
	}

	after := s.SummaryByWork()["alpha"].Total
	if after != before+1 {
		t.Errorf("alpha.Total after Arrive = %d, want %d", after, before+1)
	}
}

func TestArrive_Idempotent(t *testing.T) {
	s := From(newWorld())
	s.Arrive(makeBead("a3", "alpha"), 10)
	s.Arrive(makeBead("a3", "alpha"), 20) // duplicate — should be a no-op

	if got := s.Lookup("a3").ArrivedAt; got != 10 {
		t.Errorf("ArrivedAt after duplicate Arrive = %d, want 10 (original)", got)
	}
	if got := s.SummaryByWork()["alpha"].Total; got != 3 {
		t.Errorf("alpha.Total = %d, want 3 (no double-count)", got)
	}
}

func TestArrive_ReworkVisibleInSummary(t *testing.T) {
	s := From(newWorld())
	s.Arrive(makeBead("rwk", "alpha", "rework:true"), 5)

	sum := s.SummaryByWork()["alpha"]
	if sum.Rework < 1 {
		t.Errorf("alpha.Rework = %d, want >= 1", sum.Rework)
	}
}

func TestWorks_ReturnsCopy(t *testing.T) {
	s := From(newWorld())
	w := s.Works()
	w[0] = nil // mutate caller's copy
	if s.Works()[0] == nil {
		t.Errorf("Works() returned slice header is not copied — caller mutation leaked")
	}
}
