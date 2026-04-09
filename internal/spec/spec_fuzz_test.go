package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzReadSpecYAML(f *testing.F) {
	// Seed with valid and invalid YAML.
	f.Add([]byte(`codename: test
type: feature
status: draft
jig: feature
jig_version: 1
status_values: [draft, ready]
project:
  id: test-proj
created: 2026-04-07T10:00:00Z
updated: 2026-04-07T10:00:00Z
`))
	f.Add([]byte(`codename: minimal
type: bug
status: triaging
jig: bug
jig_version: 1
status_values: [triaging]
project:
  id: p
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
`))
	f.Add([]byte(`{{{`))
	f.Add([]byte(`codename: [broken`))
	f.Add([]byte(``))
	f.Add([]byte{0x00, 0x01, 0x02})

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "spec.yaml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}

		// Read should never panic.
		s, err := Read(path)
		if err != nil {
			return // expected for malformed input
		}

		// If it parsed, write it back — should also not panic.
		outPath := filepath.Join(dir, "spec2.yaml")
		_ = Write(outPath, s)
	})
}
