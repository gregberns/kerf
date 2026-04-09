package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/testutil"
)

// ─── Integration: Full lifecycle ─────────────────────────────────────────────

func TestIntegration_FullLifecycle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "lifecycle-proj"

	// 1. kerf new
	out := captureOutput(t, func() {
		projectFlag = proj
		newJigFlag = ""
		newTitle = "Auth Feature"
		newType = ""
		defer func() { projectFlag = ""; newTitle = "" }()
		newCmd.RunE(newCmd, []string{"auth-rewrite"})
	})
	testutil.AssertStringContains(t, out, "Work created: auth-rewrite")

	workDir := filepath.Join(bp, "projects", proj, "auth-rewrite")
	specPath := filepath.Join(workDir, "spec.yaml")

	// Verify initial state.
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec: %v", err)
	}
	if s.Status != "problem-space" {
		t.Errorf("initial status = %q, want %q", s.Status, "problem-space")
	}
	if len(s.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(s.Sessions))
	}

	// 2. Write artifact files (simulating agent work).
	os.WriteFile(filepath.Join(workDir, "01-problem-space.md"), []byte("# Problem Space\nAuth needs rewriting."), 0644)
	os.WriteFile(filepath.Join(workDir, "SESSION.md"), []byte("# Session State\n\n## Current Pass\nProblem Space — complete\n"), 0644)

	// 3. Advance status.
	out = captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		statusCmd.RunE(statusCmd, []string{"auth-rewrite", "decomposition"})
	})
	testutil.AssertStringContains(t, out, "Status updated: problem-space -> decomposition")

	s, _ = spec.Read(specPath)
	if s.Status != "decomposition" {
		t.Errorf("status after update = %q, want %q", s.Status, "decomposition")
	}

	// 4. Shelve.
	out = captureOutput(t, func() {
		projectFlag = proj
		shelveForce = false
		defer func() { projectFlag = ""; shelveForce = false }()
		shelveCmd.RunE(shelveCmd, []string{"auth-rewrite"})
	})
	testutil.AssertStringContains(t, out, "Work auth-rewrite shelved.")

	s, _ = spec.Read(specPath)
	if s.ActiveSession != nil {
		t.Error("active_session should be nil after shelve")
	}

	// 5. Resume.
	out = captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		resumeCmd.RunE(resumeCmd, []string{"auth-rewrite"})
	})
	testutil.AssertStringContains(t, out, "Resuming work: auth-rewrite")
	testutil.AssertStringContains(t, out, "SESSION.md:")
	testutil.AssertStringContains(t, out, "Problem Space")

	s, _ = spec.Read(specPath)
	if s.ActiveSession == nil {
		t.Error("active_session should be set after resume")
	}
	if len(s.Sessions) != 2 {
		t.Errorf("expected 2 sessions (new + resume), got %d", len(s.Sessions))
	}

	// 6. kerf show.
	out = captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		showCmd.RunE(showCmd, []string{"auth-rewrite"})
	})
	testutil.AssertStringContains(t, out, "Work: auth-rewrite")
	testutil.AssertStringContains(t, out, "Status: decomposition")
	testutil.AssertStringContains(t, out, "Files:")
	testutil.AssertStringContains(t, out, "Sessions:")
	testutil.AssertStringContains(t, out, "SESSION.md:")

	// 7. kerf square (should be NOT SQUARE — status not at terminal).
	out = captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		squareCmd.RunE(squareCmd, []string{"auth-rewrite"})
	})
	testutil.AssertStringContains(t, out, "NOT SQUARE")
	testutil.AssertStringContains(t, out, "Status:        fail")
}

// ─── Integration: status command ─────────────────────────────────────────────

func TestIntegration_StatusRead(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "status-proj"
	workDir := filepath.Join(bp, "projects", proj, "my-work")
	os.MkdirAll(workDir, 0755)
	writeMinimalSpec(t, filepath.Join(workDir, "spec.yaml"), "my-work", proj)

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		statusCmd.RunE(statusCmd, []string{"my-work"})
	})

	testutil.AssertStringContains(t, out, "Work: my-work")
	testutil.AssertStringContains(t, out, "Status: problem-space")
	testutil.AssertStringContains(t, out, "Status progression")
}

