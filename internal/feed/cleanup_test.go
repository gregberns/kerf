package feed

import (
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/queue"
	"github.com/gberns/kerf/internal/spec"
)

// --- helpers -------------------------------------------------------------

func mkWork(codename, status string, statusValues []string) *spec.SpecYAML {
	return &spec.SpecYAML{
		Codename:     codename,
		Status:       status,
		StatusValues: statusValues,
	}
}

func mkBead(id, epic, status string, labels ...string) beads.Bead {
	return beads.Bead{
		ID:     id,
		Epic:   epic,
		Status: status,
		Labels: labels,
	}
}

// runCleanup runs both detectors and returns (no-attach, beads-done).
func runCleanup(in Input, projectFilter *beads.Filter) ([]Item, []Item) {
	det := NewCleanupDetectors(projectFilter)
	if len(det) != 2 {
		panic("expected 2 cleanup detectors")
	}
	return det[0].Detect(in), det[1].Detect(in)
}

// --- work_no_attached_beads ---------------------------------------------

func TestCleanup_NoAttachedBeads_FiresWhenZero(t *testing.T) {
	w := mkWork("alpha", "research", []string{"research", "ready"})
	in := Input{
		Works:        []*spec.SpecYAML{w},
		AllBeads:     []beads.Bead{mkBead("k-1", "", "open", "work:other")},
		QueueEntries: []queue.Entry{{Codename: "alpha", Score: 12.5}},
	}
	noAttach, beadsDone := runCleanup(in, nil)
	if len(noAttach) != 1 {
		t.Fatalf("expected 1 no-attach item, got %d: %+v", len(noAttach), noAttach)
	}
	if len(beadsDone) != 0 {
		t.Errorf("beads-done must not fire for zero-bead work: %+v", beadsDone)
	}
	got := noAttach[0]
	if got.Kind != KindCleanup {
		t.Errorf("kind = %v, want cleanup", got.Kind)
	}
	if got.Title != "no attached beads" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Action == "" {
		t.Errorf("action should be populated")
	}
	// Work has no per-work bead_filter declared (mkWork sets none) so the
	// rank-label classifier (Plan 019 / B2) tags this case as `unwired`.
	if got.Reason != "no bead_filter declared on spec.yaml" {
		t.Errorf("reason = %q", got.Reason)
	}
	if got.RankLabel != "unwired" {
		t.Errorf("rank_label = %q, want unwired (no bead_filter on spec.yaml)", got.RankLabel)
	}
	if got.WorkCodename == nil || *got.WorkCodename != "alpha" {
		t.Errorf("work_codename = %v, want alpha", got.WorkCodename)
	}
	if got.BeadID != nil {
		t.Errorf("bead_id should be nil, got %v", got.BeadID)
	}
	if got.Score != 12.5 {
		t.Errorf("score = %v, want 12.5 (parent-work score)", got.Score)
	}
}

// TestCleanup_NoAttachedBeads_RankLabelEmpty exercises the second of the two
// observable rank-label states (Plan 019 / B2): a work whose per-work
// bead_filter is declared and parses cleanly but resolves to zero matches in
// the current bead store. The classifier should label this `empty`, not
// `unwired`, so the agent reads it as "wired but no beads yet" rather than
// "needs bootstrapping."
func TestCleanup_NoAttachedBeads_RankLabelEmpty(t *testing.T) {
	w := mkWork("alpha", "research", []string{"research", "ready"})
	w.BeadFilter = &beads.Filter{Label: "subsystem:nonexistent"}
	in := Input{
		Works:    []*spec.SpecYAML{w},
		AllBeads: []beads.Bead{mkBead("k-1", "", "open", "subsystem:other")},
	}
	noAttach, _ := runCleanup(in, nil)
	if len(noAttach) != 1 {
		t.Fatalf("expected 1 no-attach item, got %d", len(noAttach))
	}
	got := noAttach[0]
	if got.RankLabel != "empty" {
		t.Errorf("rank_label = %q, want empty (bead_filter declared, zero matches)", got.RankLabel)
	}
	if got.Reason != "resolved bead_filter matches zero beads in the store" {
		t.Errorf("reason = %q", got.Reason)
	}
}

