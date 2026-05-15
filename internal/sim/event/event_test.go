package event

import (
	"math/rand"
	"reflect"
	"testing"
)

// popAll drains a heap into a slice for assertion.
func popAll(t *testing.T, h *Heap) []Event {
	t.Helper()
	out := make([]Event, 0, h.Len())
	for h.Len() > 0 {
		out = append(out, h.Pop())
	}
	return out
}

func TestKindPriority(t *testing.T) {
	// The canonical priority is load-bearing for determinism. Pin the numeric
	// values explicitly so any future reordering of the constants fails here
	// rather than silently re-shuffling events.jsonl.
	cases := []struct {
		k    Kind
		want int
		name string
	}{
		{KindComplete, 0, "complete"},
		{KindArrival, 1, "arrival"},
		{KindAgentFree, 2, "agent-free"},
	}
	for _, c := range cases {
		if got := c.k.priority(); got != c.want {
			t.Errorf("Kind %s: priority() = %d, want %d", c.name, got, c.want)
		}
		if got := c.k.String(); got != c.name {
			t.Errorf("Kind value %d: String() = %q, want %q", c.k, got, c.name)
		}
	}
}

func TestPopOrder_TickPrimary(t *testing.T) {
	h := NewHeap()
	// Insert in reverse tick order; expect ascending pop.
	h.Push(Event{Tick: 30, Kind: KindComplete, AgentID: 0, BeadID: "a"})
	h.Push(Event{Tick: 10, Kind: KindAgentFree, AgentID: 9, BeadID: "z"})
	h.Push(Event{Tick: 20, Kind: KindArrival, AgentID: 5, BeadID: "m"})

	got := popAll(t, h)
	wantTicks := []int64{10, 20, 30}
	for i, e := range got {
		if e.Tick != wantTicks[i] {
			t.Fatalf("pop[%d].Tick = %d, want %d (full sequence: %+v)", i, e.Tick, wantTicks[i], got)
		}
	}
}

func TestPopOrder_KindSecondary(t *testing.T) {
	// All ticks equal; agent_id and bead_id intentionally arranged so that any
	// implementation that fell back on insertion order or on (agent_id, bead_id)
	// before kind would visibly fail.
	h := NewHeap()
	h.Push(Event{Tick: 5, Kind: KindAgentFree, AgentID: 0, BeadID: "a"})
	h.Push(Event{Tick: 5, Kind: KindArrival, AgentID: 1, BeadID: "b"})
	h.Push(Event{Tick: 5, Kind: KindComplete, AgentID: 2, BeadID: "c"})

	got := popAll(t, h)
	wantKinds := []Kind{KindComplete, KindArrival, KindAgentFree}
	for i, e := range got {
		if e.Kind != wantKinds[i] {
			t.Fatalf("pop[%d].Kind = %s, want %s (full sequence: %+v)",
				i, e.Kind, wantKinds[i], got)
		}
	}
}

func TestPopOrder_AgentIDTertiary(t *testing.T) {
	// Equal tick, equal kind: agent_id breaks the tie.
	h := NewHeap()
	h.Push(Event{Tick: 7, Kind: KindArrival, AgentID: 3, BeadID: "x"})
	h.Push(Event{Tick: 7, Kind: KindArrival, AgentID: 1, BeadID: "x"})
	h.Push(Event{Tick: 7, Kind: KindArrival, AgentID: 2, BeadID: "x"})

	got := popAll(t, h)
	wantAgents := []int{1, 2, 3}
	for i, e := range got {
		if e.AgentID != wantAgents[i] {
			t.Fatalf("pop[%d].AgentID = %d, want %d (full sequence: %+v)",
				i, e.AgentID, wantAgents[i], got)
		}
	}
}

func TestPopOrder_BeadIDQuaternary(t *testing.T) {
	// Equal tick + kind + agent: bead_id breaks the tie lexicographically.
	h := NewHeap()
	h.Push(Event{Tick: 1, Kind: KindComplete, AgentID: 0, BeadID: "gamma"})
	h.Push(Event{Tick: 1, Kind: KindComplete, AgentID: 0, BeadID: "alpha"})
	h.Push(Event{Tick: 1, Kind: KindComplete, AgentID: 0, BeadID: "beta"})

	got := popAll(t, h)
	wantIDs := []string{"alpha", "beta", "gamma"}
	for i, e := range got {
		if e.BeadID != wantIDs[i] {
			t.Fatalf("pop[%d].BeadID = %q, want %q (full sequence: %+v)",
				i, e.BeadID, wantIDs[i], got)
		}
	}
}

