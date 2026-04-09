package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

func TestJigListCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	out := captureOutput(t, func() {
		jigListCmd.RunE(jigListCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Available jigs:")
	testutil.AssertStringContains(t, out, "feature")
	testutil.AssertStringContains(t, out, "bug")
	testutil.AssertStringContains(t, out, "built-in")
}

func TestJigListCommand_MixedSources(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	jigsDir := filepath.Join(tmp, ".kerf", "jigs")
	os.MkdirAll(jigsDir, 0755)

	// Create a user override for feature jig.
	userContent := `---
name: feature
description: Custom feature
version: 99
status_values: [a, b]
passes:
  - name: "A"
    status: a
    output: ["a.md"]
---

# Custom
`
	os.WriteFile(filepath.Join(jigsDir, "feature.md"), []byte(userContent), 0644)

	out := captureOutput(t, func() {
		jigListCmd.RunE(jigListCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "feature")
	testutil.AssertStringContains(t, out, "user")
	testutil.AssertStringContains(t, out, "bug")
}

func TestJigShowCommand_Builtin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	out := captureOutput(t, func() {
		jigShowCmd.RunE(jigShowCmd, []string{"feature"})
	})

	testutil.AssertStringContains(t, out, "Jig: feature")
	testutil.AssertStringContains(t, out, "Status values:")
	testutil.AssertStringContains(t, out, "Passes:")
	testutil.AssertStringContains(t, out, "Problem Space")
	testutil.AssertStringContains(t, out, "File structure:")
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
		jigSaveCmd.RunE(jigSaveCmd, []string{"feature"})
	})

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
