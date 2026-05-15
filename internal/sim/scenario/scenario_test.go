package scenario

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const validGeneratorYAML = `
seed: 42
ticks: 10000
agents: 3
works:
  - codename: amber-fox
    areas: [cli, jig-system]
    deps: []
    bead_count: 8
  - codename: bright-mole
    areas: [bench-storage]
    deps: [amber-fox]
    bead_count: 5
bead_arrivals:
  generator:
    rework_rate_per_tick: 0.002
    target_works: [amber-fox, bright-mole]
agent_model:
  duration:
    kind: lognormal
    median_ticks: 30
    sigma: 0.8
`

const validExplicitYAML = `
seed: 7
ticks: 500
agents: 2
works:
  - codename: amber-fox
    areas: [cli]
    deps: []
    bead_count: 3
bead_arrivals:
  explicit:
    - tick: 1200
      work: amber-fox
      labels: [rework:true]
    - tick: 1500
      work: amber-fox
      bead_id: amber-fox/manual-1
agent_model:
  duration:
    kind: lognormal
    mean_ticks: 40
    sigma: 0.5
`

func TestLoadBytes_GeneratorRoundTrip(t *testing.T) {
	s, err := LoadBytes([]byte(validGeneratorYAML))
	if err != nil {
		t.Fatalf("LoadBytes: %v", err)
	}
	if s.Seed != 42 || s.Ticks != 10000 || s.Agents != 3 {
		t.Fatalf("scalar fields wrong: %+v", s)
	}
	if len(s.Works) != 2 {
		t.Fatalf("expected 2 works, got %d", len(s.Works))
	}
	if s.Works[0].Codename != "amber-fox" || s.Works[0].BeadCount != 8 {
		t.Fatalf("works[0] wrong: %+v", s.Works[0])
	}
	if s.Works[1].Deps[0] != "amber-fox" {
		t.Fatalf("works[1].deps wrong: %+v", s.Works[1])
	}
	if s.BeadArrivals.Generator == nil {
		t.Fatalf("expected generator set")
	}
	if s.BeadArrivals.Generator.ReworkRatePerTick != 0.002 {
		t.Fatalf("rework_rate_per_tick wrong: %v", s.BeadArrivals.Generator.ReworkRatePerTick)
	}
	if s.BeadArrivals.Explicit != nil {
		t.Fatalf("explicit should be nil")
	}
	if s.AgentModel.Duration.Kind != "lognormal" {
		t.Fatalf("kind wrong: %q", s.AgentModel.Duration.Kind)
	}
	if s.AgentModel.Duration.MedianTicks == nil || *s.AgentModel.Duration.MedianTicks != 30 {
		t.Fatalf("median_ticks wrong")
	}
	if s.SHA256() == "" {
		t.Fatalf("expected non-empty SHA256")
	}
}

func TestLoadBytes_ExplicitRoundTrip(t *testing.T) {
	s, err := LoadBytes([]byte(validExplicitYAML))
	if err != nil {
		t.Fatalf("LoadBytes: %v", err)
	}
	if s.BeadArrivals.Generator != nil {
		t.Fatalf("expected generator nil")
	}
	if len(s.BeadArrivals.Explicit) != 2 {
		t.Fatalf("expected 2 explicit arrivals, got %d", len(s.BeadArrivals.Explicit))
	}
	if s.BeadArrivals.Explicit[0].Tick != 1200 || s.BeadArrivals.Explicit[0].Work != "amber-fox" {
		t.Fatalf("explicit[0] wrong: %+v", s.BeadArrivals.Explicit[0])
	}
	if s.BeadArrivals.Explicit[0].Labels[0] != "rework:true" {
		t.Fatalf("labels wrong: %+v", s.BeadArrivals.Explicit[0].Labels)
	}
	if s.BeadArrivals.Explicit[1].BeadID != "amber-fox/manual-1" {
		t.Fatalf("bead_id wrong: %+v", s.BeadArrivals.Explicit[1])
	}
}

func TestLoad_FromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scenario.yaml")
	if err := os.WriteFile(path, []byte(validGeneratorYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Seed != 42 {
		t.Fatalf("seed wrong: %d", s.Seed)
	}
}

func TestUnknownKeysAreTolerated(t *testing.T) {
	y := validGeneratorYAML + `
extra_top_level_key: hello
`
	if _, err := LoadBytes([]byte(y)); err != nil {
		t.Fatalf("unknown top-level keys should be tolerated: %v", err)
	}
}

// --- Validation rejection cases ---

func mustParse(t *testing.T, src string) *Scenario {
	t.Helper()
	// Parse without validating so we can mutate before validation.
	var s Scenario
	// reuse the loader-with-validate flow but capture the parsed form via
	// LoadBytes on a valid baseline then mutate; here, parse the YAML and
	// skip validation manually.
	if err := yamlUnmarshalForTest([]byte(src), &s); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return &s
}

// yamlUnmarshalForTest parses YAML into a Scenario without invoking
// Validate, so tests can mutate the struct before exercising validation.
func yamlUnmarshalForTest(b []byte, out *Scenario) error {
	return yaml.Unmarshal(b, out)
}

func TestValidate_AgentsBounds(t *testing.T) {
	cases := []struct {
		name   string
		agents int
		ok     bool
	}{
		{"zero", 0, false},
		{"one", 1, true},
		{"ten", 10, true},
		{"eleven", 11, false},
		{"negative", -1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := mustParse(t, validGeneratorYAML)
			s.Agents = c.agents
			err := s.Validate()
			if c.ok && err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if !c.ok && err == nil {
				t.Fatalf("expected error for agents=%d", c.agents)
			}
		})
	}
}

