package kerftranscript

// Internal tests for the bead-ID indexer. Lives in the kerftranscript
// package (not kerftranscript_test) so it can exercise the unexported
// newIndexWith test seam without inflating the public surface.
//
// Test-helper names use the _indexer suffix to mirror the parser's
// _parser suffix convention and to guarantee no collision with parser
// test helpers (parserFixtureCase, loadParserFixture) that live in the
// external kerftranscript_test package.

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// indexerFixtureCommit describes one synthetic commit that the
// indexer-test git-log faker should emit. Keeping this struct close to
// the test that consumes it avoids a parser-style cross-file fixture
// loader.
type indexerFixtureCommit struct {
	SHA     string
	Refs    string // raw decoration string, e.g. "HEAD -> refs/heads/main"
	Subject string
	Body    string
}

// buildFakeGitLog produces a `git log` byte stream matching the format
// expected by (*Index).ingest: SHA \x00 refs \x00 subject \x00 body, each
// record separated by 0x1e.
func buildFakeGitLog_indexer(commits []indexerFixtureCommit) []byte {
	var buf bytes.Buffer
	for _, c := range commits {
		buf.WriteString(c.SHA)
		buf.WriteByte(0x00)
		buf.WriteString(c.Refs)
		buf.WriteByte(0x00)
		buf.WriteString(c.Subject)
		buf.WriteByte(0x00)
		buf.WriteString(c.Body)
		buf.WriteByte(0x1e)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// kerfPattern_indexer is a regex that matches the kerf/hk bead-ID shapes
// observed in the existing testdata: "<prefix>-<5 alnum>" optionally
// followed by ".<digits>" for subtasks. Real callers load this from
// project.yaml: bead.id_pattern.
var kerfPattern_indexer = regexp.MustCompile(`\b[a-z]+-[a-z0-9]{4,6}(?:\.\d+)?\b`)

func TestNewIndex_requiresPattern_indexer(t *testing.T) {
	_, err := NewIndex("/does/not/matter", nil)
	if err == nil {
		t.Fatalf("NewIndex with nil pattern: want error, got nil")
	}
	if !strings.Contains(err.Error(), "idPattern is required") {
		t.Errorf("error = %q, want one mentioning idPattern", err.Error())
	}
}

func TestIndex_HasCommitFor_subjectMatch_indexer(t *testing.T) {
	fake := buildFakeGitLog_indexer([]indexerFixtureCommit{
		{
			SHA:     "abc123",
			Refs:    "HEAD -> refs/heads/main",
			Subject: "kerf-8tnq: parser landing",
			Body:    "",
		},
	})
	idx, err := newIndexWith("/tmp/irrelevant", kerfPattern_indexer, func(string) ([]byte, error) {
		return fake, nil
	})
	if err != nil {
		t.Fatalf("newIndexWith: %v", err)
	}
	if !idx.HasCommitFor("kerf-8tnq") {
		t.Errorf("HasCommitFor(kerf-8tnq) = false, want true")
	}
	if idx.HasCommitFor("kerf-zzzz") {
		t.Errorf("HasCommitFor(kerf-zzzz) = true, want false")
	}
}

func TestIndex_bodyAndSubject_indexer(t *testing.T) {
	fake := buildFakeGitLog_indexer([]indexerFixtureCommit{
		{
			SHA:     "deadbe1",
			Refs:    "",
			Subject: "kerf-4skd: indexer",
			Body:    "Refs kerf-gtpu and kerf-8tnq.\n\nBead: kerf-4skd\n",
		},
	})
	idx, err := newIndexWith(".", kerfPattern_indexer, func(string) ([]byte, error) { return fake, nil })
	if err != nil {
		t.Fatalf("newIndexWith: %v", err)
	}
	want := []string{"kerf-4skd", "kerf-8tnq", "kerf-gtpu"}
	got := idx.IndexedBeadIDs()
	if len(got) != len(want) {
		t.Fatalf("IndexedBeadIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("IndexedBeadIDs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Evidence for kerf-4skd should record the subject match, not body
	// (subject occurrence wins when an ID appears in both).
	ev := idx.Evidence("kerf-4skd")
	if len(ev) != 1 {
		t.Fatalf("Evidence(kerf-4skd) len = %d, want 1", len(ev))
	}
	if ev[0].MatchedIn != "subject" {
		t.Errorf("MatchedIn = %q, want subject", ev[0].MatchedIn)
	}
	if ev[0].CommitSHA != "deadbe1" {
		t.Errorf("CommitSHA = %q, want deadbe1", ev[0].CommitSHA)
	}
	// kerf-gtpu is body-only.
	gtpu := idx.Evidence("kerf-gtpu")
	if len(gtpu) != 1 || gtpu[0].MatchedIn != "body" {
		t.Errorf("Evidence(kerf-gtpu) = %+v, want one body match", gtpu)
	}
}

func TestIndex_subtaskRollup_indexer(t *testing.T) {
	// A commit on a subtask ID (hk-qo08q.15) should be discoverable by
	// the bare parent ID (hk-qo08q) per spec §"Aliasing".
	fake := buildFakeGitLog_indexer([]indexerFixtureCommit{
		{
			SHA:     "feed001",
			Refs:    "refs/heads/wt/impl-hk-qo08q-15",
			Subject: "hk-qo08q.15: do the thing",
			Body:    "",
		},
	})
	idx, err := newIndexWith(".", kerfPattern_indexer, func(string) ([]byte, error) { return fake, nil })
	if err != nil {
		t.Fatalf("newIndexWith: %v", err)
	}
	if !idx.HasCommitFor("hk-qo08q.15") {
		t.Errorf("HasCommitFor(hk-qo08q.15) = false, want true (exact match)")
	}
	if !idx.HasCommitFor("hk-qo08q") {
		t.Errorf("HasCommitFor(hk-qo08q) = false, want true (rollup)")
	}
	// Rollup evidence must carry the subtask form as MatchedID so the
	// trail tells the truth.
	ev := idx.Evidence("hk-qo08q")
	if len(ev) != 1 {
		t.Fatalf("rollup Evidence len = %d, want 1", len(ev))
	}
	if ev[0].MatchedID != "hk-qo08q.15" {
		t.Errorf("rollup MatchedID = %q, want hk-qo08q.15", ev[0].MatchedID)
	}
}

func TestIndex_subtaskRollup_doesNotDoubleRecord_indexer(t *testing.T) {
	// When the same commit references both parent and subtask, the
	// parent gets one evidence entry, not two.
	fake := buildFakeGitLog_indexer([]indexerFixtureCommit{
		{
			SHA:     "bead777",
			Refs:    "",
			Subject: "hk-qo08q: parent rollup",
			Body:    "Closes hk-qo08q.15\n",
		},
	})
	idx, err := newIndexWith(".", kerfPattern_indexer, func(string) ([]byte, error) { return fake, nil })
	if err != nil {
		t.Fatalf("newIndexWith: %v", err)
	}
	ev := idx.Evidence("hk-qo08q")
	if len(ev) != 1 {
		t.Errorf("Evidence(hk-qo08q) len = %d, want 1 (no double-record); got %+v", len(ev), ev)
	}
}

func TestIndex_refNames_indexer(t *testing.T) {
	fake := buildFakeGitLog_indexer([]indexerFixtureCommit{
		{
			SHA:     "cafe001",
			Refs:    "HEAD -> refs/heads/main, refs/remotes/origin/main, refs/heads/wt/impl-kerf-4skd",
			Subject: "kerf-4skd: indexer",
			Body:    "",
		},
	})
	idx, err := newIndexWith(".", kerfPattern_indexer, func(string) ([]byte, error) { return fake, nil })
	if err != nil {
		t.Fatalf("newIndexWith: %v", err)
	}
	ev := idx.Evidence("kerf-4skd")
	if len(ev) != 1 {
		t.Fatalf("Evidence len = %d, want 1", len(ev))
	}
	want := []string{"HEAD", "refs/heads/main", "refs/remotes/origin/main", "refs/heads/wt/impl-kerf-4skd"}
	if len(ev[0].RefNames) != len(want) {
		t.Fatalf("RefNames = %v, want %v", ev[0].RefNames, want)
	}
	for i := range want {
		if ev[0].RefNames[i] != want[i] {
			t.Errorf("RefNames[%d] = %q, want %q", i, ev[0].RefNames[i], want[i])
		}
	}
}

func TestIndex_multipleCommitsPerBead_indexer(t *testing.T) {
	fake := buildFakeGitLog_indexer([]indexerFixtureCommit{
		{SHA: "aaa", Refs: "", Subject: "kerf-abcde: first", Body: ""},
		{SHA: "bbb", Refs: "", Subject: "kerf-abcde: second", Body: ""},
		{SHA: "ccc", Refs: "", Subject: "other-fghij: unrelated", Body: ""},
	})
	idx, err := newIndexWith(".", kerfPattern_indexer, func(string) ([]byte, error) { return fake, nil })
	if err != nil {
		t.Fatalf("newIndexWith: %v", err)
	}
	ev := idx.Evidence("kerf-abcde")
	if len(ev) != 2 {
		t.Fatalf("len = %d, want 2; got %+v", len(ev), ev)
	}
	shas := []string{ev[0].CommitSHA, ev[1].CommitSHA}
	sort.Strings(shas)
	if shas[0] != "aaa" || shas[1] != "bbb" {
		t.Errorf("SHAs = %v, want [aaa bbb]", shas)
	}
}

func TestIndex_emptyAndMalformed_indexer(t *testing.T) {
	// Empty stream: empty index, no panic.
	idx, err := newIndexWith(".", kerfPattern_indexer, func(string) ([]byte, error) { return nil, nil })
	if err != nil {
		t.Fatalf("newIndexWith(empty): %v", err)
	}
	if len(idx.IndexedBeadIDs()) != 0 {
		t.Errorf("empty stream produced %d IDs, want 0", len(idx.IndexedBeadIDs()))
	}
	// Malformed record (too few fields) is skipped, well-formed sibling kept.
	raw := []byte("malformed-record-no-nuls\x1e\nsha1\x00\x00kerf-abcde: ok\x00body\x1e\n")
	idx2, err := newIndexWith(".", kerfPattern_indexer, func(string) ([]byte, error) { return raw, nil })
	if err != nil {
		t.Fatalf("newIndexWith: %v", err)
	}
	if !idx2.HasCommitFor("kerf-abcde") {
		t.Errorf("well-formed record missing after malformed sibling")
	}
}

func TestSubtaskParent_indexer(t *testing.T) {
	cases := []struct {
		in        string
		wantOk    bool
		wantValue string
	}{
		{"kerf-abc", false, ""},
		{"kerf-abc.1", true, "kerf-abc"},
		{"kerf-abc.42", true, "kerf-abc"},
		{"kerf-abc.x", false, ""},
		{".5", false, ""},
		{"kerf-abc.", false, ""},
		{"kerf-abc.1.2", true, "kerf-abc.1"},
	}
	for _, c := range cases {
		got, ok := subtaskParent(c.in)
		if ok != c.wantOk || got != c.wantValue {
			t.Errorf("subtaskParent(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.wantValue, c.wantOk)
		}
	}
}

func TestIndex_nilReceiver_indexer(t *testing.T) {
	// Nil-safe accessors so D1 detector code can be linear without
	// guard noise when the index couldn't be built.
	var i *Index
	if i.HasCommitFor("kerf-anything") {
		t.Errorf("nil receiver HasCommitFor = true, want false")
	}
	if i.Evidence("kerf-anything") != nil {
		t.Errorf("nil receiver Evidence != nil")
	}
	if i.IndexedBeadIDs() != nil {
		t.Errorf("nil receiver IndexedBeadIDs != nil")
	}
	if i.Pattern() != nil {
		t.Errorf("nil receiver Pattern != nil")
	}
}
