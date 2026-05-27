// Package diagnostics implements the v1 transcript-based detectors
// (D1 abandoned_dispatch, D6 reviewer_absent — D6 wave pending). The
// package consumes the typed event vocabulary published by
// internal/kerftranscript and the bead-ID indexer published by the same
// package. Detectors produce findings that the `kerf next` warning
// channel renders.
//
// Spec references:
//   - specs/diagnostics.md §"D1 — abandoned dispatch"
//   - specs/commands.md §"Warning kinds" → `abandoned_dispatch`
package diagnostics

import (
	"sort"
	"time"

	"github.com/gregberns/kerf/internal/kerftranscript"
)

// WarningKindAbandonedDispatch is the `kerf next` warning kind token
// emitted for D1 findings. Mirrors the spec-defined string in
// specs/commands.md §"Warning kinds" → `abandoned_dispatch`. Kept in
// the diagnostics package so the renderer in cmd/next.go has a single
// source of truth that is not coupled to internal/feed's existing
// warning-kind constants.
const WarningKindAbandonedDispatch = "abandoned_dispatch"

// D1DispatchFloor is the dispatch-duration floor below which a
// sub-agent dispatch is suppressed from D1. Calibrated against
// plans/012_real_corpus/data/ — see specs/diagnostics.md §"Threshold".
// 60s preserves >96% of successful dispatches in both kerf and
// harmonik while suppressing the noise band below it.
const D1DispatchFloor = 60 * time.Second

// ReasonCategory enumerates the four D1 reason categories defined in
// specs/diagnostics.md §"Reason categories (programmatic)".
type ReasonCategory string

const (
	ReasonAppearsCompletedNoCommit ReasonCategory = "appears_completed_no_commit"
	ReasonErroredMidTask           ReasonCategory = "errored_mid_task"
	ReasonOrphaned                 ReasonCategory = "orphaned"
	ReasonToolLinkageBroken        ReasonCategory = "tool_linkage_broken"
)

// AbandonedDispatch is a single D1 finding. Field set follows the
// finding `detail` schema in specs/diagnostics.md §"D1 — abandoned
// dispatch" verbatim (with the `kind` discriminator implicit — the
// type itself is the kind).
type AbandonedDispatch struct {
	BeadID         string
	SessionID      string
	SubAgentID     string
	DispatchedAt   time.Time
	LastActivityAt time.Time
	ReasonCategory ReasonCategory
	// CloseCommit is the SHA recorded by a transcript bead_close event
	// that retired the bead without implementing it (e.g. a SUBSUMED
	// close). Empty when no such event was observed.
	CloseCommit string
}

// Duration returns the wall-clock duration from DispatchedAt to
// LastActivityAt. Used by the renderer for the `{duration}s` slot in
// the warning's reason string.
func (a AbandonedDispatch) Duration() time.Duration {
	return a.LastActivityAt.Sub(a.DispatchedAt)
}

// HasCommitForFn is the predicate consulted to decide whether a given
// bead ID has at least one git-log commit referencing it (after the
// indexer's parent/child rollup and worktree-branch refs). Production
// callers pass (*kerftranscript.Index).HasCommitFor; test callers pass
// a fake. A nil predicate is treated as "no commits ever" — useful for
// pure-parser smoke tests but not the production path.
type HasCommitForFn func(beadID string) bool

// invocationNow is the test seam for the "now" timestamp used by the
// `orphaned` reason category. Production callers leave it nil; tests
// override.
type invocationNow func() time.Time

// DetectD1 runs the D1 abandoned-dispatch detector over the supplied
// transcript events with the supplied has-commit predicate. The events
// MUST come from a single Parse call (or the concatenation of several);
// they need not be sorted — the detector groups by sub_agent_id and
// reads each group in event order.
//
// Findings are returned in deterministic order: by DispatchedAt
// ascending, then by BeadID ascending. The slice is non-nil but may be
// empty.
//
// Burst-dedup (capture-only per spec): calibration observed that ~53%
// of abandoned dispatches arrive within 60s of a sibling in the same
// session. Plan 013 explicitly defers any dedup to v2 — this detector
// does NOT collapse sibling bursts. A future `--collapse-bursts`
// renderer flag (or a v2 detector) may fold them; see
// specs/diagnostics.md §"Burst-dedup note (capture only)".
func DetectD1(events []kerftranscript.Event, hasCommit HasCommitForFn) []AbandonedDispatch {
	return detectD1With(events, hasCommit, time.Now)
}

