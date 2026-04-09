package config

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzLoadConfig(f *testing.F) {
	f.Add([]byte(`default_jig: feature`))
	f.Add([]byte(`snapshots:
  enabled: true
  max_snapshots: 100`))
	f.Add([]byte(`{{{`))
	f.Add([]byte(``))
	f.Add([]byte{0x00, 0x01, 0x02})
	f.Add([]byte(`default_jig: [broken`))

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		os.WriteFile(path, data, 0o644)

		// Load should never panic.
		cfg, err := Load(path)
		if err != nil {
			return
		}

		// Effective values should never panic.
		_ = cfg.EffectiveDefaultJig()
		_ = cfg.EffectiveSnapshotsEnabled()
		_ = cfg.EffectiveIntervalEnabled()
		_ = cfg.EffectiveIntervalSeconds()
		_ = cfg.EffectiveMaxSnapshots()
		_ = cfg.EffectiveStaleThresholdHours()
		_ = cfg.EffectiveRepoSpecPath()

		// Save should also not panic.
		outPath := filepath.Join(dir, "out.yaml")
		_ = Save(outPath, cfg)
	})
}
