package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadProjectConfigMissing(t *testing.T) {
	cfg, err := LoadProjectConfig("/nonexistent/project.yaml")
	if err != nil {
		t.Fatalf("Load missing file should not error: %v", err)
	}
	if cfg.Jigs != nil {
		t.Errorf("Jigs = %v, want nil", cfg.Jigs)
	}
	if cfg.Passes != nil {
		t.Errorf("Passes = %v, want nil", cfg.Passes)
	}
	if cfg.Tools != nil {
		t.Errorf("Tools = %v, want nil", cfg.Tools)
	}
}

func TestLoadProjectConfigFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")

	content := `jigs:
  - plan
  - implementation
  - spike
passes:
  implementation:
    - breakdown
    - dispatch
    - implement
    - review
tools:
  orchestrator: ntm
  tasks: bd
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProjectConfig(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	wantJigs := []string{"plan", "implementation", "spike"}
	if !reflect.DeepEqual(cfg.Jigs, wantJigs) {
		t.Errorf("Jigs = %v, want %v", cfg.Jigs, wantJigs)
	}

	wantPasses := []string{"breakdown", "dispatch", "implement", "review"}
	if !reflect.DeepEqual(cfg.Passes["implementation"], wantPasses) {
		t.Errorf("Passes[implementation] = %v, want %v", cfg.Passes["implementation"], wantPasses)
	}

	if cfg.Tools["orchestrator"] != "ntm" {
		t.Errorf("Tools[orchestrator] = %q, want %q", cfg.Tools["orchestrator"], "ntm")
	}
	if cfg.Tools["tasks"] != "bd" {
		t.Errorf("Tools[tasks] = %q, want %q", cfg.Tools["tasks"], "bd")
	}
}

func TestSaveProjectConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "project.yaml")

	cfg := &ProjectConfig{
		Jigs: []string{"plan", "bug"},
		Passes: map[string][]string{
			"implementation": {"breakdown", "implement"},
		},
		Tools: map[string]string{
			"orchestrator": "ntm",
		},
	}

	if err := SaveProjectConfig(path, cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	got, err := LoadProjectConfig(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if !reflect.DeepEqual(got.Jigs, cfg.Jigs) {
		t.Errorf("Jigs = %v, want %v", got.Jigs, cfg.Jigs)
	}
	if !reflect.DeepEqual(got.Passes, cfg.Passes) {
		t.Errorf("Passes = %v, want %v", got.Passes, cfg.Passes)
	}
	if !reflect.DeepEqual(got.Tools, cfg.Tools) {
		t.Errorf("Tools = %v, want %v", got.Tools, cfg.Tools)
	}
}

func TestIsJigActive(t *testing.T) {
	// nil Jigs — all active
	cfg := &ProjectConfig{}
	if !cfg.IsJigActive("plan") {
		t.Error("nil Jigs: plan should be active")
	}
	if !cfg.IsJigActive("anything") {
		t.Error("nil Jigs: any jig should be active")
	}

	// empty Jigs — all active
	cfg = &ProjectConfig{Jigs: []string{}}
	if !cfg.IsJigActive("plan") {
		t.Error("empty Jigs: plan should be active")
	}

	// specific list — only listed jigs active
	cfg = &ProjectConfig{Jigs: []string{"plan", "bug"}}
	if !cfg.IsJigActive("plan") {
		t.Error("plan should be active")
	}
	if !cfg.IsJigActive("bug") {
		t.Error("bug should be active")
	}
	if cfg.IsJigActive("spec") {
		t.Error("spec should not be active")
	}
	if cfg.IsJigActive("implementation") {
		t.Error("implementation should not be active")
	}
}

func TestGetActivePasses(t *testing.T) {
	// nil Passes — all active
	cfg := &ProjectConfig{}
	if got := cfg.GetActivePasses("implementation"); got != nil {
		t.Errorf("nil Passes: got %v, want nil", got)
	}

	// specific pass list
	cfg = &ProjectConfig{
		Passes: map[string][]string{
			"implementation": {"breakdown", "dispatch", "implement"},
		},
	}
	want := []string{"breakdown", "dispatch", "implement"}
	if got := cfg.GetActivePasses("implementation"); !reflect.DeepEqual(got, want) {
		t.Errorf("GetActivePasses(implementation) = %v, want %v", got, want)
	}

	// jig not in passes map — all active
	if got := cfg.GetActivePasses("plan"); got != nil {
		t.Errorf("GetActivePasses(plan) = %v, want nil", got)
	}
}

func TestProjectConfigPath(t *testing.T) {
	got := ProjectConfigPath("/home/user/.kerf", "my-project")
	want := filepath.Join("/home/user/.kerf", "projects", "my-project", "project.yaml")
	if got != want {
		t.Errorf("ProjectConfigPath = %q, want %q", got, want)
	}
}

func TestLoadProjectConfigUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")

	content := `jigs:
  - plan
unknown_future_key: some-value
nested_unknown:
  deep: true
tools:
  orchestrator: ntm
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProjectConfig(path)
	if err != nil {
		t.Fatalf("Load with unknown keys should not error: %v", err)
	}

	if !reflect.DeepEqual(cfg.Jigs, []string{"plan"}) {
		t.Errorf("Jigs = %v, want [plan]", cfg.Jigs)
	}
	if cfg.Tools["orchestrator"] != "ntm" {
		t.Errorf("Tools[orchestrator] = %q, want %q", cfg.Tools["orchestrator"], "ntm")
	}
}