func TestIntegration_StatusWriteNonRecommended(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "status-proj"
	workDir := filepath.Join(bp, "projects", proj, "my-work")
	os.MkdirAll(workDir, 0755)
	writeMinimalSpec(t, filepath.Join(workDir, "spec.yaml"), "my-work", proj)

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		statusCmd.RunE(statusCmd, []string{"my-work", "custom-status"})
	})

	testutil.AssertStringContains(t, out, "Warning:")
	testutil.AssertStringContains(t, out, "not in the feature jig's recommended statuses")
	testutil.AssertStringContains(t, out, "Status updated: problem-space -> custom-status")

	s, _ := spec.Read(filepath.Join(workDir, "spec.yaml"))
	if s.Status != "custom-status" {
		t.Errorf("status = %q, want %q", s.Status, "custom-status")
	}
}

// ─── Integration: show command ───────────────────────────────────────────────

func TestIntegration_ShowWithDeps(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "dep-proj"

	// Create dep target.
	depDir := filepath.Join(bp, "projects", proj, "dep-target")
	os.MkdirAll(depDir, 0755)
	depSpec := `codename: dep-target
type: feature
project:
  id: dep-proj
jig: feature
jig_version: 1
status: ready
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

	// Create work with dependency.
	workDir := filepath.Join(bp, "projects", proj, "main-work")
	os.MkdirAll(workDir, 0755)
	workSpec := `codename: main-work
type: feature
project:
  id: dep-proj
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
	os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(workSpec), 0644)

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		showCmd.RunE(showCmd, []string{"main-work"})
	})

	testutil.AssertStringContains(t, out, "Dependencies:")
	testutil.AssertStringContains(t, out, "dep-target")
	testutil.AssertStringContains(t, out, "must-complete-first")
	testutil.AssertStringContains(t, out, "ready")
}

// ─── Integration: square command ─────────────────────────────────────────────

func TestIntegration_SquarePass(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "sq-proj"

	// Create a work at terminal status with all files present (bug jig — no components).
	workDir := filepath.Join(bp, "projects", proj, "fixed-bug")
	os.MkdirAll(workDir, 0755)
	workSpec := `codename: fixed-bug
type: bug
project:
  id: sq-proj
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

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		squareCmd.RunE(squareCmd, []string{"fixed-bug"})
	})

	testutil.AssertStringContains(t, out, "SQUARE")
	testutil.AssertStringContains(t, out, "Status:        pass")
	testutil.AssertStringContains(t, out, "Files:         pass")
	// Should not contain "NOT SQUARE"
	if strings.Contains(out, "NOT SQUARE") {
		t.Error("expected SQUARE, got NOT SQUARE")
	}
}

func TestIntegration_SquareFailMissingFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "sq-proj"
	workDir := filepath.Join(bp, "projects", proj, "incomplete")
	os.MkdirAll(workDir, 0755)
	workSpec := `codename: incomplete
type: bug
project:
  id: sq-proj
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
	// Only spec.yaml — missing all other files.

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		squareCmd.RunE(squareCmd, []string{"incomplete"})
	})

	testutil.AssertStringContains(t, out, "NOT SQUARE")
	testutil.AssertStringContains(t, out, "Files:         fail")
	testutil.AssertStringContains(t, out, "Missing:")
}

func TestIntegration_SquareFailIncompleteDeps(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "dep-sq"

	// Create an incomplete dependency.
	depDir := filepath.Join(bp, "projects", proj, "blocker")
	os.MkdirAll(depDir, 0755)
	depSpec := `codename: blocker
type: feature
project:
  id: dep-sq
jig: feature
jig_version: 1
status: research
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

	// Create work depending on it (bug jig, all files present, at terminal).
	workDir := filepath.Join(bp, "projects", proj, "my-bug")
	os.MkdirAll(workDir, 0755)
	workSpec := `codename: my-bug
type: bug
project:
  id: dep-sq
jig: bug
jig_version: 1
status: ready
status_values: [triaging, reproducing, locating, specifying-fix, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on:
  - codename: blocker
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
		squareCmd.RunE(squareCmd, []string{"my-bug"})
	})

	testutil.AssertStringContains(t, out, "NOT SQUARE")
	testutil.AssertStringContains(t, out, "Dependencies:  fail")
	testutil.AssertStringContains(t, out, "Incomplete:")
	testutil.AssertStringContains(t, out, "blocker")
}

