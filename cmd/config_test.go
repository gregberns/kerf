package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/kerf/internal/testutil"
)

// setupProjectContext primes HOME with an empty bench and a git repo with a
// .kerf/project-identifier so project-scoped config writes can resolve.
// Returns the repo root (becomes cwd for the duration of the test).
func setupProjectContext(t *testing.T, projectID string) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0o755)

	repo := testutil.SetupGitRepo(t)
	os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755)
	os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(projectID+"\n"), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(repo)
	t.Cleanup(func() { os.Chdir(oldWd) })

	return repo
}

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

	testutil.AssertStringContains(t, out, "default_jig:")
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

func TestConfigCommand_SetProjectScopedToolsTasks(t *testing.T) {
	setupProjectContext(t, "test-proj")

	out := captureOutput(t, func() {
		if err := configCmd.RunE(configCmd, []string{"tools.tasks", "bd"}); err != nil {
			t.Fatalf("set tools.tasks: %v", err)
		}
	})
	testutil.AssertStringContains(t, out, "Set tools.tasks = bd")

	// Verify it landed in project.yaml.
	home, _ := os.UserHomeDir()
	pyPath := filepath.Join(home, ".kerf", "projects", "test-proj", "project.yaml")
	data, err := os.ReadFile(pyPath)
	if err != nil {
		t.Fatalf("read project.yaml: %v", err)
	}
	if !strings.Contains(string(data), "tasks: bd") {
		t.Errorf("project.yaml missing tools.tasks=bd; got:\n%s", data)
	}
}

func TestConfigCommand_OverwriteProjectScopedToolsTasks(t *testing.T) {
	setupProjectContext(t, "test-proj")

	if err := configCmd.RunE(configCmd, []string{"tools.tasks", "bd"}); err != nil {
		t.Fatalf("first set: %v", err)
	}
	if err := configCmd.RunE(configCmd, []string{"tools.tasks", "br"}); err != nil {
		t.Fatalf("overwrite: %v", err)
	}

	home, _ := os.UserHomeDir()
	pyPath := filepath.Join(home, ".kerf", "projects", "test-proj", "project.yaml")
	data, err := os.ReadFile(pyPath)
	if err != nil {
		t.Fatalf("read project.yaml: %v", err)
	}
	if !strings.Contains(string(data), "tasks: br") {
		t.Errorf("project.yaml not overwritten to br; got:\n%s", data)
	}
	if strings.Contains(string(data), "tasks: bd") {
		t.Errorf("project.yaml still has old value bd; got:\n%s", data)
	}
}

func TestConfigCommand_GetProjectScopedToolsTasks(t *testing.T) {
	setupProjectContext(t, "test-proj")

	if err := configCmd.RunE(configCmd, []string{"tools.tasks", "bd"}); err != nil {
		t.Fatalf("set: %v", err)
	}

	out := captureOutput(t, func() {
		if err := configCmd.RunE(configCmd, []string{"tools.tasks"}); err != nil {
			t.Fatalf("get: %v", err)
		}
	})
	testutil.AssertStringContains(t, out, "tools.tasks: bd")
}

func TestConfigCommand_UnknownKeyListsValidKeys(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0o755)

	err := configCmd.RunE(configCmd, []string{"foo", "bar"})
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	msg := err.Error()
	for _, want := range []string{"unknown configuration key 'foo'", "tools.tasks", "default_jig", "doctor.footer", "bead_filter"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q; got: %s", want, msg)
		}
	}
}

func TestConfigCommand_UnknownKeyValidKeysDeduped(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".kerf"), 0o755)

	err := configCmd.RunE(configCmd, []string{"foo", "bar"})
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	msg := err.Error()
	// Extract the comma-separated keys after "Valid keys: ".
	idx := strings.Index(msg, "Valid keys: ")
	if idx < 0 {
		t.Fatalf("error missing 'Valid keys:' prefix; got: %s", msg)
	}
	keys := strings.Split(msg[idx+len("Valid keys: "):], ", ")
	seen := make(map[string]int)
	for _, k := range keys {
		k = strings.TrimSpace(k)
		seen[k]++
	}
	for k, n := range seen {
		if n > 1 {
			t.Errorf("key %q appears %d times in valid-keys list; want 1", k, n)
		}
	}
}

func TestConfigCommand_DefaultJigWritesBothLayers(t *testing.T) {
	repo := setupProjectContext(t, "test-proj")
	_ = repo

	if err := configCmd.RunE(configCmd, []string{"default_jig", "spec"}); err != nil {
		t.Fatalf("set default_jig: %v", err)
	}

	home, _ := os.UserHomeDir()
	// Bench config.yaml.
	benchData, err := os.ReadFile(filepath.Join(home, ".kerf", "config.yaml"))
	if err != nil {
		t.Fatalf("read bench config: %v", err)
	}
	if !strings.Contains(string(benchData), "default_jig: spec") {
		t.Errorf("bench config missing default_jig=spec; got:\n%s", benchData)
	}
	// project.yaml.
	projData, err := os.ReadFile(filepath.Join(home, ".kerf", "projects", "test-proj", "project.yaml"))
	if err != nil {
		t.Fatalf("read project config: %v", err)
	}
	if !strings.Contains(string(projData), "default_jig: spec") {
		t.Errorf("project config missing default_jig=spec; got:\n%s", projData)
	}
}

func TestConfigCommand_DoctorFooterProjectScoped(t *testing.T) {
	setupProjectContext(t, "test-proj")

	if err := configCmd.RunE(configCmd, []string{"doctor.footer", "false"}); err != nil {
		t.Fatalf("set doctor.footer: %v", err)
	}

	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".kerf", "projects", "test-proj", "project.yaml"))
	if err != nil {
		t.Fatalf("read project.yaml: %v", err)
	}
	if !strings.Contains(string(data), "footer: false") {
		t.Errorf("project.yaml missing doctor.footer=false; got:\n%s", data)
	}

	// Invalid value rejected.
	if err := configCmd.RunE(configCmd, []string{"doctor.footer", "maybe"}); err == nil {
		t.Error("expected error for non-bool doctor.footer value")
	}
}
