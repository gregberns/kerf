package duration

import (
	"math"
	"math/rand"
	"testing"
)

// sampleMean returns the empirical mean of n samples from d.
func sampleMean(d Distribution, r *rand.Rand, n int) float64 {
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += d.Sample(r)
	}
	return sum / float64(n)
}

// withinPct returns true if got is within tolerance fraction of want.
func withinPct(got, want, tol float64) bool {
	if want == 0 {
		return math.Abs(got) <= tol
	}
	return math.Abs((got-want)/want) <= tol
}

func TestLogNormal_MeanMatches(t *testing.T) {
	d := LogNormal{Mu: 1.0, Sigma: 0.5}
	r := rand.New(rand.NewSource(1))
	got := sampleMean(d, r, 200000)
	want := d.Mean()
	if !withinPct(got, want, 0.02) {
		t.Fatalf("lognormal mean: got %.4f, want %.4f (±2%%)", got, want)
	}
}

func TestGamma_MeanMatches(t *testing.T) {
	d := Gamma{Shape: 14.67, Scale: 0.235}
	r := rand.New(rand.NewSource(2))
	got := sampleMean(d, r, 200000)
	want := d.Mean()
	if !withinPct(got, want, 0.02) {
		t.Fatalf("gamma mean: got %.4f, want %.4f (±2%%)", got, want)
	}
}

func TestGamma_SubUnitShape(t *testing.T) {
	d := Gamma{Shape: 0.5, Scale: 2.0}
	r := rand.New(rand.NewSource(3))
	got := sampleMean(d, r, 200000)
	want := d.Mean()
	if !withinPct(got, want, 0.03) {
		t.Fatalf("gamma(k<1) mean: got %.4f, want %.4f", got, want)
	}
}

func TestWeibull_MeanMatches(t *testing.T) {
	d := Weibull{Shape: 1.4, Scale: 200.0}
	r := rand.New(rand.NewSource(4))
	got := sampleMean(d, r, 200000)
	want := d.Mean()
	if !withinPct(got, want, 0.03) {
		t.Fatalf("weibull mean: got %.4f, want %.4f", got, want)
	}
}

func TestPointMass_AlwaysReturnsValue(t *testing.T) {
	d := PointMass{Value: 42.0}
	r := rand.New(rand.NewSource(5))
	for i := 0; i < 50; i++ {
		if v := d.Sample(r); v != 42.0 {
			t.Fatalf("point_mass returned %v, want 42", v)
		}
	}
}

func TestMixture_WeightedAverage(t *testing.T) {
	m := NewMixture([]MixtureComponent{
		{Weight: 0.5, Distribution: PointMass{Value: 10}},
		{Weight: 0.5, Distribution: PointMass{Value: 30}},
	})
	r := rand.New(rand.NewSource(6))
	got := sampleMean(m, r, 200000)
	if !withinPct(got, 20.0, 0.02) {
		t.Fatalf("mixture mean: got %.4f, want ~20", got)
	}
}

func TestMixture_HeavyWeight(t *testing.T) {
	// 95% point-mass at 0, 5% point-mass at 100 → mean 5.
	m := NewMixture([]MixtureComponent{
		{Weight: 0.95, Distribution: PointMass{Value: 0}},
		{Weight: 0.05, Distribution: PointMass{Value: 100}},
	})
	r := rand.New(rand.NewSource(7))
	got := sampleMean(m, r, 200000)
	if !withinPct(got, 5.0, 0.10) {
		t.Fatalf("biased mixture mean: got %.4f, want ~5 (±10%%)", got)
	}
}

func TestRegistry_LoadCorpus(t *testing.T) {
	// The committed YAML at the canonical path is the reference. We
	// parse the bytes inline to avoid coupling tests to working-directory.
	yaml := []byte(`
spin_up:
  family: gamma
  params:
    shape: 14.67
    scale: 0.235
task_work:
  family: lognormal
  params:
    mu: 5.33
    sigma: 0.63
merge:
  family: mixture
  components:
    - weight: 0.95
      family: point_mass
      params: { value: 0.0 }
    - weight: 0.05
      family: weibull
      params: { shape: 1.42, scale: 203 }
conflict_resolution:
  family: mixture
  components:
    - weight: 0.19
      family: lognormal
      params: { mu: 1.72, sigma: 0.48 }
    - weight: 0.81
      family: lognormal
      params: { mu: 5.16, sigma: 1.57 }
`)
	reg, err := LoadRegistryBytes(yaml)
	if err != nil {
		t.Fatalf("LoadRegistryBytes: %v", err)
	}
	for _, name := range []string{"spin_up", "task_work", "merge", "conflict_resolution"} {
		if _, ok := reg.Lookup(name); !ok {
			t.Fatalf("registry missing %s", name)
		}
	}
}

func TestRegistry_MissingFile(t *testing.T) {
	reg, err := LoadRegistry("/nonexistent/path/fitted.yaml")
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if reg != nil {
		t.Fatalf("missing file should return nil registry, got %+v", reg)
	}
	// nil-registry lookup is a clean miss.
	if _, ok := reg.Lookup("task_work"); ok {
		t.Fatalf("nil registry should not have entries")
	}
}
