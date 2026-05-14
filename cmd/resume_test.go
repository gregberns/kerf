package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/testutil"
)

func TestResumeCommand_RetrofitHint_Dirty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := setupGitRepoForTest(t)
	if err := os.WriteFile(filepath.Join(repo, "stray.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	chdirT(t, repo)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")
	specContent := `codename: blue-bear
type: feature
project:
  id: proj
jig: feature
jig_version: 1
status: research
status_values: [problem-space, decomposition, research, detailed-spec, review, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
active_session: null
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "blue-bear"), 0755)
	os.WriteFile(filepath.Join(projDir, "blue-bear", "spec.yaml"), []byte(specContent), 0644)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		defer func() { projectFlag = "" }()
		resumeCmd.RunE(resumeCmd, []string{"blue-bear"})
	})

	testutil.AssertStringContains(t, out, "kerf new --jig retrofit")
}

func TestResumeCommand_RetrofitHint_Clean(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := setupGitRepoForTest(t)
	chdirT(t, repo)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")
	specContent := `codename: blue-bear
type: feature
project:
  id: proj
jig: feature
jig_version: 1
status: research
status_values: [problem-space, decomposition, research, detailed-spec, review, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
active_session: null
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "blue-bear"), 0755)
	os.WriteFile(filepath.Join(projDir, "blue-bear", "spec.yaml"), []byte(specContent), 0644)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		defer func() { projectFlag = "" }()
		resumeCmd.RunE(resumeCmd, []string{"blue-bear"})
	})

	if strings.Contains(out, "kerf new --jig retrofit") {
		t.Errorf("did not expect retrofit hint for clean repo, got:\n%s", out)
	}
}

func TestResumeCommand_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	// Create a shelved work (no active session).
	specContent := `codename: blue-bear
type: feature
project:
  id: proj
jig: feature
jig_version: 1
status: research
status_values: [problem-space, decomposition, research, detailed-spec, review, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions:
  - id: old-sess
    started: 2026-04-08T10:00:00Z
    ended: 2026-04-08T16:00:00Z
active_session: null
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "blue-bear"), 0755)
	os.WriteFile(filepath.Join(projDir, "blue-bear", "spec.yaml"), []byte(specContent), 0644)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		defer func() { projectFlag = "" }()
		resumeCmd.RunE(resumeCmd, []string{"blue-bear"})
	})

	testutil.AssertStringContains(t, out, "Resuming work: blue-bear")
	testutil.AssertStringContains(t, out, "Status: research")
	testutil.AssertStringContains(t, out, "SESSION.md not found")
	testutil.AssertStringContains(t, out, "Next steps:")

	// Verify session was recorded.
	s, err := spec.Read(filepath.Join(projDir, "blue-bear", "spec.yaml"))
	if err != nil {
		t.Fatalf("reading spec: %v", err)
	}
	if len(s.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(s.Sessions))
	}
	if s.ActiveSession == nil {
		t.Error("expected active_session to be set")
	}
}

func TestResumeCommand_WithSessionMD(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	specContent := `codename: red-fox
type: feature
project:
  id: proj
jig: feature
jig_version: 1
status: decomposition
status_values: [problem-space, decomposition, research, detailed-spec, review, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
active_session: null
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	workDir := filepath.Join(projDir, "red-fox")
	os.MkdirAll(workDir, 0755)
	os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(specContent), 0644)
	os.WriteFile(filepath.Join(workDir, "SESSION.md"), []byte("# Session State\n\n## Current Pass\nDecomposition — in progress\n"), 0644)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		defer func() { projectFlag = "" }()
		resumeCmd.RunE(resumeCmd, []string{"red-fox"})
	})

	testutil.AssertStringContains(t, out, "SESSION.md:")
	testutil.AssertStringContains(t, out, "Decomposition")
}

