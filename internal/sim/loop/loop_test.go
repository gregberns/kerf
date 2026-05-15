package loop_test

import (
	"reflect"
	"testing"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/sim/event"
	"github.com/gberns/kerf/internal/sim/loop"
	"github.com/gberns/kerf/internal/sim/seed"
	"github.com/gberns/kerf/internal/sim/store"
)

// stubPolicy returns beads in a pre-programmed order, skipping any whose ID
// has already been returned. Returns "" once the list is exhausted or when
// no remaining bead is open (still in-store and not yet in-progress or
// closed). This is the deterministic policy used throughout the tests.
type stubPolicy struct {
	name  string
	order []string
}

func (p *stubPolicy) Name() string { return p.name }

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

// recordingHooks captures every callback in the order it fires. Used both
// for determinism tests (re-run, compare trace) and ordering tests.
type recordingHooks struct {
	calls []string
}

func (h *recordingHooks) OnEvent(e event.Event) {
	h.calls = append(h.calls,
		"event:"+e.Kind.String()+":"+itoa(int(e.Tick))+":"+itoa(e.AgentID)+":"+e.BeadID)
}
func (h *recordingHooks) OnDispatch(agentID int, beadID string, tick int64) {
	h.calls = append(h.calls,
		"dispatch:a="+itoa(agentID)+":b="+beadID+":t="+itoa(int(tick)))
}
func (h *recordingHooks) OnComplete(beadID string, tick int64) {
	h.calls = append(h.calls, "complete:b="+beadID+":t="+itoa(int(tick)))
}
func (h *recordingHooks) OnArrival(beadID string, work string, tick int64) {
	h.calls = append(h.calls,
		"arrival:b="+beadID+":w="+work+":t="+itoa(int(tick)))
}
func (h *recordingHooks) OnSnapshot(top string) {
	h.calls = append(h.calls, "snapshot:top="+top)
}

// itoa is a tiny non-fmt int-to-string used by the recorder. Pulled out so
// the recorder does not pull in fmt.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// buildStore constructs a store with two beads, both open, with the given
// per-bead durations.
func buildStore(durations map[string]int64) *store.Store {
	s := store.New()
	s.Arrive(beads.Bead{ID: "a", Title: "a", Labels: []string{"work:wA"}}, 0)
	s.Arrive(beads.Bead{ID: "b", Title: "b", Labels: []string{"work:wA"}}, 0)
	for id, d := range durations {
		s.SetDuration(id, d)
	}
	return s
}

