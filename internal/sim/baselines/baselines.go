// Package baselines implements the Phase 1 baseline policies from
// specs/simulator.md §Baselines: random, fifo-bead, fifo-work.
//
// Each baseline satisfies internal/sim/policy.Policy and is deterministic
// given the same store state and (for random) the same top-level seed.
//
// Dispatchable bead definition (specs/simulator.md §Loop Mechanics / Dispatch):
//   - The bead is open.
//   - The bead's owning work is actionable (not terminal, not shelved, with
//     must-complete-first dependencies satisfied). This is derived from
//     queue.Compute, which is the canonical source of work-level actionability.
//   - Bead-level depends_on (intra-work) is honored: a bead is not
//     dispatchable while any of its DependsOn beads remain non-closed.
package baselines

import (
	"math/rand"
	"sort"

	"github.com/gberns/kerf/internal/queue"
	"github.com/gberns/kerf/internal/sim/policy"
	"github.com/gberns/kerf/internal/sim/seed"
	"github.com/gberns/kerf/internal/sim/store"
)

// defaultWeights are passed to queue.Compute for work-level actionability
// filtering. Score values don't affect baselines — only the set of returned
// (actionable) work codenames matters — but we use defaults for stability.
func defaultWeights() queue.Weights { return queue.DefaultWeights() }

// actionableWorks returns the set of work codenames that queue.Compute deems
// actionable from the current store snapshot.
func actionableWorks(s *store.Store) map[string]struct{} {
	entries := queue.Compute(s.Works(), s.SummaryByWork(), defaultWeights())
	out := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		out[e.Codename] = struct{}{}
	}
	return out
}

// dispatchable returns the IDs of beads that are dispatchable from the
// current store, sorted by bead ID ascending for deterministic iteration.
//
// A bead is dispatchable when it is open, its owning work is in the
// actionable set, and all of its bead-level depends_on beads are closed.
func dispatchable(s *store.Store) []string {
	works := actionableWorks(s)
	beadList := s.Beads()
	out := make([]string, 0, len(beadList))
	for _, b := range beadList {
		st := s.Lookup(b.ID)
		if st == nil {
			continue
		}
		if st.Status != store.StatusOpen {
			continue
		}
		if st.WorkCode == "" {
			continue
		}
		if _, ok := works[st.WorkCode]; !ok {
			continue
		}
		if !depsSatisfied(s, st.DependsOn) {
			continue
		}
		out = append(out, b.ID)
	}
	sort.Strings(out)
	return out
}

// depsSatisfied returns true if every bead listed in deps is closed (or
// absent from the store, which treats unknown deps as not blocking — the
// generator is responsible for keeping the dep graph internally consistent).
func depsSatisfied(s *store.Store, deps []string) bool {
	for _, id := range deps {
		st := s.Lookup(id)
		if st == nil {
			continue
		}
		if st.Status != store.StatusClosed {
			return false
		}
	}
	return true
}

// --- random ----------------------------------------------------------------

// Random picks uniformly at random from the dispatchable bead set.
//
// Determinism: the random source is seeded once at construction from a
// dedicated `baseline_random` sub-seed derived from the simulator's top
// seed (specs/simulator.md §Determinism). The dispatchable set is sorted
// by bead ID before indexing so the same store state plus the same seed
// always yields the same pick.
type Random struct {
	rng *rand.Rand
}

// NewRandom returns a `random` baseline policy seeded from the given top
// seed via the `baseline_random` sub-seed.
func NewRandom(topSeed []byte) policy.Policy {
	sub := seed.Derive(topSeed, seed.BaselineRandom)
	return &Random{rng: rand.New(rand.NewSource(int64(sub)))}
}

// Name returns "random".
func (r *Random) Name() string { return "random" }

// Next returns a uniformly chosen dispatchable bead ID, or "" if none.
func (r *Random) Next(s *store.Store) string {
	cands := dispatchable(s)
	if len(cands) == 0 {
		return ""
	}
	idx := r.rng.Intn(len(cands))
	return cands[idx]
}

// --- fifo-bead -------------------------------------------------------------

// FIFOBead picks the dispatchable bead with the lowest arrival tick.
// Tiebreaker: bead_id ascending.
type FIFOBead struct{}

// NewFIFOBead returns a `fifo-bead` baseline policy.
func NewFIFOBead() policy.Policy { return &FIFOBead{} }

// Name returns "fifo-bead".
func (FIFOBead) Name() string { return "fifo-bead" }

// Next returns the oldest dispatchable bead, breaking ties by bead_id.
func (FIFOBead) Next(s *store.Store) string {
	cands := dispatchable(s)
	if len(cands) == 0 {
		return ""
	}
	sort.SliceStable(cands, func(i, j int) bool {
		bi := s.Lookup(cands[i])
		bj := s.Lookup(cands[j])
		if bi.ArrivedAt != bj.ArrivedAt {
			return bi.ArrivedAt < bj.ArrivedAt
		}
		return cands[i] < cands[j]
	})
	return cands[0]
}

// --- fifo-work -------------------------------------------------------------

// FIFOWork picks the dispatchable bead belonging to the oldest work.
//
// Ordering key:
//   - work's Created timestamp ascending
//   - then within-work arrival_tick ascending
//   - then bead_id ascending
//
// Tiebreak on the work's Created timestamp: work codename ascending.
type FIFOWork struct{}

// NewFIFOWork returns a `fifo-work` baseline policy.
func NewFIFOWork() policy.Policy { return &FIFOWork{} }

// Name returns "fifo-work".
func (FIFOWork) Name() string { return "fifo-work" }

// Next returns the oldest dispatchable bead under the fifo-work ordering.
func (FIFOWork) Next(s *store.Store) string {
	cands := dispatchable(s)
	if len(cands) == 0 {
		return ""
	}

	// Build a codename → SpecYAML lookup so we can read Created/Codename
	// directly for sort comparisons.
	works := s.Works()
	byCode := make(map[string]int, len(works))
	for i, w := range works {
		byCode[w.Codename] = i
	}

	sort.SliceStable(cands, func(i, j int) bool {
		bi := s.Lookup(cands[i])
		bj := s.Lookup(cands[j])
		wi := works[byCode[bi.WorkCode]]
		wj := works[byCode[bj.WorkCode]]

		// Primary: work created ascending.
		if !wi.Created.Equal(wj.Created) {
			return wi.Created.Before(wj.Created)
		}
		// Tiebreak on work created: codename ascending.
		if wi.Codename != wj.Codename {
			return wi.Codename < wj.Codename
		}
		// Within a work: arrival tick ascending.
		if bi.ArrivedAt != bj.ArrivedAt {
			return bi.ArrivedAt < bj.ArrivedAt
		}
		// Final tiebreak: bead_id ascending.
		return cands[i] < cands[j]
	})
	return cands[0]
}