// canonicalFixture is the expected pop order for the mixed table-driven test.
// It exercises all four tie-break levels in a single sequence.
func canonicalFixture() []Event {
	return []Event{
		// tick 1 — only one event
		{Tick: 1, Kind: KindArrival, AgentID: 0, BeadID: "w/b1"},
		// tick 5: complete < arrival < agent-free
		{Tick: 5, Kind: KindComplete, AgentID: 2, BeadID: "w/b9"},
		{Tick: 5, Kind: KindArrival, AgentID: 0, BeadID: "w/b3"},
		{Tick: 5, Kind: KindArrival, AgentID: 0, BeadID: "w/b7"},
		{Tick: 5, Kind: KindAgentFree, AgentID: 1, BeadID: ""},
		// tick 5 continues: agent-free ordered by agent_id
		{Tick: 5, Kind: KindAgentFree, AgentID: 2, BeadID: ""},
		// tick 9: same kind, different agents and beads
		{Tick: 9, Kind: KindComplete, AgentID: 0, BeadID: "w/b2"},
		{Tick: 9, Kind: KindComplete, AgentID: 0, BeadID: "w/b5"},
		{Tick: 9, Kind: KindComplete, AgentID: 3, BeadID: "w/b1"},
	}
}

func TestPopOrder_CanonicalFixture(t *testing.T) {
	want := canonicalFixture()

	// Push in a deliberately scrambled order — the heap must still pop the
	// canonical sequence.
	scrambled := []int{5, 8, 0, 3, 6, 1, 7, 2, 4}
	h := NewHeap()
	for _, i := range scrambled {
		h.Push(want[i])
	}

	got := popAll(t, h)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonical fixture pop order mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestDeterminism_AcrossPushOrders(t *testing.T) {
	// Determinism: regardless of push order, the pop sequence is the canonical
	// total ordering. Shuffle the fixture many times and confirm pop sequences
	// match byte-for-byte.
	want := canonicalFixture()

	// Build a reference pop sequence from a straightforward in-order push.
	ref := NewHeap()
	for _, e := range want {
		ref.Push(e)
	}
	refSeq := popAll(t, ref)

	r := rand.New(rand.NewSource(1))
	for trial := 0; trial < 50; trial++ {
		perm := r.Perm(len(want))
		h := NewHeap()
		for _, idx := range perm {
			h.Push(want[idx])
		}
		got := popAll(t, h)
		if !reflect.DeepEqual(got, refSeq) {
			t.Fatalf("trial %d: pop sequence differs from reference\n perm: %v\n  got: %+v\n want: %+v",
				trial, perm, got, refSeq)
		}
	}
}

func TestPeekDoesNotConsume(t *testing.T) {
	h := NewHeap()
	h.Push(Event{Tick: 10, Kind: KindArrival, AgentID: 0, BeadID: "x"})
	h.Push(Event{Tick: 5, Kind: KindComplete, AgentID: 0, BeadID: "y"})

	first := h.Peek()
	if first.Tick != 5 || first.Kind != KindComplete {
		t.Fatalf("Peek returned %+v, want tick=5 complete", first)
	}
	if h.Len() != 2 {
		t.Fatalf("Peek mutated heap: Len = %d, want 2", h.Len())
	}
	again := h.Peek()
	if !reflect.DeepEqual(first, again) {
		t.Fatalf("Peek not idempotent: %+v vs %+v", first, again)
	}
	popped := h.Pop()
	if !reflect.DeepEqual(popped, first) {
		t.Fatalf("Pop after Peek returned %+v, want %+v", popped, first)
	}
}

func TestEmptyHeap_LenZero(t *testing.T) {
	h := NewHeap()
	if h.Len() != 0 {
		t.Fatalf("fresh heap Len = %d, want 0", h.Len())
	}
}

func TestEmptyHeap_PopPanics(t *testing.T) {
	h := NewHeap()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Pop on empty heap did not panic")
		}
	}()
	_ = h.Pop()
}

func TestEmptyHeap_PeekPanics(t *testing.T) {
	h := NewHeap()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Peek on empty heap did not panic")
		}
	}()
	_ = h.Peek()
}

func TestPayloadIsPreserved(t *testing.T) {
	// The heap is opaque to Payload; it should round-trip whatever the caller
	// stashed there. This guards against accidental field stripping in any
	// future refactor of the heap internals.
	type marker struct{ S string }
	h := NewHeap()
	h.Push(Event{Tick: 1, Kind: KindComplete, AgentID: 0, BeadID: "b", Payload: &marker{S: "hello"}})
	got := h.Pop()
	m, ok := got.Payload.(*marker)
	if !ok || m == nil || m.S != "hello" {
		t.Fatalf("Payload not preserved: %+v", got.Payload)
	}
}
