package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/storage"
	"github.com/gberns/kerf/internal/testutil"
)

func TestLocalize_MigratesWorksFromBenchToRepo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	bp := filepath.Join(tmp, ".kerf")

	captureOutput(t, func() {
		projectFlag = ""
		newJigFlag = "bug"
		newTitle = "test"
		defer func() { newJigFlag = ""; newTitle = "" }()
		newCmd.RunE(newCmd, []string{"work-a"})
	})

	pidBytes, err := os.ReadFile(filepath.Join(repo, ".kerf", "project-identifier"))
	if err != nil {
		t.Fatalf("reading project-identifier: %v", err)
	}
	projectID := strings.TrimSpace(string(pidBytes))

	benchWorkDir := filepath.Join(bp, "projects", projectID, "work-a")
	if _, err := os.Stat(benchWorkDir); err != nil {
		t.Fatalf("work not on bench before localize: %v", err)
	}

	out := captureOutput(t, func() {
		projectFlag = ""
		if err := localizeCmd.RunE(localizeCmd, nil); err != nil {
			t.Fatalf("localize failed: %v", err)
		}
	})

	if !strings.Contains(out, "Localized project") {
		t.Errorf("expected 'Localized project' in output, got:\n%s", out)
	}

	localWorkDir := filepath.Join(repo, ".kerf", "works", "work-a")
	if _, err := os.Stat(filepath.Join(localWorkDir, "spec.yaml")); err != nil {
		t.Fatalf("work not present in repo after localize: %v", err)
	}

	link := filepath.Join(bp, "projects", projectID)
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("bench symlink missing: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("bench path is not a symlink: %v", info.Mode())
	}

	cfg, err := storage.LoadRepoConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage != "local" {
		t.Errorf("repo config storage = %q, want local", cfg.Storage)
	}

	// project.yaml is only present if init created one; if it existed on the
	// bench, localize must have moved it to the repo.

	// kerf list should still find the work.
	listOut := captureOutput(t, func() {
		projectFlag = ""
		listCmd.RunE(listCmd, nil)
	})
	if !strings.Contains(listOut, "work-a") {
		t.Errorf("kerf list does not see work-a after localize:\n%s", listOut)
	}

	// Reading the spec through the symlink should also work.
	s, err := spec.Read(filepath.Join(link, "work-a", "spec.yaml"))
	if err != nil {
		t.Fatalf("reading spec via symlink: %v", err)
	}
	if s.Codename != "work-a" {
		t.Errorf("codename = %q, want %q", s.Codename, "work-a")
	}
}

func TestLocalize_AlreadyLocal(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "config.yaml"), []byte("storage: local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte("test-proj\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureOutput(t, func() {
		projectFlag = ""
		if err := localizeCmd.RunE(localizeCmd, nil); err != nil {
			t.Fatalf("localize on already-local should not error: %v", err)
		}
	})
	if !strings.Contains(out, "Already using local storage") {
		t.Errorf("expected 'Already using local storage', got:\n%s", out)
	}
}
