package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/testutil"
)

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
