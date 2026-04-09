package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProperty_MissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "nonexistent.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	// All effective values should match defaults.
	if cfg.EffectiveDefaultJig() != DefaultJig {
		t.Errorf("default_jig = %q, want %q", cfg.EffectiveDefaultJig(), DefaultJig)
	}
	if cfg.EffectiveSnapshotsEnabled() != DefaultSnapshotsEnabled {
		t.Errorf("snapshots.enabled = %v, want %v", cfg.EffectiveSnapshotsEnabled(), DefaultSnapshotsEnabled)
	}
	if cfg.EffectiveIntervalEnabled() != DefaultIntervalEnabled {
		t.Errorf("interval_enabled = %v, want %v", cfg.EffectiveIntervalEnabled(), DefaultIntervalEnabled)
	}
	if cfg.EffectiveIntervalSeconds() != DefaultIntervalSeconds {
		t.Errorf("interval_seconds = %d, want %d", cfg.EffectiveIntervalSeconds(), DefaultIntervalSeconds)
	}
	if cfg.EffectiveMaxSnapshots() != DefaultMaxSnapshots {
		t.Errorf("max_snapshots = %d, want %d", cfg.EffectiveMaxSnapshots(), DefaultMaxSnapshots)
	}
	if cfg.EffectiveStaleThresholdHours() != DefaultStaleThresholdHours {
		t.Errorf("stale_threshold_hours = %d, want %d", cfg.EffectiveStaleThresholdHours(), DefaultStaleThresholdHours)
	}
	if cfg.EffectiveRepoSpecPath() != DefaultRepoSpecPath {
		t.Errorf("repo_spec_path = %q, want %q", cfg.EffectiveRepoSpecPath(), DefaultRepoSpecPath)
	}
}

func TestProperty_PartialConfigMergesWithDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Only set a few fields.
	content := `default_jig: bug
snapshots:
  max_snapshots: 50
`
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// Overridden values.
	if cfg.EffectiveDefaultJig() != "bug" {
		t.Errorf("default_jig = %q, want %q", cfg.EffectiveDefaultJig(), "bug")
	}
	if cfg.EffectiveMaxSnapshots() != 50 {
		t.Errorf("max_snapshots = %d, want 50", cfg.EffectiveMaxSnapshots())
	}

	// Non-overridden values should be defaults.
	if cfg.EffectiveSnapshotsEnabled() != DefaultSnapshotsEnabled {
		t.Errorf("snapshots.enabled should be default")
	}
	if cfg.EffectiveIntervalEnabled() != DefaultIntervalEnabled {
		t.Errorf("interval_enabled should be default")
	}
	if cfg.EffectiveIntervalSeconds() != DefaultIntervalSeconds {
		t.Errorf("interval_seconds should be default")
	}
	if cfg.EffectiveStaleThresholdHours() != DefaultStaleThresholdHours {
		t.Errorf("stale_threshold_hours should be default")
	}
}

func TestProperty_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	original := &Config{
		DefaultJig:     "bug",
		DefaultProject: "my-proj",
	}
	bTrue := true
	n42 := 42
	original.Snapshots.Enabled = &bTrue
	original.Snapshots.MaxSnapshots = &n42

	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.EffectiveDefaultJig() != "bug" {
		t.Error("default_jig mismatch after round-trip")
	}
	if loaded.DefaultProject != "my-proj" {
		t.Error("default_project mismatch after round-trip")
	}
	if !loaded.EffectiveSnapshotsEnabled() {
		t.Error("snapshots.enabled mismatch after round-trip")
	}
	if loaded.EffectiveMaxSnapshots() != 42 {
		t.Error("max_snapshots mismatch after round-trip")
	}
}

func TestProperty_UnknownKeysIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `default_jig: feature
unknown_key: some_value
future_section:
  nested: true
`
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load should not error on unknown keys: %v", err)
	}
	if cfg.EffectiveDefaultJig() != "feature" {
		t.Error("known key should still parse correctly")
	}
}

func TestProperty_SetGetRoundTrip(t *testing.T) {
	keys := []struct {
		key   string
		value string
	}{
		{"default_jig", "custom-jig"},
		{"default_project", "my-project"},
		{"snapshots.enabled", "false"},
		{"snapshots.interval_enabled", "true"},
		{"snapshots.interval_seconds", "600"},
		{"snapshots.max_snapshots", "200"},
		{"sessions.stale_threshold_hours", "48"},
		{"finalize.repo_spec_path", "specs/{codename}/"},
	}

	for _, kv := range keys {
		cfg := &Config{}
		if err := cfg.Set(kv.key, kv.value); err != nil {
			t.Errorf("Set(%q, %q): %v", kv.key, kv.value, err)
			continue
		}
		got, err := cfg.Get(kv.key)
		if err != nil {
			t.Errorf("Get(%q): %v", kv.key, err)
			continue
		}
		if got != kv.value {
			t.Errorf("Get(%q) = %q after Set, want %q", kv.key, got, kv.value)
		}
	}
}
