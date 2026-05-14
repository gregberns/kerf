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

func TestGetBeadSummary_NoBd(t *testing.T) {
	// bd is not installed in test env — should return empty string silently
	got := getBeadSummary("any-project")
	if got != "" {
		t.Errorf("expected empty string when bd unavailable, got %q", got)
	}
}
