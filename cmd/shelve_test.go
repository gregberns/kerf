package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/testutil"
)

func TestShelveCommand_WithCodename(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	// Work with active session.
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
  - id: sess-1
    started: 2026-04-09T10:00:00Z
active_session: sess-1
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
		shelveForce = false
		defer func() { projectFlag = ""; shelveForce = false }()
		shelveCmd.RunE(shelveCmd, []string{"blue-bear"})
	})

	testutil.AssertStringContains(t, out, "Work blue-bear shelved.")
	testutil.AssertStringContains(t, out, "SESSION.md")
	testutil.AssertStringContains(t, out, "Path:")

	// Verify session was ended.
	s, err := spec.Read(filepath.Join(projDir, "blue-bear", "spec.yaml"))
	if err != nil {
		t.Fatalf("reading spec: %v", err)
	}
	if s.ActiveSession != nil {
		t.Error("expected active_session to be null after shelve")
	}
	if len(s.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(s.Sessions))
	}
	if s.Sessions[0].Ended == nil {
		t.Error("expected session ended timestamp to be set")
	}
}

func TestShelveCommand_InferActive(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	// One work with active session.
	specContent := `codename: infer-me
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
  - id: sess-1
    started: 2026-04-09T10:00:00Z
active_session: sess-1
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "infer-me"), 0755)
	os.WriteFile(filepath.Join(projDir, "infer-me", "spec.yaml"), []byte(specContent), 0644)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		shelveForce = false
		defer func() { projectFlag = ""; shelveForce = false }()
		shelveCmd.RunE(shelveCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Work infer-me shelved.")
}

func TestShelveCommand_NoActiveSessionError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	// Work with no active session.
	specContent := `codename: idle-work
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
	os.MkdirAll(filepath.Join(projDir, "idle-work"), 0755)
	os.WriteFile(filepath.Join(projDir, "idle-work", "spec.yaml"), []byte(specContent), 0644)

	err := func() error {
		projectFlag = "proj"
		shelveForce = false
		defer func() { projectFlag = ""; shelveForce = false }()
		return shelveCmd.RunE(shelveCmd, []string{"idle-work"})
	}()

	if err == nil {
		t.Error("expected error for work with no active session")
	} else {
		testutil.AssertStringContains(t, err.Error(), "no active session to shelve")
	}
}

func TestShelveCommand_Force(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	specContent := `codename: stale-work
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
  - id: stale-sess
    started: 2026-04-01T10:00:00Z
active_session: stale-sess
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Join(projDir, "stale-work"), 0755)
	os.WriteFile(filepath.Join(projDir, "stale-work", "spec.yaml"), []byte(specContent), 0644)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		shelveForce = true
		defer func() { projectFlag = ""; shelveForce = false }()
		shelveCmd.RunE(shelveCmd, []string{"stale-work"})
	})

	testutil.AssertStringContains(t, out, "force-shelved")
	testutil.AssertStringContains(t, out, "Stale session cleared")

	// Should NOT contain SESSION.md instructions.
	if containsString(out, "Before ending this session") {
		t.Error("force shelve should not emit SESSION.md instructions")
	}

	// Verify session was cleared.
	s, err := spec.Read(filepath.Join(projDir, "stale-work", "spec.yaml"))
	if err != nil {
		t.Fatalf("reading spec: %v", err)
	}
	if s.ActiveSession != nil {
		t.Error("expected active_session to be null after force shelve")
	}
}

func TestShelveCommand_NoActiveInferError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	// Work without active session — infer should fail.
	specContent := `codename: idle
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
	os.MkdirAll(filepath.Join(projDir, "idle"), 0755)
	os.WriteFile(filepath.Join(projDir, "idle", "spec.yaml"), []byte(specContent), 0644)

	err := func() error {
		projectFlag = "proj"
		shelveForce = false
		defer func() { projectFlag = ""; shelveForce = false }()
		return shelveCmd.RunE(shelveCmd, []string{})
	}()

	if err == nil {
		t.Error("expected error when no active session found")
	} else {
		testutil.AssertStringContains(t, err.Error(), "no active session found")
	}
}

func TestShelveCommand_MultipleActiveError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(bp, "projects", "proj")

	// Two works with active sessions.
	for _, cn := range []string{"work-a", "work-b"} {
		specContent := `codename: ` + cn + `
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
  - id: sess-` + cn + `
    started: 2026-04-09T10:00:00Z
active_session: sess-` + cn + `
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
		os.MkdirAll(filepath.Join(projDir, cn), 0755)
		os.WriteFile(filepath.Join(projDir, cn, "spec.yaml"), []byte(specContent), 0644)
	}

	err := func() error {
		projectFlag = "proj"
		shelveForce = false
		defer func() { projectFlag = ""; shelveForce = false }()
		return shelveCmd.RunE(shelveCmd, []string{})
	}()

	if err == nil {
		t.Error("expected error for multiple active sessions")
	} else {
		testutil.AssertStringContains(t, err.Error(), "multiple")
	}
}
