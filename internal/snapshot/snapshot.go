package snapshot

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gberns/kerf/internal/spec"
)

const historyDir = ".history"

// SnapshotEntry represents a single snapshot in the history.
type SnapshotEntry struct {
	Name      string
	Timestamp time.Time
	Label     string // empty for automatic snapshots
	Status    string // from the snapshot's spec.yaml
}

// Take creates a timestamped snapshot of the work directory.
// If label is non-empty, it is appended to the directory name as {timestamp}--{label}.
func Take(workDir string, label string) (string, error) {
	now := time.Now().UTC().Truncate(time.Second)
	dirName := now.Format(time.RFC3339)
	if label != "" {
		dirName = dirName + "--" + label
	}

	histDir := filepath.Join(workDir, historyDir)
	snapDir := filepath.Join(histDir, dirName)

	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return "", fmt.Errorf("creating snapshot directory: %w", err)
	}

	if err := copyWorkDir(workDir, snapDir); err != nil {
		// Clean up on failure
		os.RemoveAll(snapDir)
		return "", fmt.Errorf("creating snapshot: %w", err)
	}

	return snapDir, nil
}

// List returns all snapshots in the work's .history/ directory, sorted newest first.
func List(workDir string) ([]SnapshotEntry, error) {
	histDir := filepath.Join(workDir, historyDir)
	entries, err := os.ReadDir(histDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading history directory: %w", err)
	}

	var snapshots []SnapshotEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		entry := parseSnapshotName(e.Name())

		// Read status from the snapshot's spec.yaml
		specPath := filepath.Join(histDir, e.Name(), "spec.yaml")
		if s, err := spec.Read(specPath); err == nil {
			entry.Status = s.Status
		}

		snapshots = append(snapshots, entry)
	}

	// Sort newest first
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})

	return snapshots, nil
}

// Restore replaces the current work state with the contents of a snapshot.
// It takes a pre-restore snapshot first. Returns the pre-restore snapshot path.
// Session data (sessions list + active_session) is preserved from the current state.
func Restore(workDir string, snapshotName string) (string, error) {
	snapDir := filepath.Join(workDir, historyDir, snapshotName)
	if _, err := os.Stat(snapDir); err != nil {
		return "", fmt.Errorf("snapshot %q not found", snapshotName)
	}

	// Read current session data before restoring
	currentSpecPath := filepath.Join(workDir, "spec.yaml")
	currentSpec, err := spec.Read(currentSpecPath)
	if err != nil {
		return "", fmt.Errorf("reading current spec.yaml: %w", err)
	}
	savedSessions := currentSpec.Sessions
	savedActiveSession := currentSpec.ActiveSession

	// Take pre-restore snapshot
	preRestorePath, err := Take(workDir, "pre-restore")
	if err != nil {
		return "", fmt.Errorf("taking pre-restore snapshot: %w", err)
	}

	// Remove current files (except .history/)
	if err := clearWorkDir(workDir); err != nil {
		return preRestorePath, fmt.Errorf("clearing work directory: %w", err)
	}

	// Copy snapshot files to work dir
	if err := copyDir(snapDir, workDir); err != nil {
		return preRestorePath, fmt.Errorf("restoring from snapshot: %w", err)
	}

	// Preserve session data
	restoredSpec, err := spec.Read(currentSpecPath)
	if err != nil {
		return preRestorePath, fmt.Errorf("reading restored spec.yaml: %w", err)
	}
	restoredSpec.Sessions = savedSessions
	restoredSpec.ActiveSession = savedActiveSession
	if err := spec.Write(currentSpecPath, restoredSpec); err != nil {
		return preRestorePath, fmt.Errorf("preserving session data: %w", err)
	}

	return preRestorePath, nil
}

// Prune removes the oldest snapshots beyond the limit.
func Prune(workDir string, maxSnapshots int) error {
	snapshots, err := List(workDir)
	if err != nil {
		return err
	}

	if len(snapshots) <= maxSnapshots {
		return nil
	}

	// List is sorted newest first; remove from the end (oldest)
	histDir := filepath.Join(workDir, historyDir)
	toRemove := snapshots[maxSnapshots:]
	for _, s := range toRemove {
		if err := os.RemoveAll(filepath.Join(histDir, s.Name)); err != nil {
			return fmt.Errorf("pruning snapshot %s: %w", s.Name, err)
		}
	}

	return nil
}

// CheckInterval returns true if enough time has elapsed since the last snapshot
// to warrant an interval-based snapshot.
func CheckInterval(workDir string, intervalSeconds int) (bool, error) {
	snapshots, err := List(workDir)
	if err != nil {
		return false, err
	}

	// No snapshots — exceeds threshold
	if len(snapshots) == 0 {
		return true, nil
	}

	// snapshots[0] is the newest
	elapsed := time.Since(snapshots[0].Timestamp)
	return elapsed >= time.Duration(intervalSeconds)*time.Second, nil
}

// parseSnapshotName extracts timestamp and optional label from a snapshot directory name.
func parseSnapshotName(name string) SnapshotEntry {
	entry := SnapshotEntry{Name: name}

	parts := strings.SplitN(name, "--", 2)
	tsStr := parts[0]
	if len(parts) == 2 {
		entry.Label = parts[1]
	}

	if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
		entry.Timestamp = t
	}

	return entry
}

// copyWorkDir copies all files/dirs from workDir into snapDir, excluding .history/.
func copyWorkDir(workDir, snapDir string) error {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.Name() == historyDir {
			continue
		}

		src := filepath.Join(workDir, e.Name())
		dst := filepath.Join(snapDir, e.Name())

		if e.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return err
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				return err
			}
		}
	}
	return nil
}

// clearWorkDir removes all files/dirs in workDir except .history/.
func clearWorkDir(workDir string) error {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.Name() == historyDir {
			continue
		}
		if err := os.RemoveAll(filepath.Join(workDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())

		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