// detectD1With is the test-seam form. Production callers use DetectD1.
func detectD1With(events []kerftranscript.Event, hasCommit HasCommitForFn, now invocationNow) []AbandonedDispatch {
	// Group events by sub_agent_id. A dispatch event opens a group;
	// subsequent tool_result events on the same sub_agent_id belong
	// to it; bead_close events on the same bead are recorded so the
	// CloseCommit slot can be populated.
	type group struct {
		dispatch    kerftranscript.Event
		toolResults []kerftranscript.Event
		closeCommit string
	}
	groups := make(map[string]*group)
	// closesByBead lets us pick up a bead_close even when it lacks a
	// sub_agent_id field (most fixture closes do not carry one).
	closesByBead := make(map[string]string)

	for _, ev := range events {
		switch ev.Kind {
		case kerftranscript.EventDispatch:
			if ev.SubAgentID == "" {
				continue
			}
			// Last dispatch wins for a given sub_agent_id — re-use is
			// rare but the latest open is the one we score.
			groups[ev.SubAgentID] = &group{dispatch: ev}
		case kerftranscript.EventToolResult:
			if ev.SubAgentID == "" {
				continue
			}
			g, ok := groups[ev.SubAgentID]
			if !ok {
				continue
			}
			g.toolResults = append(g.toolResults, ev)
		case kerftranscript.EventBeadClose:
			if ev.BeadID != "" && ev.CommitSHA != "" {
				closesByBead[ev.BeadID] = ev.CommitSHA
			}
		}
	}

	out := make([]AbandonedDispatch, 0)
	for _, g := range groups {
		d := g.dispatch
		// Determine LastActivityAt: timestamp of last tool_result for
		// this sub-agent, falling back to the dispatch timestamp.
		last := d.Timestamp
		for _, tr := range g.toolResults {
			if tr.Timestamp.After(last) {
				last = tr.Timestamp
			}
		}
		duration := last.Sub(d.Timestamp)
		if duration < D1DispatchFloor {
			continue
		}
		if d.BeadID == "" {
			continue
		}
		if hasCommit != nil && hasCommit(d.BeadID) {
			// The indexer found a real code commit referencing this
			// bead (with parent/child rollup) — not abandoned.
			continue
		}
		out = append(out, AbandonedDispatch{
			BeadID:         d.BeadID,
			SessionID:      d.SessionID,
			SubAgentID:     d.SubAgentID,
			DispatchedAt:   d.Timestamp,
			LastActivityAt: last,
			ReasonCategory: classifyReason(g.toolResults, last, now, closesByBead[d.BeadID] != ""),
			CloseCommit:    closesByBead[d.BeadID],
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if !out[i].DispatchedAt.Equal(out[j].DispatchedAt) {
			return out[i].DispatchedAt.Before(out[j].DispatchedAt)
		}
		return out[i].BeadID < out[j].BeadID
	})
	return out
}

// classifyReason picks the D1 reason category from the dispatch tail.
//
// Per specs/diagnostics.md §"Reason categories":
//   - errored_mid_task: final event is a tool_result with is_error=true.
//   - orphaned:         last event timestamp >24h before invocation and
//                       no continuation event.
//   - tool_linkage_broken: sub-agent finished but no parent-side result
//                          event was recorded. In the v1 event
//                          vocabulary the parser exposes, we can detect
//                          this as "no tool_results for this dispatch
//                          at all" (the sub-agent dispatched but never
//                          returned a tool result).
//   - appears_completed_no_commit: default; final sub-agent event is
//                                  assistant text with no tool calls
//                                  in the last 5 events.
//
// Ordering matters: errored beats orphaned beats tool_linkage_broken
// beats the default appears_completed_no_commit.
func classifyReason(toolResults []kerftranscript.Event, lastActivity time.Time, now invocationNow, hasContinuation bool) ReasonCategory {
	// errored_mid_task: final tool_result has is_error=true.
	if n := len(toolResults); n > 0 && toolResults[n-1].IsError {
		return ReasonErroredMidTask
	}
	// orphaned: last activity >24h before invocation AND no
	// continuation event. A bead_close in the same session counts as
	// a continuation — the sub-agent's owning bead reached a terminal
	// transcript event, it was not silently dropped.
	if now != nil && !hasContinuation {
		if now().Sub(lastActivity) > 24*time.Hour {
			return ReasonOrphaned
		}
	}
	// tool_linkage_broken: dispatched but no tool_result observed.
	if len(toolResults) == 0 {
		return ReasonToolLinkageBroken
	}
	return ReasonAppearsCompletedNoCommit
}
