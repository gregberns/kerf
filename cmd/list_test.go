package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

func setupGitRepoForTest(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	run("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "README")
	run("commit", "-q", "-m", "init")
	return dir
}

func chdirT(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func TestListCommand_RetrofitHint_DirtyRepoWithActiveWork(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := setupGitRepoForTest(t)
	// Dirty change.
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	chdirT(t, repo)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")
	writeMinimalSpec(t,
		filepath.Join(projDir, "blue-bear", "spec.yaml"),
		"blue-bear", "test-proj")

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		listCmd.RunE(listCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "blue-bear")
	testutil.AssertStringContains(t, out, "kerf new --jig retrofit")
}

func TestListCommand_RetrofitHint_CleanRepoNoHint(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := setupGitRepoForTest(t)
	chdirT(t, repo)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")
	writeMinimalSpec(t,
		filepath.Join(projDir, "blue-bear", "spec.yaml"),
		"blue-bear", "test-proj")

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		listCmd.RunE(listCmd, []string{})
	})

	if containsString(out, "kerf new --jig retrofit") {
		t.Errorf("did not expect retrofit hint for clean repo, got:\n%s", out)
	}
}

func TestListCommand_RetrofitHint_NoActiveWorkNoHint(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := setupGitRepoForTest(t)
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	chdirT(t, repo)

	benchDir := filepath.Join(tmp, ".kerf")
	os.MkdirAll(filepath.Join(benchDir, "projects", "test-proj"), 0755)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		listCmd.RunE(listCmd, []string{})
	})

	if containsString(out, "kerf new --jig retrofit") {
		t.Errorf("did not expect retrofit hint when no active work, got:\n%s", out)
	}
}

