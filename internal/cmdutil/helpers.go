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
	"github.com/gberns/kerf/internal/storage"
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

	// Try git repo identifier. A corrupt .kerf/project-identifier file
	// (kerf-dlb / kerf-vu0r) must surface non-zero rather than silently
	// falling through to default_project, which would route the command at
	// the wrong project. Other errors (no git repo, missing identifier) are
	// the legitimate fall-through.
	id, rerr := project.Resolve(cwd, bp)
	if rerr == nil {
		return id, nil
	}
	if project.IsCorruptIdentifier(rerr) {
		return "", rerr
	}

	// Try default_project from config.
	cfgPath := filepath.Join(bp, "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err == nil && cfg.DefaultProject != "" {
		return cfg.DefaultProject, nil
	}

	return "", fmt.Errorf("cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier")
}

// LoadWork reads a work's spec.yaml from the resolved storage location.
func LoadWork(projectID, codename string) (*spec.SpecYAML, string, error) {
	r, err := Resolver(projectID)
	if err != nil {
		return nil, "", err
	}
	workDir := r.WorkDir(codename)
	specPath := filepath.Join(workDir, "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		return nil, "", fmt.Errorf("work '%s' not found in project '%s'", codename, projectID)
	}
	return s, workDir, nil
}

// Resolver returns a storage.Resolver for the project. It locates the repo
// root from cwd (best-effort) so the repo-level config can be consulted.
func Resolver(projectID string) (*storage.Resolver, error) {
	bp, err := bench.BenchPath()
	if err != nil {
		return nil, err
	}
	repoRoot := findRepoRootForProject(projectID)
	return storage.NewResolver(bp, projectID, repoRoot)
}

// ResolverForRepo builds a resolver with an explicit repo root.
func ResolverForRepo(projectID, repoRoot string) (*storage.Resolver, error) {
	bp, err := bench.BenchPath()
	if err != nil {
		return nil, err
	}
	return storage.NewResolver(bp, projectID, repoRoot)
}

// findRepoRootForProject returns the repo root if cwd is inside a git repo
// whose .kerf/project-identifier matches projectID. Otherwise returns "".
func findRepoRootForProject(projectID string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	root, err := project.FindGitRoot(cwd)
	if err != nil {
		return ""
	}
	id, err := project.ReadIdentifier(root)
	if err != nil {
		return ""
	}
	if id != projectID {
		return ""
	}
	return root
}

// LoadWorkWithChecks loads a work's spec.yaml and runs cross-cutting checks:
// stale session warning, jig version mismatch warning, and interval snapshot check.
func LoadWorkWithChecks(projectID, codename string) (*spec.SpecYAML, string, error) {
	// TODO: add stale session warning, jig version mismatch, interval snapshot
	return LoadWork(projectID, codename)
}
