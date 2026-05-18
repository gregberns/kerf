// Package beads provides integration with the beads task tracking system (br CLI).
//
// All interaction with the br binary is isolated in this package.
// No other part of kerf should shell out to br directly.
// When br is not available, functions degrade gracefully.
package beads

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ToolError is returned by ListNamed when the configured beads-CLI binary is
// present on PATH but invocation fails (non-zero exit, malformed JSON, etc.).
// It carries enough context (tool name, exit code, stderr snippet) for callers
// to render an actionable diagnostic — distinguishing "tool ran and failed"
// from "tool not installed" (which still degrades silently to nil, nil).
//
// The error string is prefixed with "BEADS_TOOL_ERROR:" so it can be grepped
// out of CLI output and surfaced in tests.
type ToolError struct {
	Tool    string // resolved binary name (e.g. "br", "bd")
	ExitErr error  // underlying exec error (typically *exec.ExitError)
	Stderr  string // captured stderr (already trimmed)
}

func (e *ToolError) Error() string {
	snippet := e.Stderr
	const max = 400
	if len(snippet) > max {
		snippet = snippet[:max] + "...(truncated)"
	}
	if snippet == "" {
		return fmt.Sprintf("BEADS_TOOL_ERROR: tool=%q failed: %v", e.Tool, e.ExitErr)
	}
	return fmt.Sprintf("BEADS_TOOL_ERROR: tool=%q failed: %v: %s", e.Tool, e.ExitErr, snippet)
}

func (e *ToolError) Unwrap() error { return e.ExitErr }

// Bead represents a single bead (task) from the br CLI.
type Bead struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Epic      string   `json:"epic"`
	Labels    []string `json:"labels"`
	DependsOn []string `json:"depends_on"`
}

// EpicSummary holds aggregated counts for beads within an epic.
type EpicSummary struct {
	Total      int
	Complete   int
	InProgress int
	Blocked    int
	Rework     int
}

// DefaultToolName is the default beads-CLI binary name when project.yaml
// does not override it via tools.tasks.
const DefaultToolName = "br"

// ResolveToolName returns the configured beads-CLI binary name, falling back
// to DefaultToolName ("br") when the tools.tasks key is unset or empty.
//
// The argument is a tools map (typically ProjectConfig.Tools) rather than the
// full config struct, to avoid an import cycle between internal/beads and
// internal/config (which already imports internal/beads).
func ResolveToolName(tools map[string]string) string {
	if tools == nil {
		return DefaultToolName
	}
	if name, ok := tools["tasks"]; ok && name != "" {
		return name
	}
	return DefaultToolName
}

// IsAvailable checks whether the br binary is on PATH.
func IsAvailable() bool {
	return IsAvailableNamed(DefaultToolName)
}

// IsAvailableNamed checks whether the given beads-CLI binary is on PATH.
func IsAvailableNamed(toolName string) bool {
	if toolName == "" {
		toolName = DefaultToolName
	}
	_, err := exec.LookPath(toolName)
	return err == nil
}

// List shells out to `br list --format json --all --limit 0` and parses the
// JSON output into a slice of Bead. Returns an empty slice (no error) if br
// is not available, keeping callers from needing to handle the missing-tool case.
func List() ([]Bead, error) {
	return ListNamed(DefaultToolName)
}

// ListNamed shells out to `<toolName> list --format json --all --limit 0`
// using the resolved beads-CLI binary name (per project.yaml tools.tasks).
// Argv shape matches the `br` CLI; do not pass a binary that does not accept
// these flags.
//
// Errors are distinguished by category:
//   - Tool not on PATH → (nil, nil). Callers degrade silently; this is the
//     "kerf works without a bead store" case.
//   - Tool ran but failed (non-zero exit, malformed JSON) → (nil, *ToolError).
//     Callers should surface this with the tool name and stderr snippet so
//     misconfigurations (wrong binary, broken store) don't fail silently.
func ListNamed(toolName string) ([]Bead, error) {
	if toolName == "" {
		toolName = DefaultToolName
	}
	if !IsAvailableNamed(toolName) {
		return nil, nil
	}

	cmd := exec.Command(toolName, "list", "--format", "json", "--all", "--limit", "0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, &ToolError{
			Tool:    toolName,
			ExitErr: err,
			Stderr:  strings.TrimSpace(stderr.String()),
		}
	}

	beads, perr := ParseJSON(stdout.Bytes())
	if perr != nil {
		return nil, &ToolError{
			Tool:    toolName,
			ExitErr: perr,
			Stderr:  strings.TrimSpace(stderr.String()),
		}
	}
	return beads, nil
}

