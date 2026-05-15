package metrics

import (
	"reflect"
	"testing"

	"github.com/gberns/kerf/internal/sim/event"
)

// newTestCollector configures a collector with a TicksCap large enough that
// warmup is skipped (wall_ticks < ticks_cap*0.2). Tests that exercise the
// warmup partition explicitly use a different config.
func newTestCollector() *Collector {
	return NewCollector(Config{
		NumAgents:  2,
		TicksCap:   1000,
		WorkTotal:  5,
		Deadline1d: 100,
		Deadline3d: 300,
		Deadline7d: 700,
	})
}

func TestIdleAccumulation_KnownSequence(t *testing.T) {
	c := newTestCollector()

	// 2 agents. Dispatch a0@0, a1@10, complete a0@20, complete a1@30.
	c.RecordDispatch(DispatchInfo{Tick: 0, AgentID: 0, BeadID: "b0", Area: "X"})
	c.RecordDispatch(DispatchInfo{Tick: 10, AgentID: 1, BeadID: "b1", Area: "Y"})
	c.RecordComplete(CompleteInfo{Tick: 20, AgentID: 0, BeadID: "b0", Area: "X"})
	c.RecordComplete(CompleteInfo{Tick: 30, AgentID: 1, BeadID: "b1", Area: "Y"})

	s := c.Result()
	if !s.WarmupSkipped {
		t.Fatalf("expected warmup_skipped (wall=30 < ticks_cap*0.2=200), got false")
	}
	if got, want := s.Full.WallTicks, int64(30); got != want {
		t.Errorf("wall_ticks: got %d want %d", got, want)
	}
	// Idle integration:
	//  (-1→0) initialise, no delta
	//  (0→10) busy=1, idle=1 → +10
	//  (10→20) busy=2, idle=0 → +0
	//  (20→30) busy=1, idle=1 → +10
	// Total idle = 20 over (wall * num_agents) = 60 → 0.3333…
	wantIdle := 20.0 / 60.0
	if got := s.Full.AgentIdlePct; got != wantIdle {
		t.Errorf("agent_idle_pct: got %v want %v", got, wantIdle)
	}
	// Busy integration: 0 + 10*1 + 10*2 + 10*1 = 40.
	if got := s.Full.AgentTicksTotal; got != 40 {
		t.Errorf("agent_ticks_total: got %d want 40", got)
	}
}

func TestTopOfQueueChurn_BaselineNotCounted(t *testing.T) {
	c := newTestCollector()
	// Drive lastTick so snapshots have a non-zero stamp; tick well past
	// warmup cutoff (warmup_skipped means churn is computed over full run
	// regardless, but for clarity we use a tick that's also post-warmup).
	c.Observe(event.Event{Tick: 50})
	c.RecordSnapshot("A")
	c.RecordSnapshot("A")
	c.RecordSnapshot("B")
	c.RecordSnapshot("B")
	c.RecordSnapshot("C")

	s := c.Result()
	// 5 snapshots, 2 changes (A→B, B→C). Baseline A doesn't count.
	want := 2.0 / 5.0
	if got := s.Full.TopOfQueueChurn; got != want {
		t.Errorf("churn full: got %v want %v", got, want)
	}
}

func TestWarmup_NormalRun(t *testing.T) {
	// Use a large ticks_cap so the cutoff is well-defined: cap=1000,
	// cut = min(100, 0.1*wall). With wall=900, cut = min(100, 90) = 90.
	c := NewCollector(Config{NumAgents: 1, TicksCap: 1000, WorkTotal: 1})

	// Dispatch at tick 50 (warmup), complete at 100 (warmup boundary, since
	// cut = min(100, 0.1*900) = 90, completion is post-warmup).
	// Then dispatch at 200, complete at 900.
	c.RecordDispatch(DispatchInfo{Tick: 50, AgentID: 0, BeadID: "b0"})
	c.RecordComplete(CompleteInfo{Tick: 100, AgentID: 0, BeadID: "b0", WorkCompleted: true})
	c.RecordDispatch(DispatchInfo{Tick: 200, AgentID: 0, BeadID: "b1"})
	c.RecordComplete(CompleteInfo{Tick: 900, AgentID: 0, BeadID: "b1", WorkCompleted: true})

	s := c.Result()
	if s.WarmupSkipped {
		t.Fatalf("did not expect warmup_skipped (wall=900 ≥ ticks_cap*0.2=200)")
	}
	if s.WarmupTicks != 90 {
		t.Errorf("WarmupTicks: got %d want 90", s.WarmupTicks)
	}
	if s.Full.WallTicks != 900 {
		t.Errorf("Full.WallTicks: got %d want 900", s.Full.WallTicks)
	}
	if s.Warmup.WallTicks != 810 {
		t.Errorf("Warmup.WallTicks: got %d want 810", s.Warmup.WallTicks)
	}
	// work_completed: full = 2, post-warmup = 2 (both completions at tick > 90).
	if s.Full.WorkCompleted != 2 || s.Warmup.WorkCompleted != 2 {
		t.Errorf("work_completed: full=%d warmup=%d want 2/2", s.Full.WorkCompleted, s.Warmup.WorkCompleted)
	}
}

