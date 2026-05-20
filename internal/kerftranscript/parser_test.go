package kerftranscript_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/kerftranscript"
)

// parserFixtureCase is a uniquely-named test struct (avoiding collision
// with sibling beads landing in the same package, e.g. kerf-4skd).
type parserFixtureCase struct {
	name        string
	wantEvents  int
	wantKinds   []kerftranscript.EventKind
	wantBeadIDs []string
}

// loadParserFixture is the parser-test-specific fixture loader. The name
// is deliberately verbose so it cannot collide with helpers added by
// sibling beads in the same package.
func loadParserFixture(t *testing.T, name string) kerftranscript.Result {
	t.Helper()
	path := filepath.Join("testdata", name)
	res, err := kerftranscript.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
	}
	return res
}

func TestParseFile_d1AbandonA_parser(t *testing.T) {
	res := loadParserFixture(t, "d1_abandon_a.jsonl")
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected parse errors: %#v", res.Errors)
	}
	if got, want := len(res.Events), 4; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}

	wantKinds := []kerftranscript.EventKind{
		kerftranscript.EventDispatch,
		kerftranscript.EventToolResult,
		kerftranscript.EventToolResult,
		kerftranscript.EventBeadClose,
	}
	for i, ev := range res.Events {
		if ev.Kind != wantKinds[i] {
			t.Errorf("event %d: kind = %q, want %q", i, ev.Kind, wantKinds[i])
		}
		if ev.BeadID != "hk-qo08q.15" {
			t.Errorf("event %d: bead_id = %q, want hk-qo08q.15", i, ev.BeadID)
		}
		if ev.SessionID != "fed61a3d-3aa9-4c8a-91e7-0b1acb4ec1e8" {
			t.Errorf("event %d: session_id = %q", i, ev.SessionID)
		}
		if ev.LineNumber != i+1 {
			t.Errorf("event %d: LineNumber = %d, want %d", i, ev.LineNumber, i+1)
		}
	}

	// Spot-check field carriage from the spec event vocabulary table.
	d := res.Events[0]
	if d.SubAgentID != "aa848865eff923eae" {
		t.Errorf("dispatch sub_agent_id = %q", d.SubAgentID)
	}
	if d.Role != "implementer" {
		t.Errorf("dispatch role = %q, want implementer", d.Role)
	}
	if d.Timestamp.IsZero() || d.Timestamp.Location() != time.UTC {
		t.Errorf("dispatch timestamp not normalised to UTC: %v", d.Timestamp)
	}

	bc := res.Events[3]
	if bc.CommitSHA != "4a3c217" {
		t.Errorf("bead_close commit_sha = %q, want 4a3c217", bc.CommitSHA)
	}
}

func TestParseFile_d6ReviewerAbsentB_parser(t *testing.T) {
	// Multi-bead fixture: two commit_refs in the same session, one per
	// bead. specs/diagnostics.md §"Multi-bead transcript fixtures" binds
	// the parser to support per-bead and all-beads query paths.
	res := loadParserFixture(t, "d6_reviewer_absent_b.jsonl")
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected parse errors: %#v", res.Errors)
	}
	if got, want := len(res.Events), 5; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}

	all := kerftranscript.BeadIDs(res.Events)
	wantBeads := map[string]bool{"hk-zixbp": false, "hk-qo08q": false}
	for _, b := range all {
		if _, ok := wantBeads[b]; !ok {
			t.Errorf("BeadIDs returned unexpected %q", b)
		}
		wantBeads[b] = true
	}
	for b, seen := range wantBeads {
		if !seen {
			t.Errorf("BeadIDs missing %q (got %v)", b, all)
		}
	}

	// Per-bead filter must scope to exactly that bead.
	zixbp := kerftranscript.FilterByBead(res.Events, "hk-zixbp")
	if len(zixbp) != 3 {
		t.Errorf("FilterByBead(hk-zixbp) = %d events, want 3", len(zixbp))
	}
	for _, ev := range zixbp {
		if ev.BeadID != "hk-zixbp" {
			t.Errorf("FilterByBead leaked %q", ev.BeadID)
		}
	}

	// Find the commit_ref for hk-zixbp and validate the SHA carriage.
	var foundCommit bool
	for _, ev := range zixbp {
		if ev.Kind == kerftranscript.EventCommitRef {
			if ev.CommitSHA != "cc3da5c1b255fd5bd2c94e859d4d653ae6d1e5c6" {
				t.Errorf("commit_ref sha = %q", ev.CommitSHA)
			}
			foundCommit = true
		}
	}
	if !foundCommit {
		t.Error("no commit_ref event for hk-zixbp")
	}
}

