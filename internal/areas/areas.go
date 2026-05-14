// Package areas manages the project area taxonomy.
package areas

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

// namePattern validates area names: lowercase alphanumeric with hyphens.
var namePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Area represents a named region of the system.
type Area struct {
	Description string `yaml:"description"`
}

// AreasFile represents the areas.yaml file.
type AreasFile struct {
	Areas map[string]Area `yaml:"areas"`
}

// Load reads areas.yaml from the given path.
// If the file does not exist, returns an empty AreasFile.
func Load(path string) (*AreasFile, error) {
	af := &AreasFile{Areas: make(map[string]Area)}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return af, nil
		}
		return nil, fmt.Errorf("reading areas.yaml: %w", err)
	}

	if err := yaml.Unmarshal(data, af); err != nil {
		return nil, fmt.Errorf("parsing areas.yaml: %w", err)
	}

	if af.Areas == nil {
		af.Areas = make(map[string]Area)
	}

	return af, nil
}

// Save writes areas.yaml to disk, creating parent directories if needed.
func Save(path string, a *AreasFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating areas directory: %w", err)
	}

	data, err := yaml.Marshal(a)
	if err != nil {
		return fmt.Errorf("marshaling areas.yaml: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing areas.yaml: %w", err)
	}

	return nil
}

// Add adds a new area to the taxonomy. Returns an error if the name is invalid
// or already exists.
func Add(a *AreasFile, name, description string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid area name %q: must match [a-z0-9]+(-[a-z0-9]+)*", name)
	}

	if _, exists := a.Areas[name]; exists {
		return fmt.Errorf("area %q already exists", name)
	}

	a.Areas[name] = Area{Description: description}
	return nil
}

// Remove removes an area by name. Returns an error if the area does not exist.
func Remove(a *AreasFile, name string) error {
	if _, exists := a.Areas[name]; !exists {
		return fmt.Errorf("area %q not found", name)
	}

	delete(a.Areas, name)
	return nil
}

// Validate returns the list of names that are NOT in the taxonomy.
func Validate(a *AreasFile, names []string) []string {
	var invalid []string
	for _, n := range names {
		if _, exists := a.Areas[n]; !exists {
			invalid = append(invalid, n)
		}
	}
	return invalid
}

// AreasPath returns the path to areas.yaml for the given bench and project ID.
func AreasPath(benchPath, projectID string) string {
	return filepath.Join(benchPath, "projects", projectID, "areas.yaml")
}

// Names returns a sorted list of area names.
func Names(a *AreasFile) []string {
	names := make([]string, 0, len(a.Areas))
	for name := range a.Areas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