func TestWarmup_ShortRun_FallThrough(t *testing.T) {
	// wall < ticks_cap * 0.2 → warmup_skipped = true; Warmup == Full.
	c := NewCollector(Config{NumAgents: 2, TicksCap: 100, WorkTotal: 1})
	c.RecordDispatch(DispatchInfo{Tick: 0, AgentID: 0, BeadID: "b0"})
	c.RecordComplete(CompleteInfo{Tick: 10, AgentID: 0, BeadID: "b0"})

	s := c.Result()
	if !s.WarmupSkipped {
		t.Fatalf("expected warmup_skipped, got false (wall=%d, cap=%d)", s.Full.WallTicks, 100)
	}
	if !reflect.DeepEqual(s.Full, s.Warmup) {
		t.Errorf("Warmup must mirror Full when skipped:\n full=%+v\n warm=%+v", s.Full, s.Warmup)
	}
	// agent_idle_pct denominator uses full-run wall (10) * num_agents (2) = 20.
	// Idle integration: (-1→0) init, (0→10) busy=1 idle=1 → +10. Idle=10/20=0.5.
	if got := s.Full.AgentIdlePct; got != 0.5 {
		t.Errorf("agent_idle_pct fall-through: got %v want 0.5", got)
	}
}

func TestPriorityInversion_Detection(t *testing.T) {
	c := newTestCollector()
	// New-work dispatch with HadOlderRework=true → inversion.
	c.RecordDispatch(DispatchInfo{
		Tick: 5, AgentID: 0, BeadID: "new1", IsNewWork: true, HadOlderRework: true,
	})
	// New-work dispatch without older rework → no inversion.
	c.RecordDispatch(DispatchInfo{
		Tick: 10, AgentID: 1, BeadID: "new2", IsNewWork: true, HadOlderRework: false,
	})
	// Rework dispatch (cannot be an inversion regardless of flag).
	c.RecordArrival(ArrivalInfo{Tick: 0, BeadID: "rw1", IsRework: true})
	c.RecordDispatch(DispatchInfo{
		Tick: 15, AgentID: 0, BeadID: "rw1", IsRework: true, IsNewWork: false, HadOlderRework: true,
	})

	s := c.Result()
	if s.Full.PriorityInversions != 1 {
		t.Errorf("priority_inversions: got %d want 1", s.Full.PriorityInversions)
	}
}

func TestAreaCollision_SeparateAndReoverlap(t *testing.T) {
	c := newTestCollector()
	// Agents 0 and 1 both work area "X": overlap, separate, re-overlap → 2 collisions.
	c.RecordDispatch(DispatchInfo{Tick: 0, AgentID: 0, BeadID: "b0", Area: "X"})
	c.RecordDispatch(DispatchInfo{Tick: 5, AgentID: 1, BeadID: "b1", Area: "X"}) // collision 1
	c.RecordComplete(CompleteInfo{Tick: 10, AgentID: 1, BeadID: "b1", Area: "X"})
	// Now only agent 0 is active in X.
	c.RecordComplete(CompleteInfo{Tick: 15, AgentID: 0, BeadID: "b0", Area: "X"})
	// Re-dispatch both.
	c.RecordDispatch(DispatchInfo{Tick: 20, AgentID: 0, BeadID: "b2", Area: "X"})
	c.RecordDispatch(DispatchInfo{Tick: 25, AgentID: 1, BeadID: "b3", Area: "X"}) // collision 2

	s := c.Result()
	if s.Full.AreaCollisions != 2 {
		t.Errorf("area_collisions: got %d want 2", s.Full.AreaCollisions)
	}
}

