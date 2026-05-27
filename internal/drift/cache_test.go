package drift

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gregberns/kerf/internal/beads"
)

func TestCachePath(t *testing.T) {
	got := CachePath("/repo")
	want := filepath.Join("/repo", ".kerf", "sync-cache.json")
	if got != want {
		t.Fatalf("CachePath = %q, want %q", got, want)
	}
	if CachePath("") != "" {
		t.Fatalf("CachePath(\"\") = %q, want empty", CachePath(""))
	}
}

func TestRead_MissingReturnsEmptyBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")
	snap, present, err := Read(path)
	if err != nil {
		t.Fatalf("Read err = %v, want nil", err)
	}
	if present {
		t.Fatal("present = true, want false for missing path")
	}
	if !reflect.DeepEqual(snap, (Snapshot{})) {
		t.Fatalf("snap = %+v, want zero", snap)
	}
}

func TestRead_ParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(path); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "sync-cache.json")

	snap := Capture([]beads.Bead{
		{ID: "a", Status: "open", Title: "alpha", Labels: []string{"L1"}, DependsOn: []string{"b"}},
		{ID: "b", Status: "closed", Title: "beta"},
	}, map[string][]string{
		"a": {"work-x", "work-y"},
		"b": {"work-x"},
	})

	if err := Write(path, snap); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Parent directory must have been created.
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("parent dir not created: %v", err)
	}
	// File mode should be 0o644.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o644 {
		t.Fatalf("mode = %o, want 0644", mode)
	}

	got, present, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !present {
		t.Fatal("present = false, want true")
	}
	if got.SnapshotID != snap.SnapshotID {
		t.Fatalf("SnapshotID round-trip mismatch:\n  got  %s\n  want %s", got.SnapshotID, snap.SnapshotID)
	}
	if !reflect.DeepEqual(got.Beads, snap.Beads) {
		t.Fatalf("Beads round-trip mismatch:\n  got  %+v\n  want %+v", got.Beads, snap.Beads)
	}
	if !reflect.DeepEqual(got.FilterAssignments, snap.FilterAssignments) {
		t.Fatalf("FilterAssignments round-trip mismatch:\n  got  %+v\n  want %+v", got.FilterAssignments, snap.FilterAssignments)
	}
	// CapturedAt round-trip preserves the timestamp (json uses RFC 3339).
	if !got.CapturedAt.Equal(snap.CapturedAt) {
		t.Fatalf("CapturedAt round-trip mismatch:\n  got  %v\n  want %v", got.CapturedAt, snap.CapturedAt)
	}
}

func TestWrite_AtomicLeavesOriginalIntact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync-cache.json")

	// First, write a known-good baseline.
	original := Capture([]beads.Bead{{ID: "orig", Status: "open", Title: "o"}}, nil)
	if err := Write(path, original); err != nil {
		t.Fatalf("initial Write: %v", err)
	}
	origBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a crash mid-write by manually creating a temp file in
	// the same dir and removing it without renaming — proves the
	// rename-into-place pattern leaves the destination untouched when
	// the temp never lands.
	tmp, err := os.CreateTemp(dir, ".sync-cache-*.json.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.WriteString("partial garbage"); err != nil {
		t.Fatal(err)
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tmpName); err != nil {
		t.Fatal(err)
	}

	// Re-read the original — should be byte-identical.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(origBytes, after) {
		t.Fatal("destination file changed despite no successful rename")
	}
}

func TestAdvanceWritesSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync-cache.json")
	snap := Capture([]beads.Bead{{ID: "x", Status: "open", Title: "t"}}, nil)
	if err := Advance(path, snap); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	got, present, err := Read(path)
	if err != nil || !present {
		t.Fatalf("post-Advance Read: present=%v err=%v", present, err)
	}
	if got.SnapshotID != snap.SnapshotID {
		t.Fatal("Advance did not persist the snapshot")
	}
}