func TestCleanup_NoAttachedBeads_DoesNotFireWhenAttached(t *testing.T) {
	w := mkWork("alpha", "research", []string{"research", "ready"})
	in := Input{
		Works:    []*spec.SpecYAML{w},
		AllBeads: []beads.Bead{mkBead("k-1", "", "open", "work:alpha")},
	}
	noAttach, _ := runCleanup(in, nil)
	if len(noAttach) != 0 {
		t.Errorf("no-attach must not fire when beads exist; got %+v", noAttach)
	}
}

// --- work_beads_done_status_open ----------------------------------------

func TestCleanup_BeadsDoneStatusOpen_Fires(t *testing.T) {
	w := mkWork("alpha", "research", []string{"research", "review", "ready"})
	in := Input{
		Works: []*spec.SpecYAML{w},
		AllBeads: []beads.Bead{
			mkBead("k-1", "", "closed", "work:alpha"),
			mkBead("k-2", "", "done", "work:alpha"),
		},
		QueueEntries: []queue.Entry{{Codename: "alpha", Score: 7.0}},
	}
	noAttach, beadsDone := runCleanup(in, nil)
	if len(noAttach) != 0 {
		t.Errorf("no-attach must not fire when attached > 0: %+v", noAttach)
	}
	if len(beadsDone) != 1 {
		t.Fatalf("expected 1 beads-done item, got %d", len(beadsDone))
	}
	got := beadsDone[0]
	if got.Kind != KindCleanup {
		t.Errorf("kind = %v, want cleanup", got.Kind)
	}
	if got.Title != "beads done, jig walk owed" {
		t.Errorf("title = %q", got.Title)
	}
	if !strings.Contains(got.Action, "kerf status alpha") || !strings.Contains(got.Action, "kerf shelve alpha") {
		t.Errorf("action should reference both verbs with codename: %q", got.Action)
	}
	if !strings.Contains(got.Reason, "2 attached beads closed") || !strings.Contains(got.Reason, "research") {
		t.Errorf("reason should mention count + status, got %q", got.Reason)
	}
	if got.WorkCodename == nil || *got.WorkCodename != "alpha" {
		t.Errorf("work_codename mismatch: %v", got.WorkCodename)
	}
	if got.BeadID != nil {
		t.Errorf("bead_id should be nil")
	}
	if got.Score != 7.0 {
		t.Errorf("score = %v, want 7.0", got.Score)
	}
}

func TestCleanup_BeadsDoneStatusOpen_TerminalStatusSkips(t *testing.T) {
	// Work already at terminal status — no cleanup owed.
	w := mkWork("alpha", "ready", []string{"research", "ready"})
	in := Input{
		Works:    []*spec.SpecYAML{w},
		AllBeads: []beads.Bead{mkBead("k-1", "", "closed", "work:alpha")},
	}
	_, beadsDone := runCleanup(in, nil)
	if len(beadsDone) != 0 {
		t.Errorf("must not fire when work status is terminal; got %+v", beadsDone)
	}
}

func TestCleanup_BeadsDoneStatusOpen_NonAllClosedSkips(t *testing.T) {
	w := mkWork("alpha", "research", []string{"research", "ready"})
	in := Input{
		Works: []*spec.SpecYAML{w},
		AllBeads: []beads.Bead{
			mkBead("k-1", "", "closed", "work:alpha"),
			mkBead("k-2", "", "open", "work:alpha"),
		},
	}
	noAttach, beadsDone := runCleanup(in, nil)
	if len(noAttach) != 0 || len(beadsDone) != 0 {
		t.Errorf("neither detector should fire: noAttach=%+v beadsDone=%+v", noAttach, beadsDone)
	}
}

// --- mutual exclusion ---------------------------------------------------

