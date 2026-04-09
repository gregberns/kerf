package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/testutil"
)

// ─── E2E: Full lifecycle with real git repo ──────────────────────────────────

func TestE2E_FullLifecycleWithGit(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	bp := filepath.Join(tmp, ".kerf")

	// 1. kerf new — first use, will derive project ID.
	var out string
	out = captureOutput(t, func() {
		projectFlag = ""
		newJigFlag = "bug"
		newTitle = "Fix login timeout"
		newType = ""
		defer func() { newJigFlag = ""; newTitle = "" }()
		newCmd.RunE(newCmd, []string{"fix-login"})
	})
	testutil.AssertStringContains(t, out, "Project ID derived:")
	testutil.AssertStringContains(t, out, "Work created: fix-login")
	testutil.AssertStringContains(t, out, "Jig:      bug")

	// Read the derived project ID.
	pidBytes, err := os.ReadFile(filepath.Join(repo, ".kerf", "project-identifier"))
	if err != nil {
		t.Fatalf("reading project-identifier: %v", err)
	}
	projectID := strings.TrimSpace(string(pidBytes))
	if projectID == "" {
		t.Fatal("project ID is empty")
	}

	workDir := filepath.Join(bp, "projects", projectID, "fix-login")

	// 2. Write all bug jig artifacts.
	os.WriteFile(filepath.Join(workDir, "SESSION.md"), []byte("# Session\n## Current Pass\nTriage\n"), 0644)
	os.WriteFile(filepath.Join(workDir, "01-triage.md"), []byte("# Triage\nLogin times out after 30s."), 0644)
	os.WriteFile(filepath.Join(workDir, "02-reproduction.md"), []byte("# Reproduction\nSteps to reproduce."), 0644)
	os.WriteFile(filepath.Join(workDir, "03-root-cause.md"), []byte("# Root Cause\nConnection pool exhaustion."), 0644)
	os.WriteFile(filepath.Join(workDir, "04-fix-spec.md"), []byte("# Fix Spec\nIncrease pool size."), 0644)
	os.WriteFile(filepath.Join(workDir, "05-test-cases.md"), []byte("# Test Cases\nVerify under load."), 0644)

	// 3. Advance through statuses to ready.
	for _, status := range []string{"reproducing", "locating", "specifying-fix", "ready"} {
		captureOutput(t, func() {
			projectFlag = projectID
			defer func() { projectFlag = "" }()
			statusCmd.RunE(statusCmd, []string{"fix-login", status})
		})
	}

	s, _ := spec.Read(filepath.Join(workDir, "spec.yaml"))
	if s.Status != "ready" {
		t.Fatalf("status = %q, want %q", s.Status, "ready")
	}

	// 4. Square check — should pass.
	out = captureOutput(t, func() {
		projectFlag = projectID
		defer func() { projectFlag = "" }()
		squareCmd.RunE(squareCmd, []string{"fix-login"})
	})
	if strings.Contains(out, "NOT SQUARE") {
		t.Fatalf("expected SQUARE, got:\n%s", out)
	}
	testutil.AssertStringContains(t, out, "SQUARE")

	// 5. Shelve before finalize (need to end session).
	captureOutput(t, func() {
		projectFlag = projectID
		shelveForce = false
		defer func() { projectFlag = ""; shelveForce = false }()
		shelveCmd.RunE(shelveCmd, []string{"fix-login"})
	})

	// Commit the .kerf/project-identifier so the repo is clean for finalize.
	for _, args := range [][]string{
		{"add", ".kerf/project-identifier"},
		{"commit", "-m", "add project identifier"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	// 6. Finalize.
	branchName := "spec/fix-login-timeout"
	out = captureOutput(t, func() {
		projectFlag = projectID
		branchFlag = branchName
		defer func() { projectFlag = ""; branchFlag = "" }()
		finalizeCmd.RunE(finalizeCmd, []string{"fix-login"})
	})
	testutil.AssertStringContains(t, out, "Finalizing fix-login")
	testutil.AssertStringContains(t, out, "Square check: passed")
	testutil.AssertStringContains(t, out, "Branch created: "+branchName)
	testutil.AssertStringContains(t, out, "Artifacts copied to:")
	testutil.AssertStringContains(t, out, "Status: finalized")

	// Verify git branch exists.
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+branchName)
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		t.Errorf("branch %q not found in repo", branchName)
	}

	// Verify artifacts were copied (excluding spec.yaml, SESSION.md, .history/).
	artifactDir := filepath.Join(repo, ".kerf", "fix-login")
	testutil.AssertFileExists(t, filepath.Join(artifactDir, "01-triage.md"))
	testutil.AssertFileExists(t, filepath.Join(artifactDir, "02-reproduction.md"))
	testutil.AssertFileExists(t, filepath.Join(artifactDir, "03-root-cause.md"))
	testutil.AssertFileExists(t, filepath.Join(artifactDir, "04-fix-spec.md"))
	testutil.AssertFileExists(t, filepath.Join(artifactDir, "05-test-cases.md"))

	// spec.yaml and SESSION.md should NOT be in the repo artifacts.
	if _, err := os.Stat(filepath.Join(artifactDir, "spec.yaml")); err == nil {
		t.Error("spec.yaml should not be copied to repo")
	}
	if _, err := os.Stat(filepath.Join(artifactDir, "SESSION.md")); err == nil {
		t.Error("SESSION.md should not be copied to repo")
	}
	if _, err := os.Stat(filepath.Join(artifactDir, ".history")); err == nil {
		t.Error(".history/ should not be copied to repo")
	}

	// Verify spec.yaml was updated with implementation info.
	s, _ = spec.Read(filepath.Join(workDir, "spec.yaml"))
	if s.Status != "finalized" {
		t.Errorf("status = %q, want %q", s.Status, "finalized")
	}
	if s.Implementation.Branch == nil || *s.Implementation.Branch != branchName {
		t.Errorf("implementation.branch = %v, want %q", s.Implementation.Branch, branchName)
	}
	if len(s.Implementation.Commits) != 1 {
		t.Errorf("expected 1 commit hash, got %d", len(s.Implementation.Commits))
	}
}

