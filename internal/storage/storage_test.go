package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolver_BenchModeByDefault(t *testing.T) {
	bp := t.TempDir()
	repo := t.TempDir()
	r, err := NewResolver(bp, "my-proj", repo)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != ModeBench {
		t.Fatalf("Mode = %q, want %q", r.Mode, ModeBench)
	}
	wantWorks := filepath.Join(bp, "projects", "my-proj")
	if got := r.WorksDir(); got != wantWorks {
		t.Errorf("WorksDir = %q, want %q", got, wantWorks)
	}
	wantCfg := filepath.Join(bp, "projects", "my-proj", "project.yaml")
	if got := r.ProjectConfigPath(); got != wantCfg {
		t.Errorf("ProjectConfigPath = %q, want %q", got, wantCfg)
	}
}

func TestResolver_LocalMode(t *testing.T) {
	bp := t.TempDir()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "config.yaml"), []byte("storage: local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := NewResolver(bp, "my-proj", repo)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != ModeLocal {
		t.Fatalf("Mode = %q, want %q", r.Mode, ModeLocal)
	}
	wantWorks := filepath.Join(repo, ".kerf", "works")
	if got := r.WorksDir(); got != wantWorks {
		t.Errorf("WorksDir = %q, want %q", got, wantWorks)
	}
	wantCfg := filepath.Join(repo, ".kerf", "project.yaml")
	if got := r.ProjectConfigPath(); got != wantCfg {
		t.Errorf("ProjectConfigPath = %q, want %q", got, wantCfg)
	}
	wantWork := filepath.Join(repo, ".kerf", "works", "wing-foo")
	if got := r.WorkDir("wing-foo"); got != wantWork {
		t.Errorf("WorkDir = %q, want %q", got, wantWork)
	}
	wantArchive := filepath.Join(bp, "archive", "my-proj", "wing-foo")
	if got := r.ArchiveDir("wing-foo"); got != wantArchive {
		t.Errorf("ArchiveDir = %q, want %q", got, wantArchive)
	}
}

func TestResolver_NoRepoRootDefaultsToBench(t *testing.T) {
	bp := t.TempDir()
	r, err := NewResolver(bp, "my-proj", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != ModeBench {
		t.Errorf("Mode = %q, want bench", r.Mode)
	}
}

func TestSaveLoadRepoConfig(t *testing.T) {
	repo := t.TempDir()
	if err := SaveRepoConfig(repo, &RepoConfig{Storage: "local"}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRepoConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Storage != "local" {
		t.Errorf("Storage = %q, want local", got.Storage)
	}
}
