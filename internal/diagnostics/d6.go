package diagnostics

// D6 — reviewer-absent commit detector.
//
// Spec references:
//   - specs/diagnostics.md §"D6 — reviewer-absent commit"
//   - specs/diagnostics.md §"Reviewer dispatch (normative definition)"
//   - specs/commands.md §"Warning kinds" → `reviewer_absent`
//
// Co-located in package diagnostics alongside d1.go to share the
// kerftranscript event vocabulary. Symbols are deliberately D6-scoped to
// avoid collision with the D1 surface (D1DispatchFloor, DetectD1,
// AbandonedDispatch, WarningKindAbandonedDispatch, etc.).

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gberns/kerf/internal/kerftranscript"
)

// WarningKindReviewerAbsent is the `kerf next` warning kind token emitted
// for D6 findings. Mirrors the spec-defined string in specs/commands.md
// §"Warning kinds" → `reviewer_absent`.
const WarningKindReviewerAbsent = "reviewer_absent"

// D6WindowBeads is the bead-count window over which D6 reports findings.
// Calibrated against plans/012_real_corpus/data/ — see
// specs/diagnostics.md §"Threshold and window" and
// plans/013_self_diagnostics/calibration.md §"D6 — reviewer-absent
// commit". The unit is beads, not days.
const D6WindowBeads = 30

// D6MinHistoryBeads is the minimum number of dispatched beads in the
// project's transcript history before D6 begins emitting findings.
// Below this threshold the detector suppresses with the rationale
// "insufficient history" (specs/diagnostics.md §"Minimum history
// guard"). Numerically equal to D6WindowBeads by spec calibration; kept
// as a separate constant so the two roles read clearly at the call site.
const D6MinHistoryBeads = 30

// reviewerHeaderRe matches the canonical text-format header emitted by
// `kerf review` on stdout. Per specs/diagnostics.md §"Reviewer dispatch
// (normative definition)":
//
//	Reviewer prompt for <codename> — pass: <pass-name>
//
// The em-dash is the U+2014 character `kerf review` actually emits.
var reviewerHeaderRe = regexp.MustCompile(`Reviewer prompt for \S+ \x{2014} pass: \S+`)

// ReviewerAbsent is a single D6 finding. Field set follows the finding
// `detail` schema in specs/diagnostics.md §"D6 — reviewer-absent
// commit" verbatim (with the `kind` discriminator implicit — the type
// itself is the kind).
type ReviewerAbsent struct {
	BeadID                string
	SessionID             string
	CommitSHA             string
	CommittedAt           time.Time
	ImplementerSubAgentID string
}

// DetectD6Options narrows the detector's scope. BeadID, when non-empty,
// scopes the detector to a single bead per specs/diagnostics.md
// §"Multi-bead transcript fixtures (normative)": only findings for that
// bead are returned. An empty BeadID means "all beads" — every bead
// with a commit_ref event whose session is missing a reviewer dispatch
// produces one finding.
type DetectD6Options struct {
	BeadID string
}