func TestCleanup_MutualExclusion(t *testing.T) {
	// Two works: one with zero beads, one with all-closed beads + open status.
	wZero := mkWork("zero", "research", []string{"research", "ready"})
	wDone := mkWork("done", "research", []string{"research", "ready"})
	in := Input{
		Works: []*spec.SpecYAML{wZero, wDone},
		AllBeads: []beads.Bead{
			mkBead("k-1", "", "closed", "work:done"),
		},
	}
	noAttach, beadsDone := runCleanup(in, nil)

	// "zero" should only appear in no-attach.
	seenZero := 0
	for _, it := range noAttach {
		if it.WorkCodename != nil && *it.WorkCodename == "zero" {
			seenZero++
		}
	}
	for _, it := range beadsDone {
		if it.WorkCodename != nil && *it.WorkCodename == "zero" {
			t.Errorf("zero-bead work must not appear in beads-done detector")
		}
	}
	if seenZero != 1 {
		t.Errorf("zero work should appear once in no-attach, got %d", seenZero)
	}

	// "done" should only appear in beads-done.
	for _, it := range noAttach {
		if it.WorkCodename != nil && *it.WorkCodename == "done" {
			t.Errorf("done work must not appear in no-attach detector")
		}
	}
	seenDone := 0
	for _, it := range beadsDone {
		if it.WorkCodename != nil && *it.WorkCodename == "done" {
			seenDone++
		}
	}
	if seenDone != 1 {
		t.Errorf("done work should appear once in beads-done, got %d", seenDone)
	}
}

// --- archived/finalized suppression -------------------------------------

func TestCleanup_ArchivedFinalizedSuppressed(t *testing.T) {
	wZero := mkWork("zero", "research", []string{"research", "ready"})
	wDone := mkWork("done", "research", []string{"research", "ready"})
	in := Input{
		Works:    []*spec.SpecYAML{wZero, wDone},
		AllBeads: []beads.Bead{mkBead("k-1", "", "closed", "work:done")},
		ArchivedOrFinalized: map[string]bool{
			"zero": true,
			"done": true,
		},
	}
	noAttach, beadsDone := runCleanup(in, nil)
	if len(noAttach) != 0 {
		t.Errorf("archived work must produce no no-attach item: %+v", noAttach)
	}
	if len(beadsDone) != 0 {
		t.Errorf("finalized work must produce no beads-done item: %+v", beadsDone)
	}
}

// --- filter resolution honored ------------------------------------------

func TestCleanup_RespectsResolvedFilter_PerWorkOverride(t *testing.T) {
	// Per-work filter targets id-prefix; project filter is irrelevant override.
	perWork := &beads.Filter{IDPrefix: "alpha-"}
	w := &spec.SpecYAML{
		Codename:     "alpha",
		Status:       "research",
		StatusValues: []string{"research", "ready"},
		BeadFilter:   perWork,
	}
	in := Input{
		Works: []*spec.SpecYAML{w},
		AllBeads: []beads.Bead{
			mkBead("alpha-1", "", "closed"),
		},
	}
	projectFilter := &beads.Filter{Label: "work:{codename}"}
	noAttach, beadsDone := runCleanup(in, projectFilter)
	if len(noAttach) != 0 {
		t.Errorf("per-work override matches the bead; no-attach should not fire: %+v", noAttach)
	}
	if len(beadsDone) != 1 {
		t.Fatalf("per-work override matches a closed bead; beads-done should fire, got %+v", beadsDone)
	}
}

func TestCleanup_RespectsResolvedFilter_ProjectFallback(t *testing.T) {
	// No per-work; project filter governs.
	w := mkWork("alpha", "research", []string{"research", "ready"})
	in := Input{
		Works:    []*spec.SpecYAML{w},
		AllBeads: []beads.Bead{mkBead("k-1", "", "open", "subsystem:alpha")},
	}
	projectFilter := &beads.Filter{Label: "subsystem:{codename}"}
	noAttach, _ := runCleanup(in, projectFilter)
	if len(noAttach) != 0 {
		t.Errorf("project filter matches; no-attach should not fire: %+v", noAttach)
	}
}

// --- empty input --------------------------------------------------------

func TestCleanup_NoWorks(t *testing.T) {
	in := Input{}
	noAttach, beadsDone := runCleanup(in, nil)
	if len(noAttach) != 0 || len(beadsDone) != 0 {
		t.Errorf("empty input should produce no items")
	}
}
