package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/spec"
)

func setupWorkDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create a minimal spec.yaml
	s := &spec.SpecYAML{
		Codename: "test-work",
		Type:     "feature",
		Status:   "problem-space",
		Created:  time.Now().UTC().Truncate(time.Second),
		Updated:  time.Now().UTC().Truncate(time.Second),
	}
	if err := spec.Write(filepath.Join(dir, "spec.yaml"), s); err != nil {
		t.Fatalf("writing spec.yaml: %v", err)
	}

	// Create some artifact files
	if err := os.WriteFile(filepath.Join(dir, "01-problem-space.md"), []byte("# Problem\nSome content."), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestTakeSnapshot(t *testing.T) {
	workDir := setupWorkDir(t)

	snapPath, err := Take(workDir, "")
	if err != nil {
		t.Fatalf("Take: %v", err)
	}

	// Snapshot directory should exist
	if _, err := os.Stat(snapPath); err != nil {
		t.Fatalf("snapshot dir not created: %v", err)
	}

	// Should contain spec.yaml
	if _, err := os.Stat(filepath.Join(snapPath, "spec.yaml")); err != nil {
		t.Error("spec.yaml not in snapshot")
	}

	// Should contain artifact
	if _, err := os.Stat(filepath.Join(snapPath, "01-problem-space.md")); err != nil {
		t.Error("01-problem-space.md not in snapshot")
	}

	// Should NOT contain .history/
	if _, err := os.Stat(filepath.Join(snapPath, historyDir)); err == nil {
		t.Error(".history/ should not be in snapshot")
	}
}

func TestTakeNamedSnapshot(t *testing.T) {
	workDir := setupWorkDir(t)

	snapPath, err := Take(workDir, "before-research")
	if err != nil {
		t.Fatalf("Take: %v", err)
	}

	dirName := filepath.Base(snapPath)
	if !strings.Contains(dirName, "--before-research") {
		t.Errorf("snapshot dir %q should contain '--before-research'", dirName)
	}
}

func TestListSnapshots(t *testing.T) {
	workDir := setupWorkDir(t)

	// No snapshots yet
	entries, err := List(workDir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(entries))
	}

	// Take some snapshots with small delays so timestamps differ
	Take(workDir, "")
	time.Sleep(1100 * time.Millisecond)
	Take(workDir, "named")

	entries, err = List(workDir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(entries))
	}

	// Should be newest first
	if entries[0].Timestamp.Before(entries[1].Timestamp) {
		t.Error("snapshots should be sorted newest first")
	}

	// Second one should have label
	if entries[0].Label != "named" {
		t.Errorf("newest snapshot label = %q, want %q", entries[0].Label, "named")
	}
}

func TestListSnapshotsReadStatus(t *testing.T) {
	workDir := setupWorkDir(t)

	Take(workDir, "")

	entries, err := List(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(entries))
	}
	if entries[0].Status != "problem-space" {
		t.Errorf("status = %q, want %q", entries[0].Status, "problem-space")
	}
}

func TestRestoreSnapshot(t *testing.T) {
	workDir := setupWorkDir(t)

	// Take initial snapshot
	Take(workDir, "initial")

	// Modify work files
	os.WriteFile(filepath.Join(workDir, "01-problem-space.md"), []byte("# Modified"), 0o644)
	os.WriteFile(filepath.Join(workDir, "02-components.md"), []byte("# New file"), 0o644)

	// Update spec status
	s, _ := spec.Read(filepath.Join(workDir, "spec.yaml"))
	s.Status = "decomposition"
	sessionID := "session-1"
	s.ActiveSession = &sessionID
	s.Sessions = []spec.Session{{ID: &sessionID, Started: time.Now().UTC()}}
	spec.Write(filepath.Join(workDir, "spec.yaml"), s)

	// Find the snapshot name
	entries, _ := List(workDir)
	var initialName string
	for _, e := range entries {
		if e.Label == "initial" {
			initialName = e.Name
			break
		}
	}
	if initialName == "" {
		t.Fatal("initial snapshot not found")
	}

	// Restore
	preRestorePath, err := Restore(workDir, initialName)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Pre-restore snapshot should exist
	if _, err := os.Stat(preRestorePath); err != nil {
		t.Error("pre-restore snapshot should exist")
	}

	// Content should be restored
	data, _ := os.ReadFile(filepath.Join(workDir, "01-problem-space.md"))
	if string(data) != "# Problem\nSome content." {
		t.Errorf("restored content = %q, want original", string(data))
	}

	// New file should be gone
	if _, err := os.Stat(filepath.Join(workDir, "02-components.md")); err == nil {
		t.Error("02-components.md should not exist after restore")
	}

	// Session data should be preserved from current state
	restored, _ := spec.Read(filepath.Join(workDir, "spec.yaml"))
	if restored.ActiveSession == nil || *restored.ActiveSession != "session-1" {
		t.Error("active_session should be preserved from current state")
	}
	if len(restored.Sessions) != 1 {
		t.Error("sessions should be preserved from current state")
	}
}

