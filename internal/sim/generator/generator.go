// Package generator produces a deterministic GeneratedWorld from a
// scenario.Scenario. See specs/simulator.md §Synthetic Generator and
// §Generator Parameters for the normative description.
//
// Determinism (specs/simulator.md §Determinism, §Seed Splitting) is
// enforced by drawing all structural randomness from named sub-seeds
// derived from the scenario's top-level seed:
//
//   - seed.Gen    ("gen")    — clustered-DAG edges and per-work bead counts.
//   - seed.Dur    ("dur")    — per-bead duration pre-rolling.
//   - seed.Events ("events") — probabilistic rework arrivals.
//
// math/rand's global state is never used; every RNG is locally
// constructed from a derived sub-seed so that the same scenario produces
// a byte-identical GeneratedWorld on every run.
package generator

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/gberns/kerf/internal/sim/scenario"
	"github.com/gberns/kerf/internal/sim/seed"
)

// GeneratedWorld is the deterministic output of Generate: the complete set
// of works (with epic-clustered DAG dependencies), the initial and
// scheduled beads (with pre-rolled durations), and a sorted event
// timeline of post-start arrivals (scripted + probabilistic).
//
// Two GeneratedWorld values produced from byte-identical scenarios are
// themselves byte-identical under any stable serialization. The struct
// is the placeholder for B6 (internal/sim/store); B6 will consume the
// fields without modification.
type GeneratedWorld struct {
	// TopSeed echoes scenario.Seed for traceability.
	TopSeed int64

	// Works lists every work in deterministic codename order. Each work's
	// Deps slice is sorted ascending so the DAG serializes stably.
	Works []GeneratedWork

	// Beads lists every bead present at simulation start (BeadArrival.Tick == 0)
	// plus every scheduled future arrival. Sorted by (ArrivalTick, BeadID).
	Beads []GeneratedBead

	// EventTimeline holds the chronologically-sorted arrival events
	// derived from BeadArrivals — scripted explicit entries and
	// probabilistic generator-driven rework. Each entry has a 1:1
	// correspondence to a Beads entry with the same BeadID.
	EventTimeline []ScheduledArrival
}

// GeneratedWork is one work in the generated world.
type GeneratedWork struct {
	Codename         string
	Areas            []string
	Deps             []string // sorted ascending; only references earlier works
	BeadCount        int
	WorkCreatedTick  int64
	Epic             int // 0-based epic index this work belongs to
}

// GeneratedBead is one bead with its pre-rolled duration.
type GeneratedBead struct {
	BeadID      string
	Work        string
	ArrivalTick int64
	Duration    int64
	Labels      []string // e.g. {"rework:true"} for rework arrivals
}

// ScheduledArrival is a future bead-arrival event on the timeline.
type ScheduledArrival struct {
	Tick   int64
	BeadID string
	Work   string
	Labels []string
}

