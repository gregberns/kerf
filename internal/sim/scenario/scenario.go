// Package scenario defines the kerfsim scenario YAML schema, a loader, and
// a validator. See specs/simulator.md §Scenario File for the normative
// description.
package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"

	"gopkg.in/yaml.v3"
)

// Scenario is the top-level scenario document.
type Scenario struct {
	Seed         int64        `yaml:"seed"`
	Ticks        int64        `yaml:"ticks"`
	Agents       int          `yaml:"agents"`
	Works        []Work       `yaml:"works"`
	BeadArrivals BeadArrivals `yaml:"bead_arrivals"`
	AgentModel   AgentModel   `yaml:"agent_model"`

	// raw holds the bytes the scenario was loaded from, when known. It is
	// used by SHA256 to compute scenario_sha256 over the canonical source.
	raw []byte `yaml:"-"`
}

// Work is an initial work item.
type Work struct {
	Codename  string   `yaml:"codename"`
	Areas     []string `yaml:"areas"`
	Deps      []string `yaml:"deps"`
	BeadCount int      `yaml:"bead_count"`
}

// BeadArrivals controls the post-start bead arrival schedule. Exactly one of
// Generator or Explicit must be set.
type BeadArrivals struct {
	Generator *Generator         `yaml:"generator,omitempty"`
	Explicit  []ExplicitArrival  `yaml:"explicit,omitempty"`
}

// Generator describes a distribution-driven arrival stream.
type Generator struct {
	ReworkRatePerTick float64  `yaml:"rework_rate_per_tick"`
	TargetWorks       []string `yaml:"target_works"`
}

// ExplicitArrival is one entry in bead_arrivals.explicit.
type ExplicitArrival struct {
	Tick   int      `yaml:"tick"`
	Work   string   `yaml:"work"`
	Labels []string `yaml:"labels,omitempty"`
	BeadID string   `yaml:"bead_id,omitempty"`
}

// AgentModel groups the agent-side knobs of the scenario. Phase 1 carries
// only the duration model.
type AgentModel struct {
	Duration Duration `yaml:"duration"`
}

// Duration describes the bead duration distribution. Phase 1 supports
// kind: "lognormal". Exactly one of MeanTicks or MedianTicks must be set;
// Sigma is the log-normal shape parameter.
type Duration struct {
	Kind        string   `yaml:"kind"`
	MeanTicks   *float64 `yaml:"mean_ticks,omitempty"`
	MedianTicks *float64 `yaml:"median_ticks,omitempty"`
	Sigma       float64  `yaml:"sigma"`
}

// Recognized duration kinds.
const (
	DurationKindLogNormal = "lognormal"
)

// Load reads a scenario YAML file from disk, parses it, validates it, and
// returns the result.
func Load(path string) (*Scenario, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scenario: read %s: %w", path, err)
	}
	return LoadBytes(b)
}

