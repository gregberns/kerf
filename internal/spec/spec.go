package spec

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// SpecYAML represents the spec.yaml file — source of truth for work metadata.
type SpecYAML struct {
	// Identity
	Codename string  `yaml:"codename"`
	Title    *string `yaml:"title,omitempty"`
	Type     string  `yaml:"type"`
	Project  Project `yaml:"project"`

	// Jig
	Jig          string   `yaml:"jig"`
	JigVersion   int      `yaml:"jig_version"`
	Status       string   `yaml:"status"`
	StatusValues []string `yaml:"status_values"`

	// Timestamps
	Created time.Time `yaml:"created"`
	Updated time.Time `yaml:"updated"`

	// Sessions
	Sessions      []Session `yaml:"sessions"`
	ActiveSession *string   `yaml:"active_session"`

	// Dependencies
	DependsOn []Dependency `yaml:"depends_on"`

	// Implementation linkage
	Implementation Implementation `yaml:"implementation"`
}

// Project identifies which project a work belongs to.
type Project struct {
	ID string `yaml:"id"`
}

// Session tracks an agent session against a work.
type Session struct {
	ID      *string    `yaml:"id"`
	Started time.Time  `yaml:"started"`
	Ended   *time.Time `yaml:"ended,omitempty"`
	Notes   *string    `yaml:"notes,omitempty"`
}

// Dependency records a relationship to another work.
type Dependency struct {
	Codename     string  `yaml:"codename"`
	Project      *string `yaml:"project,omitempty"`
	Relationship string  `yaml:"relationship"`
}

// Implementation tracks git linkage after finalization.
type Implementation struct {
	Branch  *string  `yaml:"branch"`
	PR      *string  `yaml:"pr"`
	Commits []string `yaml:"commits"`
}

// Read parses a spec.yaml file from disk.
func Read(path string) (*SpecYAML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec.yaml: %w", err)
	}

	var s SpecYAML
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing spec.yaml: %w", err)
	}

	return &s, nil
}

// Write serializes a SpecYAML to disk, auto-setting the updated timestamp.
func Write(path string, spec *SpecYAML) error {
	spec.Updated = time.Now().UTC().Truncate(time.Second)

	data, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshaling spec.yaml: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing spec.yaml: %w", err)
	}

	return nil
}
