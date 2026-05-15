package generator

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/gberns/kerf/internal/sim/scenario"
)

// makeSyntheticScenario builds a scenario with N works carrying no
// explicit deps and bead_count == 0, forcing the generator to synthesize
// the DAG and the per-work bead counts. Used by the structural tests.
func makeSyntheticScenario(t *testing.T, seed int64, n int, withRework bool) *scenario.Scenario {
	t.Helper()
	works := make([]scenario.Work, n)
	codenames := make([]string, n)
	for i := 0; i < n; i++ {
		cn := letterCode(i)
		codenames[i] = cn
		works[i] = scenario.Work{
			Codename:  cn,
			Areas:     []string{"area-a"},
			Deps:      nil,
			BeadCount: 0,
		}
	}
	med := 30.0
	var arrivals scenario.BeadArrivals
	if withRework {
		arrivals.Generator = &scenario.Generator{
			ReworkRatePerTick: 0.01,
			TargetWorks:       codenames,
		}
	} else {
		arrivals.Generator = &scenario.Generator{
			ReworkRatePerTick: 0,
			TargetWorks:       codenames,
		}
	}
	s := &scenario.Scenario{
		Seed:         seed,
		Ticks:        2000,
		Agents:       3,
		Works:        works,
		BeadArrivals: arrivals,
		AgentModel: scenario.AgentModel{
			Duration: scenario.Duration{
				Kind:        scenario.DurationKindLogNormal,
				MedianTicks: &med,
				Sigma:       0.8,
			},
		},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("test scenario invalid: %v", err)
	}
	return s
}

// letterCode produces canonical codenames "w001", "w002", … so canonical
// sort order is the same as construction order — useful for asserting
// edge direction (deps point to lower-indexed works only).
func letterCode(i int) string {
	// zero-padded so lexicographic order matches numeric order up to 999.
	return zeroPad("w", i+1, 3)
}

