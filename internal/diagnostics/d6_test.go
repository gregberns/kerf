package diagnostics

import (
	"testing"
	"time"

	"github.com/gberns/kerf/internal/kerftranscript"
)

// Reuses loadFixtureEvents from d1_test.go (same package).

func TestDetectD6_FixtureA_SingleBeadAbsent(t *testing.T) {
	events := loadFixtureEvents(t, "d6_reviewer_absent_a.jsonl")
	got := DetectD6(events, DetectD6Options{})
	if len(got) != 1 {
		t.Fatalf("findings count = %d, want 1; got = %+v", len(got), got)
	}
	f := got[0]
	if f.BeadID != "hk-iuaed.6" {
		t.Errorf("BeadID = %q, want %q", f.BeadID, "hk-iuaed.6")
	}
	if f.SessionID != "801120b5-0000-4000-8000-000000000001" {
		t.Errorf("SessionID = %q", f.SessionID)
	}
	if f.CommitSHA != "dcd7f7e5d1a5eb4cf6dc4b292d86a5ea01562c4f" {
		t.Errorf("CommitSHA = %q", f.CommitSHA)
	}
	if got := f.CommittedAt.UTC().Format(time.RFC3339); got != "2026-05-15T20:23:40Z" {
		t.Errorf("CommittedAt = %q", got)
	}
	if f.ImplementerSubAgentID != "a11a11a11a11a11a1" {
		t.Errorf("ImplementerSubAgentID = %q", f.ImplementerSubAgentID)
	}
}

func TestDetectD6_FixtureB_PerBeadQuery(t *testing.T) {
	// Per specs/diagnostics.md §"Multi-bead transcript fixtures": the
	// _b.jsonl fixture carries two commit_refs in one session; the
	// `--bead=hk-zixbp` query yields exactly one finding for that bead.
	events := loadFixtureEvents(t, "d6_reviewer_absent_b.jsonl")
	got := DetectD6(events, DetectD6Options{BeadID: "hk-zixbp"})
	if len(got) != 1 {
		t.Fatalf("findings count = %d, want 1; got = %+v", len(got), got)
	}
	f := got[0]
	if f.BeadID != "hk-zixbp" {
		t.Errorf("BeadID = %q, want %q", f.BeadID, "hk-zixbp")
	}
	if f.CommitSHA != "cc3da5c1b255fd5bd2c94e859d4d653ae6d1e5c6" {
		t.Errorf("CommitSHA = %q", f.CommitSHA)
	}
	if f.ImplementerSubAgentID != "b22b22b22b22b22b2" {
		t.Errorf("ImplementerSubAgentID = %q", f.ImplementerSubAgentID)
	}
}

func TestDetectD6_FixtureB_AllBeadsQuery(t *testing.T) {
	// No --bead given → all beads in the fixture's universe are
	// reported. The _b.jsonl carries two commit_refs (hk-zixbp +
	// hk-qo08q), both reviewer-absent in the same session.
	events := loadFixtureEvents(t, "d6_reviewer_absent_b.jsonl")
	got := DetectD6(events, DetectD6Options{})
	if len(got) != 2 {
		t.Fatalf("findings count = %d, want 2; got = %+v", len(got), got)
	}
	// Deterministic order: by committed_at ascending. hk-qo08q (18:53)
	// before hk-zixbp (19:03).
	if got[0].BeadID != "hk-qo08q" {
		t.Errorf("got[0].BeadID = %q, want hk-qo08q", got[0].BeadID)
	}
	if got[1].BeadID != "hk-zixbp" {
		t.Errorf("got[1].BeadID = %q, want hk-zixbp", got[1].BeadID)
	}
}

func TestDetectD6_FixtureC_PerBeadQuery(t *testing.T) {
	events := loadFixtureEvents(t, "d6_reviewer_absent_c.jsonl")
	got := DetectD6(events, DetectD6Options{BeadID: "hk-qo08q"})
	if len(got) != 1 {
		t.Fatalf("findings count = %d, want 1; got = %+v", len(got), got)
	}
	f := got[0]
	if f.BeadID != "hk-qo08q" {
		t.Errorf("BeadID = %q, want %q", f.BeadID, "hk-qo08q")
	}
	if f.CommitSHA != "76a55be161a5d0c071fd511c47c57f30688ac1ec" {
		t.Errorf("CommitSHA = %q", f.CommitSHA)
	}
	if got := f.CommittedAt.UTC().Format(time.RFC3339); got != "2026-05-15T18:53:31Z" {
		t.Errorf("CommittedAt = %q", got)
	}
}

