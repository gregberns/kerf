// Package kerftranscript reads Claude Code session JSONL transcript files
// and exposes a typed event stream consumed by the diagnostics family
// (D1 abandoned_dispatch, D6 reviewer_absent). See specs/diagnostics.md
// §"Diagnostic input vocabulary" for the normative contract this package
// implements.
package kerftranscript

import "time"

// EventKind enumerates the post-parse event vocabulary defined by
// specs/diagnostics.md. The on-disk JSONL `kind` field is mapped 1:1 onto
// these constants; any other value is treated as a parse error.
type EventKind string

const (
	// EventDispatch — orchestrator launched a sub-agent against a bead.
	// Marks the start of a dispatch interval.
	EventDispatch EventKind = "dispatch"

	// EventToolResult — a tool invocation returned to a sub-agent.
	// Carries IsError when applicable.
	EventToolResult EventKind = "tool_result"

	// EventCommitRef — a commit landed referencing one or more bead IDs.
	EventCommitRef EventKind = "commit_ref"

	// EventBeadClose — a bead was closed (possibly as SUBSUMED).
	EventBeadClose EventKind = "bead_close"
)

// IsValidKind reports whether s is one of the four event kinds the parser
// emits. The set is closed; new kinds require a spec update.
func IsValidKind(s string) bool {
	switch EventKind(s) {
	case EventDispatch, EventToolResult, EventCommitRef, EventBeadClose:
		return true
	}
	return false
}

// Event is the post-parse, in-memory representation of a single transcript
// line. The field set is the union of all kinds' carried fields per
// specs/diagnostics.md §"Diagnostic input vocabulary"; fields not relevant
// to a given Kind are zero-valued.
//
// Timestamps are normalised to UTC.
type Event struct {
	// Timestamp is the RFC3339 UTC time of the event.
	Timestamp time.Time

	// Kind is one of EventDispatch, EventToolResult, EventCommitRef,
	// EventBeadClose. Lines whose `kind` field falls outside this set
	// are returned as ParseError, not Event.
	Kind EventKind

	// SessionID is the Claude session UUID.
	SessionID string

	// SubAgentID is present on dispatch and tool_result events.
	SubAgentID string

	// BeadID is the bead the event is about, when known.
	BeadID string

	// Role is the optional sub-agent role tag (e.g. "implementer",
	// "reviewer"). Present on dispatch events when set.
	Role string

	// CommitSHA is present on commit_ref and bead_close.
	CommitSHA string

	// IsError is present on tool_result for error-tagged results.
	IsError bool

	// Text is the optional free-form payload (e.g. commit message body,
	// dispatch description). Used by D6's reviewer-dispatch marker
	// detection and by D1's reason-category classifier.
	Text string

	// LineNumber is the 1-based source line in the JSONL stream. Useful
	// for cross-referencing diagnostics back to the transcript.
	LineNumber int
}

// ParseError captures a single malformed JSONL line. The parser skips
// malformed lines and continues; callers receive the accumulated errors
// alongside the successfully-parsed events. See parser.go for the policy
// rationale.
type ParseError struct {
	LineNumber int
	Raw        string
	Err        error
}

// Error implements the error interface.
func (p *ParseError) Error() string {
	return p.Err.Error()
}

// Unwrap returns the underlying error so errors.Is / errors.As work.
func (p *ParseError) Unwrap() error { return p.Err }
