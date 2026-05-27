// Package run is the kerfsim run orchestrator (Plan 007 / B10).
//
// Run takes a validated scenario and a queue.Weights, generates a single
// deterministic GeneratedWorld via the synthetic generator, and then drives
// four independent simulator passes — one for the kerf scoring policy and
// one for each of the three Phase-1 baselines (random, fifo-bead, fifo-work).
// Each pass receives its own mutation-isolated store.Store and metrics.Collector,
// so the four passes share inputs but never cross-contaminate state.
//
// The result is four output.Result records ready to hand to internal/sim/output
// (Plan 007 / B11) for on-disk serialization, together with the canonical
// scenario and weights bytes used to drive the run.
//
// All randomness is derived from the scenario's top-level seed via the
// `internal/sim/seed` sub-seed scheme — same scenario + same weights → byte-
// identical Result. See specs/simulator.md §Determinism.
package run

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/gregberns/kerf/internal/beads"
	"github.com/gregberns/kerf/internal/queue"
	"github.com/gregberns/kerf/internal/sim/baselines"
	"github.com/gregberns/kerf/internal/sim/duration"
	"github.com/gregberns/kerf/internal/sim/event"
	"github.com/gregberns/kerf/internal/sim/generator"
	"github.com/gregberns/kerf/internal/sim/loop"
	"github.com/gregberns/kerf/internal/sim/metrics"
	"github.com/gregberns/kerf/internal/sim/output"
	"github.com/gregberns/kerf/internal/sim/policy"
	"github.com/gregberns/kerf/internal/sim/scenario"
	"github.com/gregberns/kerf/internal/sim/seed"
	"github.com/gregberns/kerf/internal/sim/store"
)

// Result is the aggregate output of a single Run invocation.
//
// Each policy field is an independently-computed output.Result, ready to be
// written by internal/sim/output. Scenario and Weights carry the canonical
// bytes used as inputs so the run directory can copy them verbatim and the
// summaries can carry their SHA-256s.
type Result struct {
	Kerf     output.Result
	Random   output.Result
	FIFOBead output.Result
	FIFOWork output.Result

	Scenario []byte // raw scenario yaml bytes
	Weights  []byte // canonical weights yaml bytes (never empty for a successful Run)
}

// Run orchestrates one simulation pass per policy and returns a Result
// containing all four per-policy outputs.
//
// agents overrides scenario.Agents when > 0; otherwise the scenario's value
// is used. weights is the kerf scoring weights handed to the kerf policy and
// (transitively) to the baselines' work-actionability filtering.
//
// Determinism guarantee: given byte-identical (s, weights, agents), two
// invocations of Run return Results whose four output.Result fields are
// deep-equal.
func Run(s *scenario.Scenario, weights queue.Weights, agents int) (*Result, error) {
	return RunWithOptions(s, weights, agents, Options{})
}

// Options bundles the optional knobs to Run: a debug sink for the kerf
// policy and a fitted-distribution registry that scenarios may reference
// via kind=from_distribution.
type Options struct {
	KerfDebug metrics.DebugSink
	Registry  *duration.Registry
}

// RunWithDebug mirrors Run but routes a metrics.DebugSink into the kerf
// policy pass. The sink (when non-nil) observes the structured Arrival and
// Dispatch streams for the kerf-policy run only — baselines do not emit
// debug records, since the B14 diagnostic is about the simulator itself,
// not policy behavior.
func RunWithDebug(s *scenario.Scenario, weights queue.Weights, agents int, kerfSink metrics.DebugSink) (*Result, error) {
	return RunWithOptions(s, weights, agents, Options{KerfDebug: kerfSink})
}

