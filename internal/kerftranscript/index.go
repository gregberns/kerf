package kerftranscript

// Bead-ID indexer over `git log --all` commit messages.
//
// Implements specs/diagnostics.md §"Bead-ID resolution". The indexer scans
// every commit reachable from any ref (worktree branches included), applies
// the project's `bead.id_pattern` regex to each commit's subject and body,
// and keys the commit by **every** bead ID referenced. Subtask IDs of the
// shape `<parent>.<N>` are additionally keyed under the bare parent so D1's
// parent/child rollup query can find them.
//
// The indexer is the load-bearing surface for D1 (abandoned dispatch): a
// dispatched bead is considered to have produced code iff
// `(*Index).HasCommitFor(beadID)` returns true.
//
// This file deliberately uses indexer-scoped identifiers (Index, NewIndex,
// IndexEvidence, etc.) so it does not collide with the parser surface in
// events.go / parser.go (Event, Result, Parse, ParseFile, FilterByBead,
// BeadIDs).

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// IndexEvidence is the evidence trail recorded for a single (beadID, commit)
// pairing. Multiple bead IDs in one commit produce one IndexEvidence per
// keyed bead ID, all sharing the same CommitSHA.
type IndexEvidence struct {
	// CommitSHA is the full SHA of the commit.
	CommitSHA string
	// Subject is the commit's first line.
	Subject string
	// RefNames lists the ref decorations (branches, tags, HEAD) that
	// pointed at this commit at index-build time, as reported by
	// `git log --decorate`. Useful for D1's "worktree-branch refs" rule.
	RefNames []string
	// MatchedID is the exact substring matched by the bead.id_pattern that
	// caused this commit to be indexed under this evidence's owning bead
	// ID. When the owning bead ID is a rollup parent (the commit
	// mentioned `parent.N`), MatchedID is the subtask form, not the bare
	// parent.
	MatchedID string
	// MatchedIn is "subject" or "body" — the part of the commit message
	// where MatchedID was found. When the same ID appears in both, the
	// subject occurrence wins (the IndexEvidence carries "subject").
	MatchedIn string
}

// Index is the bead-ID -> evidence map built from `git log --all`. Use
// NewIndex to construct; the zero value is not useful.
//
// Concurrency: Index is read-only after NewIndex returns; lookup methods
// are safe for concurrent use. Mutating methods are not exposed.
type Index struct {
	// byBead maps a bead ID (canonical, as matched by the pattern, plus
	// rolled-up parents) to the evidence trail for that bead.
	byBead map[string][]IndexEvidence
	// pattern is the regex used to extract bead IDs; retained for the
	// (*Index).Pattern accessor and for tests.
	pattern *regexp.Regexp
}

// gitLogRunner is the function the indexer calls to obtain raw `git log`
// output. It is package-private and overridable in tests so the indexer
// can be exercised without a real git invocation. Production callers go
// through runGitLog.
type gitLogRunner func(repoPath string) ([]byte, error)

