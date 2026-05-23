package kerftranscript

// Unit tests for the real-Claude-Code JSONL adapter (bead kerf-ek21).
// These tests exercise parseClaudeJSONL line-by-line against synthetic
// inputs shaped like the production transcripts in
// ~/.claude/projects/<repo>/*.jsonl. A separate calibration test
// (claude_adapter_calibration_test.go) replays an anonymised slice of a
// real transcript end-to-end through Parse.

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestParseClaudeJSONL_assistantAgentDispatch(t *testing.T) {
	line := []byte(`{"type":"assistant","timestamp":"2026-05-20T21:21:02.519Z","sessionId":"sess-1","uuid":"u1","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_AAA","name":"Agent","input":{"description":"impl kerf-abcd","subagent_type":"general-purpose","prompt":"You are an implementer agent for bead **kerf-abcd**."}}]}}`)

	evs, err := parseClaudeJSONL(line, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	ev := evs[0]
	if ev.Kind != EventDispatch {
		t.Fatalf("Kind = %q, want %q", ev.Kind, EventDispatch)
	}
	if ev.SubAgentID != "toolu_AAA" {
		t.Errorf("SubAgentID = %q, want toolu_AAA", ev.SubAgentID)
	}
	if ev.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", ev.SessionID)
	}
	if !strings.Contains(ev.Text, "kerf-abcd") {
		t.Errorf("Text = %q, want to contain bead id", ev.Text)
	}
	if ev.Timestamp.IsZero() {
		t.Errorf("Timestamp zero")
	}
}

func TestParseClaudeJSONL_userSingleToolResult(t *testing.T) {
	line := []byte(`{"type":"user","timestamp":"2026-05-20T21:29:45.735Z","sessionId":"sess-1","uuid":"u2","message":{"role":"user","content":[{"tool_use_id":"toolu_AAA","type":"tool_result","is_error":false,"content":"Done."}]}}`)

	evs, err := parseClaudeJSONL(line, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	ev := evs[0]
	if ev.Kind != EventToolResult {
		t.Fatalf("Kind = %q, want %q", ev.Kind, EventToolResult)
	}
	if ev.SubAgentID != "toolu_AAA" {
		t.Errorf("SubAgentID = %q, want toolu_AAA (the tool_use_id)", ev.SubAgentID)
	}
	if ev.IsError {
		t.Errorf("IsError = true, want false")
	}
	if ev.Text != "Done." {
		t.Errorf("Text = %q, want %q", ev.Text, "Done.")
	}
}

func TestParseClaudeJSONL_userMultipleToolResults(t *testing.T) {
	line := []byte(`{"type":"user","timestamp":"2026-05-20T21:29:45.735Z","sessionId":"sess-1","uuid":"u3","message":{"role":"user","content":[` +
		`{"tool_use_id":"toolu_A","type":"tool_result","is_error":false,"content":"first"},` +
		`{"tool_use_id":"toolu_B","type":"tool_result","is_error":true,"content":[{"type":"text","text":"second"}]}` +
		`]}}`)

	evs, err := parseClaudeJSONL(line, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(evs) != 2 {
		t.Fatalf("got %d events, want 2", len(evs))
	}
	if evs[0].SubAgentID != "toolu_A" || evs[0].Text != "first" || evs[0].IsError {
		t.Errorf("first event mismatch: %+v", evs[0])
	}
	if evs[1].SubAgentID != "toolu_B" || evs[1].Text != "second" || !evs[1].IsError {
		t.Errorf("second event mismatch: %+v", evs[1])
	}
}

func TestParseClaudeJSONL_irrelevantTypes(t *testing.T) {
	cases := []string{
		`{"type":"system","timestamp":"2026-05-20T21:00:00.000Z"}`,
		`{"type":"attachment","timestamp":"2026-05-20T21:00:00.000Z"}`,
		`{"type":"permission-mode","timestamp":"2026-05-20T21:00:00.000Z"}`,
		`{"type":"ai-title","timestamp":"2026-05-20T21:00:00.000Z"}`,
		`{"type":"last-prompt","timestamp":"2026-05-20T21:00:00.000Z"}`,
		`{"type":"file-history-snapshot","timestamp":"2026-05-20T21:00:00.000Z"}`,
	}
	for _, c := range cases {
		evs, err := parseClaudeJSONL([]byte(c), 1)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c, err)
		}
		if len(evs) != 0 {
			t.Errorf("%s: got %d events, want 0", c, len(evs))
		}
	}
}

