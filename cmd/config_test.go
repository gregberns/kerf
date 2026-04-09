package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

func TestConfigCommand_ShowAll(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	out := captureOutput(t, func() {
		configCmd.RunE(configCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "kerf configuration")
	testutil.AssertStringContains(t, out, "default_jig:")
	testutil.AssertStringContains(t, out, "snapshots.enabled:")
	testutil.AssertStringContains(t, out, "finalize.repo_spec_path:")
	testutil.AssertStringContains(t, out, "sessions.stale_threshold_hours:")
}

func TestConfigCommand_GetSingle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	out := captureOutput(t, func() {
		configCmd.RunE(configCmd, []string{"default_jig"})
	})

	testutil.AssertStringContains(t, out, "default_jig: feature")
}

func TestConfigCommand_SetValue(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	out := captureOutput(t, func() {
		configCmd.RunE(configCmd, []string{"default_jig", "bug"})
	})

	testutil.AssertStringContains(t, out, "Set default_jig = bug")

	// Verify it was written.
	out = captureOutput(t, func() {
		configCmd.RunE(configCmd, []string{"default_jig"})
	})
	testutil.AssertStringContains(t, out, "default_jig: bug")
}

func TestConfigCommand_MissingFileCreation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Don't create .kerf/ — config set should create it.

	out := captureOutput(t, func() {
		configCmd.RunE(configCmd, []string{"default_jig", "bug"})
	})

	testutil.AssertStringContains(t, out, "Set default_jig = bug")
	testutil.AssertFileExists(t, filepath.Join(tmp, ".kerf", "config.yaml"))
}

func TestConfigCommand_UnknownKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	err := configCmd.RunE(configCmd, []string{"nonexistent_key"})
	if err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestConfigCommand_InvalidValue(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	err := configCmd.RunE(configCmd, []string{"snapshots.enabled", "notabool"})
	if err == nil {
		t.Error("expected error for invalid boolean value")
	}
}

func TestConfigCommand_StaleThreshold(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0755)

	out := captureOutput(t, func() {
		configCmd.RunE(configCmd, []string{"sessions.stale_threshold_hours", "48"})
	})
	testutil.AssertStringContains(t, out, "Set sessions.stale_threshold_hours = 48")

	out = captureOutput(t, func() {
		configCmd.RunE(configCmd, []string{"sessions.stale_threshold_hours"})
	})
	testutil.AssertStringContains(t, out, "sessions.stale_threshold_hours: 48")
}
