package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/beads"
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
  tasks: br
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
	if cfg.Tools["tasks"] != "br" {
		t.Errorf("Tools[tasks] = %q, want %q", cfg.Tools["tasks"], "br")
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

func TestLoadProjectConfigQueueSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")

	content := `queue:
  fan_out: 20.0
  momentum: 2.5
  rework: 30.0
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProjectConfig(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Queue == nil {
		t.Fatal("Queue section was not parsed")
	}
	if cfg.Queue.FanOut == nil || *cfg.Queue.FanOut != 20.0 {
		t.Errorf("FanOut = %v, want 20.0", cfg.Queue.FanOut)
	}
	if cfg.Queue.Momentum == nil || *cfg.Queue.Momentum != 2.5 {
		t.Errorf("Momentum = %v, want 2.5", cfg.Queue.Momentum)
	}
	if cfg.Queue.Creation != nil {
		t.Errorf("Creation should be nil (unset), got %v", *cfg.Queue.Creation)
	}
	if cfg.Queue.Rework == nil || *cfg.Queue.Rework != 30.0 {
		t.Errorf("Rework = %v, want 30.0", cfg.Queue.Rework)
	}

	defaults := ResolvedQueueWeights{FanOut: 10.0, Momentum: 5.0, Creation: 0.1, Rework: 15.0}
	got := cfg.QueueWeights(defaults)
	want := ResolvedQueueWeights{FanOut: 20.0, Momentum: 2.5, Creation: 0.1, Rework: 30.0}
	if got != want {
		t.Errorf("QueueWeights = %+v, want %+v", got, want)
	}
}

func TestQueueWeightsReworkDefaultFallback(t *testing.T) {
	// rework: omitted entirely. Defaults must carry through.
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")
	content := `queue:
  fan_out: 7.0
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadProjectConfig(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Queue.Rework != nil {
		t.Errorf("Rework should be nil (unset), got %v", *cfg.Queue.Rework)
	}
	defaults := ResolvedQueueWeights{FanOut: 10.0, Momentum: 5.0, Creation: 0.1, Rework: 15.0}
	got := cfg.QueueWeights(defaults)
	want := ResolvedQueueWeights{FanOut: 7.0, Momentum: 5.0, Creation: 0.1, Rework: 15.0}
	if got != want {
		t.Errorf("QueueWeights = %+v, want %+v", got, want)
	}
}

func TestQueueWeightsDefaultsWhenMissing(t *testing.T) {
	defaults := ResolvedQueueWeights{FanOut: 10.0, Momentum: 5.0, Creation: 0.1, Rework: 15.0}

	// nil receiver should return defaults.
	var nilCfg *ProjectConfig
	if got := nilCfg.QueueWeights(defaults); got != defaults {
		t.Errorf("nil cfg: got %+v, want %+v", got, defaults)
	}

	// Empty config with no queue section.
	cfg := &ProjectConfig{}
	if got := cfg.QueueWeights(defaults); got != defaults {
		t.Errorf("empty cfg: got %+v, want %+v", got, defaults)
	}

	// Queue set but all fields nil.
	cfg = &ProjectConfig{Queue: &QueueConfig{}}
	if got := cfg.QueueWeights(defaults); got != defaults {
		t.Errorf("empty queue: got %+v, want %+v", got, defaults)
	}
}

func TestLoadProjectConfigNoBeadFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")
	content := `jigs:
  - plan
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadProjectConfig(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.BeadFilter != nil {
		t.Errorf("BeadFilter = %+v, want nil", cfg.BeadFilter)
	}
}

func TestLoadProjectConfigBeadFilterLabel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")
	content := `bead_filter:
  label: "subsystem:{codename}"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadProjectConfig(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.BeadFilter == nil {
		t.Fatal("BeadFilter is nil")
	}
	if cfg.BeadFilter.Label != "subsystem:{codename}" {
		t.Errorf("Label = %q, want subsystem:{codename}", cfg.BeadFilter.Label)
	}
	if err := cfg.BeadFilter.Validate(); err != nil {
		t.Errorf("validation failed: %v", err)
	}
}

func TestLoadProjectConfigBeadFilterAny(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")
	content := `bead_filter:
  any:
    - label: "subsystem:{codename}"
    - id_prefix: "gw-"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadProjectConfig(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.BeadFilter == nil {
		t.Fatal("BeadFilter is nil")
	}
	if len(cfg.BeadFilter.Any) != 2 {
		t.Fatalf("Any has %d entries, want 2", len(cfg.BeadFilter.Any))
	}
	if cfg.BeadFilter.Any[0].Label != "subsystem:{codename}" {
		t.Errorf("Any[0].Label = %q", cfg.BeadFilter.Any[0].Label)
	}
	if cfg.BeadFilter.Any[1].IDPrefix != "gw-" {
		t.Errorf("Any[1].IDPrefix = %q", cfg.BeadFilter.Any[1].IDPrefix)
	}
}

func TestLoadProjectConfigBeadFilterInvalidEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")
	content := `bead_filter: {}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProjectConfig(path)
	if err == nil {
		t.Fatal("expected error for empty bead_filter, got nil")
	}
	if !strings.Contains(err.Error(), "bead_filter") {
		t.Errorf("error %q should mention bead_filter", err)
	}
}

func TestLoadProjectConfigBeadFilterInvalidMixed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")
	content := `bead_filter:
  label: "subsystem:{codename}"
  any:
    - id_prefix: "gw-"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProjectConfig(path)
	if err == nil {
		t.Fatal("expected error for mixed direct+any bead_filter, got nil")
	}
}

func TestSaveLoadProjectConfigBeadFilterRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")

	cfg := &ProjectConfig{
		BeadFilter: &beads.Filter{
			Any: []beads.Filter{
				{Label: "subsystem:{codename}"},
				{IDPrefix: "gw-"},
			},
		},
	}
	if err := SaveProjectConfig(path, cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	got, err := LoadProjectConfig(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !reflect.DeepEqual(got.BeadFilter, cfg.BeadFilter) {
		t.Errorf("BeadFilter round-trip mismatch:\n got %+v\nwant %+v", got.BeadFilter, cfg.BeadFilter)
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

// TestMaturity — Plan 014 / B1 (kerf-3ka1): the project-level
// `maturity` field on project.yaml. Default is `stable` when unset;
// `experimental`, `stable`, and `frozen` parse; unknown values error
// with a message naming the three legal values.
func TestMaturity(t *testing.T) {
	t.Run("default is stable when unset", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "project.yaml")
		if err := os.WriteFile(path, []byte("jigs:\n  - plan\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadProjectConfig(path)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if cfg.Maturity != "" {
			t.Errorf("raw Maturity = %q, want \"\" (unset)", cfg.Maturity)
		}
		if got := cfg.ResolvedMaturity(); got != MaturityStable {
			t.Errorf("ResolvedMaturity() = %q, want %q", got, MaturityStable)
		}
		// nil receiver also defaults to stable.
		if got := (*ProjectConfig)(nil).ResolvedMaturity(); got != MaturityStable {
			t.Errorf("nil cfg ResolvedMaturity() = %q, want %q", got, MaturityStable)
		}
		// missing-file path also resolves to stable.
		missing, err := LoadProjectConfig(filepath.Join(dir, "nope.yaml"))
		if err != nil {
			t.Fatalf("missing-file Load error: %v", err)
		}
		if got := missing.ResolvedMaturity(); got != MaturityStable {
			t.Errorf("missing-file ResolvedMaturity() = %q, want %q", got, MaturityStable)
		}
	})

	t.Run("all three enum values parse", func(t *testing.T) {
		for _, v := range []string{MaturityExperimental, MaturityStable, MaturityFrozen} {
			dir := t.TempDir()
			path := filepath.Join(dir, "project.yaml")
			content := "maturity: " + v + "\n"
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadProjectConfig(path)
			if err != nil {
				t.Fatalf("Load(%q) error: %v", v, err)
			}
			if cfg.Maturity != v {
				t.Errorf("Maturity = %q, want %q", cfg.Maturity, v)
			}
			if got := cfg.ResolvedMaturity(); got != v {
				t.Errorf("ResolvedMaturity() = %q, want %q", got, v)
			}
		}
	})

	t.Run("unknown value errors cleanly", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "project.yaml")
		if err := os.WriteFile(path, []byte("maturity: bogus\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadProjectConfig(path)
		if err == nil {
			t.Fatal("expected error for unknown maturity, got nil")
		}
		msg := err.Error()
		for _, want := range []string{"bogus", MaturityExperimental, MaturityStable, MaturityFrozen} {
			if !strings.Contains(msg, want) {
				t.Errorf("error %q should mention %q", msg, want)
			}
		}
	})

	t.Run("save/load round-trip", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "project.yaml")
		cfg := &ProjectConfig{Maturity: MaturityFrozen}
		if err := SaveProjectConfig(path, cfg); err != nil {
			t.Fatalf("Save error: %v", err)
		}
		got, err := LoadProjectConfig(path)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if got.Maturity != MaturityFrozen {
			t.Errorf("round-trip Maturity = %q, want %q", got.Maturity, MaturityFrozen)
		}
	})
}

// TestDoctorFooterEnabled — Plan 017 / B13 (kerf-bwd): default-on,
// project.yaml override, env-var override, env wins on conflict.
func TestDoctorFooterEnabled(t *testing.T) {
	t.Setenv(DoctorFooterEnvVar, "")

	// nil receiver → default true.
	if !(*ProjectConfig)(nil).DoctorFooterEnabled() {
		t.Error("nil cfg: want true")
	}
	// Empty config → default true.
	if !(&ProjectConfig{}).DoctorFooterEnabled() {
		t.Error("empty cfg: want true")
	}
	// Empty doctor block → default true.
	if !(&ProjectConfig{Doctor: &DoctorConfig{}}).DoctorFooterEnabled() {
		t.Error("doctor{} cfg: want true")
	}
	// Explicit false in config.
	f := false
	off := &ProjectConfig{Doctor: &DoctorConfig{Footer: &f}}
	if off.DoctorFooterEnabled() {
		t.Error("cfg footer=false: want false")
	}
	// Env var alone suppresses.
	t.Setenv(DoctorFooterEnvVar, "0")
	if (&ProjectConfig{}).DoctorFooterEnabled() {
		t.Error("env=0 with empty cfg: want false")
	}
	// Env=1 overrides cfg=false (env wins).
	t.Setenv(DoctorFooterEnvVar, "1")
	if !off.DoctorFooterEnabled() {
		t.Error("env=1 must override cfg=false")
	}
	// Env=0 overrides cfg=true (env wins, opposite direction).
	tr := true
	on := &ProjectConfig{Doctor: &DoctorConfig{Footer: &tr}}
	t.Setenv(DoctorFooterEnvVar, "0")
	if on.DoctorFooterEnabled() {
		t.Error("env=0 must override cfg=true")
	}
	// Unparseable env → falls through to config.
	t.Setenv(DoctorFooterEnvVar, "maybe")
	if off.DoctorFooterEnabled() {
		t.Error("garbage env: cfg=false should still apply")
	}
}
