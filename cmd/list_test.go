package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

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

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
