package metrics_test

import (
	"testing"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/sim/event"
	"github.com/gberns/kerf/internal/sim/loop"
	"github.com/gberns/kerf/internal/sim/metrics"
	"github.com/gberns/kerf/internal/sim/seed"
	"github.com/gberns/kerf/internal/sim/store"
	"github.com/gberns/kerf/internal/spec"
)

// stubPolicy returns beads in a pre-programmed order, skipping any that
// are not currently open. Mirrors the helper in loop_test.go.
type stubPolicy struct {
	order []string
}

func (p *stubPolicy) Name() string { return "stub" }
func (p *stubPolicy) Next(s *store.Store) string {
	for _, id := range p.order {
		b := s.Lookup(id)
		if b == nil {
			continue
		}
		if b.Status == store.StatusOpen {
			return id
		}
	}
	return ""
}

// buildStore creates a store with one work and two open beads carrying
// the given per-bead durations.
func buildStore(t *testing.T, durations map[string]int64) *store.Store {
	t.Helper()
	s := store.New()
	s.AddWork(&spec.SpecYAML{Codename: "wA", Areas: []string{"area-a"}})
	s.Arrive(beads.Bead{ID: "a", Title: "a", Labels: []string{"work:wA"}}, 0)
	s.Arrive(beads.Bead{ID: "b", Title: "b", Labels: []string{"work:wA"}}, 0)
	for id, d := range durations {
		s.SetDuration(id, d)
	}
	return s
}

// TestLoopHooks_SmokeRun drives the loop with the metrics adapter and
// verifies the resulting Summary reflects the run.
func TestLoopHooks_SmokeRun(t *testing.T) {
	s := buildStore(t, map[string]int64{"a": 10, "b": 5})
	c := metrics.NewCollector(metrics.Config{
		NumAgents:  1,
		TicksCap:   1000,
		WorkTotal:  1,
		Deadline1d: 100,
		Deadline3d: 300,
		Deadline7d: 700,
	})
	h := metrics.NewLoopHooks(c, s)

	l := &loop.Loop{
		Store:     s,
		Policy:    &stubPolicy{order: []string{"a", "b"}},
		Hooks:     h,
		Heap:      event.NewHeap(),
		NumAgents: 1,
		TicksCap:  1000,
		Seeds:     seed.From(0),
	}
	reason, wall, err := l.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if reason != loop.StopAllClosed {
		t.Fatalf("stop reason: got %q want %q", reason, loop.StopAllClosed)
	}
	if wall != 15 {
		t.Fatalf("wall ticks: got %d want 15", wall)
	}

	sum := c.Result()
	// Both beads belong to one work, so the work completes when bead b
	// closes at tick 15.
	if sum.Full.WorkCompleted != 1 {
		t.Errorf("Full.WorkCompleted = %d, want 1", sum.Full.WorkCompleted)
	}
	if sum.Full.WorkTotal != 1 {
		t.Errorf("Full.WorkTotal = %d, want 1", sum.Full.WorkTotal)
	}
	if sum.Full.WallTicks != 15 {
		t.Errorf("Full.WallTicks = %d, want 15", sum.Full.WallTicks)
	}
	// One agent, no idle gap (each completion immediately re-dispatches).
	if sum.Full.AgentIdlePct != 0 {
		t.Errorf("Full.AgentIdlePct = %v, want 0", sum.Full.AgentIdlePct)
	}
	// Work completed before any deadline.
	if sum.Full.GoalCompletion1d != 1 {
		t.Errorf("GoalCompletion1d = %d, want 1", sum.Full.GoalCompletion1d)
	}
}

// TestLoopHooks_Determinism runs the same scenario twice and asserts the
// resulting Summary is identical. The adapter is stateless beyond its
// references to the collector and store, so determinism reduces to the
// loop's own determinism guarantee.
func TestLoopHooks_Determinism(t *testing.T) {
	run := func() metrics.Summary {
		s := buildStore(t, map[string]int64{"a": 7, "b": 3})
		c := metrics.NewCollector(metrics.Config{
			NumAgents:  1,
			TicksCap:   1000,
			WorkTotal:  1,
			Deadline1d: 100,
			Deadline3d: 300,
			Deadline7d: 700,
		})
		h := metrics.NewLoopHooks(c, s)
		l := &loop.Loop{
			Store:     s,
			Policy:    &stubPolicy{order: []string{"a", "b"}},
			Hooks:     h,
			Heap:      event.NewHeap(),
			NumAgents: 1,
			TicksCap:  1000,
			Seeds:     seed.From(0),
		}
		if _, _, err := l.Run(); err != nil {
			t.Fatalf("Run: %v", err)
		}
		return c.Result()
	}
	a := run()
	b := run()
	if a != b {
		t.Fatalf("non-deterministic Summary:\nfirst:  %+v\nsecond: %+v", a, b)
	}
}

