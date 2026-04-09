package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

// ─── Spec-first finalization ────────────────────────────────────────────────

func TestE2E_SpecFirstFinalize(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	bp := filepath.Join(tmp, ".kerf")
	proj := "spec-proj"

	// Create a square spec work.
	workDir := createSquareSpecWork(t, bp, proj, "spec-work")

	// Add spec drafts.
	draftsDir := filepath.Join(workDir, "05-spec-drafts")
	os.MkdirAll(draftsDir, 0755)
	os.WriteFile(filepath.Join(draftsDir, "jig-system.md"), []byte("# Jig System Spec"), 0644)
	os.WriteFile(filepath.Join(draftsDir, "finalization.md"), []byte("# Finalization Spec"), 0644)

	// Commit project identifier so repo is clean.
	os.MkdirAll(filepath.Join(repo, ".kerf"), 0755)
	os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(proj), 0644)
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

	// Finalize.
	out := captureOutput(t, func() {
		projectFlag = proj
		branchFlag = "spec/jig-redesign"
		defer func() { projectFlag = ""; branchFlag = "" }()
		finalizeCmd.RunE(finalizeCmd, []string{"spec-work"})
	})

	// Verify dual output messages.
	testutil.AssertStringContains(t, out, "Artifacts copied to:")
	testutil.AssertStringContains(t, out, "Spec drafts applied to: specs/")

	// Verify spec drafts landed in spec_path (specs/).
	testutil.AssertFileExists(t, filepath.Join(repo, "specs", "jig-system.md"))
	testutil.AssertFileExists(t, filepath.Join(repo, "specs", "finalization.md"))
	testutil.AssertFileContains(t, filepath.Join(repo, "specs", "jig-system.md"), "# Jig System Spec")
	testutil.AssertFileContains(t, filepath.Join(repo, "specs", "finalization.md"), "# Finalization Spec")

	// Verify 05-spec-drafts/ was excluded from repo_spec_path.
	artifactDir := filepath.Join(repo, ".kerf", "spec-work")
	if _, err := os.Stat(filepath.Join(artifactDir, "05-spec-drafts")); err == nil {
		t.Error("05-spec-drafts/ should not be copied to repo_spec_path for spec-first works")
	}

	// Verify other artifacts were copied to repo_spec_path.
	testutil.AssertFileExists(t, filepath.Join(artifactDir, "01-problem-space.md"))
	testutil.AssertFileExists(t, filepath.Join(artifactDir, "05-changelog.md"))

	// Verify commit includes files from both destinations.
	cmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
	cmd.Dir = repo
	diffOut, err := cmd.Output()
	if err != nil {
		t.Fatalf("git diff-tree: %v", err)
	}
	diffStr := string(diffOut)
	testutil.AssertStringContains(t, diffStr, "specs/jig-system.md")
	testutil.AssertStringContains(t, diffStr, "specs/finalization.md")
	testutil.AssertStringContains(t, diffStr, ".kerf/spec-work/01-problem-space.md")
}

func TestE2E_PlanFirstFinalize_NoSpecPathBehavior(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	bp := filepath.Join(tmp, ".kerf")
	proj := "plan-proj"

	// Create a square bug work with an 05-spec-drafts/ dir (non-spec jig).
	workDir := createSquareBugWork(t, bp, proj, "plan-work")

	// Add 05-spec-drafts to a bug work — should be copied normally.
	draftsDir := filepath.Join(workDir, "05-spec-drafts")
	os.MkdirAll(draftsDir, 0755)
	os.WriteFile(filepath.Join(draftsDir, "something.md"), []byte("content"), 0644)

	// Commit project identifier so repo is clean.
	os.MkdirAll(filepath.Join(repo, ".kerf"), 0755)
	os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(proj), 0644)
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

	out := captureOutput(t, func() {
		projectFlag = proj
		branchFlag = "fix/plan-work"
		defer func() { projectFlag = ""; branchFlag = "" }()
		finalizeCmd.RunE(finalizeCmd, []string{"plan-work"})
	})

	// Should NOT have spec drafts message.
	if strings.Contains(out, "Spec drafts applied to") {
		t.Error("plan-first work should not have spec drafts message")
	}

	// 05-spec-drafts/ should be copied to repo_spec_path (not excluded).
	artifactDir := filepath.Join(repo, ".kerf", "plan-work")
	testutil.AssertFileExists(t, filepath.Join(artifactDir, "05-spec-drafts", "something.md"))

	// specs/ directory should NOT exist (no spec_path behavior).
	if _, err := os.Stat(filepath.Join(repo, "specs")); err == nil {
		t.Error("specs/ should not be created for plan-first work")
	}
}

