// Package event provides the deterministic min-heap that drives kerfsim's
// event-driven tick loop.
//
// The ordering rule is load-bearing for determinism: events.jsonl must be
// byte-identical across implementations and runs given the same inputs. See
// specs/simulator.md §Loop Mechanics and §Event Ordering.
package event

import (
	"container/heap"
)

// Kind enumerates the Phase 1 event kinds the tick loop processes.
//
// The numeric values are the canonical kind priority used as the secondary
// tie-breaker in the total ordering: lower value = processed first when ticks
// match. See specs/simulator.md §Event Ordering.
type Kind int

const (
	// KindComplete fires when a bead finishes.
	KindComplete Kind = 0
	// KindArrival fires when a bead enters the queue.
	KindArrival Kind = 1
	// KindAgentFree fires when an agent becomes idle and may pull from the queue.
	KindAgentFree Kind = 2
)

// priority returns the canonical kind priority used in the total ordering.
// The values are exactly the iota-style constants above; the method exists so
// callers (and tests) read intent rather than relying on the raw int value.
func (k Kind) priority() int {
	return int(k)
}

// String renders the canonical kind name. The names are stable and are the
// same strings that appear in events.jsonl.
func (k Kind) String() string {
	switch k {
	case KindComplete:
		return "complete"
	case KindArrival:
		return "arrival"
	case KindAgentFree:
		return "agent-free"
	default:
		return "unknown"
	}
}

// Event is the unit of work the tick loop processes.
//
// The fields participating in the total ordering are Tick, Kind, AgentID,
// BeadID — in that order. Payload carries kind-specific data and is opaque
// to the heap.
type Event struct {
	Tick    int64
	Kind    Kind
	AgentID int
	BeadID  string
	Payload any
}

// less implements the canonical total ordering on events:
//
//  1. Tick ascending
//  2. Kind priority ascending (complete=0, arrival=1, agent-free=2)
//  3. AgentID ascending
//  4. BeadID ascending (lexicographic)
//
// Equal in all four => equal-priority (heap order undefined between them,
// but in practice such events are interchangeable for the loop).
func less(a, b Event) bool {
	if a.Tick != b.Tick {
		return a.Tick < b.Tick
	}
	if a.Kind != b.Kind {
		return a.Kind.priority() < b.Kind.priority()
	}
	if a.AgentID != b.AgentID {
		return a.AgentID < b.AgentID
	}
	return a.BeadID < b.BeadID
}

// innerHeap is the unexported slice-with-heap.Interface used by container/heap.
// External callers go through Heap, which hides the heap.Push/heap.Pop ritual.
type innerHeap []Event

func (h innerHeap) Len() int            { return len(h) }
func (h innerHeap) Less(i, j int) bool  { return less(h[i], h[j]) }
func (h innerHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *innerHeap) Push(x any)         { *h = append(*h, x.(Event)) }
func (h *innerHeap) Pop() any {
	old := *h
	n := len(old)
	v := old[n-1]
	*h = old[:n-1]
	return v
}

// Heap is a min-heap of events ordered by the canonical total ordering. The
// zero value is a valid empty heap.
type Heap struct {
	h innerHeap
}

// NewHeap returns a fresh empty event heap. The zero value of Heap is also
// usable; this constructor exists for readability at call sites.
func NewHeap() *Heap {
	return &Heap{}
}

// Len returns the number of events currently in the heap.
func (eh *Heap) Len() int {
	return eh.h.Len()
}

// Push inserts e into the heap.
func (eh *Heap) Push(e Event) {
	heap.Push(&eh.h, e)
}

// Pop removes and returns the smallest event under the canonical total
// ordering. Pop on an empty heap panics; callers should check Len() first.
func (eh *Heap) Pop() Event {
	if eh.h.Len() == 0 {
		panic("event.Heap: Pop on empty heap")
	}
	return heap.Pop(&eh.h).(Event)
}

// Peek returns the smallest event without removing it. Peek on an empty heap
// panics; callers should check Len() first. The returned Event is a copy.
func (eh *Heap) Peek() Event {
	if eh.h.Len() == 0 {
		panic("event.Heap: Peek on empty heap")
	}
	return eh.h[0]
}
