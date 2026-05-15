// Package loop implements the kerfsim event-driven tick loop.
//
// The loop pops events off a deterministic min-heap, advances the simulation
// clock, mutates the bead store, and dispatches idle agents through a
// policy.Policy. All observable side effects (event firings, dispatches,
// completions, top-of-queue snapshots) are exposed through the Hooks
// interface, which the metrics collector (B9b) implements.
//
// See specs/simulator.md §Loop Mechanics, §Event Ordering, §Stop Conditions,
// §Determinism.
package loop

import (
	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/sim/event"
	"github.com/gberns/kerf/internal/sim/policy"
	"github.com/gberns/kerf/internal/sim/store"
)

// Stop reason strings returned by Run. Values are stable and are recorded in
// run summaries.
const (
	StopAllClosed     = "all-closed"
	StopTicksCap      = "ticks-cap"
	StopIdleThreshold = "idle-threshold"
)

// Hooks is the observation seam for the tick loop. The loop fires callbacks
// at every canonical observation point; metrics collection (B9a/B9b) and any
// other observer hooks in through here.
//
// Callbacks must not mutate loop state. The loop calls them synchronously
// from the same goroutine that drives the heap, so they may safely read the
// store snapshot if needed.
type Hooks interface {
	// OnEvent fires for every event popped from the heap, in canonical
	// order, before the loop applies any store mutations for that event.
	OnEvent(e event.Event)
	// OnDispatch fires when an agent is assigned a bead.
	OnDispatch(agentID int, beadID string, tick int64)
	// OnComplete fires when a bead transitions to closed.
	OnComplete(beadID string, tick int64)
	// OnArrival fires when a new bead enters the store.
	OnArrival(beadID string, work string, tick int64)
	// OnSnapshot fires after every mutating event (dispatch, arrival,
	// completion) with the current top-of-queue bead ID (or "" if the
	// store has no dispatchable bead). The top is determined by calling
	// the loop's Policy.Next on the post-mutation store.
	OnSnapshot(top string)
}

// NoOpHooks is a zero-effect Hooks implementation, intended for headless
// runs and tests that exercise the loop's mechanics without observing them.
type NoOpHooks struct{}

func (NoOpHooks) OnEvent(event.Event)                  {}
func (NoOpHooks) OnDispatch(int, string, int64)        {}
func (NoOpHooks) OnComplete(string, int64)             {}
func (NoOpHooks) OnArrival(string, string, int64)      {}
func (NoOpHooks) OnSnapshot(string)                    {}

// Loop is the simulator's event-driven tick loop. Construct one with all
// required fields and call Run.
//
// The loop does not own its store: callers supply one (typically built via
// store.From). The loop does not construct its initial heap either, beyond
// pushing the N initial agent-free events — callers may pre-populate the
// heap with scripted arrival events (KindArrival, Payload = beads.Bead) and
// the loop will process them in canonical order.
//
// Determinism: given identical Store, Policy, initial Heap contents,
// NumAgents, TicksCap, and Seeds, two Run invocations produce identical
// event-pop sequences and identical hook firings.
type Loop struct {
	// Store is the in-memory bead store mutated as the simulation runs.
	Store *store.Store
	// Policy chooses the next bead for an idle agent.
	Policy policy.Policy
	// Hooks observes the simulation. Must not be nil — pass NoOpHooks{} to
	// disable observation.
	Hooks Hooks
	// Heap is the event heap. The loop pushes the initial agent-free
	// events; callers may pre-populate with arrival events.
	Heap *event.Heap
	// NumAgents is the agent count (>=1).
	NumAgents int
	// TicksCap is the simulation clock cap. The loop stops as soon as an
	// event would advance the clock strictly past TicksCap.
	TicksCap int64
	// Seeds derives sub-seeds from the top-level seed. The loop does not
	// consume any sub-seeds directly in Phase 1 — score-tie resolution is
	// internal to the kerf policy and baseline policies — but the field
	// is kept on the loop so policies and future fidelity layers can
	// reach it through a single channel.
	Seeds func(name string) uint64
}