// ─── E2E: Finalize pre-flight failures ───────────────────────────────────────

func TestE2E_FinalizeFailsNotSquare(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	bp := filepath.Join(tmp, ".kerf")
	proj := "fin-proj"

	// Create work not at terminal status.
	workDir := filepath.Join(bp, "projects", proj, "unfinished")
	os.MkdirAll(workDir, 0755)
	writeMinimalSpec(t, filepath.Join(workDir, "spec.yaml"), "unfinished", proj)

	err := func() error {
		projectFlag = proj
		branchFlag = "test-branch"
		defer func() { projectFlag = ""; branchFlag = "" }()
		return finalizeCmd.RunE(finalizeCmd, []string{"unfinished"})
	}()

	if err == nil {
		t.Error("expected error for non-square work")
	} else {
		testutil.AssertStringContains(t, err.Error(), "not square")
	}
}

func TestE2E_FinalizeFailsDirtyRepo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	bp := filepath.Join(tmp, ".kerf")
	proj := "dirty-proj"

	// Create a square bug work.
	workDir := createSquareBugWork(t, bp, proj, "dirty-test")

	// Make repo dirty.
	os.WriteFile(filepath.Join(repo, "uncommitted.txt"), []byte("dirty"), 0644)

	err := func() error {
		projectFlag = proj
		branchFlag = "dirty-branch"
		defer func() { projectFlag = ""; branchFlag = "" }()
		return finalizeCmd.RunE(finalizeCmd, []string{"dirty-test"})
	}()
	_ = workDir

	if err == nil {
		t.Error("expected error for dirty repo")
	} else {
		testutil.AssertStringContains(t, err.Error(), "uncommitted changes")
	}
}

func TestE2E_FinalizeFailsExistingBranch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	bp := filepath.Join(tmp, ".kerf")
	proj := "branch-proj"

	createSquareBugWork(t, bp, proj, "branch-test")

	// Create the branch first.
	cmd := exec.Command("git", "branch", "existing-branch")
	cmd.Dir = repo
	cmd.Run()

	err := func() error {
		projectFlag = proj
		branchFlag = "existing-branch"
		defer func() { projectFlag = ""; branchFlag = "" }()
		return finalizeCmd.RunE(finalizeCmd, []string{"branch-test"})
	}()

	if err == nil {
		t.Error("expected error for existing branch")
	} else {
		testutil.AssertStringContains(t, err.Error(), "already exists")
	}
}

// ─── E2E: Works with dependencies ───────────────────────────────────────────

