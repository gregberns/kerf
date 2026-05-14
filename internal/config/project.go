package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectConfig represents per-project jig configuration stored in
// ~/.kerf/projects/{project-id}/project.yaml
type ProjectConfig struct {
	Jigs   []string            `yaml:"jigs,omitempty"`
	Passes map[string][]string `yaml:"passes,omitempty"`
	Tools  map[string]string   `yaml:"tools,omitempty"`
	Queue  *QueueConfig        `yaml:"queue,omitempty"`
}

// QueueConfig holds project overrides for queue scoring weights.
// Fields are pointers so an unset field falls back to the queue package default.
type QueueConfig struct {
	FanOut   *float64 `yaml:"fan_out,omitempty"`
	Momentum *float64 `yaml:"momentum,omitempty"`
	Creation *float64 `yaml:"creation,omitempty"`
}

// ResolvedQueueWeights is the result of overlaying QueueConfig on defaults.
type ResolvedQueueWeights struct {
	FanOut   float64
	Momentum float64
	Creation float64
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
