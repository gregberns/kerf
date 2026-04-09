package dep

import (
	"path/filepath"

	"github.com/gberns/kerf/internal/spec"
)

// DepResult holds the resolution result for a single dependency.
type DepResult struct {
	Codename     string
	Project      string
	Relationship string
	Status       string
	Complete     bool
	Unresolvable bool
}

// Resolve looks up a dependency's status on the bench.
func Resolve(d spec.Dependency, benchPath string, currentProject string) *DepResult {
	project := currentProject
	if d.Project != nil {
		project = *d.Project
	}

	result := &DepResult{
		Codename:     d.Codename,
		Project:      project,
		Relationship: d.Relationship,
	}

	specPath := filepath.Join(benchPath, "projects", project, d.Codename, "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		result.Unresolvable = true
		return result
	}

	result.Status = s.Status
	result.Complete = IsComplete(s.Status, s.StatusValues)
	return result
}

// IsComplete returns true if status is at or past the terminal (last) value in statusValues.
// A status not found in statusValues is considered past terminal (e.g., "finalized").
func IsComplete(status string, statusValues []string) bool {
	if len(statusValues) == 0 {
		return false
	}

	terminal := statusValues[len(statusValues)-1]
	if status == terminal {
		return true
	}

	// If status doesn't appear in the list at all, it's past terminal.
	for _, v := range statusValues {
		if v == status {
			return false // found before terminal
		}
	}
	return true // not in list → past terminal
}

// CheckBlocking checks all must-complete-first dependencies and returns their results.
func CheckBlocking(deps []spec.Dependency, benchPath string, currentProject string) []DepResult {
	var results []DepResult
	for _, d := range deps {
		if d.Relationship != "must-complete-first" {
			continue
		}
		result := Resolve(d, benchPath, currentProject)
		results = append(results, *result)
	}
	return results
}
