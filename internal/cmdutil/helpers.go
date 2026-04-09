// Package cmdutil provides shared helpers for kerf commands.
package cmdutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/project"
	"github.com/gberns/kerf/internal/spec"
)

// ResolveProject resolves the project ID from the --project flag,
// .kerf/project-identifier, config default_project, or errors.
func ResolveProject(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	bp, err := bench.BenchPath()
	if err != nil {
		return "", err
	}

	// Try git repo identifier.
	if id, err := project.Resolve(cwd, bp); err == nil {
		return id, nil
	}

	// Try default_project from config.
	cfgPath := filepath.Join(bp, "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err == nil && cfg.DefaultProject != "" {
		return cfg.DefaultProject, nil
	}

	return "", fmt.Errorf("cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier")
}

// LoadWork reads a work's spec.yaml from the bench.
func LoadWork(projectID, codename string) (*spec.SpecYAML, string, error) {
	bp, err := bench.BenchPath()
	if err != nil {
		return nil, "", err
	}

	workDir := bench.WorkDir(bp, projectID, codename)
	specPath := filepath.Join(workDir, "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		return nil, "", fmt.Errorf("work '%s' not found in project '%s'", codename, projectID)
	}
	return s, workDir, nil
}

// LoadWorkWithChecks loads a work's spec.yaml and runs cross-cutting checks:
// stale session warning, jig version mismatch warning, and interval snapshot check.
func LoadWorkWithChecks(projectID, codename string) (*spec.SpecYAML, string, error) {
	// TODO: add stale session warning, jig version mismatch, interval snapshot
	return LoadWork(projectID, codename)
}