// ParseJSON parses br's JSON output into a slice of Bead.
// Accepts either a bare array or the wrapped `{"issues":[...]}` envelope
// that current br versions emit.
func ParseJSON(data []byte) ([]Bead, error) {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var wrapped struct {
			Issues []Bead `json:"issues"`
		}
		if err := json.Unmarshal(data, &wrapped); err != nil {
			return nil, err
		}
		return wrapped.Issues, nil
	}
	var beads []Bead
	if err := json.Unmarshal(data, &beads); err != nil {
		return nil, err
	}
	return beads, nil
}

// CountByStatus groups beads by their Status field and returns a count map.
func CountByStatus(beads []Bead) map[string]int {
	counts := make(map[string]int)
	for _, b := range beads {
		counts[b.Status]++
	}
	return counts
}

// CountByEpic groups beads by their Epic field and returns an EpicSummary per epic.
func CountByEpic(beads []Bead) map[string]EpicSummary {
	epics := make(map[string]EpicSummary)
	for _, b := range beads {
		s := epics[b.Epic]
		s.Total++
		switch {
		case isComplete(b.Status):
			s.Complete++
		case isBlocked(b.Status):
			s.Blocked++
		case isInProgress(b.Status):
			s.InProgress++
		}
		epics[b.Epic] = s
	}
	return epics
}

// Available returns beads that are neither complete nor blocked.
// A bead is blocked if its status is "blocked".
// A bead is complete if its status is one of the recognized terminal statuses.
func Available(beads []Bead) []Bead {
	var result []Bead
	for _, b := range beads {
		if !isComplete(b.Status) && !isBlocked(b.Status) {
			result = append(result, b)
		}
	}
	return result
}

// IsRework reports whether the bead is tagged as rework. A bead is rework if
// any of its labels equals "rework:true" (case-insensitive) or begins with the
// "finding:" prefix (case-insensitive). The finding: prefix typically carries
// an attribution to the originating work (e.g. "finding:work-a").
func IsRework(b Bead) bool {
	for _, label := range b.Labels {
		if strings.EqualFold(label, "rework:true") {
			return true
		}
		if len(label) >= len("finding:") && strings.EqualFold(label[:len("finding:")], "finding:") {
			return true
		}
	}
	return false
}

// ReworkCount returns the number of beads in the slice that are tagged as rework.
func ReworkCount(beads []Bead) int {
	n := 0
	for _, b := range beads {
		if IsRework(b) {
			n++
		}
	}
	return n
}

// ForWork filters beads whose labels contain "work:<codename>".
//
// This is the back-compat entry point preserved from before Plan 006. It is
// intentionally case-insensitive to keep existing callers (cmd/next.go,
// cmd/show.go, cmd/square.go, cmd/map.go, internal/queue test helpers) and
// the historical TestForWork_CaseInsensitive contract behaving identically.
//
// Conceptually this is equivalent to applying Resolve(nil, nil) — the
// default filter "work:{codename}" — except that Filter.Match is
// case-sensitive per spec. Callers that need spec-conformant case-sensitive
// matching or a configured filter should use ForWorkWithFilter with an
// explicitly resolved *Filter.
//
// TODO(reviewer): confirm the case-sensitivity divergence between
// Filter.Match (case-sensitive per spec) and this wrapper (case-insensitive
// for back-compat) is acceptable until callers are migrated in later beads.
func ForWork(beads []Bead, workCodename string) []Bead {
	target := "work:" + workCodename
	var result []Bead
	for _, b := range beads {
		for _, label := range b.Labels {
			if strings.EqualFold(label, target) {
				result = append(result, b)
				break
			}
		}
	}
	return result
}

// --- status helpers ---

func isComplete(status string) bool {
	s := strings.ToLower(status)
	return s == "closed" || s == "done" || s == "complete"
}

// IsClosed reports whether the bead's status counts as a terminal/closed
// status (closed, done, complete). Mirrors the unexported isComplete helper
// used by Available / CountByEpic — exported so command-layer callers can
// split open vs. closed for reporting.
func IsClosed(b Bead) bool {
	return isComplete(b.Status)
}

func isBlocked(status string) bool {
	return strings.ToLower(status) == "blocked"
}

func isInProgress(status string) bool {
	s := strings.ToLower(status)
	return s == "in-progress" || s == "in_progress" || s == "active" || s == "wip"
}