// TestRun_AllClosedStopReason runs until every bead is closed. A single
// agent, two beads, both completable.
func TestRun_AllClosedStopReason(t *testing.T) {
	s := buildStore(map[string]int64{"a": 10, "b": 5})
	hooks := &recordingHooks{}
	l := &loop.Loop{
		Store:     s,
		Policy:    &stubPolicy{name: "stub", order: []string{"a", "b"}},
		Hooks:     hooks,
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
	// Last completion fires at tick 10 + 5 = 15.
	if wall != 15 {
		t.Fatalf("wall ticks: got %d want 15", wall)
	}
}

// TestRun_TicksCapStopReason runs until the tick cap is reached before any
// bead can finish.
func TestRun_TicksCapStopReason(t *testing.T) {
	s := buildStore(map[string]int64{"a": 1000, "b": 1000})
	l := &loop.Loop{
		Store:     s,
		Policy:    &stubPolicy{name: "stub", order: []string{"a", "b"}},
		Hooks:     loop.NoOpHooks{},
		Heap:      event.NewHeap(),
		NumAgents: 1,
		TicksCap:  50,
		Seeds:     seed.From(0),
	}
	reason, _, err := l.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if reason != loop.StopTicksCap {
		t.Fatalf("stop reason: got %q want %q", reason, loop.StopTicksCap)
	}
}

// TestRun_IdleThresholdStopReason runs on an empty store: no beads, so the
// agent immediately idles and the heap drains.
func TestRun_IdleThresholdStopReason(t *testing.T) {
	s := store.New()
	l := &loop.Loop{
		Store:     s,
		Policy:    &stubPolicy{name: "stub"},
		Hooks:     loop.NoOpHooks{},
		Heap:      event.NewHeap(),
		NumAgents: 1,
		TicksCap:  100,
		Seeds:     seed.From(0),
	}
	reason, _, err := l.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if reason != loop.StopIdleThreshold {
		t.Fatalf("stop reason: got %q want %q", reason, loop.StopIdleThreshold)
	}
}

// TestRun_Determinism runs the same configuration twice and asserts the
// recorded hook trace is byte-identical.
func TestRun_Determinism(t *testing.T) {
	build := func() (*loop.Loop, *recordingHooks) {
		s := buildStore(map[string]int64{"a": 7, "b": 3})
		hooks := &recordingHooks{}
		return &loop.Loop{
			Store:     s,
			Policy:    &stubPolicy{name: "stub", order: []string{"a", "b"}},
			Hooks:     hooks,
			Heap:      event.NewHeap(),
			NumAgents: 2,
			TicksCap:  1000,
			Seeds:     seed.From(42),
		}, hooks
	}
	l1, h1 := build()
	l2, h2 := build()
	if _, _, err := l1.Run(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := l2.Run(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h1.calls, h2.calls) {
		t.Fatalf("non-deterministic trace:\n#1: %v\n#2: %v", h1.calls, h2.calls)
	}
	if len(h1.calls) == 0 {
		t.Fatal("expected non-empty trace")
	}
}

// TestRun_DispatchCompletionOrdering asserts the completion event for a
// dispatched bead lands at exactly dispatch_tick + duration.
func TestRun_DispatchCompletionOrdering(t *testing.T) {
	s := buildStore(map[string]int64{"a": 12, "b": 4})
	hooks := &recordingHooks{}
	l := &loop.Loop{
		Store:     s,
		Policy:    &stubPolicy{name: "stub", order: []string{"a", "b"}},
		Hooks:     hooks,
		Heap:      event.NewHeap(),
		NumAgents: 1,
		TicksCap:  1000,
		Seeds:     seed.From(0),
	}
	if _, _, err := l.Run(); err != nil {
		t.Fatal(err)
	}
	aState := s.Lookup("a")
	if aState.DispatchAt != 0 || aState.CompleteAt != 12 {
		t.Fatalf("bead a: dispatch=%d complete=%d want 0/12",
			aState.DispatchAt, aState.CompleteAt)
	}
	bState := s.Lookup("b")
	if bState.DispatchAt != 12 || bState.CompleteAt != 16 {
		t.Fatalf("bead b: dispatch=%d complete=%d want 12/16",
			bState.DispatchAt, bState.CompleteAt)
	}
}

// TestRun_MutualExclusion verifies that between two dispatches on the same
// agent, exactly one completion fires for that agent.
func TestRun_MutualExclusion(t *testing.T) {
	s := buildStore(map[string]int64{"a": 5, "b": 8})
	hooks := &recordingHooks{}
	l := &loop.Loop{
		Store:     s,
		Policy:    &stubPolicy{name: "stub", order: []string{"a", "b"}},
		Hooks:     hooks,
		Heap:      event.NewHeap(),
		NumAgents: 1,
		TicksCap:  1000,
		Seeds:     seed.From(0),
	}
	if _, _, err := l.Run(); err != nil {
		t.Fatal(err)
	}

	// Extract the sequence of dispatch/complete entries in order. With a
	// single agent the expected pattern is exactly: dispatch, complete,
	// dispatch, complete.
	var seq []string
	for _, c := range hooks.calls {
		if len(c) >= 9 && c[:9] == "dispatch:" {
			seq = append(seq, "D")
		} else if len(c) >= 9 && c[:9] == "complete:" {
			seq = append(seq, "C")
		}
	}
	want := []string{"D", "C", "D", "C"}
	if !reflect.DeepEqual(seq, want) {
		t.Fatalf("dispatch/complete sequence: got %v want %v", seq, want)
	}
}

// TestRun_SnapshotFiresOnEveryMutation asserts that every dispatch,
// arrival, and completion is followed (in the same loop iteration) by an
// OnSnapshot callback.
func TestRun_SnapshotFiresOnEveryMutation(t *testing.T) {
	s := store.New()
	s.Arrive(beads.Bead{ID: "a", Labels: []string{"work:wA"}}, 0)
	s.SetDuration("a", 5)
	hooks := &recordingHooks{}
	heap := event.NewHeap()
	// Schedule an arrival that fires before bead "a" completes, so the
	// all-closed stop condition does not race the arrival.
	heap.Push(event.Event{
		Tick:    3,
		Kind:    event.KindArrival,
		BeadID:  "b",
		Payload: beads.Bead{ID: "b", Labels: []string{"work:wA"}},
	})
	s.SetDuration("b", 1)
	l := &loop.Loop{
		Store:     s,
		Policy:    &stubPolicy{name: "stub", order: []string{"a", "b"}},
		Hooks:     hooks,
		Heap:      heap,
		NumAgents: 1,
		TicksCap:  1000,
		Seeds:     seed.From(0),
	}
	if _, _, err := l.Run(); err != nil {
		t.Fatal(err)
	}

	// Walk the trace; every dispatch/arrival/complete call must be
	// followed (skipping other event/snapshot entries) by a snapshot
	// before the next mutating call. Stricter form: each mutating
	// callback is immediately followed by an OnSnapshot entry.
	dispatches, arrivals, completes, snapshots := 0, 0, 0, 0
	for i, c := range hooks.calls {
		switch {
		case len(c) >= 9 && c[:9] == "dispatch:":
			dispatches++
			assertNextIsSnapshot(t, hooks.calls, i, "dispatch")
		case len(c) >= 8 && c[:8] == "arrival:":
			arrivals++
			assertNextIsSnapshot(t, hooks.calls, i, "arrival")
		case len(c) >= 9 && c[:9] == "complete:":
			completes++
			assertNextIsSnapshot(t, hooks.calls, i, "complete")
		case len(c) >= 9 && c[:9] == "snapshot:":
			snapshots++
		}
	}
	if dispatches < 2 || arrivals < 1 || completes < 2 {
		t.Fatalf("expected >=2 dispatches, >=1 arrival, >=2 completes; got %d/%d/%d",
			dispatches, arrivals, completes)
	}
	if snapshots != dispatches+arrivals+completes {
		t.Fatalf("snapshot count: got %d want %d",
			snapshots, dispatches+arrivals+completes)
	}
}

// assertNextIsSnapshot checks that the entry immediately after index i is
// an OnSnapshot record. Fails the test with context about which mutating
// call lacked the follow-up.
func assertNextIsSnapshot(t *testing.T, calls []string, i int, kind string) {
	t.Helper()
	if i+1 >= len(calls) {
		t.Fatalf("%s at trace[%d] has no following call", kind, i)
	}
	n := calls[i+1]
	if !(len(n) >= 9 && n[:9] == "snapshot:") {
		t.Fatalf("%s at trace[%d] not followed by snapshot: next=%q",
			kind, i, n)
	}
}

// TestRun_NilHooksReplaceable confirms NoOpHooks works as the zero-impact
// observer without panicking.
func TestRun_NoOpHooks(t *testing.T) {
	s := buildStore(map[string]int64{"a": 2, "b": 2})
	l := &loop.Loop{
		Store:     s,
		Policy:    &stubPolicy{name: "stub", order: []string{"a", "b"}},
		Hooks:     loop.NoOpHooks{},
		Heap:      event.NewHeap(),
		NumAgents: 1,
		TicksCap:  1000,
		Seeds:     seed.From(0),
	}
	if _, _, err := l.Run(); err != nil {
		t.Fatal(err)
	}
}
