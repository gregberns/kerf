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
//   2. priority_inversions had a latent semantic issue independent of
//      saturation: the original spec defined "older" by arrival tick, but
//      generator-rework arrives strictly AFTER initial new-work beads
//      (which all arrive at tick 0), so a rework bead could never be
//      "older" than an initial new-work bead by that definition. The
//      metric was structurally unreachable.
//
//      Plan 011 / pillar E fixes this: per specs/simulator.md §Metric
//      Definitions the metric now counts every new-work dispatch that
//      occurs while ANY rework bead is queue-eligible — no arrival-tick
//      comparison. Under a saturated random baseline, contention between
//      new-work and rework is guaranteed, so priority_inversions must be
//      strictly positive.
//
// This test asserts both:
//   - the wait pipeline (rework_p{50,95}_wait > 0 under saturation), and
//   - the priority_inversions metric is no longer structurally zero.
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
	// Plan 011 / pillar E regression: priority_inversions must be > 0 on
	// the saturated scenario. Under random ordering with both rework and
	// new-work concurrently eligible, the policy will frequently pick
	// new-work while rework remains in the queue. A zero here would mean
	// either the metric definition regressed back to the unreachable
	// arrival-tick gate, or the eligibility lookup is broken.
	if res.Random.Full.PriorityInversions == 0 {
		t.Errorf("random.priority_inversions = 0 on saturated scenario; expected > 0 (Plan 011 / E — see specs/simulator.md §Metric Definitions and plans/008_exploratory_testing/sim_scenarios/diagnosis.md)")
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

// mergeModelScenario builds a small scenario with an explicit merge_model
// so the Plan 012 Pillar-C bookkeeping has something to count. confProb
// is the per-bead conflict probability; conflictMean is the mean of the
// (point-mass) conflict-duration draw used to make the contribution
// detectable.
func mergeModelScenario(t *testing.T, confProb float64) *scenario.Scenario {
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
			BeadCount: 40,
		}},
		BeadArrivals: scenario.BeadArrivals{Generator: &scenario.Generator{
			ReworkRatePerTick: 0,
			TargetWorks:       []string{"a"},
		}},
		AgentModel: scenario.AgentModel{Duration: scenario.Duration{
			Kind:        scenario.DurationKindLogNormal,
			MedianTicks: &med,
			Sigma:       0.5,
		}},
		MergeModel: &scenario.MergeModel{
			ConflictProbability: confProb,
			ConflictDuration: &scenario.Duration{
				Kind:  scenario.DurationKindPointMass,
				Value: 100.0,
			},
		},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("scenario invalid: %v", err)
	}
	return s
}

// TestRun_MergeModel_AllConflict asserts that conflict_probability=1.0
// marks every bead as a merge-conflict bead and bumps the total
// effective duration well above the bare task draw.
func TestRun_MergeModel_AllConflict(t *testing.T) {
	s := mergeModelScenario(t, 1.0)
	r, err := Run(s, queue.DefaultWeights(), s.Agents)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Kerf.MergesWithConflict == 0 {
		t.Fatal("expected at least one conflict bead at p=1.0")
	}
	if r.Kerf.MergesHappyPath != 0 {
		t.Errorf("expected zero happy-path merges at p=1.0, got %d", r.Kerf.MergesHappyPath)
	}
	// All four policies share the same world → same conflict bookkeeping.
	for name, blk := range map[string]output.Result{
		"random":    r.Random,
		"fifo-bead": r.FIFOBead,
		"fifo-work": r.FIFOWork,
	} {
		if blk.MergesWithConflict != r.Kerf.MergesWithConflict {
			t.Errorf("policy %s: merges_with_conflict %d, want %d", name, blk.MergesWithConflict, r.Kerf.MergesWithConflict)
		}
	}
}

// TestRun_MergeModel_NoConflict asserts that conflict_probability=0.0
// leaves every bead on the happy path and produces zero conflict beads.
func TestRun_MergeModel_NoConflict(t *testing.T) {
	s := mergeModelScenario(t, 0.0)
	r, err := Run(s, queue.DefaultWeights(), s.Agents)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Kerf.MergesWithConflict != 0 {
		t.Errorf("expected zero conflict beads at p=0.0, got %d", r.Kerf.MergesWithConflict)
	}
	if r.Kerf.MergesHappyPath == 0 {
		t.Fatal("expected non-zero happy-path merges at p=0.0")
	}
}

// TestRun_MergeModel_ContributionIncreasesDuration asserts that adding a
// non-trivial conflict-duration draw measurably increases the simulator's
// effective wall_ticks compared to a baseline run with the same seed and
// no merge_model.
func TestRun_MergeModel_ContributionIncreasesDuration(t *testing.T) {
	base := mergeModelScenario(t, 0.0)
	base.MergeModel = nil // strip entirely so the legacy fast-path is used
	rBase, err := Run(base, queue.DefaultWeights(), base.Agents)
	if err != nil {
		t.Fatalf("base Run: %v", err)
	}

	heavy := mergeModelScenario(t, 1.0)
	rHeavy, err := Run(heavy, queue.DefaultWeights(), heavy.Agents)
	if err != nil {
		t.Fatalf("heavy Run: %v", err)
	}
	if rHeavy.Kerf.WallTicks <= rBase.Kerf.WallTicks {
		t.Errorf("expected heavier merge model to lengthen wall_ticks: base=%d heavy=%d",
			rBase.Kerf.WallTicks, rHeavy.Kerf.WallTicks)
	}
}
