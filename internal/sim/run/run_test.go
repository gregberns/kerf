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
