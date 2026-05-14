package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

func TestNextCommand_EmptyProject(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	os.MkdirAll(filepath.Join(benchDir, "projects", "test-proj"), 0755)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		nextCmd.RunE(nextCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "No actionable works")
}

func TestNextCommand_SingleWork(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")

	writeSpecWithAreas(t,
		filepath.Join(projDir, "blue-fox", "spec.yaml"),
		"blue-fox", "test-proj", "research", "Auth rewrite", []string{"api"})

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		nextCmd.RunE(nextCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Next actions for test-proj")
	testutil.AssertStringContains(t, out, "1.")
	testutil.AssertStringContains(t, out, "blue-fox")
	testutil.AssertStringContains(t, out, "research")
	testutil.AssertStringContains(t, out, "Auth rewrite")
}

func TestNextCommand_MultipleWorksOrderedByScore(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")

	// alpha is a dependency of beta (must-complete-first), so alpha should
	// score higher due to fan-out.
	writeSpecWithAreas(t,
		filepath.Join(projDir, "alpha", "spec.yaml"),
		"alpha", "test-proj", "research", "Foundation work", []string{"api", "database"})
	writeSpecWithDep(t,
		filepath.Join(projDir, "beta", "spec.yaml"),
		"beta", "test-proj", "spec", "Dependent work", "alpha")
	writeSpecWithAreas(t,
		filepath.Join(projDir, "gamma", "spec.yaml"),
		"gamma", "test-proj", "research", "Independent work", nil)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		nextCmd.RunE(nextCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Next actions for test-proj")

	// alpha should appear before gamma because it unblocks beta.
	// beta should not appear because its must-complete-first dep (alpha) is not met.
	alphaIdx := strings.Index(out, "alpha")
	gammaIdx := strings.Index(out, "gamma")

	if alphaIdx < 0 {
		t.Fatal("alpha should appear in output")
	}
	if gammaIdx < 0 {
		t.Fatal("gamma should appear in output")
	}
	if alphaIdx > gammaIdx {
		t.Errorf("alpha should appear before gamma (alpha at %d, gamma at %d)", alphaIdx, gammaIdx)
	}

	// beta should NOT appear because its dependency is unmet.
	betaIdx := strings.Index(out, "beta")
	if betaIdx >= 0 {
		t.Errorf("beta should not appear in output (blocked by unmet dependency on alpha)")
	}

	// alpha should show unblocks reason.
	testutil.AssertStringContains(t, out, "unblocks")
}

func TestNextCommand_AreaFilter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")

	writeSpecWithAreas(t,
		filepath.Join(projDir, "blue-fox", "spec.yaml"),
		"blue-fox", "test-proj", "research", "Auth rewrite", []string{"api", "database"})
	writeSpecWithAreas(t,
		filepath.Join(projDir, "red-elk", "spec.yaml"),
		"red-elk", "test-proj", "spec", "Rate limiting", []string{"api"})
	writeSpecWithAreas(t,
		filepath.Join(projDir, "green-owl", "spec.yaml"),
		"green-owl", "test-proj", "research", "Schema migration", []string{"database"})

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		nextArea = "database"
		defer func() {
			projectFlag = ""
			nextArea = ""
		}()
		nextCmd.RunE(nextCmd, []string{})
	})

	// database filter: blue-fox and green-owl should appear, red-elk should not.
	testutil.AssertStringContains(t, out, "blue-fox")
	testutil.AssertStringContains(t, out, "green-owl")
	if strings.Contains(out, "red-elk") {
		t.Error("red-elk should not appear when filtering by database area")
	}
}

func TestNextCommand_BlockedDepsExcluded(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")

	// alpha is active and has no deps — should appear.
	writeSpecWithAreas(t,
		filepath.Join(projDir, "alpha", "spec.yaml"),
		"alpha", "test-proj", "research", "First work", nil)

	// beta depends on alpha (must-complete-first) — alpha is not at terminal
	// status, so beta should NOT appear.
	writeSpecWithDep(t,
		filepath.Join(projDir, "beta", "spec.yaml"),
		"beta", "test-proj", "spec", "Blocked work", "alpha")

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		nextCmd.RunE(nextCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "alpha")
	if strings.Contains(out, "beta") {
		t.Error("beta should not appear — blocked by unmet must-complete-first dependency on alpha")
	}
}

func TestNextCommand_LimitFlag(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")

	writeSpecWithAreas(t,
		filepath.Join(projDir, "alpha", "spec.yaml"),
		"alpha", "test-proj", "research", "First", nil)
	writeSpecWithAreas(t,
		filepath.Join(projDir, "beta", "spec.yaml"),
		"beta", "test-proj", "research", "Second", nil)
	writeSpecWithAreas(t,
		filepath.Join(projDir, "gamma", "spec.yaml"),
		"gamma", "test-proj", "research", "Third", nil)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		nextLimit = 1
		defer func() {
			projectFlag = ""
			nextLimit = 0
		}()
		nextCmd.RunE(nextCmd, []string{})
	})

	// Should show "1." but not "2." or "3."
	testutil.AssertStringContains(t, out, "1.")
	if strings.Contains(out, "2.") {
		t.Error("should only show 1 result with --limit 1")
	}
}

func TestNextCommand_TerminalWorksExcluded(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")

	// Create one work at terminal status ("ready" is the last status_values entry).
	writeSpecWithAreas(t,
		filepath.Join(projDir, "done-work", "spec.yaml"),
		"done-work", "test-proj", "ready", "Finished thing", nil)

	// Create one work that is still active.
	writeSpecWithAreas(t,
		filepath.Join(projDir, "active-work", "spec.yaml"),
		"active-work", "test-proj", "research", "Active thing", nil)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		nextCmd.RunE(nextCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "active-work")
	if strings.Contains(out, "done-work") {
		t.Error("done-work at terminal status should not appear")
	}
}