// TestLoopHooks_AreaCollisionAndPriorityInversion exercises the lookups
// the adapter performs at hook-call time:
//   - HadOlderRework: when an older rework bead is eligible at the time
//     a new-work bead is dispatched.
//   - Area: read from the bead's work; two agents dispatching beads on
//     the same area concurrently registers as a collision.
func TestLoopHooks_AreaCollisionAndPriorityInversion(t *testing.T) {
	s := store.New()
	s.AddWork(&spec.SpecYAML{Codename: "wA", Areas: []string{"area-a"}})
	// Rework bead, arrived at tick 0.
	s.Arrive(beads.Bead{
		ID:     "r0",
		Title:  "rework one",
		Labels: []string{"work:wA", "rework:true"},
	}, 0)
	// New-work bead, arrived later (tick 1) — r0 is strictly older.
	s.Arrive(beads.Bead{
		ID:     "n1",
		Title:  "new work",
		Labels: []string{"work:wA"},
	}, 1)
	s.SetDuration("r0", 20)
	s.SetDuration("n1", 5)

	c := metrics.NewCollector(metrics.Config{
		NumAgents:  2,
		TicksCap:   1000,
		WorkTotal:  1,
		Deadline1d: 1000,
		Deadline3d: 1000,
		Deadline7d: 1000,
	})
	h := metrics.NewLoopHooks(c, s)

	// Policy dispatches n1 first (new-work) — which should trigger a
	// priority inversion because r0 (older rework) is still eligible —
	// then r0 next. Two agents pick up beads on tick 0 → same area →
	// one area collision.
	l := &loop.Loop{
		Store:     s,
		Policy:    &stubPolicy{order: []string{"n1", "r0"}},
		Hooks:     h,
		Heap:      event.NewHeap(),
		NumAgents: 2,
		TicksCap:  1000,
		Seeds:     seed.From(0),
	}
	if _, _, err := l.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	sum := c.Result()
	if sum.Full.PriorityInversions != 1 {
		t.Errorf("PriorityInversions = %d, want 1", sum.Full.PriorityInversions)
	}
	if sum.Full.AreaCollisions != 1 {
		t.Errorf("AreaCollisions = %d, want 1", sum.Full.AreaCollisions)
	}
	if sum.Full.WorkCompleted != 1 {
		t.Errorf("WorkCompleted = %d, want 1", sum.Full.WorkCompleted)
	}
}

// TestLoopHooks_NoPriorityInversionWithoutOlderRework verifies the
// older-rework eligibility check ignores beads with unmet dependencies.
func TestLoopHooks_NoPriorityInversionWithoutOlderRework(t *testing.T) {
	s := store.New()
	s.AddWork(&spec.SpecYAML{Codename: "wA", Areas: []string{"area-a"}})
	// Rework bead with an unmet dep — should NOT count as eligible.
	s.Arrive(beads.Bead{
		ID:        "r0",
		Title:     "blocked rework",
		Labels:    []string{"work:wA", "rework:true"},
		DependsOn: []string{"missing-dep"},
	}, 0)
	s.Arrive(beads.Bead{
		ID:     "n1",
		Title:  "new work",
		Labels: []string{"work:wA"},
	}, 0)
	s.SetDuration("r0", 5)
	s.SetDuration("n1", 5)

	c := metrics.NewCollector(metrics.Config{
		NumAgents: 1,
		TicksCap:  1000,
		WorkTotal: 1,
	})
	h := metrics.NewLoopHooks(c, s)

	l := &loop.Loop{
		Store:     s,
		Policy:    &stubPolicy{order: []string{"n1"}},
		Hooks:     h,
		Heap:      event.NewHeap(),
		NumAgents: 1,
		TicksCap:  1000,
		Seeds:     seed.From(0),
	}
	if _, _, err := l.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	sum := c.Result()
	if sum.Full.PriorityInversions != 0 {
		t.Errorf("PriorityInversions = %d, want 0 (blocked rework is not eligible)", sum.Full.PriorityInversions)
	}
}
