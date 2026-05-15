package run

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gberns/kerf/internal/queue"
	"github.com/gberns/kerf/internal/sim/output"
	"github.com/gberns/kerf/internal/sim/scenario"
)

// saturatedReworkScenario constructs a scenario with intentionally tight
// agent saturation and a high rework arrival rate, so the queue is
// expected to hold at least one rework and one new-work bead concurrently
// for many ticks. Used by TestRun_BaselineRandom_ProducesInversions (B14
// regression).
//
// The shape: 1 work, 200 initial beads, 2 agents, very short median
// duration, and a 10%/tick rework spawn. Under random ordering, the agent
// dispatches uniformly across the eligible set, so with a rework bead in
// the queue alongside new-work beads, the probability that random picks a
// new-work bead is high → priority_inversions must accumulate.
func saturatedReworkScenario(t *testing.T) *scenario.Scenario {
	t.Helper()
	med := 5.0
	s := &scenario.Scenario{
		Seed:   42,
		Ticks:  2000,
		Agents: 2,
		Works: []scenario.Work{{
			Codename:  "a",
			Areas:     []string{"x"},
			Deps:      nil,
			BeadCount: 200,
		}},
		BeadArrivals: scenario.BeadArrivals{Generator: &scenario.Generator{
			ReworkRatePerTick: 0.1,
			TargetWorks:       []string{"a"},
		}},
		AgentModel: scenario.AgentModel{Duration: scenario.Duration{
			Kind:        scenario.DurationKindLogNormal,
			MedianTicks: &med,
			Sigma:       0.5,
		}},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("scenario invalid: %v", err)
	}
	return s
}

// findRepoRoot walks up from the test's working directory until it finds the
// repo root (the directory containing go.mod). Used to locate the canned
// scenario files without hard-coding a relative path.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found walking up from working dir")
		}
		dir = parent
	}
}

// loadSmallLinear loads the canned small-linear scenario from the repo's
// /scenarios directory.
func loadSmallLinear(t *testing.T) *scenario.Scenario {
	t.Helper()
	root := findRepoRoot(t)
	s, err := scenario.Load(filepath.Join(root, "scenarios", "small-linear.yaml"))
	if err != nil {
		t.Fatalf("load small-linear: %v", err)
	}
	return s
}

// hasDispatch reports whether the given event stream contains at least one
// dispatch event.
func hasDispatch(events []output.EventEntry) bool {
	for _, e := range events {
		if e.Kind == "dispatch" {
			return true
		}
	}
	return false
}

// dispatchSequence projects an event log to the ordered list of dispatched
// bead IDs.
func dispatchSequence(events []output.EventEntry) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		if e.Kind == "dispatch" {
			out = append(out, e.Bead)
		}
	}
	return out
}

