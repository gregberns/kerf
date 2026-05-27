// Package policy defines the contract a bead-selection strategy implements
// so the simulator loop can dispatch beads to idle agents.
//
// The Policy interface is the single coupling point between the loop and
// the ordering logic. Implementations live elsewhere:
//
//   - kerf's scoring policy wraps internal/queue.Compute (see internal/sim/run
//     in plan 007 / B10).
//   - Baseline policies (random, fifo-bead, fifo-work) live in
//     internal/sim/baselines (B8).
//
// See specs/simulator.md §Loop Mechanics (Dispatch) and §Baselines.
package policy

import (
	"github.com/gregberns/kerf/internal/sim/store"
)

// Policy chooses the next bead for an idle agent. Implementations read the
// current store snapshot via the queue-adapter accessors and return the bead
// ID to dispatch, or "" when no bead is currently dispatchable.
//
// Policy implementations must be deterministic given the store contents and
// any internal seed state — see specs/simulator.md §Determinism.
type Policy interface {
	// Name returns a stable identifier for the policy, used in run output
	// keys (e.g. "kerf", "random", "fifo-bead", "fifo-work").
	Name() string

	// Next returns the bead ID to dispatch next, or "" if no bead is
	// dispatchable from the current store state.
	Next(s *store.Store) string
}