func TestValidate_BothArrivalFormsSet(t *testing.T) {
	s := mustParse(t, validGeneratorYAML)
	s.BeadArrivals.Explicit = []ExplicitArrival{{Tick: 100, Work: "amber-fox"}}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "only one of generator or explicit") {
		t.Fatalf("expected both-set error, got %v", err)
	}
}

func TestValidate_NeitherArrivalFormSet(t *testing.T) {
	s := mustParse(t, validGeneratorYAML)
	s.BeadArrivals = BeadArrivals{}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "one of generator or explicit must be set") {
		t.Fatalf("expected neither-set error, got %v", err)
	}
}

func TestValidate_UnknownDurationKind(t *testing.T) {
	s := mustParse(t, validGeneratorYAML)
	s.AgentModel.Duration.Kind = "weibull"
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "not recognized") {
		t.Fatalf("expected unknown-kind error, got %v", err)
	}
}

func TestValidate_MeanAndMedianBothSet(t *testing.T) {
	s := mustParse(t, validGeneratorYAML)
	s.AgentModel.Duration.MeanTicks = Float64Ptr(40)
	// MedianTicks already set in fixture
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "only one of mean_ticks or median_ticks") {
		t.Fatalf("expected both-set error, got %v", err)
	}
}

func TestValidate_NeitherMeanNorMedianSet(t *testing.T) {
	s := mustParse(t, validGeneratorYAML)
	s.AgentModel.Duration.MeanTicks = nil
	s.AgentModel.Duration.MedianTicks = nil
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "one of mean_ticks or median_ticks must be set") {
		t.Fatalf("expected neither-set error, got %v", err)
	}
}

func TestValidate_ExplicitUnknownCodename(t *testing.T) {
	s := mustParse(t, validExplicitYAML)
	s.BeadArrivals.Explicit = append(s.BeadArrivals.Explicit, ExplicitArrival{
		Tick: 200, Work: "ghost-werewolf",
	})
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "unknown work codename") {
		t.Fatalf("expected unknown-codename error, got %v", err)
	}
}

func TestValidate_GeneratorUnknownTargetWork(t *testing.T) {
	s := mustParse(t, validGeneratorYAML)
	s.BeadArrivals.Generator.TargetWorks = []string{"ghost-werewolf"}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "unknown work codename") {
		t.Fatalf("expected unknown-target-work error, got %v", err)
	}
}

func TestValidate_NegativeExplicitTick(t *testing.T) {
	s := mustParse(t, validExplicitYAML)
	s.BeadArrivals.Explicit[0].Tick = -1
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "tick must be >= 0") {
		t.Fatalf("expected negative-tick error, got %v", err)
	}
}

func TestValidate_SeedAndTicksMustBePositive(t *testing.T) {
	s := mustParse(t, validGeneratorYAML)
	s.Seed = 0
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for seed=0")
	}
	s = mustParse(t, validGeneratorYAML)
	s.Ticks = 0
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for ticks=0")
	}
}

func TestValidate_DuplicateCodename(t *testing.T) {
	s := mustParse(t, validGeneratorYAML)
	s.Works = append(s.Works, Work{Codename: "amber-fox", BeadCount: 1})
	if err := s.Validate(); err == nil {
		t.Fatal("expected duplicate-codename error")
	}
}

// --- Conversion helpers ---

func TestDuration_Mu_FromMedian(t *testing.T) {
	d := Duration{Kind: DurationKindLogNormal, MedianTicks: Float64Ptr(30), Sigma: 0.5}
	mu, err := d.Mu()
	if err != nil {
		t.Fatalf("Mu: %v", err)
	}
	want := math.Log(30)
	if math.Abs(mu-want) > 1e-12 {
		t.Fatalf("mu from median: got %v want %v", mu, want)
	}
}

func TestDuration_Mu_FromMean(t *testing.T) {
	sigma := 0.5
	mean := 40.0
	d := Duration{Kind: DurationKindLogNormal, MeanTicks: Float64Ptr(mean), Sigma: sigma}
	mu, err := d.Mu()
	if err != nil {
		t.Fatalf("Mu: %v", err)
	}
	want := math.Log(mean) - (sigma*sigma)/2.0
	if math.Abs(mu-want) > 1e-12 {
		t.Fatalf("mu from mean: got %v want %v", mu, want)
	}
}

func TestDuration_Mu_RequiresLogNormal(t *testing.T) {
	d := Duration{Kind: "weibull", MedianTicks: Float64Ptr(30), Sigma: 0.5}
	if _, err := d.Mu(); err == nil {
		t.Fatal("expected error for non-lognormal kind")
	}
}