func TestRestoreNonexistent(t *testing.T) {
	workDir := setupWorkDir(t)
	_, err := Restore(workDir, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

func TestPrune(t *testing.T) {
	workDir := setupWorkDir(t)

	// Take 5 snapshots
	for i := 0; i < 5; i++ {
		if _, err := Take(workDir, ""); err != nil {
			t.Fatal(err)
		}
		time.Sleep(1100 * time.Millisecond)
	}

	entries, _ := List(workDir)
	if len(entries) != 5 {
		t.Fatalf("expected 5 snapshots, got %d", len(entries))
	}

	// Prune to 3
	if err := Prune(workDir, 3); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	entries, _ = List(workDir)
	if len(entries) != 3 {
		t.Errorf("expected 3 snapshots after prune, got %d", len(entries))
	}

	// The 3 newest should remain
	for _, e := range entries {
		if e.Timestamp.IsZero() {
			t.Error("remaining snapshots should have valid timestamps")
		}
	}
}

func TestPruneNoPruneNeeded(t *testing.T) {
	workDir := setupWorkDir(t)

	Take(workDir, "")

	if err := Prune(workDir, 10); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	entries, _ := List(workDir)
	if len(entries) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(entries))
	}
}

func TestCheckInterval(t *testing.T) {
	workDir := setupWorkDir(t)

	// No snapshots — should fire
	shouldFire, err := CheckInterval(workDir, 60)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldFire {
		t.Error("should fire when no snapshots exist")
	}

	// Take a snapshot
	Take(workDir, "")

	// Should not fire with large interval
	shouldFire, err = CheckInterval(workDir, 3600)
	if err != nil {
		t.Fatal(err)
	}
	if shouldFire {
		t.Error("should not fire with large interval")
	}

	// Should fire with 0-second interval
	shouldFire, err = CheckInterval(workDir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldFire {
		t.Error("should fire with 0-second interval")
	}
}

func TestSnapshotWithSubdirectories(t *testing.T) {
	workDir := setupWorkDir(t)

	// Create a subdirectory structure
	researchDir := filepath.Join(workDir, "03-research", "auth-flow")
	os.MkdirAll(researchDir, 0o755)
	os.WriteFile(filepath.Join(researchDir, "findings.md"), []byte("# Auth Flow Findings"), 0o644)

	snapPath, err := Take(workDir, "")
	if err != nil {
		t.Fatal(err)
	}

	// Verify subdirectory was copied
	data, err := os.ReadFile(filepath.Join(snapPath, "03-research", "auth-flow", "findings.md"))
	if err != nil {
		t.Fatalf("subdirectory file not in snapshot: %v", err)
	}
	if string(data) != "# Auth Flow Findings" {
		t.Errorf("content = %q, want %q", string(data), "# Auth Flow Findings")
	}
}

func TestByteLevelCorrectness(t *testing.T) {
	workDir := setupWorkDir(t)

	// Write binary-ish content
	binaryContent := []byte{0x00, 0x01, 0xFF, 0xFE, 0x0A, 0x0D}
	os.WriteFile(filepath.Join(workDir, "data.bin"), binaryContent, 0o644)

	snapPath, err := Take(workDir, "")
	if err != nil {
		t.Fatal(err)
	}

	restored, err := os.ReadFile(filepath.Join(snapPath, "data.bin"))
	if err != nil {
		t.Fatal(err)
	}

	if len(restored) != len(binaryContent) {
		t.Fatalf("length mismatch: got %d, want %d", len(restored), len(binaryContent))
	}
	for i := range binaryContent {
		if restored[i] != binaryContent[i] {
			t.Errorf("byte %d: got 0x%02x, want 0x%02x", i, restored[i], binaryContent[i])
		}
	}
}

func TestParseSnapshotName(t *testing.T) {
	tests := []struct {
		name      string
		wantLabel string
	}{
		{"2026-04-07T14:30:00Z", ""},
		{"2026-04-08T16:00:00Z--before-research", "before-research"},
		{"2026-04-08T16:00:00Z--pre-restore", "pre-restore"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := parseSnapshotName(tt.name)
			if entry.Label != tt.wantLabel {
				t.Errorf("Label = %q, want %q", entry.Label, tt.wantLabel)
			}
			if entry.Timestamp.IsZero() {
				t.Error("Timestamp should be parsed")
			}
		})
	}
}

func TestListNoHistoryDir(t *testing.T) {
	dir := t.TempDir()
	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0, got %d", len(entries))
	}
}