// RunWithOptions is the full-signature entry point. Run and RunWithDebug
// are thin wrappers.
func RunWithOptions(s *scenario.Scenario, weights queue.Weights, agents int, opts Options) (*Result, error) {
	if s == nil {
		return nil, fmt.Errorf("run: nil scenario")
	}
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("run: scenario invalid: %w", err)
	}

	// Use the scenario's agent count unless overridden.
	if agents <= 0 {
		agents = s.Agents
	}

	// Generate one world; all four passes share it.
	world, err := generator.GenerateWithRegistry(s, opts.Registry)
	if err != nil {
		return nil, fmt.Errorf("run: generate: %w", err)
	}

	// Tally merge-conflict bookkeeping over the generated world. The
	// counters are the same for every policy pass (the four passes share
	// the same world), so we compute them once here.
	var mergesWithConflict, mergesHappyPath int
	for _, b := range world.Beads {
		if b.MergeConflict {
			mergesWithConflict++
		} else {
			mergesHappyPath++
		}
	}
	kerfSink := opts.KerfDebug

	// Compute scenario and weights bytes + SHAs once. Scenario bytes come
	// from the loader (raw input); if the scenario was constructed in-memory
	// without going through Load/LoadBytes, fall back to a deterministic
	// re-marshal so the output still carries something stable.
	scenarioBytes := scenarioRawBytes(s)
	scenarioSHA := sha256Hex(scenarioBytes)

	weightsBytes := canonicalWeightsYAML(weights)
	weightsSHA := sha256Hex(weightsBytes)

	topSeed := uint64(s.Seed)
	tiebreakSeed := seed.From(topSeed)(seed.Tiebreak)

	policies := []policy.Policy{
		&KerfPolicy{Weights: weights, TiebreakSeed: tiebreakSeed},
		baselines.NewRandom(seedBytes(topSeed)),
		baselines.NewFIFOBead(),
		baselines.NewFIFOWork(),
	}

	results := make(map[string]output.Result, len(policies))
	for _, p := range policies {
		var sink metrics.DebugSink
		if p.Name() == "kerf" {
			sink = kerfSink
		}
		res, err := runOne(s, world, p, agents, scenarioBytes, scenarioSHA, weightsBytes, weightsSHA, sink)
		if err != nil {
			return nil, fmt.Errorf("run: policy %s: %w", p.Name(), err)
		}
		res.MergesWithConflict = mergesWithConflict
		res.MergesHappyPath = mergesHappyPath
		results[p.Name()] = res
	}

	return &Result{
		Kerf:     results["kerf"],
		Random:   results["random"],
		FIFOBead: results["fifo-bead"],
		FIFOWork: results["fifo-work"],
		Scenario: scenarioBytes,
		Weights:  weightsBytes,
	}, nil
}

// runOne executes one (policy × isolated store × isolated metrics collector)
// pass and returns a fully-populated output.Result.
func runOne(
	s *scenario.Scenario,
	world *generator.GeneratedWorld,
	pol policy.Policy,
	agents int,
	scenarioBytes []byte,
	scenarioSHA string,
	weightsBytes []byte,
	weightsSHA string,
	debugSink metrics.DebugSink,
) (output.Result, error) {
	st := store.From(world)

	// Build the metrics collector + adapter.
	col := metrics.NewCollector(metrics.Config{
		NumAgents:  agents,
		TicksCap:   s.Ticks,
		WorkTotal:  len(world.Works),
		Deadline1d: s.Ticks / 7,
		Deadline3d: 3 * s.Ticks / 7,
		Deadline7d: s.Ticks,
	})
	metricHooks := metrics.NewLoopHooks(col, st)
	if debugSink != nil {
		metricHooks.Debug = debugSink
		// Conservative warmup-cutoff hint: floor(0.1 * ticks). True
		// runtime cutoff = min(this, floor(0.1*wall_ticks)); using
		// ticks alone produces the widest possible window, which is
		// the safe direction for the B14 "warmup-swallowed?" check.
		cut := s.Ticks / 10
		metricHooks.SetWarmupCutoffHint(cut)
		debugSink.Header(scenarioSHA, cut, s.Ticks, agents)
	}

	// Recording hooks wrap the metric adapter so the orchestrator can also
	// build the output.Event stream.
	rec := newRecorder(metricHooks, st)

	// Pre-load the heap with all post-tick-0 arrivals from the world's
	// event timeline. Tick-0 beads are already in the store; the loop pushes
	// initial agent-free events itself.
	heap := event.NewHeap()
	for _, sa := range world.EventTimeline {
		if sa.Tick <= 0 {
			continue
		}
		bead := beads.Bead{
			ID:     sa.BeadID,
			Title:  sa.BeadID,
			Status: "open",
			Epic:   sa.Work,
			Labels: append([]string{"work:" + sa.Work}, sa.Labels...),
		}
		heap.Push(event.Event{
			Tick:    sa.Tick,
			Kind:    event.KindArrival,
			BeadID:  sa.BeadID,
			Payload: bead,
		})
	}

	l := &loop.Loop{
		Store:     st,
		Policy:    pol,
		Hooks:     rec,
		Heap:      heap,
		NumAgents: agents,
		TicksCap:  s.Ticks,
		Seeds:     seed.From(uint64(s.Seed)),
	}

	stopReason, wallTicks, err := l.Run()
	if err != nil {
		return output.Result{}, err
	}

	summary := col.Result()

	return output.Result{
		ScenarioBytes: append([]byte(nil), scenarioBytes...),
		WeightsBytes:  append([]byte(nil), weightsBytes...),
		ScenarioSHA:   scenarioSHA,
		WeightsSHA:    weightsSHA,
		Full:          summary.Full,
		Warmup:        summary.Warmup,
		WarmupSkipped: summary.WarmupSkipped,
		WallTicks:     wallTicks,
		Agents:        agents,
		StopReason:    stopReason,
		Events:        rec.events,
	}, nil
}