func zeroPad(prefix string, n, width int) string {
	s := []byte{}
	for _, c := range prefix {
		s = append(s, byte(c))
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	for len(digits) < width {
		digits = append([]byte{'0'}, digits...)
	}
	return string(append(s, digits...))
}

// stableSerialize produces a canonical JSON of the GeneratedWorld for
// deep-equality checks across runs. JSON-with-sorted-keys is sufficient
// because every slice in GeneratedWorld is itself sorted canonically.
func stableSerialize(t *testing.T, w *GeneratedWorld) string {
	t.Helper()
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestGenerate_Determinism(t *testing.T) {
	s := makeSyntheticScenario(t, 42, 20, true)
	a, err := Generate(s)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	b, err := Generate(s)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	sa := stableSerialize(t, a)
	sb := stableSerialize(t, b)
	if sa != sb {
		t.Fatalf("same scenario produced different worlds:\nA=%s\nB=%s", sa, sb)
	}
}

func TestGenerate_DifferentSeedsDiffer(t *testing.T) {
	a, err := Generate(makeSyntheticScenario(t, 1, 20, true))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(makeSyntheticScenario(t, 2, 20, true))
	if err != nil {
		t.Fatal(err)
	}
	if stableSerialize(t, a) == stableSerialize(t, b) {
		t.Fatalf("different seeds produced identical worlds")
	}
}

func TestGenerate_DAGAcyclic(t *testing.T) {
	for _, seed := range []int64{1, 7, 42, 99, 12345} {
		s := makeSyntheticScenario(t, seed, 30, false)
		w, err := Generate(s)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if err := assertAcyclic(w.Works); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		// Belt-and-braces: every dep must reference a lower-indexed work.
		idx := map[string]int{}
		for i, ww := range w.Works {
			idx[ww.Codename] = i
		}
		for i, ww := range w.Works {
			for _, d := range ww.Deps {
				if idx[d] >= i {
					t.Fatalf("seed %d: work %s depends on non-older %s (idx %d >= %d)",
						seed, ww.Codename, d, idx[d], i)
				}
			}
		}
	}
}

func TestGenerate_EpicCountInRange(t *testing.T) {
	for _, seed := range []int64{1, 2, 3, 4, 5, 99, 1000} {
		w, err := Generate(makeSyntheticScenario(t, seed, 30, false))
		if err != nil {
			t.Fatal(err)
		}
		max := 0
		for _, ww := range w.Works {
			if ww.Epic > max {
				max = ww.Epic
			}
		}
		k := max + 1 // 0-indexed
		if k < 3 || k > 6 {
			t.Fatalf("seed %d: epic count %d outside [3,6]", seed, k)
		}
	}
}

func TestGenerate_BeadCountLogNormalShape(t *testing.T) {
	// Sanity check: across many works the mean bead count should fall
	// somewhere near the log-normal mean exp(mu + sigma^2/2) with mu = ln(12),
	// sigma = 0.6 → exp(ln(12) + 0.18) ≈ 14.36. Loose bounds, single-run.
	w, err := Generate(makeSyntheticScenario(t, 7, 200, false))
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, ww := range w.Works {
		total += ww.BeadCount
	}
	mean := float64(total) / float64(len(w.Works))
	expected := math.Exp(math.Log(12) + 0.6*0.6/2)
	if mean < expected*0.7 || mean > expected*1.4 {
		t.Fatalf("bead count mean %.2f outside loose bounds of expected %.2f", mean, expected)
	}
}

func TestGenerate_ProbabilisticEventsReproducible(t *testing.T) {
	// Two generations of the same scenario must produce the same rework
	// event timeline (the events sub-seed is deterministic).
	s := makeSyntheticScenario(t, 123, 5, true)
	a, err := Generate(s)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.EventTimeline) != len(b.EventTimeline) {
		t.Fatalf("timeline length differs: %d vs %d", len(a.EventTimeline), len(b.EventTimeline))
	}
	for i := range a.EventTimeline {
		ai, bi := a.EventTimeline[i], b.EventTimeline[i]
		if ai.Tick != bi.Tick || ai.BeadID != bi.BeadID || ai.Work != bi.Work {
			t.Fatalf("timeline[%d] differs: %+v vs %+v", i, ai, bi)
		}
		if len(ai.Labels) != len(bi.Labels) {
			t.Fatalf("timeline[%d] labels differ: %v vs %v", i, ai.Labels, bi.Labels)
		}
		for k := range ai.Labels {
			if ai.Labels[k] != bi.Labels[k] {
				t.Fatalf("timeline[%d] labels[%d] differ: %s vs %s", i, k, ai.Labels[k], bi.Labels[k])
			}
		}
	}
	// And at the specified rate (0.01) over 2000 ticks the timeline
	// should be non-trivially populated.
	if len(a.EventTimeline) == 0 {
		t.Fatalf("expected some rework arrivals at rate 0.01 over 2000 ticks")
	}
}

func TestGenerate_DurationsPreRolled(t *testing.T) {
	w, err := Generate(makeSyntheticScenario(t, 42, 10, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Beads) == 0 {
		t.Fatalf("expected beads")
	}
	for _, b := range w.Beads {
		if b.Duration < 1 {
			t.Fatalf("bead %s has non-positive duration %d", b.BeadID, b.Duration)
		}
	}
}

func TestGenerate_RespectsExplicitDepsAndBeadCounts(t *testing.T) {
	med := 30.0
	s := &scenario.Scenario{
		Seed:   42,
		Ticks:  1000,
		Agents: 3,
		Works: []scenario.Work{
			{Codename: "a", Areas: []string{"cli"}, BeadCount: 4},
			{Codename: "b", Areas: []string{"cli"}, Deps: []string{"a"}, BeadCount: 2},
		},
		BeadArrivals: scenario.BeadArrivals{
			Generator: &scenario.Generator{ReworkRatePerTick: 0, TargetWorks: []string{"a"}},
		},
		AgentModel: scenario.AgentModel{
			Duration: scenario.Duration{Kind: scenario.DurationKindLogNormal, MedianTicks: &med, Sigma: 0.8},
		},
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	w, err := Generate(s)
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Works[0].BeadCount; got != 4 {
		t.Fatalf("work a bead_count = %d, want 4", got)
	}
	if got := w.Works[1].BeadCount; got != 2 {
		t.Fatalf("work b bead_count = %d, want 2", got)
	}
	if len(w.Works[1].Deps) != 1 || w.Works[1].Deps[0] != "a" {
		t.Fatalf("work b deps = %v, want [a]", w.Works[1].Deps)
	}
	// 4 + 2 initial beads, all at tick 0.
	if len(w.Beads) != 6 {
		t.Fatalf("expected 6 beads, got %d", len(w.Beads))
	}
}

func TestGenerate_ExplicitArrivalsAppearOnTimeline(t *testing.T) {
	med := 30.0
	s := &scenario.Scenario{
		Seed:   42,
		Ticks:  1000,
		Agents: 3,
		Works: []scenario.Work{
			{Codename: "a", Areas: []string{"cli"}, BeadCount: 2},
		},
		BeadArrivals: scenario.BeadArrivals{
			Explicit: []scenario.ExplicitArrival{
				{Tick: 100, Work: "a", Labels: []string{"rework:true"}},
				{Tick: 300, Work: "a", BeadID: "a/custom"},
			},
		},
		AgentModel: scenario.AgentModel{
			Duration: scenario.Duration{Kind: scenario.DurationKindLogNormal, MedianTicks: &med, Sigma: 0.8},
		},
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	w, err := Generate(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.EventTimeline) != 2 {
		t.Fatalf("expected 2 timeline entries, got %d", len(w.EventTimeline))
	}
	if w.EventTimeline[0].Tick != 100 || w.EventTimeline[1].Tick != 300 {
		t.Fatalf("timeline ticks = %v, want [100,300]", []int64{w.EventTimeline[0].Tick, w.EventTimeline[1].Tick})
	}
	found := false
	for _, b := range w.Beads {
		if b.BeadID == "a/custom" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected explicit bead_id a/custom to appear in beads list")
	}
}