// DetectD6 runs the D6 reviewer-absent detector over the supplied
// transcript events. It returns one finding per (bead_id, commit_sha)
// pair whose Claude session contains no reviewer dispatch per the
// normative reviewer-dispatch definition.
//
// Findings are returned in deterministic order: by CommittedAt
// ascending, then by BeadID ascending, then by CommitSHA ascending. The
// slice is non-nil but may be empty.
//
// The 30-bead window and the minimum-history guard are intentionally
// NOT applied here: they are wiring-layer concerns (cmd/next.go) that
// depend on the dispatch-history universe across the entire transcript
// corpus. Callers that want the guarded view should consult
// DispatchedBeadCount and take the last D6WindowBeads of the result.
func DetectD6(events []kerftranscript.Event, opts DetectD6Options) []ReviewerAbsent {
	// Build the set of sessions that contain a reviewer dispatch, plus
	// a per-session bead → implementer sub-agent map (for the finding
	// detail schema's `implementer_sub_agent_id` slot).
	reviewerSessions := make(map[string]bool)
	implementerBySession := make(map[string]map[string]string) // session -> bead -> sub_agent
	for _, ev := range events {
		if ev.Kind != kerftranscript.EventDispatch {
			continue
		}
		if isReviewerDispatch(ev) {
			if ev.SessionID != "" {
				reviewerSessions[ev.SessionID] = true
			}
			continue
		}
		if ev.SessionID == "" || ev.BeadID == "" || ev.SubAgentID == "" {
			continue
		}
		if implementerBySession[ev.SessionID] == nil {
			implementerBySession[ev.SessionID] = make(map[string]string)
		}
		// First implementer dispatch for the bead in the session wins.
		if _, seen := implementerBySession[ev.SessionID][ev.BeadID]; !seen {
			implementerBySession[ev.SessionID][ev.BeadID] = ev.SubAgentID
		}
	}

	// One finding per commit_ref whose session has no reviewer dispatch.
	type key struct{ bead, sha string }
	seen := make(map[key]bool)
	out := make([]ReviewerAbsent, 0)
	for _, ev := range events {
		if ev.Kind != kerftranscript.EventCommitRef {
			continue
		}
		if ev.BeadID == "" || ev.CommitSHA == "" {
			continue
		}
		if opts.BeadID != "" && ev.BeadID != opts.BeadID {
			continue
		}
		if reviewerSessions[ev.SessionID] {
			continue
		}
		k := key{ev.BeadID, ev.CommitSHA}
		if seen[k] {
			continue
		}
		seen[k] = true
		impl := ""
		if m, ok := implementerBySession[ev.SessionID]; ok {
			impl = m[ev.BeadID]
		}
		out = append(out, ReviewerAbsent{
			BeadID:                ev.BeadID,
			SessionID:             ev.SessionID,
			CommitSHA:             ev.CommitSHA,
			CommittedAt:           ev.Timestamp,
			ImplementerSubAgentID: impl,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if !out[i].CommittedAt.Equal(out[j].CommittedAt) {
			return out[i].CommittedAt.Before(out[j].CommittedAt)
		}
		if out[i].BeadID != out[j].BeadID {
			return out[i].BeadID < out[j].BeadID
		}
		return out[i].CommitSHA < out[j].CommitSHA
	})
	return out
}

// isReviewerDispatch reports whether ev is a sub-agent dispatch event
// whose payload text carries one of the two canonical `kerf review`
// output markers, per specs/diagnostics.md §"Reviewer dispatch
// (normative definition)":
//
//  1. The text-format header `Reviewer prompt for <codename> — pass: <pass-name>`.
//  2. A JSON object containing the `kerf review --format=json` keys
//     `{ "codename", "pass", "artifacts", "criteria" }`.
//
// A dispatch with neither marker present is NOT a reviewer dispatch
// even if its description contains the word "review" — kerf workflows
// commonly use "review" informally for non-reviewer work (calibration
// measured ~28 kerf false positives against a looser substring rule).
func isReviewerDispatch(ev kerftranscript.Event) bool {
	if ev.Kind != kerftranscript.EventDispatch {
		return false
	}
	if ev.Text == "" {
		return false
	}
	// Marker 1: text-format header line.
	if reviewerHeaderRe.MatchString(ev.Text) {
		return true
	}
	// Marker 2: JSON shape with the four canonical keys.
	if hasReviewerJSONShape(ev.Text) {
		return true
	}
	return false
}

// hasReviewerJSONShape scans s for a JSON object containing all four
// `kerf review --format=json` top-level keys ("codename", "pass",
// "artifacts", "criteria"). The scanner attempts a JSON decode at every
// '{' offset so dispatch payloads that wrap JSON in surrounding prose
// are still detected.
func hasReviewerJSONShape(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		var obj map[string]json.RawMessage
		dec := json.NewDecoder(strings.NewReader(s[i:]))
		if err := dec.Decode(&obj); err != nil {
			continue
		}
		_, hasCodename := obj["codename"]
		_, hasPass := obj["pass"]
		_, hasArtifacts := obj["artifacts"]
		_, hasCriteria := obj["criteria"]
		if hasCodename && hasPass && hasArtifacts && hasCriteria {
			return true
		}
	}
	return false
}

// DispatchedBeadCount returns the number of distinct bead IDs that
// appear on any dispatch event across the supplied transcript event
// stream. This is the universe size the wiring layer compares against
// D6MinHistoryBeads to decide whether D6 emits anything at all (spec:
// "D6 emits no findings until the project has at least 30 beads in its
// dispatch history").
func DispatchedBeadCount(events []kerftranscript.Event) int {
	seen := make(map[string]bool)
	for _, ev := range events {
		if ev.Kind != kerftranscript.EventDispatch {
			continue
		}
		if ev.BeadID == "" {
			continue
		}
		seen[ev.BeadID] = true
	}
	return len(seen)
}
