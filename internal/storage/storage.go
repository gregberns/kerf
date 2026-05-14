// Package storage resolves filesystem paths for a kerf project, choosing
// between bench storage (~/.kerf/projects/{project-id}/) and local storage
// (.kerf/works/ in the repo) based on the repo-level config file.
package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Mode is the storage mode for a project.
type Mode string

const (
	// ModeBench stores works on the bench at ~/.kerf/projects/{project-id}/.
	ModeBench Mode = "bench"
	// ModeLocal stores works in the repo at .kerf/works/.
	ModeLocal Mode = "local"
)

// RepoConfig is the repo-level config file at {repo-root}/.kerf/config.yaml.
type RepoConfig struct {
	Storage string `yaml:"storage,omitempty"`
}

// RepoConfigPath returns the path to .kerf/config.yaml inside the repo.
func RepoConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".kerf", "config.yaml")
}

// LoadRepoConfig reads .kerf/config.yaml from the repo root. If the file does
// not exist, returns a zero RepoConfig and nil error.
func LoadRepoConfig(repoRoot string) (*RepoConfig, error) {
	if repoRoot == "" {
		return &RepoConfig{}, nil
	}
	path := RepoConfigPath(repoRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &RepoConfig{}, nil
		}
		return nil, fmt.Errorf("reading repo config: %w", err)
	}
	cfg := &RepoConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing repo config: %w", err)
	}
	return cfg, nil
}

// SaveRepoConfig writes .kerf/config.yaml to the repo root, creating the
// .kerf directory if needed.
func SaveRepoConfig(repoRoot string, cfg *RepoConfig) error {
	if repoRoot == "" {
		return fmt.Errorf("repo root is empty")
	}
	dir := filepath.Join(repoRoot, ".kerf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating .kerf directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling repo config: %w", err)
	}
	return os.WriteFile(RepoConfigPath(repoRoot), data, 0o644)
}

// Resolver resolves paths for a project based on its storage mode.
type Resolver struct {
	Mode      Mode
	BenchPath string
	ProjectID string
	RepoRoot  string
}

// NewResolver builds a Resolver for the given project. If repoRoot is empty,
// the resolver always uses bench mode. Otherwise it reads .kerf/config.yaml
// from the repo root; if storage is "local" the resolver uses local mode.
func NewResolver(benchPath, projectID, repoRoot string) (*Resolver, error) {
	mode := ModeBench
	if repoRoot != "" {
		cfg, err := LoadRepoConfig(repoRoot)
		if err != nil {
			return nil, err
		}
		if cfg.Storage == string(ModeLocal) {
			mode = ModeLocal
		}
	}
	return &Resolver{
		Mode:      mode,
		BenchPath: benchPath,
		ProjectID: projectID,
		RepoRoot:  repoRoot,
	}, nil
}

// WorksDir is the directory containing all work directories for the project.
func (r *Resolver) WorksDir() string {
	if r.Mode == ModeLocal && r.RepoRoot != "" {
		return filepath.Join(r.RepoRoot, ".kerf", "works")
	}
	return filepath.Join(r.BenchPath, "projects", r.ProjectID)
}

// WorkDir is the directory for a single work.
func (r *Resolver) WorkDir(codename string) string {
	return filepath.Join(r.WorksDir(), codename)
}

// ProjectConfigPath is where project.yaml lives.
func (r *Resolver) ProjectConfigPath() string {
	if r.Mode == ModeLocal && r.RepoRoot != "" {
		return filepath.Join(r.RepoRoot, ".kerf", "project.yaml")
	}
	return filepath.Join(r.BenchPath, "projects", r.ProjectID, "project.yaml")
}

// AreasPath is where areas.yaml lives for the project.
func (r *Resolver) AreasPath() string {
	if r.Mode == ModeLocal && r.RepoRoot != "" {
		return filepath.Join(r.RepoRoot, ".kerf", "areas.yaml")
	}
	return filepath.Join(r.BenchPath, "projects", r.ProjectID, "areas.yaml")
}

// ArchiveDir is the directory for an archived work. Archives always live on
// the bench, regardless of storage mode.
func (r *Resolver) ArchiveDir(codename string) string {
	return filepath.Join(r.BenchPath, "archive", r.ProjectID, codename)
}

// ListWorks returns codenames of all active works for the project.
func (r *Resolver) ListWorks() ([]string, error) {
	return listDirs(r.WorksDir())
}

// ListArchivedWorks returns codenames of archived works.
func (r *Resolver) ListArchivedWorks() ([]string, error) {
	return listDirs(filepath.Join(r.BenchPath, "archive", r.ProjectID))
}

// WorkExists returns true if the work directory exists.
func (r *Resolver) WorkExists(codename string) bool {
	info, err := os.Stat(r.WorkDir(codename))
	return err == nil && info.IsDir()
}

// IsArchived returns true if the work exists in the archive.
func (r *Resolver) IsArchived(codename string) bool {
	info, err := os.Stat(r.ArchiveDir(codename))
	return err == nil && info.IsDir()
}

// CreateWork creates the work directory.
func (r *Resolver) CreateWork(codename string) error {
	return os.MkdirAll(r.WorkDir(codename), 0o755)
}

// MoveToArchive moves a work from its current location to the bench archive.
func (r *Resolver) MoveToArchive(codename string) error {
	src := r.WorkDir(codename)
	dst := r.ArchiveDir(codename)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("creating archive directory: %w", err)
	}
	return os.Rename(src, dst)
}

// DeleteWork removes the work directory (active or archived).
func (r *Resolver) DeleteWork(codename string) error {
	dir := r.WorkDir(codename)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		dir = r.ArchiveDir(codename)
	}
	return os.RemoveAll(dir)
}

func listDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
