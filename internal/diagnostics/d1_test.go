package diagnostics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/kerftranscript"
)

// fixedNow returns a fixed "now" close enough to the d1 fixtures'
// timestamps that the orphaned (>24h) branch does not fire. The
// fixtures are dated 2026-05-15; pin "now" to the same date.
func fixedNow() time.Time {
	return time.Date(2026, 5, 15, 20, 0, 0, 0, time.UTC)
}

// noCommitsEver is a HasCommitForFn that always reports false. Used
// when the fixture transcript has no companion git log — the detector
// then treats every dispatch as abandoned (subject to the floor +
// reason rules).
func noCommitsEver(_ string) bool { return false }

func loadFixtureEvents(t *testing.T, name string) []kerftranscript.Event {
	t.Helper()
	path := filepath.Join("..", "kerftranscript", "testdata", name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture %s: %v", path, err)
	}
	defer f.Close()
	res, err := kerftranscript.Parse(f)
	if err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	return res.Events
}

func TestDetectD1_AbandonA(t *testing.T) {
	events := loadFixtureEvents(t, "d1_abandon_a.jsonl")
	got := detectD1With(events, noCommitsEver, fixedNow)
	if len(got) != 1 {
		t.Fatalf("findings count = %d, want 1; got = %+v", len(got), got)
	}
	f := got[0]
	if f.BeadID != "hk-qo08q.15" {
		t.Errorf("BeadID = %q, want %q", f.BeadID, "hk-qo08q.15")
	}
	if f.SessionID != "fed61a3d-3aa9-4c8a-91e7-0b1acb4ec1e8" {
		t.Errorf("SessionID = %q", f.SessionID)
	}
	if f.SubAgentID != "aa848865eff923eae" {
		t.Errorf("SubAgentID = %q", f.SubAgentID)
	}
	if f.ReasonCategory != ReasonAppearsCompletedNoCommit {
		t.Errorf("ReasonCategory = %q, want %q", f.ReasonCategory, ReasonAppearsCompletedNoCommit)
	}
	if f.CloseCommit != "4a3c217" {
		t.Errorf("CloseCommit = %q, want %q", f.CloseCommit, "4a3c217")
	}
	if got := f.DispatchedAt.UTC().Format(time.RFC3339); got != "2026-05-15T18:10:11Z" {
		t.Errorf("DispatchedAt = %q", got)
	}
	if got := f.LastActivityAt.UTC().Format(time.RFC3339); got != "2026-05-15T18:12:02Z" {
		t.Errorf("LastActivityAt = %q", got)
	}
}

func TestDetectD1_AbandonB(t *testing.T) {
	events := loadFixtureEvents(t, "d1_abandon_b.jsonl")
	got := detectD1With(events, noCommitsEver, fixedNow)
	if len(got) != 1 {
		t.Fatalf("findings count = %d, want 1", len(got))
	}
	f := got[0]
	if f.BeadID != "hk-2ubs8" {
		t.Errorf("BeadID = %q, want %q", f.BeadID, "hk-2ubs8")
	}
	if f.ReasonCategory != ReasonAppearsCompletedNoCommit {
		t.Errorf("ReasonCategory = %q", f.ReasonCategory)
	}
}

func TestDetectD1_FloorSuppresses(t *testing.T) {
	// Dispatch + tool_result 30s apart: below 60s floor.
	base := time.Date(2026, 5, 15, 18, 0, 0, 0, time.UTC)
	events := []kerftranscript.Event{
		{Kind: kerftranscript.EventDispatch, Timestamp: base, SubAgentID: "sa1", BeadID: "x-aaaa"},
		{Kind: kerftranscript.EventToolResult, Timestamp: base.Add(30 * time.Second), SubAgentID: "sa1", BeadID: "x-aaaa"},
	}
	got := detectD1With(events, noCommitsEver, fixedNow)
	if len(got) != 0 {
		t.Errorf("got %d findings, want 0 (below 60s floor)", len(got))
	}
}

func TestDetectD1_HasCommitSuppresses(t *testing.T) {
	events := loadFixtureEvents(t, "d1_abandon_a.jsonl")
	always := func(_ string) bool { return true }
	got := detectD1With(events, always, fixedNow)
	if len(got) != 0 {
		t.Errorf("got %d findings, want 0 when indexer says committed", len(got))
	}
}

func TestDetectD1_ErroredMidTask(t *testing.T) {
	base := time.Date(2026, 5, 15, 18, 0, 0, 0, time.UTC)
	events := []kerftranscript.Event{
		{Kind: kerftranscript.EventDispatch, Timestamp: base, SubAgentID: "sa1", BeadID: "x-bbbb"},
		{Kind: kerftranscript.EventToolResult, Timestamp: base.Add(2 * time.Minute), SubAgentID: "sa1", BeadID: "x-bbbb", IsError: true},
	}
	got := detectD1With(events, noCommitsEver, fixedNow)
	if len(got) != 1 || got[0].ReasonCategory != ReasonErroredMidTask {
		t.Fatalf("want one errored_mid_task finding, got %+v", got)
	}
}