func TestIntegration_SquareUnresolvableDeps(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "unres"
	workDir := filepath.Join(bp, "projects", proj, "my-work")
	os.MkdirAll(workDir, 0755)

	// Bug jig work at ready, all files, but dep on nonexistent work.
	workSpec := `codename: my-work
type: bug
project:
  id: unres
jig: bug
jig_version: 1
status: ready
status_values: [triaging, reproducing, locating, specifying-fix, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on:
  - codename: ghost
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
		squareCmd.RunE(squareCmd, []string{"my-work"})
	})

	// Unresolvable deps don't cause failure.
	testutil.AssertStringContains(t, out, "SQUARE")
	testutil.AssertStringContains(t, out, "Unresolvable:")
	if strings.Contains(out, "NOT SQUARE") {
		t.Error("unresolvable deps should not fail the check")
	}
}

// ─── Integration: Multi-work ─────────────────────────────────────────────────

func TestIntegration_MultiWorkSameProject(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	proj := "multi-proj"

	// Create two works.
	for _, cn := range []string{"work-alpha", "work-beta"} {
		captureOutput(t, func() {
			projectFlag = proj
			newJigFlag = ""
			newTitle = ""
			newType = ""
			defer func() { projectFlag = "" }()
			newCmd.RunE(newCmd, []string{cn})
		})
	}

	bp := filepath.Join(tmp, ".kerf")

	// Both should exist.
	for _, cn := range []string{"work-alpha", "work-beta"} {
		specPath := filepath.Join(bp, "projects", proj, cn, "spec.yaml")
		testutil.AssertFileExists(t, specPath)
	}

	// Shelve work-alpha (it has active session from new).
	captureOutput(t, func() {
		projectFlag = proj
		shelveForce = false
		defer func() { projectFlag = ""; shelveForce = false }()
		shelveCmd.RunE(shelveCmd, []string{"work-alpha"})
	})

	// Shelve work-beta.
	captureOutput(t, func() {
		projectFlag = proj
		shelveForce = false
		defer func() { projectFlag = ""; shelveForce = false }()
		shelveCmd.RunE(shelveCmd, []string{"work-beta"})
	})

	// Resume work-alpha — should succeed.
	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		resumeCmd.RunE(resumeCmd, []string{"work-alpha"})
	})
	testutil.AssertStringContains(t, out, "Resuming work: work-alpha")

	// Show work-beta — should still be accessible.
	out = captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		showCmd.RunE(showCmd, []string{"work-beta"})
	})
	testutil.AssertStringContains(t, out, "Work: work-beta")
}

// ─── Integration: Config interactions ────────────────────────────────────────

func TestIntegration_ConfigDefaultJig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	os.MkdirAll(bp, 0755)

	// Set default_jig to bug.
	configContent := "default_jig: bug\n"
	os.WriteFile(filepath.Join(bp, "config.yaml"), []byte(configContent), 0644)

	out := captureOutput(t, func() {
		projectFlag = "cfg-proj"
		newJigFlag = "" // should use config default.
		newTitle = ""
		newType = ""
		defer func() { projectFlag = "" }()
		newCmd.RunE(newCmd, []string{"cfg-work"})
	})

	testutil.AssertStringContains(t, out, "Jig:      bug")

	s, err := spec.Read(filepath.Join(bp, "projects", "cfg-proj", "cfg-work", "spec.yaml"))
	if err != nil {
		t.Fatalf("reading spec: %v", err)
	}
	if s.Jig != "bug" {
		t.Errorf("jig = %q, want %q", s.Jig, "bug")
	}
}

// ─── Integration: Snapshot on status change ──────────────────────────────────

func TestIntegration_StatusWriteCreatesSnapshot(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "snap-proj"
	workDir := filepath.Join(bp, "projects", proj, "snap-work")
	os.MkdirAll(workDir, 0755)
	writeMinimalSpec(t, filepath.Join(workDir, "spec.yaml"), "snap-work", proj)

	captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		statusCmd.RunE(statusCmd, []string{"snap-work", "decomposition"})
	})

	// Check .history/ was created with a snapshot.
	histDir := filepath.Join(workDir, ".history")
	entries, err := os.ReadDir(histDir)
	if err != nil {
		t.Fatalf("reading .history: %v", err)
	}
	if len(entries) < 1 {
		t.Error("expected at least one snapshot after status change")
	}
}