func TestListCommand_EmptyProject(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	os.MkdirAll(filepath.Join(benchDir, "projects", "my-proj"), 0755)

	out := captureOutput(t, func() {
		projectFlag = "my-proj"
		defer func() { projectFlag = "" }()
		listCmd.RunE(listCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "No works found")
	testutil.AssertStringContains(t, out, "kerf new")
}

func TestListCommand_WithWorks(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")

	// Create two works.
	writeMinimalSpec(t,
		filepath.Join(projDir, "blue-bear", "spec.yaml"),
		"blue-bear", "test-proj")
	writeMinimalSpec(t,
		filepath.Join(projDir, "red-fox", "spec.yaml"),
		"red-fox", "test-proj")

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		listCmd.RunE(listCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "On the bench for test-proj")
	testutil.AssertStringContains(t, out, "blue-bear")
	testutil.AssertStringContains(t, out, "red-fox")
	testutil.AssertStringContains(t, out, "Commands:")
}

func TestListCommand_StatusFilter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")

	writeMinimalSpec(t,
		filepath.Join(projDir, "blue-bear", "spec.yaml"),
		"blue-bear", "test-proj")

	// Write a second work with different status.
	content := `codename: red-fox
type: bug
project:
  id: test-proj
jig: bug
jig_version: 1
status: investigating
status_values: [investigating, root-cause, fix-spec, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "red-fox"), 0755)
	os.WriteFile(filepath.Join(projDir, "red-fox", "spec.yaml"), []byte(content), 0644)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		listStatusFilter = "investigating"
		defer func() { projectFlag = ""; listStatusFilter = "" }()
		listCmd.RunE(listCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "red-fox")
	// blue-bear has status problem-space, should be filtered out.
	if containsString(out, "blue-bear") {
		t.Error("expected blue-bear to be filtered out by --status investigating")
	}
}

func TestListCommand_WithAll(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")
	archDir := filepath.Join(benchDir, "archive", "test-proj")

	writeMinimalSpec(t,
		filepath.Join(projDir, "active-work", "spec.yaml"),
		"active-work", "test-proj")
	writeMinimalSpec(t,
		filepath.Join(archDir, "old-work", "spec.yaml"),
		"old-work", "test-proj")

	// Without --all: should not show archived.
	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		listAll = false
		defer func() { projectFlag = ""; listAll = false }()
		listCmd.RunE(listCmd, []string{})
	})
	testutil.AssertStringContains(t, out, "active-work")
	if containsString(out, "old-work") {
		t.Error("archived work should not appear without --all")
	}

	// With --all: should show both.
	out = captureOutput(t, func() {
		projectFlag = "test-proj"
		listAll = true
		defer func() { projectFlag = ""; listAll = false }()
		listCmd.RunE(listCmd, []string{})
	})
	testutil.AssertStringContains(t, out, "active-work")
	testutil.AssertStringContains(t, out, "old-work")
	testutil.AssertStringContains(t, out, "[archived]")
}

func TestListCommand_Dependencies(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")

	writeMinimalSpec(t,
		filepath.Join(projDir, "dep-target", "spec.yaml"),
		"dep-target", "test-proj")

	// Work with a dependency.
	content := `codename: depends-on-target
type: feature
project:
  id: test-proj
jig: feature
jig_version: 1
status: research
status_values: [problem-space, decomposition, research, detailed-spec, review, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on:
  - codename: dep-target
    relationship: must-complete-first
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "depends-on-target"), 0755)
	os.WriteFile(filepath.Join(projDir, "depends-on-target", "spec.yaml"), []byte(content), 0644)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		listCmd.RunE(listCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Dependencies:")
	testutil.AssertStringContains(t, out, "depends-on-target -> dep-target")
}

// writeSpecWithCreator writes a minimal spec whose first session entry
// (the creator, per specs/sessions.md §"Creator Attribution") has the
// given id. If creatorID is empty the session is recorded with id: null
// (anonymous creator).
func writeSpecWithCreator(t *testing.T, path, codename, projectID, creatorID string) {
	t.Helper()
	idLine := "null"
	if creatorID != "" {
		idLine = creatorID
	}
	content := `codename: ` + codename + `
type: plan
project:
  id: ` + projectID + `
jig: plan
jig_version: 1
status: problem-space
status_values: [problem-space, analyze, decompose, research, change-spec, integration, tasks, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions:
  - id: ` + idLine + `
    started: 2026-04-09T00:00:00Z
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeSpecWithCreator: %v", err)
	}
}

func TestListCommand_CreatedBy_SelfFiltersToMatchingCreator(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("KERF_SESSION_ID", "alice-session")

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")
	writeSpecWithCreator(t,
		filepath.Join(projDir, "mine-work", "spec.yaml"),
		"mine-work", "test-proj", "alice-session")
	writeSpecWithCreator(t,
		filepath.Join(projDir, "their-work", "spec.yaml"),
		"their-work", "test-proj", "bob-session")

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		listCreatedBy = "self"
		defer func() {
			projectFlag = ""
			listCreatedBy = "all"
		}()
		listCmd.RunE(listCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "mine-work")
	if containsString(out, "their-work") {
		t.Errorf("expected their-work to be filtered out, got:\n%s", out)
	}
}

func TestListCommand_CreatedBy_AllShowsAttributionMarkers(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("KERF_SESSION_ID", "alice-session")

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")
	writeSpecWithCreator(t,
		filepath.Join(projDir, "mine-work", "spec.yaml"),
		"mine-work", "test-proj", "alice-session")
	writeSpecWithCreator(t,
		filepath.Join(projDir, "their-work", "spec.yaml"),
		"their-work", "test-proj", "bob-cafebabe-and-more")
	writeSpecWithCreator(t,
		filepath.Join(projDir, "anon-work", "spec.yaml"),
		"anon-work", "test-proj", "")

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		listCreatedBy = "all"
		defer func() {
			projectFlag = ""
			listCreatedBy = "all"
		}()
		listCmd.RunE(listCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "mine-work")
	testutil.AssertStringContains(t, out, "(you)")
	testutil.AssertStringContains(t, out, "their-work")
	testutil.AssertStringContains(t, out, "(by bob-cafe")
	testutil.AssertStringContains(t, out, "anon-work")
	testutil.AssertStringContains(t, out, "(by anon)")
}

func TestListCommand_CreatedBy_SingleAgentBaseline(t *testing.T) {
	// No KERF_SESSION_ID set: --created-by self should match works with
	// anonymous (null id) creator sessions — i.e., the single-agent
	// default with no regression.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("KERF_SESSION_ID", "")

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")
	writeMinimalSpec(t,
		filepath.Join(projDir, "blue-bear", "spec.yaml"),
		"blue-bear", "test-proj")

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		listCreatedBy = "self"
		defer func() {
			projectFlag = ""
			listCreatedBy = "all"
		}()
		listCmd.RunE(listCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "blue-bear")
}

func TestListCommand_CreatedBy_InvalidValueErrors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")
	writeMinimalSpec(t,
		filepath.Join(projDir, "blue-bear", "spec.yaml"),
		"blue-bear", "test-proj")

	projectFlag = "test-proj"
	listCreatedBy = "bogus"
	defer func() {
		projectFlag = ""
		listCreatedBy = "all"
	}()
	err := listCmd.RunE(listCmd, []string{})
	if err == nil {
		t.Fatal("expected error for invalid --created-by value, got nil")
	}
	if !containsString(err.Error(), "invalid --created-by") {
		t.Errorf("expected 'invalid --created-by' in error, got: %v", err)
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