// TestRun_AllPoliciesProduceResults runs the orchestrator on the small-linear
// scenario and confirms each of the four policies produced a non-empty result
// with a recognized stop reason.
func TestRun_AllPoliciesProduceResults(t *testing.T) {
	s := loadSmallLinear(t)
	res, err := Run(s, queue.DefaultWeights(), 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	cases := []struct {
		name string
		r    output.Result
	}{
		{"kerf", res.Kerf},
		{"random", res.Random},
		{"fifo-bead", res.FIFOBead},
		{"fifo-work", res.FIFOWork},
	}
	for _, c := range cases {
		switch c.r.StopReason {
		case "all-closed", "ticks-cap", "idle-threshold":
			// recognized
		default:
			t.Errorf("%s: stop reason %q is not one of the spec-defined values", c.name, c.r.StopReason)
		}
		if !hasDispatch(c.r.Events) {
			t.Errorf("%s: no dispatch events recorded", c.name)
		}
		if c.r.ScenarioSHA == "" {
			t.Errorf("%s: scenario_sha256 empty", c.name)
		}
		if c.r.WeightsSHA == "" {
			t.Errorf("%s: weights_sha256 empty", c.name)
		}
		if c.r.Agents <= 0 {
			t.Errorf("%s: agents = %d, want > 0", c.name, c.r.Agents)
		}
	}

	if len(res.Scenario) == 0 {
		t.Error("Scenario bytes empty")
	}
	if len(res.Weights) == 0 {
		t.Error("Weights bytes empty")
	}
}

// TestRun_Determinism asserts that two Run() invocations with byte-identical
// inputs produce deep-equal Results. This is the load-bearing determinism
// property of the simulator.
func TestRun_Determinism(t *testing.T) {
	s1 := loadSmallLinear(t)
	s2 := loadSmallLinear(t)
	w := queue.DefaultWeights()

	a, err := Run(s1, w, 0)
	if err != nil {
		t.Fatalf("Run #1: %v", err)
	}
	b, err := Run(s2, w, 0)
	if err != nil {
		t.Fatalf("Run #2: %v", err)
	}

	if !reflect.DeepEqual(a.Kerf, b.Kerf) {
		t.Errorf("kerf result diverges between runs")
	}
	if !reflect.DeepEqual(a.Random, b.Random) {
		t.Errorf("random result diverges between runs")
	}
	if !reflect.DeepEqual(a.FIFOBead, b.FIFOBead) {
		t.Errorf("fifo-bead result diverges between runs")
	}
	if !reflect.DeepEqual(a.FIFOWork, b.FIFOWork) {
		t.Errorf("fifo-work result diverges between runs")
	}
	if !reflect.DeepEqual(a.Scenario, b.Scenario) {
		t.Errorf("scenario bytes diverge between runs")
	}
	if !reflect.DeepEqual(a.Weights, b.Weights) {
		t.Errorf("weights bytes diverge between runs")
	}
}

// TestRun_MutationIsolation asserts that two different policies under the
// same world produce different observable outputs — confirming that each
// policy ran against its own store with its own dispatch decisions. Kerf
// and random necessarily disagree on order on any non-trivial input.
func TestRun_MutationIsolation(t *testing.T) {
	s := loadSmallLinear(t)
	res, err := Run(s, queue.DefaultWeights(), 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	kerfDispatches := dispatchSequence(res.Kerf.Events)
	randDispatches := dispatchSequence(res.Random.Events)
	if reflect.DeepEqual(kerfDispatches, randDispatches) {
		t.Errorf("kerf and random produced identical dispatch sequences — isolation or policy logic suspect")
	}
}

// TestRun_BaselineRandom_ProducesInversions is the B14 regression signal:
// on a saturated scenario where the queue concurrently holds rework and
// new-work beads, the metric pipeline must report nonzero rework wait
// times.
//
// Background (plans/008_exploratory_testing/sim_scenarios/diagnosis.md):
// the canned and Plan-008 synthetic scenarios reported
// priority_inversions = 0 and rework_p{50,95}_wait = 0 across every
// policy. The diagnosis was twofold:
//
//   1. Under-saturation: scenarios ran with agent_idle_pct ≥ 0.79, so
//      rework arrivals landed on already-idle agents and were dispatched
//      in the same tick — yielding zero wait by construction. A saturated
//      scenario (≤ 0.01 idle) does record real wait times, proving the
//      metric pipeline is correct.
//
//   2. priority_inversions has a latent semantic issue independent of
//      saturation: the spec defines "older" by arrival tick, but
//      generator-rework arrives strictly AFTER initial new-work beads
//      (which all arrive at tick 0), so a rework bead can never be
//      "older" than an initial new-work bead by this definition. This
//      makes the metric structurally unreachable when initial beads
//      dominate. Tracked as a follow-up bead (see diagnosis.md).
//
// This test asserts the wait pipeline (which is fully functional) on a
// saturated shape. priority_inversions is intentionally NOT asserted —
// the follow-up bead owns that question.
func TestRun_BaselineRandom_ProducesInversions(t *testing.T) {
	s := saturatedReworkScenario(t)
	res, err := Run(s, queue.DefaultWeights(), 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Saturation precondition: the test only signals correctly when the
	// scenario actually saturates agents. Fail loudly if not.
	if res.Random.Full.AgentIdlePct > 0.2 {
		t.Fatalf("test precondition violated: agent_idle_pct=%.3f > 0.2 — scenario is not saturated, test cannot diagnose the metric",
			res.Random.Full.AgentIdlePct)
	}
	// Under a saturated random baseline, rework beads necessarily wait
	// in queue. Zero rework wait here would indicate the wait pipeline
	// is broken — re-open B14.
	if res.Random.Full.ReworkP95Wait == 0 {
		t.Errorf("random.rework_p95_wait = 0 on saturated scenario; rework arrivals appear to never wait — metric pipeline regression")
	}
	if res.Random.Full.ReworkP50Wait == 0 {
		t.Errorf("random.rework_p50_wait = 0 on saturated scenario; majority of rework arrivals appear to never wait — metric pipeline regression")
	}
}

// TestRun_WallTicksParity confirms that each policy's recorded wall_ticks is
// at least the maximum tick observed in its event stream — wall_ticks tracks
// the latest tick reached by the loop (specs/simulator.md §Metrics).
func TestRun_WallTicksParity(t *testing.T) {
	s := loadSmallLinear(t)
	res, err := Run(s, queue.DefaultWeights(), 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	cases := []struct {
		name string
		r    output.Result
	}{
		{"kerf", res.Kerf},
		{"random", res.Random},
		{"fifo-bead", res.FIFOBead},
		{"fifo-work", res.FIFOWork},
	}
	for _, c := range cases {
		var maxT int64
		for _, e := range c.r.Events {
			if e.T > maxT {
				maxT = e.T
			}
		}
		if c.r.WallTicks < maxT {
			t.Errorf("%s: wall_ticks=%d < max event tick=%d", c.name, c.r.WallTicks, maxT)
		}
	}
}