func TestE2E_SpecFirstFinalize_MissingSpecDrafts(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	bp := filepath.Join(tmp, ".kerf")
	proj := "nospec-proj"

	// Create a square spec work WITHOUT 05-spec-drafts/.
	createSquareSpecWork(t, bp, proj, "nospec-work")

	// Commit project identifier so repo is clean.
	os.MkdirAll(filepath.Join(repo, ".kerf"), 0755)
	os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(proj), 0644)
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

	// Should succeed with warning, not error.
	out := captureOutput(t, func() {
		projectFlag = proj
		branchFlag = "spec/nospec"
		defer func() { projectFlag = ""; branchFlag = "" }()
		err := finalizeCmd.RunE(finalizeCmd, []string{"nospec-work"})
		if err != nil {
			t.Fatalf("finalize should not error with missing spec drafts: %v", err)
		}
	})

	testutil.AssertStringContains(t, out, "Warning: 05-spec-drafts/ not found")
	testutil.AssertStringContains(t, out, "Status: finalized")
}

func TestE2E_SpecFirstFinalize_SpecPathCreated(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	bp := filepath.Join(tmp, ".kerf")
	proj := "create-proj"

	workDir := createSquareSpecWork(t, bp, proj, "create-work")

	// Add spec drafts.
	draftsDir := filepath.Join(workDir, "05-spec-drafts")
	os.MkdirAll(draftsDir, 0755)
	os.WriteFile(filepath.Join(draftsDir, "new-spec.md"), []byte("# New Spec"), 0644)

	// Verify specs/ doesn't exist yet.
	if _, err := os.Stat(filepath.Join(repo, "specs")); err == nil {
		t.Fatal("specs/ should not exist before finalization")
	}

	// Commit project identifier so repo is clean.
	os.MkdirAll(filepath.Join(repo, ".kerf"), 0755)
	os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(proj), 0644)
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

	captureOutput(t, func() {
		projectFlag = proj
		branchFlag = "spec/create"
		defer func() { projectFlag = ""; branchFlag = "" }()
		err := finalizeCmd.RunE(finalizeCmd, []string{"create-work"})
		if err != nil {
			t.Fatalf("finalize failed: %v", err)
		}
	})

	// specs/ should now exist and contain the draft.
	testutil.AssertFileExists(t, filepath.Join(repo, "specs", "new-spec.md"))
	testutil.AssertFileContains(t, filepath.Join(repo, "specs", "new-spec.md"), "# New Spec")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// createSquareSpecWork creates a spec jig work at ready status with all required files.
func createSquareSpecWork(t *testing.T, benchPath, projectID, codename string) string {
	t.Helper()
	workDir := filepath.Join(benchPath, "projects", projectID, codename)
	os.MkdirAll(workDir, 0755)

	workSpec := `codename: ` + codename + `
type: spec
project:
  id: ` + projectID + `
jig: spec
jig_version: 1
status: ready
status_values: [problem-space, decompose, research, change-design, spec-draft, integration, tasks, ready]
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
	os.WriteFile(filepath.Join(workDir, "01-problem-space.md"), []byte("problem"), 0644)
	os.WriteFile(filepath.Join(workDir, "02-components.md"), []byte("components"), 0644)
	os.WriteFile(filepath.Join(workDir, "05-changelog.md"), []byte("changelog"), 0644)
	os.WriteFile(filepath.Join(workDir, "06-integration.md"), []byte("integration"), 0644)
	os.WriteFile(filepath.Join(workDir, "07-tasks.md"), []byte("tasks"), 0644)

	return workDir
}