func TestDetectD6_ReviewerDispatchSuppresses_TextMarker(t *testing.T) {
	// A reviewer dispatch carrying the canonical text-format header
	// suppresses the D6 finding for any commit in the same session.
	base := time.Date(2026, 5, 15, 18, 0, 0, 0, time.UTC)
	events := []kerftranscript.Event{
		{
			Kind: kerftranscript.EventDispatch, Timestamp: base,
			SessionID: "s1", SubAgentID: "impl-1", BeadID: "x-aaaa",
			Role: "implementer", Text: "dispatch implementer for x-aaaa",
		},
		{
			Kind: kerftranscript.EventDispatch, Timestamp: base.Add(time.Minute),
			SessionID: "s1", SubAgentID: "rev-1", BeadID: "x-aaaa",
			Role: "reviewer",
			// Em-dash (U+2014) per spec.
			Text: "Reviewer prompt for codename-alpha — pass: implementer",
		},
		{
			Kind: kerftranscript.EventCommitRef, Timestamp: base.Add(2 * time.Minute),
			SessionID: "s1", BeadID: "x-aaaa", CommitSHA: "abc123",
		},
	}
	got := DetectD6(events, DetectD6Options{})
	if len(got) != 0 {
		t.Errorf("got %d findings, want 0 (text marker should suppress); got = %+v", len(got), got)
	}
}

func TestDetectD6_ReviewerDispatchSuppresses_JSONMarker(t *testing.T) {
	base := time.Date(2026, 5, 15, 18, 0, 0, 0, time.UTC)
	jsonPayload := `Some preamble. {"codename":"alpha","pass":"implementer","artifacts":["a"],"criteria":["c"]}`
	events := []kerftranscript.Event{
		{
			Kind: kerftranscript.EventDispatch, Timestamp: base,
			SessionID: "s2", SubAgentID: "impl-2", BeadID: "x-bbbb",
			Role: "implementer", Text: "dispatch implementer",
		},
		{
			Kind: kerftranscript.EventDispatch, Timestamp: base.Add(time.Minute),
			SessionID: "s2", SubAgentID: "rev-2", BeadID: "x-bbbb",
			Role: "reviewer", Text: jsonPayload,
		},
		{
			Kind: kerftranscript.EventCommitRef, Timestamp: base.Add(2 * time.Minute),
			SessionID: "s2", BeadID: "x-bbbb", CommitSHA: "def456",
		},
	}
	got := DetectD6(events, DetectD6Options{})
	if len(got) != 0 {
		t.Errorf("got %d findings, want 0 (JSON marker should suppress); got = %+v", len(got), got)
	}
}

func TestDetectD6_LooseReviewSubstringDoesNotSuppress(t *testing.T) {
	// Critical FP-avoidance check: a dispatch whose text merely mentions
	// "review" is NOT a reviewer dispatch per the normative definition.
	// Without either canonical marker, the commit must still produce a
	// finding. This is the load-bearing rule that prevents the ~28 kerf
	// false positives observed in calibration.
	base := time.Date(2026, 5, 15, 18, 0, 0, 0, time.UTC)
	events := []kerftranscript.Event{
		{
			Kind: kerftranscript.EventDispatch, Timestamp: base,
			SessionID: "s3", SubAgentID: "impl-3", BeadID: "x-cccc",
			Role: "implementer", Text: "please review the spec before implementing",
		},
		{
			Kind: kerftranscript.EventCommitRef, Timestamp: base.Add(time.Minute),
			SessionID: "s3", BeadID: "x-cccc", CommitSHA: "789abc",
		},
	}
	got := DetectD6(events, DetectD6Options{})
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1 (loose 'review' substring must not suppress); got = %+v", len(got), got)
	}
}

func TestDispatchedBeadCount(t *testing.T) {
	base := time.Date(2026, 5, 15, 18, 0, 0, 0, time.UTC)
	events := []kerftranscript.Event{
		{Kind: kerftranscript.EventDispatch, Timestamp: base, BeadID: "x-1", SubAgentID: "a"},
		{Kind: kerftranscript.EventDispatch, Timestamp: base, BeadID: "x-2", SubAgentID: "b"},
		{Kind: kerftranscript.EventDispatch, Timestamp: base, BeadID: "x-1", SubAgentID: "c"}, // dup bead
		{Kind: kerftranscript.EventCommitRef, Timestamp: base, BeadID: "x-3"},                 // commit not counted
	}
	if got := DispatchedBeadCount(events); got != 2 {
		t.Errorf("DispatchedBeadCount = %d, want 2", got)
	}
}