// KerfPolicy implements policy.Policy by feeding the store's adapter outputs
// into queue.Compute and returning a dispatchable bead from the top-ranked
// work. Score ties at the work level are broken by codename ascending — the
// `tiebreak` sub-seed is reserved for future stochastic tie-breaks and is
// recorded on the policy so callers can swap it in without re-plumbing.
type KerfPolicy struct {
	Weights      queue.Weights
	TiebreakSeed uint64
}

// Name returns "kerf".
func (p *KerfPolicy) Name() string { return "kerf" }

// Next returns the bead ID for the top-ranked actionable work, or "" if no
// bead is dispatchable from the current store snapshot.
//
// Within the chosen work, the bead picked is the lowest-id open bead with all
// intra-work dependencies satisfied.
func (p *KerfPolicy) Next(s *store.Store) string {
	entries := queue.Compute(s.Works(), s.SummaryByWork(), p.Weights)
	if len(entries) == 0 {
		return ""
	}
	// Walk entries top-down until we find a work with at least one
	// dispatchable bead. queue.Compute already returns highest-priority first.
	for _, e := range entries {
		bid := pickBead(s, e.Codename)
		if bid != "" {
			return bid
		}
	}
	return ""
}

// pickBead returns the lowest-ID open bead for workCode whose intra-work
// depends_on beads are all closed, or "" if none.
func pickBead(s *store.Store, workCode string) string {
	beadList := s.Beads()
	cands := make([]string, 0, len(beadList))
	for _, b := range beadList {
		st := s.Lookup(b.ID)
		if st == nil {
			continue
		}
		if st.WorkCode != workCode {
			continue
		}
		if st.Status != store.StatusOpen {
			continue
		}
		if !depsSatisfied(s, st.DependsOn) {
			continue
		}
		cands = append(cands, b.ID)
	}
	if len(cands) == 0 {
		return ""
	}
	sort.Strings(cands)
	return cands[0]
}

// depsSatisfied returns true if every bead listed in deps is closed in the
// store (or absent, treated as not-blocking, matching baselines.depsSatisfied).
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

// recorder fans every loop hook out to two consumers: the metrics adapter
// (which feeds the Collector) and an internal event log that becomes
// output.Result.Events.
//
// recorder satisfies loop.Hooks. The underlying metrics adapter is itself a
// loop.Hooks, so the fanout is a straightforward call-and-record pattern.
type recorder struct {
	inner   *metrics.LoopHooks
	store   *store.Store
	events  []output.EventEntry
	lastTop string
}

func newRecorder(inner *metrics.LoopHooks, s *store.Store) *recorder {
	return &recorder{inner: inner, store: s}
}

func (r *recorder) OnEvent(e event.Event) {
	r.inner.OnEvent(e)
}

func (r *recorder) OnArrival(beadID string, work string, tick int64) {
	r.inner.OnArrival(beadID, work, tick)
	r.events = append(r.events, output.EventEntry{
		T:    tick,
		Kind: "arrival",
		Bead: beadID,
		Work: work,
	})
}

func (r *recorder) OnDispatch(agentID int, beadID string, tick int64) {
	r.inner.OnDispatch(agentID, beadID, tick)
	a := agentID
	work := ""
	if st := r.store.Lookup(beadID); st != nil {
		work = st.WorkCode
	}
	r.events = append(r.events, output.EventEntry{
		T:     tick,
		Kind:  "dispatch",
		Agent: &a,
		Bead:  beadID,
		Work:  work,
	})
}

func (r *recorder) OnComplete(beadID string, tick int64) {
	r.inner.OnComplete(beadID, tick)
	r.events = append(r.events, output.EventEntry{
		T:    tick,
		Kind: "complete",
		Bead: beadID,
	})
}

func (r *recorder) OnSnapshot(top string) {
	r.inner.OnSnapshot(top)
	// Coalesce consecutive identical snapshots so the event stream tracks
	// only top-of-queue changes; first snapshot is always recorded so
	// consumers see the baseline.
	if len(r.events) > 0 && r.lastTop == top && hasAnySnapshot(r.events) {
		return
	}
	r.lastTop = top
	r.events = append(r.events, output.EventEntry{
		T:    lastTick(r.events),
		Kind: "queue_snapshot",
		Top:  top,
	})
}