// Generate consumes a validated scenario and produces a GeneratedWorld.
//
// The function is pure: no wall-clock reads, no global state, no
// math/rand global. Given the same scenario it returns a structurally
// identical GeneratedWorld every time.
func Generate(s *scenario.Scenario) (*GeneratedWorld, error) {
	if s == nil {
		return nil, fmt.Errorf("generator: nil scenario")
	}
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("generator: scenario invalid: %w", err)
	}

	// Top seed is a non-negative scenario seed; convert to canonical
	// uint64 representation for sub-seed derivation.
	topSeed := uint64(s.Seed)
	derive := seed.From(topSeed)

	// gen sub-seed: structural randomness (epic assignment, edge draws,
	// bead-count log-normal samples). See specs/simulator.md §Seed Splitting.
	genRNG := rand.New(rand.NewSource(int64(derive(seed.Gen))))

	// dur sub-seed: bead durations pre-rolled once at generation time so
	// that swapping weights at run time does not perturb the work itself.
	durRNG := rand.New(rand.NewSource(int64(derive(seed.Dur))))

	// events sub-seed: probabilistic rework arrivals from the generator
	// spec in bead_arrivals.generator.
	evRNG := rand.New(rand.NewSource(int64(derive(seed.Events))))

	mu, err := s.AgentModel.Duration.Mu()
	if err != nil {
		return nil, fmt.Errorf("generator: duration mu: %w", err)
	}
	sigma := s.AgentModel.Duration.Sigma

	// 1) Build works in scenario order. Synthesize epic assignment and
	//    clustered-DAG edges for any work that does not already carry
	//    explicit deps. Bead counts come from log-normal(median=12) for
	//    works that declare bead_count == 0; otherwise the scenario value
	//    is respected.
	works := make([]GeneratedWork, len(s.Works))
	epicCount := pickEpicCount(genRNG, len(s.Works))

	for i, w := range s.Works {
		epic := -1
		if len(s.Works) > 0 {
			epic = assignEpic(genRNG, i, len(s.Works), epicCount)
		}
		works[i] = GeneratedWork{
			Codename:        w.Codename,
			Areas:           append([]string(nil), w.Areas...),
			BeadCount:       w.BeadCount,
			WorkCreatedTick: 0,
			Epic:            epic,
		}
		if works[i].BeadCount <= 0 {
			works[i].BeadCount = drawBeadCount(genRNG)
		}
		works[i].Deps = generateDeps(genRNG, i, works, w.Deps)
	}

	// 2) Initial beads — bead_count per work, all arriving at tick 0.
	var beads []GeneratedBead
	for _, w := range works {
		for j := 0; j < w.BeadCount; j++ {
			bid := fmt.Sprintf("%s/b%d", w.Codename, j+1)
			beads = append(beads, GeneratedBead{
				BeadID:      bid,
				Work:        w.Codename,
				ArrivalTick: 0,
				Duration:    drawDuration(durRNG, mu, sigma),
			})
		}
	}

	// 3) Scripted arrivals — bead_arrivals.explicit. Tick comes from the
	//    scenario; duration is still pre-rolled from the dur sub-seed.
	var timeline []ScheduledArrival
	if s.BeadArrivals.Explicit != nil {
		// Index explicit-arrival counts per work so generated IDs do not
		// collide with the initial beads.
		nextIdx := make(map[string]int)
		for _, w := range works {
			nextIdx[w.Codename] = w.BeadCount
		}
		for i, ea := range s.BeadArrivals.Explicit {
			bid := ea.BeadID
			if bid == "" {
				nextIdx[ea.Work]++
				bid = fmt.Sprintf("%s/b%d", ea.Work, nextIdx[ea.Work])
			}
			labels := append([]string(nil), ea.Labels...)
			tick := int64(ea.Tick)
			beads = append(beads, GeneratedBead{
				BeadID:      bid,
				Work:        ea.Work,
				ArrivalTick: tick,
				Duration:    drawDuration(durRNG, mu, sigma),
				Labels:      labels,
			})
			timeline = append(timeline, ScheduledArrival{
				Tick:   tick,
				BeadID: bid,
				Work:   ea.Work,
				Labels: labels,
			})
			_ = i
		}
	}

	// 4) Probabilistic rework arrivals from generator.rework_rate_per_tick.
	//    Each tick in [1, ticks-1] independently draws a Bernoulli; on a
	//    hit, a rework-labelled bead is appended to one of the target
	//    works (round-robin over target_works in scenario order). All
	//    draws come from the events sub-seed.
	if g := s.BeadArrivals.Generator; g != nil && g.ReworkRatePerTick > 0 && len(g.TargetWorks) > 0 {
		targets := append([]string(nil), g.TargetWorks...)
		// Track per-work suffix counters to keep bead IDs unique.
		nextIdx := make(map[string]int)
		for _, w := range works {
			nextIdx[w.Codename] = w.BeadCount
		}
		// Walk the tick space deterministically. The cap is scenario.Ticks.
		for t := int64(1); t < s.Ticks; t++ {
			if evRNG.Float64() < g.ReworkRatePerTick {
				// Choose a target work using the events RNG so the
				// rotation order is itself deterministic but seed-derived.
				wIdx := evRNG.Intn(len(targets))
				wname := targets[wIdx]
				nextIdx[wname]++
				bid := fmt.Sprintf("%s/r%d", wname, nextIdx[wname])
				labels := []string{"rework:true"}
				beads = append(beads, GeneratedBead{
					BeadID:      bid,
					Work:        wname,
					ArrivalTick: t,
					Duration:    drawDuration(durRNG, mu, sigma),
					Labels:      labels,
				})
				timeline = append(timeline, ScheduledArrival{
					Tick:   t,
					BeadID: bid,
					Work:   wname,
					Labels: labels,
				})
			}
		}
	}

	// 5) Canonical sorts so the serialized form is byte-identical between
	//    runs of the same scenario.
	sort.SliceStable(beads, func(i, j int) bool {
		if beads[i].ArrivalTick != beads[j].ArrivalTick {
			return beads[i].ArrivalTick < beads[j].ArrivalTick
		}
		return beads[i].BeadID < beads[j].BeadID
	})
	sort.SliceStable(timeline, func(i, j int) bool {
		if timeline[i].Tick != timeline[j].Tick {
			return timeline[i].Tick < timeline[j].Tick
		}
		return timeline[i].BeadID < timeline[j].BeadID
	})

	// Cycle-check on the generated DAG. The construction (deps only point
	// to lower-indexed works) is acyclic by induction; the check is a
	// belt-and-braces guard so a future construction bug is caught here
	// instead of in the loop.
	if err := assertAcyclic(works); err != nil {
		return nil, fmt.Errorf("generator: %w", err)
	}

	return &GeneratedWorld{
		TopSeed:       s.Seed,
		Works:         works,
		Beads:         beads,
		EventTimeline: timeline,
	}, nil
}

