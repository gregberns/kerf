package drift

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// cacheFilename is the relative path, under the repo root, of the
// project-local drift baseline. See specs/architecture.md §"In the
// repo, inside git" — `.kerf/sync-cache.json` lives in the repo in
// both storage modes (bench and local) so that worktrees do not share
// drift baselines across branches.
const cacheFilename = ".kerf/sync-cache.json"

// CachePath returns the absolute path to the project's sync cache.
//
// Per specs/architecture.md, `.kerf/sync-cache.json` always lives in
// the repo root, independent of storage mode (bench vs local). The
// path is therefore `{repoRoot}/.kerf/sync-cache.json`. CachePath
// returns an empty string when repoRoot is empty — callers that have
// no repo root (e.g. ad-hoc bench-only invocations with no enclosing
// repo) cannot persist a drift baseline.
//
// (B2 spec ambiguity resolved against beads.md, which described a
// per-mode split — the canonical spec is architecture.md, and it pins
// the cache to the repo in both modes.)
func CachePath(repoRoot string) string {
	if repoRoot == "" {
		return ""
	}
	return filepath.Join(repoRoot, cacheFilename)
}

// Read loads a Snapshot from path.
//
// Return values:
//   - (zero, false, nil) — path does not exist; treat as empty baseline.
//   - (snapshot, true, nil) — file existed and parsed successfully.
//   - (zero, false, err) — file existed but read or parse failed.
func Read(path string) (Snapshot, bool, error) {
	if path == "" {
		return Snapshot{}, false, errors.New("drift: cache path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Snapshot{}, false, nil
		}
		return Snapshot{}, false, fmt.Errorf("drift: read %s: %w", path, err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, false, fmt.Errorf("drift: parse %s: %w", path, err)
	}
	return snap, true, nil
}

// Write persists a Snapshot to path atomically. The parent directory is
// created if needed (0o755). The file is written as a temp file in the
// same directory and renamed into place so an interrupted write leaves
// the previous baseline intact.
//
// File mode is 0o644.
func Write(path string, snap Snapshot) error {
	if path == "" {
		return errors.New("drift: cache path is empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("drift: mkdir %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("drift: marshal snapshot: %w", err)
	}
	// Trailing newline for tooling-friendliness.
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".sync-cache-*.json.tmp")
	if err != nil {
		return fmt.Errorf("drift: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if anything below fails before the rename.
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("drift: write temp file: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("drift: chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("drift: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("drift: rename %s -> %s: %w", tmpName, path, err)
	}
	return nil
}

// Advance is the canonical baseline-advance path. It is documented as
// the single legitimate way to replace the drift baseline: it must be
// called only from `kerf triage --ack`. Other commands (`kerf new`,
// `kerf pin`, `kerf work edit`, etc.) MUST NOT advance the baseline —
// see specs/coordination.md §"Baseline advancement".
//
// Functionally equivalent to Write; the separate name exists to make
// "this is an --ack" auditable in callers.
func Advance(path string, current Snapshot) error {
	return Write(path, current)
}
