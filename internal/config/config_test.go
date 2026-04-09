package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load missing file should not error: %v", err)
	}
	// Should return defaults
	if cfg.EffectiveDefaultJig() != "" {
		t.Errorf("default_jig = %q, want %q", cfg.EffectiveDefaultJig(), "")
	}
	if cfg.EffectiveSpecPath() != "specs/" {
		t.Errorf("spec_path = %q, want %q", cfg.EffectiveSpecPath(), "specs/")
	}
	if !cfg.EffectiveSnapshotsEnabled() {
		t.Error("snapshots.enabled should default to true")
	}
	if cfg.EffectiveIntervalEnabled() {
		t.Error("snapshots.interval_enabled should default to false")
	}
	if cfg.EffectiveIntervalSeconds() != 300 {
		t.Errorf("snapshots.interval_seconds = %d, want 300", cfg.EffectiveIntervalSeconds())
	}
	if cfg.EffectiveMaxSnapshots() != 100 {
		t.Errorf("snapshots.max_snapshots = %d, want 100", cfg.EffectiveMaxSnapshots())
	}
	if cfg.EffectiveStaleThresholdHours() != 24 {
		t.Errorf("sessions.stale_threshold_hours = %d, want 24", cfg.EffectiveStaleThresholdHours())
	}
	if cfg.EffectiveRepoSpecPath() != ".kerf/{codename}/" {
		t.Errorf("finalize.repo_spec_path = %q, want %q", cfg.EffectiveRepoSpecPath(), ".kerf/{codename}/")
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	enabled := true
	intervalEnabled := true
	intervalSecs := 600
	maxSnaps := 50
	staleHours := 48

	cfg := &Config{
		DefaultJig:     "bug",
		DefaultProject: "my-project",
		Snapshots: SnapshotsConfig{
			Enabled:         &enabled,
			IntervalEnabled: &intervalEnabled,
			IntervalSeconds: &intervalSecs,
			MaxSnapshots:    &maxSnaps,
		},
		Sessions: SessionsConfig{
			StaleThresholdHours: &staleHours,
		},
		Finalize: FinalizeConfig{
			RepoSpecPath: "specs/{codename}/",
		},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if got.EffectiveDefaultJig() != "bug" {
		t.Errorf("default_jig = %q, want %q", got.EffectiveDefaultJig(), "bug")
	}
	if got.DefaultProject != "my-project" {
		t.Errorf("default_project = %q, want %q", got.DefaultProject, "my-project")
	}
	if !got.EffectiveSnapshotsEnabled() {
		t.Error("snapshots.enabled should be true")
	}
	if !got.EffectiveIntervalEnabled() {
		t.Error("snapshots.interval_enabled should be true")
	}
	if got.EffectiveIntervalSeconds() != 600 {
		t.Errorf("snapshots.interval_seconds = %d, want 600", got.EffectiveIntervalSeconds())
	}
	if got.EffectiveMaxSnapshots() != 50 {
		t.Errorf("snapshots.max_snapshots = %d, want 50", got.EffectiveMaxSnapshots())
	}
	if got.EffectiveStaleThresholdHours() != 48 {
		t.Errorf("sessions.stale_threshold_hours = %d, want 48", got.EffectiveStaleThresholdHours())
	}
	if got.EffectiveRepoSpecPath() != "specs/{codename}/" {
		t.Errorf("finalize.repo_spec_path = %q, want %q", got.EffectiveRepoSpecPath(), "specs/{codename}/")
	}
}

func TestGetSet(t *testing.T) {
	cfg := &Config{}

	// Set and get
	if err := cfg.Set("default_jig", "bug"); err != nil {
		t.Fatalf("Set default_jig: %v", err)
	}
	v, err := cfg.Get("default_jig")
	if err != nil {
		t.Fatalf("Get default_jig: %v", err)
	}
	if v != "bug" {
		t.Errorf("default_jig = %q, want %q", v, "bug")
	}

	// Boolean
	if err := cfg.Set("snapshots.enabled", "false"); err != nil {
		t.Fatalf("Set snapshots.enabled: %v", err)
	}
	v, err = cfg.Get("snapshots.enabled")
	if err != nil {
		t.Fatalf("Get snapshots.enabled: %v", err)
	}
	if v != "false" {
		t.Errorf("snapshots.enabled = %q, want %q", v, "false")
	}

	// Integer
	if err := cfg.Set("snapshots.max_snapshots", "200"); err != nil {
		t.Fatalf("Set snapshots.max_snapshots: %v", err)
	}
	v, err = cfg.Get("snapshots.max_snapshots")
	if err != nil {
		t.Fatalf("Get snapshots.max_snapshots: %v", err)
	}
	if v != "200" {
		t.Errorf("snapshots.max_snapshots = %q, want %q", v, "200")
	}

	// stale_threshold_hours
	if err := cfg.Set("sessions.stale_threshold_hours", "12"); err != nil {
		t.Fatalf("Set sessions.stale_threshold_hours: %v", err)
	}
	v, err = cfg.Get("sessions.stale_threshold_hours")
	if err != nil {
		t.Fatalf("Get sessions.stale_threshold_hours: %v", err)
	}
	if v != "12" {
		t.Errorf("sessions.stale_threshold_hours = %q, want %q", v, "12")
	}
}

func TestUnknownKey(t *testing.T) {
	cfg := &Config{}

	_, err := cfg.Get("unknown.key")
	if err == nil {
		t.Error("expected error for unknown key Get")
	}

	err = cfg.Set("unknown.key", "value")
	if err == nil {
		t.Error("expected error for unknown key Set")
	}
}

func TestInvalidValues(t *testing.T) {
	cfg := &Config{}

	if err := cfg.Set("snapshots.enabled", "notabool"); err == nil {
		t.Error("expected error for non-bool snapshots.enabled")
	}

	if err := cfg.Set("snapshots.max_snapshots", "notanint"); err == nil {
		t.Error("expected error for non-int snapshots.max_snapshots")
	}

	if err := cfg.Set("sessions.stale_threshold_hours", "notanint"); err == nil {
		t.Error("expected error for non-int sessions.stale_threshold_hours")
	}
}

func TestUnknownKeysIgnoredOnRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `default_jig: bug
unknown_future_key: some-value
snapshots:
  enabled: false
  future_nested: true
`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load with unknown keys should not error: %v", err)
	}
	if cfg.EffectiveDefaultJig() != "bug" {
		t.Errorf("default_jig = %q, want %q", cfg.EffectiveDefaultJig(), "bug")
	}
}

func TestValidKeys(t *testing.T) {
	keys := ValidKeys()
	if len(keys) != 9 {
		t.Errorf("ValidKeys length = %d, want 9", len(keys))
	}
}

func TestDefaultProjectGet(t *testing.T) {
	cfg := &Config{}
	v, err := cfg.Get("default_project")
	if err != nil {
		t.Fatalf("Get default_project: %v", err)
	}
	if v != "" {
		t.Errorf("default_project = %q, want empty", v)
	}

	if err := cfg.Set("default_project", "acme-webapp"); err != nil {
		t.Fatalf("Set default_project: %v", err)
	}
	v, err = cfg.Get("default_project")
	if err != nil {
		t.Fatalf("Get default_project: %v", err)
	}
	if v != "acme-webapp" {
		t.Errorf("default_project = %q, want %q", v, "acme-webapp")
	}
}
