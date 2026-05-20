package kerftranscript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// rawLine is the on-disk wire shape. Decoded via encoding/json then
// validated and normalised into an Event. Field tags match the fixture
// schema documented in internal/kerftranscript/testdata/README.md, which
// in turn projects the Claude Code session JSONL down to the columns the
// detectors consume.
type rawLine struct {
	Timestamp  string `json:"timestamp"`
	Kind       string `json:"kind"`
	SessionID  string `json:"session_id"`
	SubAgentID string `json:"sub_agent_id"`
	BeadID     string `json:"bead_id"`
	Role       string `json:"role"`
	CommitSHA  string `json:"commit_sha"`
	IsError    *bool  `json:"is_error,omitempty"`
	Text       string `json:"text"`
}

// Result is the parser's return shape: every successfully parsed event in
// source order, plus the malformed lines that were skipped. Result.Events
// is always non-nil (possibly empty); Result.Errors is non-nil only when
// at least one line failed validation.
//
// Policy: malformed lines are skipped and reported, the parser continues.
// Transcripts in the wild contain partial writes and unknown future
// kinds; failing the whole parse on a single bad line would make the
// diagnostics family fragile against real corpora. specs/diagnostics.md
// is ambiguous on this point as of B2; defaulting to skip-and-continue
// per the bead brief.
type Result struct {
	Events []Event
	Errors []ParseError
}

// Parse reads a Claude Code session JSONL stream from r and returns the
// typed event vocabulary consumed by the diagnostics detectors. Malformed
// lines are accumulated in Result.Errors and the parser continues; the
// returned error is non-nil only for an underlying io failure on r.
//
// Empty lines and whitespace-only lines are silently ignored. JSONL files
// produced by Claude Code occasionally contain a trailing newline; that
// is not a parse error.
func Parse(r io.Reader) (Result, error) {
	res := Result{Events: make([]Event, 0, 64)}

	scanner := bufio.NewScanner(r)
	// Claude transcript lines routinely exceed the default 64KB scan
	// buffer (assistant text + tool_use payloads). Bump to 4MB; matches
	// the soft cap used by the Python extractor archived in plan 012.
	const maxLine = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxLine)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Bytes()
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}

		ev, perr := decodeLine(raw, lineNo)
		if perr != nil {
			res.Errors = append(res.Errors, *perr)
			continue
		}
		res.Events = append(res.Events, ev)
	}
	if err := scanner.Err(); err != nil {
		return res, fmt.Errorf("kerftranscript: read transcript: %w", err)
	}
	return res, nil
}

// ParseFile is a thin convenience wrapper around Parse that opens path.
// Per specs/diagnostics.md §"Discovery failure is silent", callers that
// want "file not found = no findings" should check os.IsNotExist on the
// returned error; ParseFile itself does not absorb missing files because
// that policy belongs in the discovery layer, not the parser.
func ParseFile(path string) (Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return Result{Events: []Event{}}, fmt.Errorf("kerftranscript: open %s: %w", path, err)
	}
	defer f.Close()
	return Parse(f)
}

// FilterByBead returns the subset of events whose BeadID equals beadID.
// The result preserves source order. Used by detectors that need the
// per-bead query path required by specs/diagnostics.md §"Multi-bead
// transcript fixtures".
func FilterByBead(events []Event, beadID string) []Event {
	out := make([]Event, 0, len(events))
	for _, ev := range events {
		if ev.BeadID == beadID {
			out = append(out, ev)
		}
	}
	return out
}

// BeadIDs returns the set of distinct bead IDs referenced by events, in
// first-seen order. Empty bead IDs are excluded. Used by the all-beads
// query path required by specs/diagnostics.md §"Multi-bead transcript
// fixtures" when the detector is invoked without `--bead`.
func BeadIDs(events []Event) []string {
	seen := make(map[string]struct{}, len(events))
	out := make([]string, 0)
	for _, ev := range events {
		if ev.BeadID == "" {
			continue
		}
		if _, ok := seen[ev.BeadID]; ok {
			continue
		}
		seen[ev.BeadID] = struct{}{}
		out = append(out, ev.BeadID)
	}
	return out
}

// decodeLine validates one JSONL line. Returns either an Event or a
// ParseError, never both.
func decodeLine(raw []byte, lineNo int) (Event, *ParseError) {
	var rl rawLine
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&rl); err != nil {
		return Event{}, &ParseError{LineNumber: lineNo, Raw: string(raw), Err: fmt.Errorf("invalid json: %w", err)}
	}

	if rl.Kind == "" {
		return Event{}, &ParseError{LineNumber: lineNo, Raw: string(raw), Err: errors.New("missing required field: kind")}
	}
	if !IsValidKind(rl.Kind) {
		return Event{}, &ParseError{LineNumber: lineNo, Raw: string(raw), Err: fmt.Errorf("unknown kind: %q", rl.Kind)}
	}

	ev := Event{
		Kind:       EventKind(rl.Kind),
		SessionID:  rl.SessionID,
		SubAgentID: rl.SubAgentID,
		BeadID:     rl.BeadID,
		Role:       rl.Role,
		CommitSHA:  rl.CommitSHA,
		Text:       rl.Text,
		LineNumber: lineNo,
	}
	if rl.IsError != nil {
		ev.IsError = *rl.IsError
	}

	if rl.Timestamp != "" {
		ts, err := time.Parse(time.RFC3339, rl.Timestamp)
		if err != nil {
			return Event{}, &ParseError{LineNumber: lineNo, Raw: string(raw), Err: fmt.Errorf("invalid timestamp %q: %w", rl.Timestamp, err)}
		}
		ev.Timestamp = ts.UTC()
	}

	return ev, nil
}
