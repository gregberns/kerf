package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

func TestComputePassStatus(t *testing.T) {
	statusValues := []string{"breakdown", "dispatch", "implement", "review", "squared"}

	tests := []struct {
		name          string
		currentStatus string
		passStatus    string
		want          string
	}{
		{"past pass is done", "implement", "breakdown", "done"},
		{"current pass is active", "implement", "implement", "active"},
		{"future pass is pending", "implement", "review", "pending"},
		{"first pass active", "breakdown", "breakdown", "active"},
		{"last pass active", "squared", "squared", "active"},
		{"all done when past terminal", "finalized", "squared", "done"},
		{"unknown pass status", "implement", "nonexistent", "unknown"},
		{"first pass done when second active", "dispatch", "breakdown", "done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computePassStatus(statusValues, tt.currentStatus, tt.passStatus)
			if got != tt.want {
				t.Errorf("computePassStatus(%v, %q, %q) = %q, want %q",
					statusValues, tt.currentStatus, tt.passStatus, got, tt.want)
			}
		})
	}
}

func TestComputePassStatus_EmptyStatusValues(t *testing.T) {
	got := computePassStatus([]string{}, "anything", "anything")
	if got != "unknown" {
		t.Errorf("got %q, want %q", got, "unknown")
	}
}

func TestShowComposableJig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "show-test-proj"

	// Create a work with the implementation jig
	out := captureOutput(t, func() {
		projectFlag = proj
		newJigFlag = "implementation"
		newTitle = "Build parser"
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = ""; newTitle = "" }()
		newCmd.RunE(newCmd, []string{"build-parser"})
	})
	testutil.AssertStringContains(t, out, "Work created: build-parser")

	workDir := filepath.Join(bp, "projects", proj, "build-parser")
	os.WriteFile(filepath.Join(workDir, "SESSION.md"), []byte("# Session\n"), 0644)

	// Advance to dispatch so breakdown is "done"
	captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		statusCmd.RunE(statusCmd, []string{"build-parser", "dispatch"})
	})

	// Now run show
	out = captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		showCmd.RunE(showCmd, []string{"build-parser"})
	})

	testutil.AssertStringContains(t, out, "Pass status:")
	testutil.AssertStringContains(t, out, "done")
	testutil.AssertStringContains(t, out, "active")
	testutil.AssertStringContains(t, out, "pending")
}

func TestShowNonComposableJig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	proj := "show-nocomp-proj"

	// Create a work with the bug jig (not composable)
	captureOutput(t, func() {
		projectFlag = proj
		newJigFlag = "bug"
		newTitle = "Fix crash"
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = ""; newTitle = "" }()
		newCmd.RunE(newCmd, []string{"fix-crash"})
	})

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		showCmd.RunE(showCmd, []string{"fix-crash"})
	})

	// Should NOT contain pass status section for non-composable jigs
	if containsString(out, "Pass status:") {
		t.Error("non-composable jig should not show Pass status section")
	}
}

func TestGetBeadSummary_NoBr(t *testing.T) {
	// br may or may not be installed in test env; if no beads exist for the
	// (nonexistent) test project, the function returns empty string silently.
	got := getBeadSummary("any-project")
	if got != "" {
		t.Errorf("expected empty string when beads tool unavailable or no beads, got %q", got)
	}
}

// TestShow_BeadsAttached_RendersCounts verifies that getBeadSummary, when the
// configured beads tool (`br`) is available and returns beads, renders a
// "Beads: N total, C closed, O open" summary line. The implementation must
// reach beads via internal/beads.ListNamed (no direct exec.Command shell-out
// in cmd/show.go) — Plan 008 / Bead 1.
func TestShow_BeadsAttached_RendersCounts(t *testing.T) {
	stubBr(t, `[
		{"id":"x-1","status":"open","labels":[]},
		{"id":"x-2","status":"closed","labels":[]},
		{"id":"x-3","status":"done","labels":[]},
		{"id":"x-4","status":"in-progress","labels":[]}
	]`)

	got := getBeadSummary("any-project")
	// 4 total: 2 terminal (closed+done) + 2 non-terminal (open, in-progress)
	want := "Beads: 4 total, 2 closed, 2 open"
	if got != want {
		t.Errorf("getBeadSummary = %q, want %q", got, want)
	}
}

// TestShow_BeadToolUnavailable_DegradesGracefully verifies that when the
// beads CLI is not on PATH, getBeadSummary returns "" (no error, no panic,
// no partial output). The caller in runShow then simply omits the line.
func TestShow_BeadToolUnavailable_DegradesGracefully(t *testing.T) {
	// Point PATH at an empty dir so `br` cannot be resolved.
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	got := getBeadSummary("any-project")
	if got != "" {
		t.Errorf("expected empty summary when br unavailable, got %q", got)
	}
}