func TestGoalCompletion_AtDeadlines(t *testing.T) {
	c := NewCollector(Config{
		NumAgents:  1,
		TicksCap:   10000,
		WorkTotal:  4,
		Deadline1d: 100,
		Deadline3d: 300,
		Deadline7d: 700,
	})
	// Four work completions at varying ticks.
	c.RecordComplete(CompleteInfo{Tick: 50, AgentID: 0, BeadID: "b0", WorkID: "w0", WorkCompleted: true})
	c.RecordComplete(CompleteInfo{Tick: 100, AgentID: 0, BeadID: "b1", WorkID: "w1", WorkCompleted: true}) // boundary 1d
	c.RecordComplete(CompleteInfo{Tick: 250, AgentID: 0, BeadID: "b2", WorkID: "w2", WorkCompleted: true})
	c.RecordComplete(CompleteInfo{Tick: 800, AgentID: 0, BeadID: "b3", WorkID: "w3", WorkCompleted: true}) // past 7d
	c.Observe(event.Event{Tick: 2100}) // extend wall_ticks past warmup-skip threshold (cap*0.2 = 2000)

	s := c.Result()
	if s.Full.GoalCompletion1d != 2 {
		t.Errorf("goal_1d: got %d want 2", s.Full.GoalCompletion1d)
	}
	if s.Full.GoalCompletion3d != 3 {
		t.Errorf("goal_3d: got %d want 3", s.Full.GoalCompletion3d)
	}
	if s.Full.GoalCompletion7d != 3 {
		t.Errorf("goal_7d: got %d want 3", s.Full.GoalCompletion7d)
	}
}

func TestReworkWaitPercentiles(t *testing.T) {
	c := NewCollector(Config{NumAgents: 1, TicksCap: 100, WorkTotal: 1})
	// 4 rework waits: arrivals→dispatches with gaps 10, 20, 30, 40.
	for i, gap := range []int64{10, 20, 30, 40} {
		id := []string{"r0", "r1", "r2", "r3"}[i]
		c.RecordArrival(ArrivalInfo{Tick: 0, BeadID: id, IsRework: true})
		c.RecordDispatch(DispatchInfo{Tick: gap, AgentID: 0, BeadID: id, IsRework: true})
		c.RecordComplete(CompleteInfo{Tick: gap + 1, AgentID: 0, BeadID: id})
	}
	s := c.Result()
	// Sorted waits: [10, 20, 30, 40].
	// p50 nearest-rank: ceil(0.5*4)-1 = 1 → 20.
	// p95: ceil(0.95*4)-1 = 3 → 40.
	if s.Full.ReworkP50Wait != 20 {
		t.Errorf("p50: got %d want 20", s.Full.ReworkP50Wait)
	}
	if s.Full.ReworkP95Wait != 40 {
		t.Errorf("p95: got %d want 40", s.Full.ReworkP95Wait)
	}
}

func TestDeterminism_SameStreamSameMetrics(t *testing.T) {
	build := func() Summary {
		c := newTestCollector()
		c.RecordArrival(ArrivalInfo{Tick: 0, BeadID: "rw", IsRework: true})
		c.RecordDispatch(DispatchInfo{Tick: 5, AgentID: 0, BeadID: "b0", Area: "X", IsNewWork: true, HadOlderRework: true})
		c.RecordSnapshot("b0")
		c.RecordDispatch(DispatchInfo{Tick: 10, AgentID: 1, BeadID: "b1", Area: "X"})
		c.RecordSnapshot("b1")
		c.RecordComplete(CompleteInfo{Tick: 20, AgentID: 0, BeadID: "b0", Area: "X", WorkCompleted: true})
		c.RecordComplete(CompleteInfo{Tick: 30, AgentID: 1, BeadID: "b1", Area: "X", WorkCompleted: true})
		return c.Result()
	}
	a := build()
	b := build()
	if !reflect.DeepEqual(a, b) {
		t.Errorf("not deterministic:\n a=%+v\n b=%+v", a, b)
	}
}

func TestSnapshot_FirstMutatingEvent_NotCounted(t *testing.T) {
	// Explicit test for the pinned rule: the first mutating event (first
	// snapshot) sets the baseline; numerator counts changes from the
	// second mutating event onward.
	c := NewCollector(Config{NumAgents: 1, TicksCap: 100, WorkTotal: 1})
	c.Observe(event.Event{Tick: 50}) // pin tick into the post-warmup region
	c.RecordSnapshot("A")            // baseline — not counted
	c.RecordSnapshot("B")            // change — counted
	s := c.Result()
	if s.WarmupSkipped {
		t.Fatalf("did not expect warmup_skipped: wall=%d cap=100", s.Full.WallTicks)
	}
	// 2 snapshots seen, 1 change → 0.5.
	if got, want := s.Full.TopOfQueueChurn, 0.5; got != want {
		t.Errorf("churn: got %v want %v", got, want)
	}
}
