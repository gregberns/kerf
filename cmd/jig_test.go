package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/kerf/internal/testutil"
)

func TestJigListCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	out := captureOutput(t, func() {
		jigListCmd.RunE(jigListCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Available jigs:")
	testutil.AssertStringContains(t, out, "plan (also: feature)")
	testutil.AssertStringContains(t, out, "spec")
	testutil.AssertStringContains(t, out, "bug")
	testutil.AssertStringContains(t, out, "built-in")
	// Phase column is shown.
	testutil.AssertStringContains(t, out, "planning")
	testutil.AssertStringContains(t, out, "implementation")
	// Composable jig shows passes.
	testutil.AssertStringContains(t, out, "Passes:")
	// Implementation jig shows tools.
	testutil.AssertStringContains(t, out, "Tools: br, ntm, agent-mail")
}

func TestJigListCommand_MixedSources(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	jigsDir := filepath.Join(tmp, ".kerf", "jigs")
	os.MkdirAll(jigsDir, 0755)

	// Create a user override for plan jig.
	userContent := `---
name: plan
description: Custom plan
version: 99
status_values: [a, b]
passes:
  - name: "A"
    status: a
    output: ["a.md"]
---

# Custom
`
	os.WriteFile(filepath.Join(jigsDir, "plan.md"), []byte(userContent), 0644)

	out := captureOutput(t, func() {
		jigListCmd.RunE(jigListCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "plan")
	testutil.AssertStringContains(t, out, "user")
	testutil.AssertStringContains(t, out, "bug")
}

func TestJigShowCommand_Builtin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	out := captureOutput(t, func() {
		jigShowCmd.RunE(jigShowCmd, []string{"plan"})
	})

	testutil.AssertStringContains(t, out, "Jig: plan")
	testutil.AssertStringContains(t, out, "Status values:")
	testutil.AssertStringContains(t, out, "Passes:")
	testutil.AssertStringContains(t, out, "Problem Space")
	testutil.AssertStringContains(t, out, "File structure:")
}

func TestJigShowCommand_Alias(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	out := captureOutput(t, func() {
		jigShowCmd.RunE(jigShowCmd, []string{"feature"})
	})

	// "feature" resolves to "plan" via alias
	testutil.AssertStringContains(t, out, "Jig: plan")
}

func TestJigShowCommand_NotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	err := jigShowCmd.RunE(jigShowCmd, []string{"nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent jig")
	}
}

func TestJigSaveCommand_FromBuiltin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	out := captureOutput(t, func() {
		jigSaveFrom = ""
		jigSaveCmd.RunE(jigSaveCmd, []string{"plan"})
	})

	testutil.AssertStringContains(t, out, "Jig 'plan' saved to")
	testutil.AssertFileExists(t, filepath.Join(tmp, ".kerf", "jigs", "plan.md"))
}

func TestJigSaveCommand_FromAlias(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	out := captureOutput(t, func() {
		jigSaveFrom = ""
		jigSaveCmd.RunE(jigSaveCmd, []string{"feature"})
	})

	// "feature" alias resolves to plan jig content via ReadBuiltinRaw
	testutil.AssertStringContains(t, out, "Jig 'feature' saved to")
	testutil.AssertFileExists(t, filepath.Join(tmp, ".kerf", "jigs", "feature.md"))
}

func TestJigSaveCommand_FromFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	// Write a custom jig file.
	customJig := `---
name: custom
description: A custom jig
version: 1
status_values: [start, end]
passes:
  - name: "Start"
    status: start
    output: ["out.md"]
---

# Custom
`
	customPath := filepath.Join(tmp, "custom.md")
	os.WriteFile(customPath, []byte(customJig), 0644)

	out := captureOutput(t, func() {
		jigSaveFrom = customPath
		defer func() { jigSaveFrom = "" }()
		jigSaveCmd.RunE(jigSaveCmd, []string{"custom"})
	})

	testutil.AssertStringContains(t, out, "Jig 'custom' saved to")
	testutil.AssertFileExists(t, filepath.Join(tmp, ".kerf", "jigs", "custom.md"))
}

func TestJigLoadCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	// Write a jig file to load.
	jigContent := `---
name: loaded
description: A loaded jig
version: 1
status_values: [a, b]
passes:
  - name: "A"
    status: a
    output: ["a.md"]
---

# Loaded
`
	srcPath := filepath.Join(tmp, "loaded.md")
	os.WriteFile(srcPath, []byte(jigContent), 0644)

	out := captureOutput(t, func() {
		jigLoadCmd.RunE(jigLoadCmd, []string{"loaded", srcPath})
	})

	testutil.AssertStringContains(t, out, "Jig 'loaded' loaded from")
	testutil.AssertFileExists(t, filepath.Join(tmp, ".kerf", "jigs", "loaded.md"))
}

func TestJigLoadCommand_InvalidFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	badPath := filepath.Join(tmp, "bad.md")
	os.WriteFile(badPath, []byte("not a jig"), 0644)

	err := jigLoadCmd.RunE(jigLoadCmd, []string{"bad", badPath})
	if err == nil {
		t.Error("expected error for invalid jig file")
	}
}

func TestJigSyncCommand(t *testing.T) {
	out := captureOutput(t, func() {
		jigSyncCmd.RunE(jigSyncCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Jig sync is not yet available.")
}

func TestJigListCommand_PhaseFilter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	// Filter to implementation phase.
	oldPhase := jigPhaseFilter
	jigPhaseFilter = "implementation"
	defer func() { jigPhaseFilter = oldPhase }()

	out := captureOutput(t, func() {
		jigListCmd.RunE(jigListCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "implementation")
	// Should not contain planning-phase jigs.
	if strings.Contains(out, "planning") {
		t.Errorf("output should not contain planning-phase jigs when filtered to implementation, got:\n%s", out)
	}
}

func TestJigListCommand_PhaseFilterNoMatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	oldPhase := jigPhaseFilter
	jigPhaseFilter = "nonexistent-phase"
	defer func() { jigPhaseFilter = oldPhase }()

	out := captureOutput(t, func() {
		jigListCmd.RunE(jigListCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "No jigs available.")
}

func TestJigListCommand_WithProjectConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create bench directory.
	benchDir := filepath.Join(tmp, ".kerf")
	os.MkdirAll(benchDir, 0755)

	// Create a git repo so project resolution works.
	repoDir := filepath.Join(tmp, "repo")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.MkdirAll(filepath.Join(repoDir, ".kerf"), 0755)
	os.WriteFile(filepath.Join(repoDir, ".kerf", "project-identifier"), []byte("test-proj"), 0644)

	// Create project.yaml with active jigs.
	projDir := filepath.Join(benchDir, "projects", "test-proj")
	os.MkdirAll(projDir, 0755)
	projConfig := `jigs:
  - plan
  - implementation
passes:
  implementation:
    - breakdown
    - implement
`
	os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte(projConfig), 0644)

	// Change to repo dir so project resolution finds it.
	oldDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(oldDir)

	oldPhase := jigPhaseFilter
	jigPhaseFilter = ""
	defer func() { jigPhaseFilter = oldPhase }()

	out := captureOutput(t, func() {
		jigListCmd.RunE(jigListCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Jigs for test-proj:")
	testutil.AssertStringContains(t, out, "Active:")
	testutil.AssertStringContains(t, out, "Available (not active):")
	// Plan should be in Active section.
	testutil.AssertStringContains(t, out, "plan")
	// Composable jig with custom passes.
	testutil.AssertStringContains(t, out, "Passes: breakdown, implement")
}
