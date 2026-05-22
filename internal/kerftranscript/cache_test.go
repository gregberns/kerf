package kerftranscript

// Tests for the parsed-transcript cache (Bead kerf-jcbb).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeJSONL creates a transcript file at path with two minimal valid
// events. Returns the file path.
func writeJSONL(t *testing.T, path string, beadID string) {
	t.Helper()
	line1 := `{"timestamp":"2025-01-01T00:00:00Z","kind":"dispatch","session_id":"S1","sub_agent_id":"A1","bead_id":"` + beadID + `","role":"implementer","text":"work ` + beadID + `"}`
	line2 := `{"timestamp":"2025-01-01T00:01:00Z","kind":"commit_ref","session_id":"S1","bead_id":"` + beadID + `","commit_sha":"abc123"}`
	if err := os.WriteFile(path, []byte(line1+"\n"+line2+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// countingParser wraps ParseFile and records per-path call counts.
type countingParser struct {
	calls map[string]int
}

func newCountingParser() *countingParser {
	return &countingParser{calls: map[string]int{}}
}

func (c *countingParser) parse(path string) (Result, error) {
	c.calls[path]++
	return ParseFile(path)
}

// setMTime bumps a file's mtime so the cache treats it as stale. Adds
// 2 seconds to the current mtime to clear any FS resolution rounding.
func setMTime(t *testing.T, path string) {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	newT := st.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(path, newT, newT); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func TestLoadOrParse_emptyCacheParsesAndWrites(t *testing.T) {
	repo := t.TempDir()
	p1 := filepath.Join(repo, "a.jsonl")
	writeJSONL(t, p1, "bd-aaa")

	cp := newCountingParser()
	events, err := loadOrParseWith(repo, "HEAD1", []string{p1}, cp.parse)
	if err != nil {
		t.Fatalf("LoadOrParse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if cp.calls[p1] != 1 {
		t.Fatalf("parse calls for %s = %d, want 1", p1, cp.calls[p1])
	}
	// Cache file written.
	if _, err := os.Stat(CachePath(repo)); err != nil {
		t.Fatalf("cache file not written: %v", err)
	}
}

func TestLoadOrParse_warmCacheSkipsParse(t *testing.T) {
	repo := t.TempDir()
	p1 := filepath.Join(repo, "a.jsonl")
	writeJSONL(t, p1, "bd-aaa")

	cp := newCountingParser()
	if _, err := loadOrParseWith(repo, "HEAD1", []string{p1}, cp.parse); err != nil {
		t.Fatal(err)
	}
	// Second call with same mtime + HEAD → no re-parse.
	events, err := loadOrParseWith(repo, "HEAD1", []string{p1}, cp.parse)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if cp.calls[p1] != 1 {
		t.Fatalf("parse calls for %s = %d, want 1 (cache miss)", p1, cp.calls[p1])
	}
}

func TestLoadOrParse_mtimeBumpReparsesOneFile(t *testing.T) {
	repo := t.TempDir()
	p1 := filepath.Join(repo, "a.jsonl")
	p2 := filepath.Join(repo, "b.jsonl")
	writeJSONL(t, p1, "bd-aaa")
	writeJSONL(t, p2, "bd-bbb")

	cp := newCountingParser()
	if _, err := loadOrParseWith(repo, "HEAD1", []string{p1, p2}, cp.parse); err != nil {
		t.Fatal(err)
	}
	if cp.calls[p1] != 1 || cp.calls[p2] != 1 {
		t.Fatalf("initial parse calls = %v, want both 1", cp.calls)
	}

	// Bump only p1; p2 should remain cached.
	setMTime(t, p1)
	if _, err := loadOrParseWith(repo, "HEAD1", []string{p1, p2}, cp.parse); err != nil {
		t.Fatal(err)
	}
	if cp.calls[p1] != 2 {
		t.Fatalf("p1 parse calls = %d, want 2 (re-parsed)", cp.calls[p1])
	}
	if cp.calls[p2] != 1 {
		t.Fatalf("p2 parse calls = %d, want 1 (still cached)", cp.calls[p2])
	}
}

func TestLoadOrParse_headSHAChangeInvalidatesAll(t *testing.T) {
	repo := t.TempDir()
	p1 := filepath.Join(repo, "a.jsonl")
	p2 := filepath.Join(repo, "b.jsonl")
	writeJSONL(t, p1, "bd-aaa")
	writeJSONL(t, p2, "bd-bbb")

	cp := newCountingParser()
	if _, err := loadOrParseWith(repo, "HEAD1", []string{p1, p2}, cp.parse); err != nil {
		t.Fatal(err)
	}
	// HEAD moved → both files re-parsed even though mtimes unchanged.
	if _, err := loadOrParseWith(repo, "HEAD2", []string{p1, p2}, cp.parse); err != nil {
		t.Fatal(err)
	}
	if cp.calls[p1] != 2 || cp.calls[p2] != 2 {
		t.Fatalf("after HEAD change, parse calls = %v, want both 2", cp.calls)
	}
}

func TestLoadOrParse_corruptJSONRebuilds(t *testing.T) {
	repo := t.TempDir()
	p1 := filepath.Join(repo, "a.jsonl")
	writeJSONL(t, p1, "bd-aaa")

	// Pre-populate cache with garbage.
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CachePath(repo), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp := newCountingParser()
	events, err := loadOrParseWith(repo, "HEAD1", []string{p1}, cp.parse)
	if err != nil {
		t.Fatalf("LoadOrParse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if cp.calls[p1] != 1 {
		t.Fatalf("parse calls = %d, want 1 (rebuilt)", cp.calls[p1])
	}
	// Cache should now be valid JSON again.
	data, err := os.ReadFile(CachePath(repo))
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("cache not valid JSON after rebuild: %v", err)
	}
	if cf.Version != cacheVersion {
		t.Fatalf("cache version = %d, want %d", cf.Version, cacheVersion)
	}
}

func TestLoadOrParse_versionMismatchRebuilds(t *testing.T) {
	repo := t.TempDir()
	p1 := filepath.Join(repo, "a.jsonl")
	writeJSONL(t, p1, "bd-aaa")

	// Pre-populate cache with a wrong-version payload that otherwise
	// looks valid.
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := cacheFile{
		Version: 99, // not cacheVersion
		HeadSHA: "HEAD1",
		Entries: map[string]cacheEntry{
			p1: {MTimeNano: 1, Events: []Event{{Kind: EventDispatch, BeadID: "stale"}}},
		},
	}
	data, _ := json.Marshal(stale)
	if err := os.WriteFile(CachePath(repo), data, 0o644); err != nil {
		t.Fatal(err)
	}

	cp := newCountingParser()
	events, err := loadOrParseWith(repo, "HEAD1", []string{p1}, cp.parse)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2 (stale entry must not leak)", len(events))
	}
	if cp.calls[p1] != 1 {
		t.Fatalf("parse calls = %d, want 1", cp.calls[p1])
	}
}