// LoadBytes parses and validates a scenario from raw YAML bytes.
func LoadBytes(b []byte) (*Scenario, error) {
	var s Scenario
	if err := yaml.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("scenario: parse: %w", err)
	}
	s.raw = append([]byte(nil), b...)
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// Validate enforces the rules listed in specs/simulator.md §Validation Rules.
func (s *Scenario) Validate() error {
	if s == nil {
		return fmt.Errorf("scenario: nil")
	}
	if s.Seed <= 0 {
		return fmt.Errorf("scenario: seed must be positive (got %d)", s.Seed)
	}
	if s.Ticks <= 0 {
		return fmt.Errorf("scenario: ticks must be positive (got %d)", s.Ticks)
	}
	if s.Agents < 1 || s.Agents > 10 {
		return fmt.Errorf("scenario: agents must be in [1, 10] (got %d)", s.Agents)
	}
	if len(s.Works) == 0 {
		return fmt.Errorf("scenario: works is required")
	}
	codenames := make(map[string]struct{}, len(s.Works))
	for i, w := range s.Works {
		if w.Codename == "" {
			return fmt.Errorf("scenario: works[%d].codename is required", i)
		}
		if _, dup := codenames[w.Codename]; dup {
			return fmt.Errorf("scenario: works[%d]: duplicate codename %q", i, w.Codename)
		}
		codenames[w.Codename] = struct{}{}
		if w.BeadCount < 0 {
			return fmt.Errorf("scenario: works[%d] (%s): bead_count must be >= 0", i, w.Codename)
		}
	}

	// bead_arrivals: exactly one of generator/explicit.
	hasGen := s.BeadArrivals.Generator != nil
	hasExp := s.BeadArrivals.Explicit != nil
	if hasGen && hasExp {
		return fmt.Errorf("scenario: bead_arrivals: only one of generator or explicit may be set")
	}
	if !hasGen && !hasExp {
		return fmt.Errorf("scenario: bead_arrivals: one of generator or explicit must be set")
	}
	if hasExp {
		for i, ea := range s.BeadArrivals.Explicit {
			if ea.Tick < 0 {
				return fmt.Errorf("scenario: bead_arrivals.explicit[%d]: tick must be >= 0 (got %d)", i, ea.Tick)
			}
			if ea.Work == "" {
				return fmt.Errorf("scenario: bead_arrivals.explicit[%d]: work is required", i)
			}
			if _, ok := codenames[ea.Work]; !ok {
				return fmt.Errorf("scenario: bead_arrivals.explicit[%d]: unknown work codename %q", i, ea.Work)
			}
		}
	}
	if hasGen {
		for i, tw := range s.BeadArrivals.Generator.TargetWorks {
			if _, ok := codenames[tw]; !ok {
				return fmt.Errorf("scenario: bead_arrivals.generator.target_works[%d]: unknown work codename %q", i, tw)
			}
		}
	}

	// agent_model.duration.
	d := s.AgentModel.Duration
	if d.Kind == "" {
		return fmt.Errorf("scenario: agent_model.duration.kind is required")
	}
	switch d.Kind {
	case DurationKindLogNormal:
		// supported
	default:
		return fmt.Errorf("scenario: agent_model.duration.kind %q is not recognized", d.Kind)
	}
	hasMean := d.MeanTicks != nil
	hasMedian := d.MedianTicks != nil
	if hasMean && hasMedian {
		return fmt.Errorf("scenario: agent_model.duration: only one of mean_ticks or median_ticks may be set")
	}
	if !hasMean && !hasMedian {
		return fmt.Errorf("scenario: agent_model.duration: one of mean_ticks or median_ticks must be set")
	}
	if hasMean && *d.MeanTicks <= 0 {
		return fmt.Errorf("scenario: agent_model.duration.mean_ticks must be > 0")
	}
	if hasMedian && *d.MedianTicks <= 0 {
		return fmt.Errorf("scenario: agent_model.duration.median_ticks must be > 0")
	}
	if d.Sigma <= 0 {
		return fmt.Errorf("scenario: agent_model.duration.sigma must be > 0")
	}
	return nil
}

// Mu returns the log-normal location parameter (mu) implied by the duration
// model. For a log-normal distribution with shape sigma:
//
//	median = exp(mu)            => mu = ln(median)
//	mean   = exp(mu + sigma^2/2) => mu = ln(mean) - sigma^2/2
func (d Duration) Mu() (float64, error) {
	if d.Kind != DurationKindLogNormal {
		return 0, fmt.Errorf("scenario: Mu requires kind=%q (got %q)", DurationKindLogNormal, d.Kind)
	}
	switch {
	case d.MedianTicks != nil && d.MeanTicks == nil:
		if *d.MedianTicks <= 0 {
			return 0, fmt.Errorf("scenario: median_ticks must be > 0")
		}
		return math.Log(*d.MedianTicks), nil
	case d.MeanTicks != nil && d.MedianTicks == nil:
		if *d.MeanTicks <= 0 {
			return 0, fmt.Errorf("scenario: mean_ticks must be > 0")
		}
		return math.Log(*d.MeanTicks) - (d.Sigma*d.Sigma)/2.0, nil
	default:
		return 0, fmt.Errorf("scenario: exactly one of mean_ticks or median_ticks must be set")
	}
}

// SHA256 returns the hex-encoded SHA-256 of the raw scenario bytes the
// scenario was loaded from. Returns "" if the scenario was constructed
// in-memory without going through Load/LoadBytes.
func (s *Scenario) SHA256() string {
	if len(s.raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(s.raw)
	return hex.EncodeToString(sum[:])
}

// Float64Ptr is a small helper for tests and programmatic scenario
// construction.
func Float64Ptr(v float64) *float64 { return &v }
