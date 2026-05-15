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

	// MergeModel is the optional synthetic conflict-merge model. When
	// non-nil, the run loop adds a merge-phase contribution to every
	// bead's effective completion tick: with probability
	// ConflictProbability the contribution is a draw from
	// ConflictDuration; otherwise it is a draw from BaseDuration. See
	// specs/simulator.md §Merge Model.
	MergeModel *MergeModel `yaml:"merge_model,omitempty"`

	// raw holds the bytes the scenario was loaded from, when known. It is
	// used by SHA256 to compute scenario_sha256 over the canonical source.
	raw []byte `yaml:"-"`
}

// Work is an initial work item.
//
// Deps is a pointer to a slice so that the scenario loader can distinguish
// "the deps key was omitted" (nil pointer — generator may synthesize
// older-sibling edges) from "the deps key was present and empty"
// (non-nil pointer to a zero-length slice — generator must respect it
// verbatim). See specs/simulator.md §Scenario File and the generator's
// generateDeps for the consumer side of this contract.
type Work struct {
	Codename  string    `yaml:"codename"`
	Areas     []string  `yaml:"areas"`
	Deps      *[]string `yaml:"deps,omitempty"`
	BeadCount int       `yaml:"bead_count"`
}

// DepsSlice returns the work's declared deps as a plain slice. A nil
// pointer (deps key absent) and a non-nil empty slice (deps: []) both
// return an empty slice from this accessor; callers that need to
// distinguish the two cases should inspect w.Deps directly.
func (w Work) DepsSlice() []string {
	if w.Deps == nil {
		return nil
	}
	return *w.Deps
}

// DepsPtr is a small helper for tests and programmatic scenario
// construction that need to set an explicit (possibly empty) deps slice.
func DepsPtr(deps []string) *[]string { return &deps }

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
// only the duration model; Plan 012 adds an optional per-phase spin_up.
type AgentModel struct {
	Duration Duration  `yaml:"duration"`
	SpinUp   *Duration `yaml:"spin_up,omitempty"`
}

// Duration describes a per-phase duration distribution. Phase 1 supports
// kind: "lognormal" (inline mu/sigma via mean_ticks/median_ticks). Plan
// 012 adds kind: "from_distribution" — a named reference into the fitted
// distributions registry — plus kind: "gamma" / "weibull" / "point_mass"
// / "mixture" for inline specs that don't need the registry.
//
// Exactly one parameterisation must be present for a given kind:
//   - lognormal: one of MeanTicks/MedianTicks plus Sigma.
//   - from_distribution: Distribution (the registry key).
//   - gamma: Shape + Scale.
//   - weibull: Shape + Scale.
//   - point_mass: Value.
//   - mixture: Components.
type Duration struct {
	Kind string `yaml:"kind"`

	// Log-normal inline params (kind == "lognormal").
	MeanTicks   *float64 `yaml:"mean_ticks,omitempty"`
	MedianTicks *float64 `yaml:"median_ticks,omitempty"`
	Sigma       float64  `yaml:"sigma,omitempty"`

	// Registry reference (kind == "from_distribution").
	Distribution string `yaml:"distribution,omitempty"`

	// Gamma / Weibull inline params.
	Shape float64 `yaml:"shape,omitempty"`
	Scale float64 `yaml:"scale,omitempty"`

	// Point-mass inline param.
	Value float64 `yaml:"value,omitempty"`

	// Mixture inline components.
	Components []DurationComponent `yaml:"components,omitempty"`
}

// DurationComponent is one component of a mixture Duration spec. Mirrors
// rawMixtureComponent in the duration registry but stays on the scenario
// side so the YAML schema is self-contained.
type DurationComponent struct {
	Weight float64 `yaml:"weight"`
	Kind   string  `yaml:"kind,omitempty"`
	// Alias for kind to ease porting from fitted_distributions.yaml format.
	Family string `yaml:"family,omitempty"`

	MeanTicks    *float64 `yaml:"mean_ticks,omitempty"`
	MedianTicks  *float64 `yaml:"median_ticks,omitempty"`
	Sigma        float64  `yaml:"sigma,omitempty"`
	Mu           float64  `yaml:"mu,omitempty"`
	Shape        float64  `yaml:"shape,omitempty"`
	Scale        float64  `yaml:"scale,omitempty"`
	Value        float64  `yaml:"value,omitempty"`
	Distribution string   `yaml:"distribution,omitempty"`

	// Nested params block, matching fitted_distributions.yaml shape.
	Params map[string]float64 `yaml:"params,omitempty"`
}