func TestParseFile_allD1AndD6Fixtures_parser(t *testing.T) {
	// Smoke: every shipped fixture parses with zero errors and emits
	// only spec-defined kinds.
	fixtures := []string{
		"d1_abandon_a.jsonl",
		"d1_abandon_b.jsonl",
		"d6_reviewer_absent_a.jsonl",
		"d6_reviewer_absent_b.jsonl",
		"d6_reviewer_absent_c.jsonl",
	}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			res := loadParserFixture(t, name)
			if len(res.Errors) != 0 {
				t.Fatalf("parse errors: %#v", res.Errors)
			}
			if len(res.Events) == 0 {
				t.Fatalf("no events parsed")
			}
			for _, ev := range res.Events {
				if !kerftranscript.IsValidKind(string(ev.Kind)) {
					t.Errorf("invalid kind in output: %q", ev.Kind)
				}
			}
		})
	}
}

func TestParse_skipsMalformedLinesAndReportsThem_parser(t *testing.T) {
	// Mixed input: valid line, invalid JSON, missing kind, unknown
	// kind, blank line, valid line. Per the parser policy doc-comment
	// (skip-and-continue), all four bad lines should appear in
	// Result.Errors and both good lines in Result.Events.
	input := strings.Join([]string{
		`{"timestamp":"2026-05-15T18:10:11Z","kind":"dispatch","session_id":"s1","sub_agent_id":"a1","bead_id":"hk-x"}`,
		`{not valid json`,
		`{"timestamp":"2026-05-15T18:10:12Z","session_id":"s1","bead_id":"hk-x"}`,
		`{"kind":"future_event","session_id":"s1"}`,
		``,
		`{"timestamp":"2026-05-15T18:10:13Z","kind":"commit_ref","bead_id":"hk-x","commit_sha":"abc"}`,
	}, "\n")

	res, err := kerftranscript.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := len(res.Events), 2; got != want {
		t.Fatalf("events = %d, want %d", got, want)
	}
	if got, want := len(res.Errors), 3; got != want {
		t.Fatalf("errors = %d, want %d (errors: %#v)", got, want, res.Errors)
	}

	// LineNumber tracks the on-disk line, including the blank line.
	if res.Events[0].LineNumber != 1 {
		t.Errorf("first valid event LineNumber = %d, want 1", res.Events[0].LineNumber)
	}
	if res.Events[1].LineNumber != 6 {
		t.Errorf("second valid event LineNumber = %d, want 6", res.Events[1].LineNumber)
	}
}

func TestParse_invalidTimestampIsParseError_parser(t *testing.T) {
	input := `{"timestamp":"not-a-time","kind":"dispatch","bead_id":"hk-x"}`
	res, err := kerftranscript.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(res.Events))
	}
	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 parse error, got %d", len(res.Errors))
	}
	if !strings.Contains(res.Errors[0].Err.Error(), "timestamp") {
		t.Errorf("error not about timestamp: %v", res.Errors[0].Err)
	}
}

func TestParse_isErrorBoolCarried_parser(t *testing.T) {
	input := strings.Join([]string{
		`{"kind":"tool_result","is_error":true,"bead_id":"hk-x"}`,
		`{"kind":"tool_result","is_error":false,"bead_id":"hk-x"}`,
		`{"kind":"tool_result","bead_id":"hk-x"}`,
	}, "\n")
	res, err := kerftranscript.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Events) != 3 {
		t.Fatalf("events = %d, want 3", len(res.Events))
	}
	if !res.Events[0].IsError {
		t.Error("event 0 IsError should be true")
	}
	if res.Events[1].IsError {
		t.Error("event 1 IsError should be false")
	}
	if res.Events[2].IsError {
		t.Error("event 2 IsError should default to false")
	}
}

func TestParseFile_missingFile_parser(t *testing.T) {
	_, err := kerftranscript.ParseFile(filepath.Join("testdata", "does-not-exist.jsonl"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFilterByBead_emptyInput_parser(t *testing.T) {
	out := kerftranscript.FilterByBead(nil, "hk-x")
	if len(out) != 0 {
		t.Errorf("FilterByBead(nil) = %v, want empty", out)
	}
}

func TestBeadIDs_dedupesPreservingFirstSeenOrder_parser(t *testing.T) {
	evs := []kerftranscript.Event{
		{BeadID: "b"},
		{BeadID: "a"},
		{BeadID: ""},
		{BeadID: "b"},
		{BeadID: "c"},
	}
	got := kerftranscript.BeadIDs(evs)
	want := []string{"b", "a", "c"}
	if len(got) != len(want) {
		t.Fatalf("BeadIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("BeadIDs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