func TestE2E_DependencyWarningAtSquare(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "dep-e2e"

	// Create an incomplete dependency work.
	depDir := filepath.Join(bp, "projects", proj, "prerequisite")
	os.MkdirAll(depDir, 0755)
	depSpec := `codename: prerequisite
type: feature
project:
  id: dep-e2e
jig: feature
jig_version: 1
status: decomposition
status_values: [problem-space, decomposition, research, detailed-spec, review, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.WriteFile(filepath.Join(depDir, "spec.yaml"), []byte(depSpec), 0644)

	// Create a square bug work that depends on the prerequisite.
	workDir := filepath.Join(bp, "projects", proj, "dependent")
	os.MkdirAll(workDir, 0755)
	workSpec := `codename: dependent
type: bug
project:
  id: dep-e2e
jig: bug
jig_version: 1
status: ready
status_values: [triaging, reproducing, locating, specifying-fix, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on:
  - codename: prerequisite
    relationship: must-complete-first
implementation:
  branch: null
  pr: null
  commits: []
`
	os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(workSpec), 0644)
	os.WriteFile(filepath.Join(workDir, "SESSION.md"), []byte("s"), 0644)
	os.WriteFile(filepath.Join(workDir, "01-triage.md"), []byte("t"), 0644)
	os.WriteFile(filepath.Join(workDir, "02-reproduction.md"), []byte("r"), 0644)
	os.WriteFile(filepath.Join(workDir, "03-root-cause.md"), []byte("c"), 0644)
	os.WriteFile(filepath.Join(workDir, "04-fix-spec.md"), []byte("f"), 0644)
	os.WriteFile(filepath.Join(workDir, "05-test-cases.md"), []byte("tc"), 0644)

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		squareCmd.RunE(squareCmd, []string{"dependent"})
	})

	testutil.AssertStringContains(t, out, "NOT SQUARE")
	testutil.AssertStringContains(t, out, "Dependencies:  fail")
	testutil.AssertStringContains(t, out, "prerequisite")
}

// ─── E2E: Multiple projects on bench ─────────────────────────────────────────

func TestE2E_MultipleProjects(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")

	// Create works in two different projects.
	for _, proj := range []string{"proj-alpha", "proj-beta"} {
		captureOutput(t, func() {
			projectFlag = proj
			newJigFlag = ""
			newTitle = ""
			newType = ""
			defer func() { projectFlag = "" }()
			newCmd.RunE(newCmd, []string{"shared-name"})
		})
	}

	// Both should exist independently.
	for _, proj := range []string{"proj-alpha", "proj-beta"} {
		specPath := filepath.Join(bp, "projects", proj, "shared-name", "spec.yaml")
		testutil.AssertFileExists(t, specPath)

		s, err := spec.Read(specPath)
		if err != nil {
			t.Fatalf("reading spec for %s: %v", proj, err)
		}
		if s.Project.ID != proj {
			t.Errorf("project ID = %q, want %q", s.Project.ID, proj)
		}
	}

	// Show each independently.
	for _, proj := range []string{"proj-alpha", "proj-beta"} {
		out := captureOutput(t, func() {
			projectFlag = proj
			defer func() { projectFlag = "" }()
			showCmd.RunE(showCmd, []string{"shared-name"})
		})
		testutil.AssertStringContains(t, out, "Project: "+proj)
	}
}

// ─── E2E: Jig loading from file ─────────────────────────────────────────────

func TestE2E_JigLoadFromFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")

	// Write a custom jig file.
	customJig := `---
name: custom
description: Custom test jig
version: 1
status_values:
  - start
  - finish
passes:
  - name: "Start"
    status: start
    output: ["output.md"]
file_structure:
  - spec.yaml
  - output.md
---

# Custom Jig

Do the custom thing.
`
	customPath := filepath.Join(tmp, "custom-jig.md")
	os.WriteFile(customPath, []byte(customJig), 0644)

	// Load the jig via jig save --from.
	out := captureOutput(t, func() {
		jigSaveFrom = customPath
		defer func() { jigSaveFrom = "" }()
		jigSaveCmd.RunE(jigSaveCmd, []string{"custom"})
	})
	testutil.AssertStringContains(t, out, "saved")

	// Verify it exists in user jigs dir.
	testutil.AssertFileExists(t, filepath.Join(bp, "jigs", "custom.md"))

	// Use it to create a work.
	out = captureOutput(t, func() {
		projectFlag = "custom-proj"
		newJigFlag = "custom"
		newTitle = ""
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		newCmd.RunE(newCmd, []string{"custom-work"})
	})
	testutil.AssertStringContains(t, out, "Jig:      custom")

	s, err := spec.Read(filepath.Join(bp, "projects", "custom-proj", "custom-work", "spec.yaml"))
	if err != nil {
		t.Fatalf("reading spec: %v", err)
	}
	if s.Jig != "custom" {
		t.Errorf("jig = %q, want %q", s.Jig, "custom")
	}
	if s.Status != "start" {
		t.Errorf("status = %q, want %q", s.Status, "start")
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// createSquareBugWork creates a bug jig work at ready status with all files present.
func createSquareBugWork(t *testing.T, benchPath, projectID, codename string) string {
	t.Helper()
	workDir := filepath.Join(benchPath, "projects", projectID, codename)
	os.MkdirAll(workDir, 0755)

	workSpec := `codename: ` + codename + `
type: bug
project:
  id: ` + projectID + `
jig: bug
jig_version: 1
status: ready
status_values: [triaging, reproducing, locating, specifying-fix, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(workSpec), 0644)
	os.WriteFile(filepath.Join(workDir, "SESSION.md"), []byte("# Session"), 0644)
	os.WriteFile(filepath.Join(workDir, "01-triage.md"), []byte("triage"), 0644)
	os.WriteFile(filepath.Join(workDir, "02-reproduction.md"), []byte("repro"), 0644)
	os.WriteFile(filepath.Join(workDir, "03-root-cause.md"), []byte("cause"), 0644)
	os.WriteFile(filepath.Join(workDir, "04-fix-spec.md"), []byte("fix"), 0644)
	os.WriteFile(filepath.Join(workDir, "05-test-cases.md"), []byte("tests"), 0644)

	return workDir
}
