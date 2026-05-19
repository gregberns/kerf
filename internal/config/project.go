package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gberns/kerf/internal/beads"
	"gopkg.in/yaml.v3"
)

// DoctorFooterEnvVar is the environment variable that overrides the
// project-level `doctor.footer` setting for the storage-drift footer on
// `kerf next` and `kerf triage`. `0`/`false`/`no`/`off` suppress the
// footer; `1`/`true`/`yes`/`on` force-enable it. See
// specs/architecture.md §"Project Configuration" → `doctor.footer` and
// specs/commands.md §"Storage-drift footer".
const DoctorFooterEnvVar = "KERF_DOCTOR_FOOTER"

// ProjectConfig represents per-project jig configuration stored in
// ~/.kerf/projects/{project-id}/project.yaml
type ProjectConfig struct {
	// DefaultJig, when non-empty, is the project-wide default jig name used
	// by `kerf new` when `--jig` is not provided. It takes precedence over
	// the bench-wide `default_jig` in ~/.kerf/config.yaml. See
	// architecture.md §"Project Configuration" for the schema slot and
	// commands.md §"kerf new" for the resolution order.
	DefaultJig string              `yaml:"default_jig,omitempty"`
	Jigs       []string            `yaml:"jigs,omitempty"`
	Passes     map[string][]string `yaml:"passes,omitempty"`
	Tools      map[string]string   `yaml:"tools,omitempty"`
	Queue      *QueueConfig        `yaml:"queue,omitempty"`
	// BeadFilter, when set, is the project-wide bead_filter used to attach
	// beads to works. Per-work filters override this; if both are nil the
	// built-in default ("work:{codename}") is used. See coordination spec
	// §"Resolution order".
	BeadFilter *beads.Filter `yaml:"bead_filter,omitempty"`
	// Doctor holds knobs for diagnostic surfaces. Currently just the
	// drift-footer toggle consumed by `kerf next` and `kerf triage`. See
	// specs/architecture.md §"Project Configuration" → `doctor.footer`.
	Doctor *DoctorConfig `yaml:"doctor,omitempty"`
}

// DoctorConfig holds project-level doctor surface knobs.
type DoctorConfig struct {
	// Footer toggles the storage-drift footer on `kerf next` and
	// `kerf triage`. Nil falls back to the default (true).
	Footer *bool `yaml:"footer,omitempty"`
}

// DoctorFooterEnabled returns whether the storage-drift footer should
// render. Resolution order (highest precedence first):
//  1. KERF_DOCTOR_FOOTER env var, if set to a recognised value.
//  2. `doctor.footer` in project.yaml.
//  3. Default: true.
//
// Receivers may be nil — `(*ProjectConfig)(nil).DoctorFooterEnabled()`
// answers the same as a config with no `doctor` block.
func (c *ProjectConfig) DoctorFooterEnabled() bool {
	if v, ok := parseFooterEnv(os.Getenv(DoctorFooterEnvVar)); ok {
		return v
	}
	if c != nil && c.Doctor != nil && c.Doctor.Footer != nil {
		return *c.Doctor.Footer
	}
	return true
}

// parseFooterEnv parses the KERF_DOCTOR_FOOTER env var. Returns
// (value, true) on a recognised truthy/falsy string, (false, false)
// when unset or unparseable (the caller then falls back to config).
func parseFooterEnv(raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return false, false
	case "0", "false", "no", "off":
		return false, true
	case "1", "true", "yes", "on":
		return true, true
	}
	return false, false
}

// QueueConfig holds project overrides for queue scoring weights.
// Fields are pointers so an unset field falls back to the queue package default.
type QueueConfig struct {
	FanOut   *float64 `yaml:"fan_out,omitempty"`
	Momentum *float64 `yaml:"momentum,omitempty"`
	Creation *float64 `yaml:"creation,omitempty"`
	Rework   *float64 `yaml:"rework,omitempty"`
}

// ResolvedQueueWeights is the result of overlaying QueueConfig on defaults.
type ResolvedQueueWeights struct {
	FanOut   float64
	Momentum float64
	Creation float64
	Rework   float64
}

// QueueWeights returns the effective queue weights for this project, with any
// unset fields filled in from the supplied defaults.
func (c *ProjectConfig) QueueWeights(defaults ResolvedQueueWeights) ResolvedQueueWeights {
	out := defaults
	if c == nil || c.Queue == nil {
		return out
	}
	if c.Queue.FanOut != nil {
		out.FanOut = *c.Queue.FanOut
	}
	if c.Queue.Momentum != nil {
		out.Momentum = *c.Queue.Momentum
	}
	if c.Queue.Creation != nil {
		out.Creation = *c.Queue.Creation
	}
	if c.Queue.Rework != nil {
		out.Rework = *c.Queue.Rework
	}
	return out
}

// LoadProjectConfig reads and parses project.yaml from the given path.
// If the file does not exist, returns a zero-value ProjectConfig.
func LoadProjectConfig(path string) (*ProjectConfig, error) {
	cfg := &ProjectConfig{}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading project config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing project config: %w", err)
	}

	if cfg.BeadFilter != nil {
		if err := cfg.BeadFilter.Validate(); err != nil {
			return nil, fmt.Errorf("project config: invalid bead_filter: %w", err)
		}
	}

	return cfg, nil
}

// SaveProjectConfig writes project.yaml to disk, creating parent directories if needed.
func SaveProjectConfig(path string, cfg *ProjectConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating project config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling project config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing project config: %w", err)
	}

	return nil
}

// IsJigActive returns true if the jig is available for this project.
// When Jigs is nil or empty, all jigs are available.
func (c *ProjectConfig) IsJigActive(jigName string) bool {
	if len(c.Jigs) == 0 {
		return true
	}
	for _, j := range c.Jigs {
		if j == jigName {
			return true
		}
	}
	return false
}

// GetActivePasses returns the list of active passes for a composable jig.
// If Passes is nil or the jig has no entry, returns nil (all passes active).
func (c *ProjectConfig) GetActivePasses(jigName string) []string {
	if c.Passes == nil {
		return nil
	}
	passes, ok := c.Passes[jigName]
	if !ok {
		return nil
	}
	return passes
}

// ProjectConfigPath returns the path to project.yaml for the given bench and project ID.
func ProjectConfigPath(benchPath, projectID string) string {
	return filepath.Join(benchPath, "projects", projectID, "project.yaml")
}