// pickEpicCount draws an epic count in [3, 6] per specs/simulator.md
// §Generator Parameters. When the work count is smaller than 3 the epic
// count is clamped to the work count to avoid empty epics.
func pickEpicCount(r *rand.Rand, numWorks int) int {
	if numWorks <= 0 {
		return 0
	}
	const lo, hi = 3, 6
	k := lo + r.Intn(hi-lo+1) // uniform in [3, 6]
	if k > numWorks {
		k = numWorks
	}
	return k
}

// assignEpic assigns a work index to an epic deterministically. Works are
// spread across epics by contiguous blocks so each epic owns a connected
// slice of the codename ordering — this gives the intra-epic edge draw a
// non-trivial "older sibling" pool. A draw from the generator RNG is
// consumed per work so the assignment is seed-sensitive.
func assignEpic(r *rand.Rand, idx, total, epics int) int {
	if epics <= 0 {
		return 0
	}
	// Consume one RNG draw per work so adding works changes the
	// downstream stream — keeps the seed sensitivity load-bearing.
	jitter := r.Intn(epics)
	// Block assignment by index, with an RNG-derived rotation so two
	// different seeds produce different epic groupings even at the same
	// total/epics ratio.
	block := (idx*epics)/total + jitter
	return block % epics
}

// drawBeadCount samples per-work bead counts from log-normal with median
// 12 (specs/simulator.md §Generator Parameters). Sigma is fixed at 0.6
// for a moderate spread; this is the Phase-1 spec default. The result is
// clamped to [1, ...] so every work has at least one bead.
func drawBeadCount(r *rand.Rand) int {
	const medianBeads = 12.0
	const sigma = 0.6
	mu := math.Log(medianBeads)
	x := math.Exp(mu + sigma*r.NormFloat64())
	n := int(math.Round(x))
	if n < 1 {
		n = 1
	}
	return n
}

// drawDuration draws a log-normal duration in ticks using the
// scenario-provided mu and sigma. Clamped to >= 1 tick. The dur RNG is
// the only source of duration randomness (specs/simulator.md §Seed
// Splitting → "dur").
func drawDuration(r *rand.Rand, mu, sigma float64) int64 {
	x := math.Exp(mu + sigma*r.NormFloat64())
	n := int64(math.Round(x))
	if n < 1 {
		n = 1
	}
	return n
}

// generateDeps returns the dependency list for work i. If the scenario
// provided explicit deps for the work, they are used verbatim (sorted).
// Otherwise the generator draws intra-epic edges at probability 0.6 and
// inter-epic edges at probability 0.05 against every older sibling
// (specs/simulator.md §Generator Parameters). Construction is acyclic by
// induction: edges only point to lower-indexed works.
func generateDeps(r *rand.Rand, i int, works []GeneratedWork, explicit []string) []string {
	if len(explicit) > 0 {
		out := append([]string(nil), explicit...)
		sort.Strings(out)
		return out
	}
	const intra = 0.6
	const inter = 0.05
	if i == 0 {
		return nil
	}
	myEpic := works[i].Epic
	var deps []string
	for j := 0; j < i; j++ {
		p := inter
		if works[j].Epic == myEpic {
			p = intra
		}
		if r.Float64() < p {
			deps = append(deps, works[j].Codename)
		}
	}
	sort.Strings(deps)
	return deps
}

// assertAcyclic verifies the generated DAG is acyclic. Detection runs in
// O(V+E) via Kahn's algorithm; any cycle (none should exist by
// construction) is returned as an error.
func assertAcyclic(works []GeneratedWork) error {
	idx := make(map[string]int, len(works))
	for i, w := range works {
		idx[w.Codename] = i
	}
	indeg := make([]int, len(works))
	adj := make([][]int, len(works))
	for i, w := range works {
		for _, dep := range w.Deps {
			j, ok := idx[dep]
			if !ok {
				return fmt.Errorf("dag: work %q references unknown dep %q", w.Codename, dep)
			}
			// Edge: dep (j) -> work (i)
			adj[j] = append(adj[j], i)
			indeg[i]++
		}
	}
	queue := make([]int, 0, len(works))
	for i, d := range indeg {
		if d == 0 {
			queue = append(queue, i)
		}
	}
	visited := 0
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		visited++
		for _, m := range adj[n] {
			indeg[m]--
			if indeg[m] == 0 {
				queue = append(queue, m)
			}
		}
	}
	if visited != len(works) {
		return fmt.Errorf("dag: cycle detected (visited %d of %d works)", visited, len(works))
	}
	return nil
}
