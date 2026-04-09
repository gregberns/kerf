package snapshot

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/spec"
)

func TestProperty_SnapshotRestoreIntegrity(t *testing.T) {
	// Create work with various file types.
	workDir := t.TempDir()

	s := &spec.SpecYAML{
		Codename:     "integrity-test",
		Type:         "feature",
		Status:       "research",
		StatusValues: []string{"draft", "research", "ready"},
		Created:      time.Now().UTC().Truncate(time.Second),
		Updated:      time.Now().UTC().Truncate(time.Second),
		Project:      spec.Project{ID: "test"},
		Jig:          "feature",
		JigVersion:   1,
	}
	spec.Write(filepath.Join(workDir, "spec.yaml"), s)

	// Create various file types.
	files := map[string][]byte{
		"01-problem.md":               []byte("# Problem Space\n\nDetailed analysis."),
		"02-components.md":            []byte("# Components\n\n- Auth\n- Database"),
		"data/results.json":           []byte(`{"key": "value", "count": 42}`),
		"data/nested/deep/file.txt":   []byte("deep content"),
		"binary.bin":                  {0x00, 0x01, 0xFF, 0xFE, 0x89, 0x50, 0x4E, 0x47},
		"empty.txt":                   {},
		"unicode.md":                  []byte("日本語テスト — café résumé naïve"),
	}
	for path, content := range files {
		fullPath := filepath.Join(workDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0o755)
		if err := os.WriteFile(fullPath, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Take snapshot.
	snapPath, err := Take(workDir, "integrity")
	if err != nil {
		t.Fatal(err)
	}

	// Verify every file in the snapshot matches the original byte-for-byte.
	for path, originalContent := range files {
		snapFile := filepath.Join(snapPath, path)
		snappedContent, err := os.ReadFile(snapFile)
		if err != nil {
			t.Errorf("file %q missing from snapshot: %v", path, err)
			continue
		}
		origHash := sha256.Sum256(originalContent)
		snapHash := sha256.Sum256(snappedContent)
		if origHash != snapHash {
			t.Errorf("file %q: snapshot content differs from original (len orig=%d, snap=%d)",
				path, len(originalContent), len(snappedContent))
		}
	}

	// Now modify files, then restore.
	os.WriteFile(filepath.Join(workDir, "01-problem.md"), []byte("MODIFIED"), 0o644)
	os.WriteFile(filepath.Join(workDir, "new-file.md"), []byte("new"), 0o644)
	os.Remove(filepath.Join(workDir, "02-components.md"))

	// Get snapshot name for restore.
	entries, _ := List(workDir)
	var snapName string
	for _, e := range entries {
		if e.Label == "integrity" {
			snapName = e.Name
			break
		}
	}

	_, err = Restore(workDir, snapName)
	if err != nil {
		t.Fatal(err)
	}

	// Verify restored files match originals.
	for path, originalContent := range files {
		restoredContent, err := os.ReadFile(filepath.Join(workDir, path))
		if err != nil {
			t.Errorf("file %q missing after restore: %v", path, err)
			continue
		}
		origHash := sha256.Sum256(originalContent)
		restoredHash := sha256.Sum256(restoredContent)
		if origHash != restoredHash {
			t.Errorf("file %q: restored content differs from original", path)
		}
	}

	// new-file.md should be gone after restore.
	if _, err := os.Stat(filepath.Join(workDir, "new-file.md")); err == nil {
		t.Error("new-file.md should not exist after restore")
	}
}

func TestProperty_SnapshotExcludesHistory(t *testing.T) {
	workDir := t.TempDir()

	s := &spec.SpecYAML{
		Codename:     "test",
		Type:         "feature",
		Status:       "draft",
		StatusValues: []string{"draft"},
		Created:      time.Now().UTC().Truncate(time.Second),
		Updated:      time.Now().UTC().Truncate(time.Second),
		Project:      spec.Project{ID: "test"},
		Jig:          "feature",
		JigVersion:   1,
	}
	spec.Write(filepath.Join(workDir, "spec.yaml"), s)

	// Take first snapshot.
	Take(workDir, "first")

	// Take second snapshot — should not contain .history/ from first.
	snapPath, _ := Take(workDir, "second")

	if _, err := os.Stat(filepath.Join(snapPath, ".history")); err == nil {
		t.Error("snapshot should not contain .history/")
	}
}

func TestProperty_PrunePreservesNewest(t *testing.T) {
	workDir := t.TempDir()

	s := &spec.SpecYAML{
		Codename:     "prune-test",
		Type:         "feature",
		Status:       "draft",
		StatusValues: []string{"draft"},
		Created:      time.Now().UTC().Truncate(time.Second),
		Updated:      time.Now().UTC().Truncate(time.Second),
		Project:      spec.Project{ID: "test"},
		Jig:          "feature",
		JigVersion:   1,
	}
	spec.Write(filepath.Join(workDir, "spec.yaml"), s)

	// Take snapshots with distinct timestamps.
	var names []string
	for i := 0; i < 5; i++ {
		snapPath, _ := Take(workDir, "")
		names = append(names, filepath.Base(snapPath))
		time.Sleep(1100 * time.Millisecond)
	}

	// Prune to 2.
	Prune(workDir, 2)

	entries, _ := List(workDir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 after prune, got %d", len(entries))
	}

	// The 2 newest should survive.
	for _, e := range entries {
		if e.Name != names[3] && e.Name != names[4] {
			t.Errorf("unexpected surviving snapshot: %s", e.Name)
		}
	}
}