func TestParseClaudeJSONL_assistantTextOnlyNoEvents(t *testing.T) {
	// Assistant message with only text/thinking/non-Agent tool_use
	// blocks must NOT produce events.
	line := []byte(`{"type":"assistant","timestamp":"2026-05-20T21:00:00.000Z","sessionId":"s","uuid":"u","message":{"role":"assistant","content":[{"type":"text","text":"hi"},{"type":"tool_use","id":"toolu_X","name":"Bash","input":{"command":"ls"}}]}}`)
	evs, err := parseClaudeJSONL(line, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(evs) != 0 {
		t.Fatalf("got %d events, want 0", len(evs))
	}
}

func TestParseClaudeJSONL_malformedLineError(t *testing.T) {
	_, err := parseClaudeJSONL([]byte(`{not json`), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseClaudeJSONL_missingTypeError(t *testing.T) {
	_, err := parseClaudeJSONL([]byte(`{"foo":"bar"}`), 1)
	if err == nil {
		t.Fatal("expected error on missing type, got nil")
	}
}

func TestParse_routesByLineShape(t *testing.T) {
	// Two-line file: one fixture-schema line (kind=dispatch), one real
	// Claude-shape line (type=assistant with Agent tool_use). Parse
	// must accept both, producing two events with the correct kinds.
	fixture := `{"timestamp":"2026-05-15T18:10:11Z","kind":"dispatch","session_id":"S","sub_agent_id":"aa","bead_id":"kerf-zzzz","text":"hi"}`
	claude := `{"type":"assistant","timestamp":"2026-05-20T21:21:02.519Z","sessionId":"sess-1","uuid":"u1","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_X","name":"Agent","input":{"prompt":"impl kerf-abcd"}}]}}`
	doc := fixture + "\n" + claude + "\n"

	res, err := Parse(bytes.NewReader([]byte(doc)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected parse errors: %+v", res.Errors)
	}
	if len(res.Events) != 2 {
		t.Fatalf("got %d events, want 2", len(res.Events))
	}
	if res.Events[0].Kind != EventDispatch || res.Events[0].BeadID != "kerf-zzzz" {
		t.Errorf("fixture event mismatch: %+v", res.Events[0])
	}
	if res.Events[1].Kind != EventDispatch || res.Events[1].SubAgentID != "toolu_X" {
		t.Errorf("claude event mismatch: %+v", res.Events[1])
	}
}

func TestExtractBeadIDs_populatesFromText(t *testing.T) {
	pat := regexp.MustCompile(`kerf-[a-z0-9]+`)
	in := []Event{
		{Kind: EventDispatch, Text: "implementer for bead kerf-abcd"},
		{Kind: EventToolResult, Text: "Done with kerf-xyzy"},
		{Kind: EventDispatch, BeadID: "kerf-keep", Text: "kerf-other"}, // existing wins
		{Kind: EventCommitRef, Text: "kerf-ignored"},                    // wrong kind
	}
	got := ExtractBeadIDs(in, pat)
	if got[0].BeadID != "kerf-abcd" {
		t.Errorf("dispatch[0] BeadID = %q, want kerf-abcd", got[0].BeadID)
	}
	if got[1].BeadID != "kerf-xyzy" {
		t.Errorf("tool_result[1] BeadID = %q, want kerf-xyzy", got[1].BeadID)
	}
	if got[2].BeadID != "kerf-keep" {
		t.Errorf("dispatch[2] BeadID = %q, want kerf-keep (preserved)", got[2].BeadID)
	}
	if got[3].BeadID != "" {
		t.Errorf("commit_ref[3] BeadID = %q, want empty (wrong kind)", got[3].BeadID)
	}
	// Input must not have been mutated.
	if in[0].BeadID != "" {
		t.Errorf("input mutated: in[0].BeadID = %q", in[0].BeadID)
	}
}

func TestExtractBeadIDs_nilPatternNoop(t *testing.T) {
	in := []Event{{Kind: EventDispatch, Text: "kerf-abcd"}}
	got := ExtractBeadIDs(in, nil)
	if len(got) != 1 || got[0].BeadID != "" {
		t.Errorf("nil pattern should be no-op, got %+v", got)
	}
}