func TestResumeCommand_ActiveSessionError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	// Work with an active session.
	specContent := `codename: active-work
type: feature
project:
  id: proj
jig: feature
jig_version: 1
status: research
status_values: [problem-space, decomposition, research, detailed-spec, review, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions:
  - id: active-sess
    started: 2026-04-09T10:00:00Z
active_session: active-sess
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "active-work"), 0755)
	os.WriteFile(filepath.Join(projDir, "active-work", "spec.yaml"), []byte(specContent), 0644)

	err := func() error {
		projectFlag = "proj"
		defer func() { projectFlag = "" }()
		return resumeCmd.RunE(resumeCmd, []string{"active-work"})
	}()

	if err == nil {
		t.Error("expected error for work with active session")
	} else {
		testutil.AssertStringContains(t, err.Error(), "has an active session")
		testutil.AssertStringContains(t, err.Error(), "kerf shelve")
	}
}

func TestResumeCommand_WithJigChain(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	// Create a shelved work.
	specContent := `codename: green-owl
type: feature
project:
  id: proj
jig: plan
jig_version: 1
status: research
status_values: [problem-space, decomposition, research, detailed-spec, review, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
active_session: null
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "green-owl"), 0755)
	os.WriteFile(filepath.Join(projDir, "green-owl", "spec.yaml"), []byte(specContent), 0644)

	// Create project.yaml with active jigs including a composable one.
	projConfig := `jigs:
  - plan
  - implementation
passes:
  implementation:
    - breakdown
    - implement
`
	os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte(projConfig), 0644)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		defer func() { projectFlag = "" }()
		resumeCmd.RunE(resumeCmd, []string{"green-owl"})
	})

	testutil.AssertStringContains(t, out, "Active jig chain:")
	testutil.AssertStringContains(t, out, "This project uses:")
	testutil.AssertStringContains(t, out, "plan")
	testutil.AssertStringContains(t, out, "implementation (breakdown, implement)")
}

func TestResumeCommand_NoJigChainWithoutProjectConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	specContent := `codename: gray-cat
type: feature
project:
  id: proj
jig: plan
jig_version: 1
status: research
status_values: [problem-space, decomposition, research, detailed-spec, review, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
active_session: null
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "gray-cat"), 0755)
	os.WriteFile(filepath.Join(projDir, "gray-cat", "spec.yaml"), []byte(specContent), 0644)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		defer func() { projectFlag = "" }()
		resumeCmd.RunE(resumeCmd, []string{"gray-cat"})
	})

	// No project.yaml → no jig chain section.
	if strings.Contains(out, "Active jig chain:") {
		t.Error("expected no jig chain without project.yaml")
	}
}

func TestResumeCommand_AreaOverlap(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	// First work (active, in "adapter" area) — should appear in overlap output.
	otherSpec := `codename: bold-crane
type: feature
project:
  id: proj
jig: feature
jig_version: 1
status: research
status_values: [problem-space, decomposition, research, detailed-spec, review, ready]
areas: [adapter]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
active_session: null
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "bold-crane"), 0755)
	os.WriteFile(filepath.Join(projDir, "bold-crane", "spec.yaml"), []byte(otherSpec), 0644)

	// Target work to resume (shelved, shares the "adapter" area).
	targetSpec := `codename: green-oak
type: feature
project:
  id: proj
jig: feature
jig_version: 1
status: research
status_values: [problem-space, decomposition, research, detailed-spec, review, ready]
areas: [adapter]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
active_session: null
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "green-oak"), 0755)
	os.WriteFile(filepath.Join(projDir, "green-oak", "spec.yaml"), []byte(targetSpec), 0644)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		defer func() { projectFlag = "" }()
		resumeCmd.RunE(resumeCmd, []string{"green-oak"})
	})

	testutil.AssertStringContains(t, out, "Area overlap:")
	testutil.AssertStringContains(t, out, "adapter")
	testutil.AssertStringContains(t, out, "bold-crane")
}

func TestResumeCommand_WorkNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	os.MkdirAll(filepath.Join(tmp, ".kerf", "projects", "proj"), 0755)

	err := func() error {
		projectFlag = "proj"
		defer func() { projectFlag = "" }()
		return resumeCmd.RunE(resumeCmd, []string{"nonexistent"})
	}()

	if err == nil {
		t.Error("expected error for nonexistent work")
	} else {
		testutil.AssertStringContains(t, err.Error(), "not found")
	}
}
