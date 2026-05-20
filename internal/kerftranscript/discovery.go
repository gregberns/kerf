package kerftranscript

// Transcript discovery. Implements specs/diagnostics.md §"Transcript
// discovery and parsing":
//
//   - Canonical path template:
//     ~/.claude/projects/<encoded-repo>/, where <encoded-repo> is the
//     absolute repo path with `/` replaced by `-` and a leading `-`.
//   - Override: KERF_TRANSCRIPT_DIR, when set and non-empty, replaces
//     the canonical template wholesale (used as given; no further
//     encoding).
//   - Scoping (v1): Claude Code on macOS only. Other harnesses and
//     other operating systems are not supported; missing-directory and
//     no-jsonl-files are silent no-ops (zero findings, no error).

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TranscriptDirEnvVar is the env var that overrides the canonical
// transcript-directory template. Spec: specs/diagnostics.md §"Override".
const TranscriptDirEnvVar = "KERF_TRANSCRIPT_DIR"

// ResolveTranscriptDir returns the path to scan for Claude Code session
// transcripts for the given absolute repoPath, applying the
// KERF_TRANSCRIPT_DIR override when set. The returned path is not
// validated for existence — that is the discovery layer's job (see
// DiscoverTranscripts).
//
// repoPath should be the absolute repo root. When empty the canonical
// template cannot be formed and the function returns "" (the caller
// then treats discovery as a silent no-op).
func ResolveTranscriptDir(repoPath string) string {
	if v := strings.TrimSpace(os.Getenv(TranscriptDirEnvVar)); v != "" {
		return v
	}
	if repoPath == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		abs = repoPath
	}
	// Encoding: replace every `/` with `-`. The absolute path already
	// starts with `/`, which becomes the spec-required leading `-`.
	encoded := strings.ReplaceAll(abs, "/", "-")
	return filepath.Join(home, ".claude", "projects", encoded)
}

// DiscoverTranscripts returns the list of *.jsonl files in dir, sorted
// lexicographically. A non-existent dir, an unreadable dir, or a dir
// containing no *.jsonl files yields (nil, nil) — silent no-op per
// specs/diagnostics.md §"Discovery failure is silent". Non-IO errors
// other than not-exist propagate.
func DiscoverTranscripts(dir string) ([]string, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		// Permission-denied / other IO: treat as silent (spec: "do not
		// surface a kerf next warning of their own for 'no
		// transcripts'"). We still return the error so callers that
		// want to log it may; the diagnostic call path discards it.
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	sort.Strings(out)
	return out, nil
}