// MergeModel is the synthetic conflict-merge contribution applied to a
// bead's effective completion tick. See specs/simulator.md §Merge Model.
type MergeModel struct {
	// BaseDuration is the "happy path" merge time draw. When omitted,
	// the happy path contributes 0 ticks (point mass at zero).
	BaseDuration *Duration `yaml:"base_duration,omitempty"`

	// ConflictProbability is the per-bead probability that the merge
	// phase incurs a conflict-resolution contribution. Must be in
	// [0, 1]; defaults to 0.04 (matching the kerf+harmonik corpus).
	ConflictProbability float64 `yaml:"conflict_probability"`

	// ConflictDuration is the conflict-resolution draw, applied with
	// probability ConflictProbability. Required when ConflictProbability
	// > 0.
	ConflictDuration *Duration `yaml:"conflict_duration,omitempty"`
}

// Recognized duration kinds.
const (
	DurationKindLogNormal        = "lognormal"
	DurationKindFromDistribution = "from_distribution"
	DurationKindGamma            = "gamma"
	DurationKindWeibull          = "weibull"
	DurationKindPointMass        = "point_mass"
	DurationKindMixture          = "mixture"
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
	if err := validateDuration("agent_model.duration", s.AgentModel.Duration); err != nil {
		return err
	}
	if s.AgentModel.SpinUp != nil {
		if err := validateDuration("agent_model.spin_up", *s.AgentModel.SpinUp); err != nil {
			return err
		}
	}

	// merge_model: optional. When present, validate sub-fields.
	if mm := s.MergeModel; mm != nil {
		if mm.ConflictProbability < 0 || mm.ConflictProbability > 1 {
			return fmt.Errorf("scenario: merge_model.conflict_probability must be in [0, 1] (got %g)", mm.ConflictProbability)
		}
		if mm.BaseDuration != nil {
			if err := validateDuration("merge_model.base_duration", *mm.BaseDuration); err != nil {
				return err
			}
		}
		if mm.ConflictProbability > 0 {
			if mm.ConflictDuration == nil {
				return fmt.Errorf("scenario: merge_model.conflict_duration is required when conflict_probability > 0")
			}
			if err := validateDuration("merge_model.conflict_duration", *mm.ConflictDuration); err != nil {
				return err
			}
		} else if mm.ConflictDuration != nil {
			if err := validateDuration("merge_model.conflict_duration", *mm.ConflictDuration); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateDuration enforces the per-kind required-field invariants for a
// Duration spec. The lognormal branch matches the Phase-1 contract (one
// of mean_ticks/median_ticks; sigma > 0); the other branches enforce the
// minimum set the registry/loader needs at sample time.
func validateDuration(label string, d Duration) error {
	if d.Kind == "" {
		return fmt.Errorf("scenario: %s.kind is required", label)
	}
	switch d.Kind {
	case DurationKindLogNormal:
		hasMean := d.MeanTicks != nil
		hasMedian := d.MedianTicks != nil
		if hasMean && hasMedian {
			return fmt.Errorf("scenario: %s: only one of mean_ticks or median_ticks may be set", label)
		}
		if !hasMean && !hasMedian {
			return fmt.Errorf("scenario: %s: one of mean_ticks or median_ticks must be set", label)
		}
		if hasMean && *d.MeanTicks <= 0 {
			return fmt.Errorf("scenario: %s.mean_ticks must be > 0", label)
		}
		if hasMedian && *d.MedianTicks <= 0 {
			return fmt.Errorf("scenario: %s.median_ticks must be > 0", label)
		}
		if d.Sigma <= 0 {
			return fmt.Errorf("scenario: %s.sigma must be > 0", label)
		}
	case DurationKindFromDistribution:
		if d.Distribution == "" {
			return fmt.Errorf("scenario: %s.distribution is required for kind=from_distribution", label)
		}
	case DurationKindGamma, DurationKindWeibull:
		if d.Shape <= 0 {
			return fmt.Errorf("scenario: %s.shape must be > 0 for kind=%s", label, d.Kind)
		}
		if d.Scale <= 0 {
			return fmt.Errorf("scenario: %s.scale must be > 0 for kind=%s", label, d.Kind)
		}
	case DurationKindPointMass:
		if d.Value < 0 {
			return fmt.Errorf("scenario: %s.value must be >= 0", label)
		}
	case DurationKindMixture:
		if len(d.Components) == 0 {
			return fmt.Errorf("scenario: %s.components is required for kind=mixture", label)
		}
	default:
		return fmt.Errorf("scenario: %s.kind %q is not recognized", label, d.Kind)
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