// runGitLog invokes `git log --all` with a record format chosen so that
//   - commit boundaries are NUL-terminated records separated by RS (0x1e),
//   - each record is `SHA\x00refs\x00subject\x00body`.
//
// This avoids any ambiguity from newlines inside commit bodies.
func runGitLog(repoPath string) ([]byte, error) {
	const (
		fieldSep  = "%x00"
		recordSep = "%x1e"
	)
	format := "--format=%H" + fieldSep + "%D" + fieldSep + "%s" + fieldSep + "%b" + recordSep
	cmd := exec.Command("git", "log", "--all", "--decorate=full", format)
	cmd.Dir = repoPath
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kerftranscript: git log --all in %s: %w (stderr: %s)", repoPath, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// NewIndex builds a bead-ID index over `git log --all` in repoPath using
// idPattern to extract bead IDs from commit subjects and bodies. The
// pattern source is the project's own configuration (project.yaml:
// bead.id_pattern); kerf does not hard-code a regex.
//
// idPattern is required; passing nil returns an error. repoPath must be a
// path inside (or at the root of) a git repository.
func NewIndex(repoPath string, idPattern *regexp.Regexp) (*Index, error) {
	return newIndexWith(repoPath, idPattern, runGitLog)
}

// newIndexWith is the test seam. Production callers use NewIndex.
func newIndexWith(repoPath string, idPattern *regexp.Regexp, runner gitLogRunner) (*Index, error) {
	if idPattern == nil {
		return nil, fmt.Errorf("kerftranscript: NewIndex: idPattern is required (load from project.yaml: bead.id_pattern)")
	}
	raw, err := runner(repoPath)
	if err != nil {
		return nil, err
	}
	idx := &Index{
		byBead:  make(map[string][]IndexEvidence),
		pattern: idPattern,
	}
	idx.ingest(raw)
	return idx, nil
}

// ingest parses the raw `git log` output produced by runGitLog and
// populates the by-bead map. Malformed records are skipped; this matches
// the parser's continue-on-error policy.
func (i *Index) ingest(raw []byte) {
	// Records are separated by RS (0x1e).
	for _, rec := range bytes.Split(raw, []byte{0x1e}) {
		rec = bytes.TrimLeft(rec, "\n")
		if len(bytes.TrimSpace(rec)) == 0 {
			continue
		}
		parts := bytes.SplitN(rec, []byte{0x00}, 4)
		if len(parts) < 4 {
			continue
		}
		sha := string(bytes.TrimSpace(parts[0]))
		refs := parseRefNames(string(parts[1]))
		subject := string(parts[2])
		body := string(parts[3])
		if sha == "" {
			continue
		}

		// Per spec: extract from subject AND body, key by every ID
		// referenced, then roll up subtask IDs to bare parents.
		seenForCommit := make(map[string]string) // beadID -> "subject" | "body"

		for _, m := range i.pattern.FindAllString(subject, -1) {
			if _, ok := seenForCommit[m]; !ok {
				seenForCommit[m] = "subject"
			}
		}
		for _, m := range i.pattern.FindAllString(body, -1) {
			if _, ok := seenForCommit[m]; !ok {
				seenForCommit[m] = "body"
			}
		}

		// Roll up subtask IDs to their bare parent. Parent IDs are
		// indexed alongside the subtask so a D1 query for the parent
		// finds the subtask commit.
		for id, where := range seenForCommit {
			ev := IndexEvidence{
				CommitSHA: sha,
				Subject:   subject,
				RefNames:  refs,
				MatchedID: id,
				MatchedIn: where,
			}
			i.byBead[id] = append(i.byBead[id], ev)
			if parent, ok := subtaskParent(id); ok {
				// Don't double-record if the parent was also
				// matched independently from the same commit.
				if _, parentMatched := seenForCommit[parent]; !parentMatched {
					i.byBead[parent] = append(i.byBead[parent], ev)
				}
			}
		}
	}
}

// subtaskParent returns (parent, true) when id has the form
// "<parent>.<N>" where N is purely digits. Otherwise it returns ("",
// false). The parent half is whatever precedes the final ".N" segment.
// Multi-level subtasks (a.b.1) roll up only one step (a.b).
func subtaskParent(id string) (string, bool) {
	dot := strings.LastIndex(id, ".")
	if dot <= 0 || dot == len(id)-1 {
		return "", false
	}
	suffix := id[dot+1:]
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return id[:dot], true
}

// parseRefNames splits the `git log --decorate=full %D` ref decoration
// into individual ref names. The decoration looks like
// "HEAD -> refs/heads/main, refs/remotes/origin/main, refs/tags/v1".
// Empty input yields nil.
func parseRefNames(deco string) []string {
	deco = strings.TrimSpace(deco)
	if deco == "" {
		return nil
	}
	out := make([]string, 0, 4)
	for _, part := range strings.Split(deco, ",") {
		p := strings.TrimSpace(part)
		// Strip "HEAD -> " prefix on the leading entry.
		if strings.HasPrefix(p, "HEAD -> ") {
			head := strings.TrimSpace(strings.TrimPrefix(p, "HEAD -> "))
			out = append(out, "HEAD")
			if head != "" {
				out = append(out, head)
			}
			continue
		}
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// HasCommitFor reports whether the index contains at least one commit
// referencing beadID, after the parent/child rollup described in
// specs/diagnostics.md §"Bead-ID resolution". This is the D1 detector's
// "did this dispatch produce code?" predicate.
func (i *Index) HasCommitFor(beadID string) bool {
	if i == nil {
		return false
	}
	_, ok := i.byBead[beadID]
	return ok
}

// Evidence returns the evidence trail for beadID, or nil when the index
// has no commits keyed under it. The returned slice is the index's own
// storage; callers must not mutate it.
func (i *Index) Evidence(beadID string) []IndexEvidence {
	if i == nil {
		return nil
	}
	return i.byBead[beadID]
}

// IndexedBeadIDs returns every bead ID present in the index, sorted
// lexicographically. The name is deliberately distinct from the parser's
// package-level BeadIDs function (which extracts bead IDs from a slice of
// transcript events).
func (i *Index) IndexedBeadIDs() []string {
	if i == nil {
		return nil
	}
	out := make([]string, 0, len(i.byBead))
	for id := range i.byBead {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Pattern returns the regex the index was built with. Useful for callers
// (e.g. D1's detector) that also need to scan transcript event text with
// the same pattern for consistent ID extraction.
func (i *Index) Pattern() *regexp.Regexp {
	if i == nil {
		return nil
	}
	return i.pattern
}
