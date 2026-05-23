package kerftranscript

// Parsed-transcript cache for `kerf next` diagnostics.
//
// Bead kerf-jcbb (plan-013 perf): every `kerf next` invocation reads all
// ~/.claude/projects/<repo>/*.jsonl and re-parses them. The dominant cost
// is the JSONL parse (megabyte-scale assistant payloads); we cache the
// parsed Event slice keyed on file mtime and the repo's HEAD SHA.
//
// Scope guardrails (per the bead brief — do not relax without a follow-up
// bead):
//   - Single-process, single-machine. No file locks; if two `kerf next`
//     invocations race, the worst case is a redundant re-parse.
//   - No schema migration. cacheVersion bumps drop the file silently.
//   - One recovery path. Any decode failure → delete + rebuild.
//   - No background refresh, TTL, or warming.
//   - HEAD SHA change invalidates ALL entries. This is conservative —
//     transcript parsing is independent of git state — but the bead spec
//     pins HEAD SHA as the only git-side invalidator and lists this case
//     as a required test. `git status --porcelain` drift (uncommitted
//     index changes) is intentionally NOT a cache invalidator; the next
//     invocation with a fresh HEAD SHA picks up the change.
//
// The cache file is `{repoRoot}/.kerf/diagnostics-cache.json`, matching
// the bead brief and the sibling drift cache layout.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

// cacheVersion is the on-disk schema version. Any other version on disk
// causes the cache to be silently dropped and rebuilt. There is no
// migration path; bump this constant when the on-disk shape changes.
const cacheVersion = 1

// CacheFilename is the relative path, under the repo root, of the
// diagnostics cache. Exported so tests and tooling can locate it.
const CacheFilename = ".kerf/diagnostics-cache.json"

// CachePath returns the absolute path to the diagnostics cache. Empty
// repoRoot yields "" — callers without a repo root cannot persist the
// cache and should fall through to direct parsing.
func CachePath(repoRoot string) string {
	if repoRoot == "" {
		return ""
	}
	return filepath.Join(repoRoot, CacheFilename)
}

// cacheFile is the on-disk JSON shape. Field tags are stable; renaming
// any of them requires bumping cacheVersion.
type cacheFile struct {
	Version int                    `json:"version"`
	HeadSHA string                 `json:"head_sha"`
	Entries map[string]cacheEntry  `json:"entries"`
}

// cacheEntry is the per-transcript-file payload: the mtime at parse time
// (Unix nanoseconds) and the parsed events. Stored under the transcript's
// absolute path.
type cacheEntry struct {
	MTimeNano int64   `json:"mtime_nano"`
	Events    []Event `json:"events"`
}

// parseFn is the function used to parse a single transcript file. It is
// the package's ParseFile by default; tests override it to count calls.
type parseFn func(path string) (Result, error)

// LoadOrParse returns the union of parsed transcript events for paths,
// using the on-disk cache at CachePath(repoRoot) when entries are warm.
//
// An entry is warm when:
//   - the cache file's version matches cacheVersion,
//   - the cache file's headSHA equals headSHA, AND
//   - the cached entry's mtime equals the file's current mtime.
//
// Cold entries are re-parsed and the cache is rewritten at the end.
//
// On any cache read failure (missing file, decode error, version
// mismatch, HEAD SHA mismatch), the cache is dropped and rebuilt from
// scratch. There is no graceful degradation cascade — one simple path.
//
// repoRoot may be empty (no enclosing repo); in that case the function
// degrades to parse-every-file and writes nothing.
func LoadOrParse(repoRoot, headSHA string, paths []string) ([]Event, error) {
	return loadOrParseWith(repoRoot, headSHA, paths, ParseFile)
}

// loadOrParseWith is the test seam. Production callers use LoadOrParse.
func loadOrParseWith(repoRoot, headSHA string, paths []string, parse parseFn) ([]Event, error) {
	cachePath := CachePath(repoRoot)

	// Load existing cache (best-effort). Any failure resets to empty.
	cache := loadCache(cachePath, headSHA)

	// Walk paths in source order; per-file mtime check decides reuse.
	out := make([]Event, 0, 256)
	newEntries := make(map[string]cacheEntry, len(paths))
	for _, p := range paths {
		mtime, ok := fileMTimeNano(p)
		if !ok {
			// File vanished between discovery and parse. Skip; do not
			// carry a stale cache entry forward.
			continue
		}
		if entry, ok := cache.Entries[p]; ok && entry.MTimeNano == mtime {
			out = append(out, entry.Events...)
			newEntries[p] = entry
			continue
		}
		res, err := parse(p)
		if err != nil {
			// Skip unreadable files (matches the existing D1/D6 call
			// sites' silent-skip policy). Do not poison the cache with
			// a partial entry.
			continue
		}
		// Bead kerf-ek21: surface per-file parse-error counts. Before
		// this change Result.Errors was silently discarded, which let
		// the kind-vs-type schema mismatch hide for the full lifetime
		// of the diagnostics family (every real-Claude transcript line
		// failed validation, yet `kerf next` produced no warnings and
		// no signal). A one-line stderr summary is the minimum useful
		// signal; the proper warning-kind treatment ("parser_errors")
		// is deferred to a separate spec bead.
		if len(res.Errors) > 0 {
			log.Printf("kerftranscript: %s: %d parse error(s) (first: line %d: %v)",
				p, len(res.Errors), res.Errors[0].LineNumber, res.Errors[0].Err)
		}
		newEntries[p] = cacheEntry{MTimeNano: mtime, Events: res.Events}
		out = append(out, res.Events...)
	}

	// Write back the freshly-computed entry set (always, so removed
	// transcripts age out of the cache). Best-effort; ignore IO errors.
	if cachePath != "" {
		_ = writeCache(cachePath, cacheFile{
			Version: cacheVersion,
			HeadSHA: headSHA,
			Entries: newEntries,
		})
	}

	return out, nil
}

// loadCache reads the on-disk cache. Any failure mode (missing file,
// decode error, version mismatch, HEAD SHA mismatch) returns an empty
// cacheFile so the caller treats every entry as cold. On decode failure
// the file is deleted so the next write starts from a clean slate.
func loadCache(path, headSHA string) cacheFile {
	empty := cacheFile{Version: cacheVersion, HeadSHA: headSHA, Entries: map[string]cacheEntry{}}
	if path == "" {
		return empty
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return empty
		}
		// Unreadable cache; drop it (best-effort).
		_ = os.Remove(path)
		return empty
	}
	var c cacheFile
	if err := json.Unmarshal(data, &c); err != nil {
		_ = os.Remove(path)
		return empty
	}
	if c.Version != cacheVersion {
		_ = os.Remove(path)
		return empty
	}
	if c.HeadSHA != headSHA {
		// HEAD moved; conservatively invalidate all entries (the bead
		// spec pins HEAD SHA as the sole git-side invalidator).
		return empty
	}
	if c.Entries == nil {
		c.Entries = map[string]cacheEntry{}
	}
	return c
}

// writeCache persists the cache atomically (write-temp + rename).
func writeCache(path string, c cacheFile) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("kerftranscript: mkdir %s: %w", dir, err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("kerftranscript: marshal cache: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".diagnostics-cache-*.json.tmp")
	if err != nil {
		return fmt.Errorf("kerftranscript: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// fileMTimeNano returns (mtime as Unix nanoseconds, true) for path, or
// (0, false) if the file is not stat-able.
func fileMTimeNano(path string) (int64, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	return st.ModTime().UnixNano(), true
}