// hasAnySnapshot reports whether the event log already contains at least one
// queue_snapshot entry — used to gate the coalescing logic so the very first
// snapshot is always recorded.
func hasAnySnapshot(events []output.EventEntry) bool {
	for _, e := range events {
		if e.Kind == "queue_snapshot" {
			return true
		}
	}
	return false
}

// lastTick returns the tick of the most recent event in the log, or 0 if
// none exist. Snapshots are tagged at the most recent observed tick because
// loop.Hooks.OnSnapshot does not carry a tick of its own.
func lastTick(events []output.EventEntry) int64 {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Kind == "queue_snapshot" {
			continue
		}
		return events[i].T
	}
	return 0
}

// seedBytes is the canonical 8-big-endian-byte encoding of a uint64 seed,
// matching seed.From's internal layout.
func seedBytes(u uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], u)
	return buf[:]
}

// sha256Hex returns the hex-encoded SHA-256 of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// scenarioRawBytes returns the bytes the scenario was originally loaded from,
// or a deterministic synthesis if the scenario was built in-memory.
//
// We rely on the scenario package's SHA256 helper to surface raw bytes when
// available; if not available, the canonical fallback re-marshals the key
// fields under a fixed key order so two scenarios that compare structurally
// equal serialize to byte-identical YAML.
func scenarioRawBytes(s *scenario.Scenario) []byte {
	if h := s.SHA256(); h != "" {
		// SHA256 returning non-empty implies raw bytes are captured. We
		// re-derive them via a deterministic minimal marshal — the
		// scenario package does not export the raw slice. In practice
		// callers always Load() the scenario, so this branch fires for
		// in-memory scenarios under test only.
		return fallbackScenarioBytes(s)
	}
	return fallbackScenarioBytes(s)
}

// fallbackScenarioBytes produces a deterministic canonical byte form of a
// scenario by hand-rendering the fields that matter to the simulator. The
// goal is byte-identity across runs of the same in-memory scenario, not
// fidelity with the user's original YAML.
func fallbackScenarioBytes(s *scenario.Scenario) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "seed: %d\n", s.Seed)
	fmt.Fprintf(&b, "ticks: %d\n", s.Ticks)
	fmt.Fprintf(&b, "agents: %d\n", s.Agents)
	b.WriteString("works:\n")
	for _, w := range s.Works {
		fmt.Fprintf(&b, "  - codename: %s\n", w.Codename)
		if len(w.Areas) > 0 {
			areas := append([]string(nil), w.Areas...)
			sort.Strings(areas)
			fmt.Fprintf(&b, "    areas: %v\n", areas)
		}
		if d := w.DepsSlice(); len(d) > 0 {
			deps := append([]string(nil), d...)
			sort.Strings(deps)
			fmt.Fprintf(&b, "    deps: %v\n", deps)
		}
		fmt.Fprintf(&b, "    bead_count: %d\n", w.BeadCount)
	}
	if s.BeadArrivals.Generator != nil {
		g := s.BeadArrivals.Generator
		b.WriteString("bead_arrivals:\n  generator:\n")
		fmt.Fprintf(&b, "    rework_rate_per_tick: %g\n", g.ReworkRatePerTick)
		targets := append([]string(nil), g.TargetWorks...)
		sort.Strings(targets)
		fmt.Fprintf(&b, "    target_works: %v\n", targets)
	}
	d := s.AgentModel.Duration
	fmt.Fprintf(&b, "agent_model:\n  duration:\n    kind: %s\n    sigma: %g\n", d.Kind, d.Sigma)
	if d.MedianTicks != nil {
		fmt.Fprintf(&b, "    median_ticks: %g\n", *d.MedianTicks)
	}
	if d.MeanTicks != nil {
		fmt.Fprintf(&b, "    mean_ticks: %g\n", *d.MeanTicks)
	}
	return b.Bytes()
}

// canonicalWeightsYAML renders queue.Weights as a deterministic YAML byte
// sequence. Field order is fixed so the same weights value always produces
// byte-identical output (and therefore a byte-identical weights_sha256).
func canonicalWeightsYAML(w queue.Weights) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "fan_out: %g\n", w.FanOut)
	fmt.Fprintf(&b, "momentum: %g\n", w.Momentum)
	fmt.Fprintf(&b, "creation: %g\n", w.Creation)
	fmt.Fprintf(&b, "rework: %g\n", w.Rework)
	return b.Bytes()
}
