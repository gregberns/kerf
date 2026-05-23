package kerftranscript

// Real-Claude-Code JSONL → Event vocabulary adapter.
//
// Bead kerf-ek21: the original parser (parser.go) reads the fixture-only
// schema documented in testdata/README.md (field `kind` ∈ {dispatch,
// tool_result, commit_ref, bead_close}). Production Claude Code
// transcripts use field `type` ∈ {assistant, user, system, attachment,
// permission-mode, last-prompt, ai-title, queue-operation,
// file-history-snapshot, ...}, and structure tool calls / results as
// content blocks nested inside `message.content[]`. Without this adapter
// every real transcript line is rejected by the fixture parser, 100% of
// production transcripts produce zero events, and D1/D6 never fire.
//
// Mapping (intentionally minimal — see bead brief "Pick the smallest
// viable mapping"):
//
//   - assistant message → for each `tool_use` block whose `name` is
//     "Agent" or "Task", emit one EventDispatch. The dispatch's
//     SubAgentID is the tool_use `id` (toolu_*); Text is the `prompt`
//     input verbatim so D6's reviewer-dispatch marker scan and D1's
//     bead-ID regex both have material to chew on.
//
//   - user message → for each `tool_result` content block, emit one
//     EventToolResult. SubAgentID is the `tool_use_id` so D1 can
//     correlate results back to the originating dispatch. IsError is
//     copied verbatim. Text is the concatenated text blocks (handles
//     both the string-content shape and the array-of-blocks shape).
//
//   - everything else (system, attachment, permission-mode,
//     last-prompt, ai-title, file-history-snapshot, assistant text,
//     thinking, non-Agent tool_use, etc.) → zero events. Not an error;
//     just irrelevant to the v1 detector vocabulary.
//
//   - commit_ref / bead_close events have NO source in real
//     transcripts. Per kerf-0fxv, commit_ref is authoritatively
//     `git log --all` (see internal/kerftranscript/index.go); bead_close
//     is out of scope for v1 transcript-based detection. The adapter
//     therefore emits zero of those, and that is correct.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// claudeLine is the on-disk shape of a real Claude Code JSONL line. Only
// the fields the adapter consumes are decoded; everything else (model,
// usage, requestId, parentUuid, cwd, gitBranch, …) is ignored.
type claudeLine struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"sessionId"`
	UUID      string          `json:"uuid"`
	Message   json.RawMessage `json:"message"`
}

// claudeMessage is the inner `message` shape on assistant/user lines.
// `content` is either a JSON string (a user-typed prompt) or an array of
// content blocks (assistant tool_use / user tool_result). We decode it as
// RawMessage and branch on the first non-whitespace byte.
type claudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// claudeContentBlock is the union of the content-block shapes the adapter
// inspects. Unused variants (text, thinking, image, …) decode silently
// into the same struct with the irrelevant fields left zero.
type claudeContentBlock struct {
	Type       string          `json:"type"`
	Name       string          `json:"name,omitempty"`         // tool_use
	ID         string          `json:"id,omitempty"`           // tool_use
	Input      json.RawMessage `json:"input,omitempty"`        // tool_use
	ToolUseID  string          `json:"tool_use_id,omitempty"`  // tool_result
	IsError    *bool           `json:"is_error,omitempty"`     // tool_result
	Content    json.RawMessage `json:"content,omitempty"`      // tool_result (string OR array)
}

// agentToolInput is the input shape for the Agent / Task tool. We only
// pull the prompt; description and subagent_type are not load-bearing
// for the v1 detectors but the prompt drives bead-ID extraction and
// reviewer-marker detection.
type agentToolInput struct {
	Prompt string `json:"prompt"`
}

// parseClaudeJSONL maps a single real-Claude-Code transcript line to
// zero or more Events. Returns (nil, nil) for lines that legitimately
// produce no events (system messages, assistant text-only, etc.) — that
// is the success path, not an error.
//
// The caller (parser.go decodeLine) supplies lineNo so emitted Events
// carry source line numbers consistent with the fixture path.
func parseClaudeJSONL(raw []byte, lineNo int) ([]Event, error) {
	var cl claudeLine
	if err := json.Unmarshal(raw, &cl); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	if cl.Type == "" {
		return nil, fmt.Errorf("missing required field: type")
	}

	switch cl.Type {
	case "assistant":
		return parseAssistantLine(cl, lineNo)
	case "user":
		return parseUserLine(cl, lineNo)
	default:
		// system, attachment, permission-mode, last-prompt, ai-title,
		// file-history-snapshot, queue-operation, … — known-irrelevant.
		// Unknown future types are also treated as irrelevant rather
		// than errors; if a new type carries diagnostic signal a future
		// bead will wire it explicitly.
		return nil, nil
	}
}

// parseAssistantLine emits one EventDispatch per Agent/Task tool_use
// content block. Other content blocks (text, thinking, non-Agent
// tool_use, e.g. Bash/Read/Edit) produce no events.
func parseAssistantLine(cl claudeLine, lineNo int) ([]Event, error) {
	ts, err := parseClaudeTimestamp(cl.Timestamp)
	if err != nil {
		return nil, err
	}
	blocks, err := decodeContentBlocks(cl.Message)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, 1)
	for _, b := range blocks {
		if b.Type != "tool_use" {
			continue
		}
		if !isSubAgentTool(b.Name) {
			continue
		}
		prompt := extractAgentPrompt(b.Input)
		out = append(out, Event{
			Timestamp:  ts,
			Kind:       EventDispatch,
			SessionID:  cl.SessionID,
			SubAgentID: b.ID,
			Text:       prompt,
			LineNumber: lineNo,
		})
	}
	return out, nil
}

// parseUserLine emits one EventToolResult per tool_result content block.
// User lines whose content is a plain string (the human typed something)
// produce no events.
func parseUserLine(cl claudeLine, lineNo int) ([]Event, error) {
	ts, err := parseClaudeTimestamp(cl.Timestamp)
	if err != nil {
		return nil, err
	}
	blocks, err := decodeContentBlocks(cl.Message)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, 1)
	for _, b := range blocks {
		if b.Type != "tool_result" {
			continue
		}
		isErr := false
		if b.IsError != nil {
			isErr = *b.IsError
		}
		out = append(out, Event{
			Timestamp:  ts,
			Kind:       EventToolResult,
			SessionID:  cl.SessionID,
			SubAgentID: b.ToolUseID,
			IsError:    isErr,
			Text:       flattenToolResultContent(b.Content),
			LineNumber: lineNo,
		})
	}
	return out, nil
}

// isSubAgentTool reports whether a tool_use block's `name` denotes a
// sub-agent dispatch. Real transcripts use either "Agent" (current
// Claude Code) or "Task" (older builds). Both map to dispatch.
func isSubAgentTool(name string) bool {
	return name == "Agent" || name == "Task"
}

// decodeContentBlocks decodes the inner `message.content` of an
// assistant or user line. When `content` is a JSON string (user-typed
// prompt), it returns nil with no error.
func decodeContentBlocks(msg json.RawMessage) ([]claudeContentBlock, error) {
	if len(msg) == 0 {
		return nil, nil
	}
	var m claudeMessage
	if err := json.Unmarshal(msg, &m); err != nil {
		return nil, fmt.Errorf("invalid message: %w", err)
	}
	trimmed := bytes.TrimSpace(m.Content)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '"' {
		// Plain string content — no blocks, no events.
		return nil, nil
	}
	if trimmed[0] != '[' {
		return nil, nil
	}
	var blocks []claudeContentBlock
	if err := json.Unmarshal(trimmed, &blocks); err != nil {
		return nil, fmt.Errorf("invalid content blocks: %w", err)
	}
	return blocks, nil
}

// extractAgentPrompt pulls the `prompt` field out of an Agent/Task
// tool_use input. Returns "" on any decode failure — the resulting
// dispatch event still carries SubAgentID/Timestamp/SessionID, just
// without bead-ID-bearing text.
func extractAgentPrompt(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var in agentToolInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return in.Prompt
}

// flattenToolResultContent normalises the tool_result `content` field
// (which may be a plain string OR an array of {type:"text", text:"…"}
// blocks) into a single string. Non-text blocks are skipped.
func flattenToolResultContent(content json.RawMessage) string {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return ""
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err == nil {
			return s
		}
		return ""
	}
	if trimmed[0] != '[' {
		return ""
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(trimmed, &parts); err != nil {
		return ""
	}
	var buf bytes.Buffer
	for i, p := range parts {
		if p.Type != "text" {
			continue
		}
		if i > 0 && buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(p.Text)
	}
	return buf.String()
}

// parseClaudeTimestamp accepts both RFC3339 and RFC3339Nano (real Claude
// transcripts use the ms-precision Nano form, e.g.
// "2026-05-20T21:21:02.519Z"). The returned time is normalised to UTC.
func parseClaudeTimestamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", s)
}

// shapeProbe is the minimal struct used to decide which parser path to
// route a JSONL line through. Both schemas are JSON objects; the only
// difference at the top level is which discriminator field is present.
type shapeProbe struct {
	Kind string `json:"kind"`
	Type string `json:"type"`
}

// hasClaudeShape reports whether raw is a real-Claude-Code line (top-
// level `type` field, no top-level `kind` field). A naive substring
// scan over the full line is insufficient because tool_result content
// can quote prose like `"kind"` from kerf's own source files; the
// fixture-vs-Claude routing must look at the top-level JSON keys, not
// any nested text.
func hasClaudeShape(raw []byte) bool {
	var p shapeProbe
	if err := json.Unmarshal(raw, &p); err != nil {
		return false
	}
	if p.Kind != "" {
		return false
	}
	return p.Type != ""
}

// ExtractBeadIDs walks events and populates Event.BeadID on dispatch
// and tool_result events whose BeadID is empty, by applying pattern to
// Event.Text and taking the first match. Returns a NEW slice; the input
// is not mutated.
//
// The Claude adapter does not know the project's bead.id_pattern (the
// parser deliberately stays config-free so the cache is pattern-agnostic).
// Callers in cmd/next.go invoke this helper after LoadOrParse with the
// pattern compiled from project.yaml. When pattern is nil this is a
// no-op pass-through.
func ExtractBeadIDs(events []Event, pattern *regexp.Regexp) []Event {
	if pattern == nil {
		return events
	}
	out := make([]Event, len(events))
	copy(out, events)
	for i := range out {
		if out[i].BeadID != "" {
			continue
		}
		if out[i].Kind != EventDispatch && out[i].Kind != EventToolResult {
			continue
		}
		if out[i].Text == "" {
			continue
		}
		if m := pattern.FindString(out[i].Text); m != "" {
			out[i].BeadID = m
		}
	}
	return out
}