// Run executes the simulation. It returns the matched stop reason and the
// final wall-tick count (the last simulation-clock tick reached).
//
// Stop conditions, checked in this order before each event pop:
//   1. All beads in the store are closed → StopAllClosed.
//   2. The heap is empty AND every agent is idle → StopIdleThreshold.
//   3. The next event's tick would exceed TicksCap → StopTicksCap.
//
// Mutating events (dispatch, arrival, completion) fire an OnSnapshot
// callback with the current Policy.Next(store) result.
func (l *Loop) Run() (stopReason string, wallTicks int64, err error) {
	// Mutual-exclusion bookkeeping: idle[a] tracks whether agent a is
	// currently free. Used to wake idle agents on arrival and to test the
	// "all agents idle" half of the idle-threshold stop condition.
	idle := make([]bool, l.NumAgents)
	for i := range idle {
		idle[i] = true
	}

	// Seed the heap with the initial agent-free events. These are what
	// drive the first dispatches.
	for a := 0; a < l.NumAgents; a++ {
		l.Heap.Push(event.Event{
			Tick:    0,
			Kind:    event.KindAgentFree,
			AgentID: a,
		})
	}

	for {
		// Stop condition 1: every bead closed.
		if l.Store.AllClosed() && hasAnyBead(l.Store) {
			return StopAllClosed, wallTicks, nil
		}
		// Stop condition 2: nothing to do.
		if l.Heap.Len() == 0 && allIdle(idle) {
			return StopIdleThreshold, wallTicks, nil
		}
		if l.Heap.Len() == 0 {
			// No events but some agents in-flight — impossible in
			// Phase 1 because every dispatch schedules a
			// completion. Bail with idle-threshold to avoid an
			// infinite loop in pathological inputs.
			return StopIdleThreshold, wallTicks, nil
		}

		// Stop condition 3: peek; if the next event lies beyond the
		// cap, stop without processing it. wallTicks remains at the
		// last tick actually reached.
		next := l.Heap.Peek()
		if next.Tick > l.TicksCap {
			return StopTicksCap, wallTicks, nil
		}

		ev := l.Heap.Pop()
		wallTicks = ev.Tick
		l.Hooks.OnEvent(ev)

		switch ev.Kind {
		case event.KindArrival:
			b, ok := ev.Payload.(beads.Bead)
			if !ok {
				// Non-bead payload: treat as a no-op arrival.
				// The store has nothing to add; still fire a
				// snapshot to keep the contract symmetric.
				l.Hooks.OnSnapshot(currentTop(l.Store, l.Policy))
				continue
			}
			l.Store.Arrive(b, ev.Tick)
			l.Hooks.OnArrival(b.ID, workCodeFromBead(b), ev.Tick)
			// Wake any currently-idle agents so they re-enter
			// dispatch consideration at this tick.
			for a := 0; a < l.NumAgents; a++ {
				if idle[a] {
					l.Heap.Push(event.Event{
						Tick:    ev.Tick,
						Kind:    event.KindAgentFree,
						AgentID: a,
					})
				}
			}
			l.Hooks.OnSnapshot(currentTop(l.Store, l.Policy))

		case event.KindComplete:
			l.Store.Complete(ev.BeadID, ev.Tick)
			l.Hooks.OnComplete(ev.BeadID, ev.Tick)
			// Free the agent that owned this bead and re-enter
			// dispatch at the same tick.
			idle[ev.AgentID] = true
			l.Heap.Push(event.Event{
				Tick:    ev.Tick,
				Kind:    event.KindAgentFree,
				AgentID: ev.AgentID,
			})
			l.Hooks.OnSnapshot(currentTop(l.Store, l.Policy))

		case event.KindAgentFree:
			// Stale agent-free events (e.g. a duplicate wake from
			// an arrival fired while the agent was actually
			// in-flight) are filtered here.
			if !idle[ev.AgentID] {
				continue
			}
			beadID := l.Policy.Next(l.Store)
			if beadID == "" {
				// No dispatchable bead — agent remains idle.
				// No snapshot fires (non-mutating outcome).
				continue
			}
			l.Store.Dispatch(beadID, ev.AgentID, ev.Tick)
			idle[ev.AgentID] = false
			l.Hooks.OnDispatch(ev.AgentID, beadID, ev.Tick)
			// Schedule the matching completion. Duration comes
			// from the store's pre-rolled value; zero means
			// "completes on the same tick".
			dur := l.Store.Duration(beadID)
			l.Heap.Push(event.Event{
				Tick:    ev.Tick + dur,
				Kind:    event.KindComplete,
				AgentID: ev.AgentID,
				BeadID:  beadID,
			})
			l.Hooks.OnSnapshot(currentTop(l.Store, l.Policy))
		}
	}
}

// allIdle reports whether every agent is currently idle.
func allIdle(idle []bool) bool {
	for _, v := range idle {
		if !v {
			return false
		}
	}
	return true
}

// hasAnyBead reports whether the store contains at least one bead. The
// all-closed stop condition must not fire on an empty store (a vacuously
// "all closed" store would otherwise stop the run before any arrival).
func hasAnyBead(s *store.Store) bool {
	return len(s.Beads()) > 0
}

// currentTop returns the current top-of-queue bead ID by consulting the
// policy on a non-mutating read, or "" if no bead is dispatchable.
func currentTop(s *store.Store, p policy.Policy) string {
	return p.Next(s)
}

// workCodeFromBead extracts a work codename from a bead's `work:<codename>`
// label, or returns "" if none is present.
func workCodeFromBead(b beads.Bead) string {
	const prefix = "work:"
	for _, l := range b.Labels {
		if len(l) > len(prefix) && l[:len(prefix)] == prefix {
			return l[len(prefix):]
		}
	}
	return ""
}
