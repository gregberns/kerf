package feed

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/drift"
	"github.com/gberns/kerf/internal/queue"
	"github.com/gberns/kerf/internal/spec"
)

func sp(s string) *string { return &s }

// --- JSON round-trip: absent pointers emit literal null ------------------

func TestItemJSON_NullForAbsentPointers(t *testing.T) {
	it := Item{
		Kind:   KindWarning,
		Score:  0,
		Title:  "spec missing",
		Action: "fix",
		Reason: "no codename",
		// WorkCodename and BeadID intentionally nil.
	}
	out, err := json.Marshal(it)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `"work_codename":null`) {
		t.Errorf("expected literal null work_codename, got %s", s)
	}
	if !strings.Contains(s, `"bead_id":null`) {
		t.Errorf("expected literal null bead_id, got %s", s)
	}
	if !strings.Contains(s, `"kind":"warning"`) {
		t.Errorf("expected kind=warning, got %s", s)
	}

	// Round-trip preserves null -> nil.
	var rt Item
	if err := json.Unmarshal(out, &rt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rt.WorkCodename != nil || rt.BeadID != nil {
		t.Errorf("round-trip should yield nil pointers, got %+v", rt)
	}
}

func TestItemJSON_GoldenBytes(t *testing.T) {
	it := Item{
		Kind:         KindBead,
		Score:        12.5,
		Title:        "implement parser",
		Action:       "claim and start",
		WorkCodename: sp("token-refresh"),
		BeadID:       sp("kerf-abc"),
		Reason:       "rework bonus",
	}
	out, err := json.Marshal(it)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"kind":"bead","score":12.5,"title":"implement parser","action":"claim and start","work_codename":"token-refresh","bead_id":"kerf-abc","reason":"rework bonus"}`
	if string(out) != want {
		t.Errorf("golden mismatch:\n got:  %s\n want: %s", string(out), want)
	}
}

// --- Assemble: beads-then-cleanups order ---------------------------------

func TestAssemble_BeadsBeforeCleanups(t *testing.T) {
	bds := []Item{
		{Kind: KindBead, Score: 1.0, Title: "b-low", WorkCodename: sp("w1"), BeadID: sp("k-1")},
		{Kind: KindBead, Score: 9.0, Title: "b-high", WorkCodename: sp("w2"), BeadID: sp("k-2")},
	}
	// Cleanup with higher score than lowest bead — must still sort AFTER all beads.
	cleanups := []Item{
		{Kind: KindCleanup, Score: 5.0, Title: "c-mid", WorkCodename: sp("w3")},
	}
	warnings := []Item{
		{Kind: KindWarning, Title: "missing spec"},
	}

	main, warns := AssembleWithWarnings(bds, cleanups, warnings, Input{})
	if len(main) != 3 {
		t.Fatalf("want 3 main items, got %d: %+v", len(main), main)
	}
	if main[0].Kind != KindBead || main[1].Kind != KindBead {
		t.Errorf("first two items should be beads, got %v %v", main[0].Kind, main[1].Kind)
	}
	if main[0].Score != 9.0 {
		t.Errorf("highest-score bead should be first, got %v", main[0].Score)
	}
	if main[2].Kind != KindCleanup {
		t.Errorf("cleanup should sort after all beads, got %v at index 2", main[2].Kind)
	}
	if len(warns) != 1 || warns[0].Kind != KindWarning {
		t.Errorf("warnings should be returned separately: %+v", warns)
	}
}

// --- Cleanup tie-break: equal score -> created ascending ----------------

func TestAssemble_CleanupTieBreakByCreated(t *testing.T) {
	older := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	cleanups := []Item{
		{Kind: KindCleanup, Score: 3.0, Title: "newer", WorkCodename: sp("w-new")},
		{Kind: KindCleanup, Score: 3.0, Title: "older", WorkCodename: sp("w-old")},
	}
	state := Input{
		WorkCreated: map[string]time.Time{"w-old": older, "w-new": newer},
	}
	main, _ := AssembleWithWarnings(nil, cleanups, nil, state)
	if len(main) != 2 {
		t.Fatalf("want 2 items, got %d", len(main))
	}
	if main[0].Title != "older" {
		t.Errorf("tie-break: older work should come first, got %q then %q",
			main[0].Title, main[1].Title)
	}
}

// --- Filter precedence truth table --------------------------------------

func TestResolveKindSelection_TruthTable(t *testing.T) {
	type tc struct {
		name           string
		kinds, only, inc []string
		want           []Kind
	}
	cases := []tc{
		{"default = all", nil, nil, nil, []Kind{KindBead, KindCleanup, KindWarning}},
		{"--kinds=bead", []string{"bead"}, nil, nil, []Kind{KindBead}},
		{"--kinds=bead,cleanup", []string{"bead", "cleanup"}, nil, nil, []Kind{KindBead, KindCleanup}},
		{"--only intersects base", nil, []string{"warning"}, nil, []Kind{KindWarning}},
		{"--kinds=bead,cleanup --only=cleanup", []string{"bead", "cleanup"}, []string{"cleanup"}, nil, []Kind{KindCleanup}},
		{"--include adds", []string{"bead"}, nil, []string{"warning"}, []Kind{KindBead, KindWarning}},
		{"--only then --include", []string{"bead", "cleanup"}, []string{"bead"}, []string{"warning"}, []Kind{KindBead, KindWarning}},
		// Repeated identical flags act as union (idempotent).
		{"repeated --kinds union", []string{"bead", "bead"}, nil, nil, []Kind{KindBead}},
		{"repeated --only union", nil, []string{"bead", "bead"}, nil, []Kind{KindBead}},
		{"repeated --include union", []string{"bead"}, nil, []string{"warning", "warning"}, []Kind{KindBead, KindWarning}},
		// --only with multiple values: intersects with union of onlys.
		{"--only multi values", []string{"bead", "cleanup", "warning"}, []string{"bead", "warning"}, nil, []Kind{KindBead, KindWarning}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ResolveKindSelection(c.kinds, c.only, c.inc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("want %v kinds, got %v: sel=%v", c.want, mapKeys(got), got)
			}
			for _, k := range c.want {
				if !got.Has(k) {
					t.Errorf("missing %s in result %v", k, mapKeys(got))
				}
			}
		})
	}
}

func mapKeys(s KindSelection) []Kind {
	out := make([]Kind, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return out
}

func TestResolveKindSelection_UnknownToken(t *testing.T) {
	if _, err := ResolveKindSelection([]string{"bogus"}, nil, nil); err == nil {
		t.Error("expected error for unknown kind in --kinds")
	}
	if _, err := ResolveKindSelection(nil, []string{"bogus"}, nil); err == nil {
		t.Error("expected error for unknown kind in --only")
	}
	if _, err := ResolveKindSelection(nil, nil, []string{"bogus"}); err == nil {
		t.Error("expected error for unknown kind in --include")
	}
}

func TestApplyKindFilter(t *testing.T) {
	items := []Item{
		{Kind: KindBead, Title: "b"},
		{Kind: KindCleanup, Title: "c"},
		{Kind: KindWarning, Title: "w"},
	}
	sel, _ := ResolveKindSelection([]string{"bead", "warning"}, nil, nil)
	got := ApplyKindFilter(items, sel)
	if len(got) != 2 || got[0].Title != "b" || got[1].Title != "w" {
		t.Errorf("filter result: %+v", got)
	}
}

// --- Exclusion rules -----------------------------------------------------

func TestExclude_BeadBlocked(t *testing.T) {
	bds := []Item{
		{Kind: KindBead, Title: "blocked-bead", WorkCodename: sp("w-blocked"), BeadID: sp("k-1")},
		{Kind: KindBead, Title: "ok-bead", WorkCodename: sp("w-ok"), BeadID: sp("k-2")},
	}
	state := Input{BlockedWorks: map[string]bool{"w-blocked": true}}
	main, _ := AssembleWithWarnings(bds, nil, nil, state)
	if len(main) != 1 || main[0].Title != "ok-bead" {
		t.Errorf("blocked bead should be excluded; got %+v", main)
	}
}

func TestExclude_CleanupNotExcludedByBlocked(t *testing.T) {
	cleanups := []Item{
		{Kind: KindCleanup, Title: "cleanup-on-blocked", WorkCodename: sp("w-blocked")},
	}
	state := Input{BlockedWorks: map[string]bool{"w-blocked": true}}
	main, _ := AssembleWithWarnings(nil, cleanups, nil, state)
	if len(main) != 1 {
		t.Errorf("cleanup on blocked work must NOT be excluded; got %+v", main)
	}
}

func TestExclude_ArchivedExcludesBoth(t *testing.T) {
	bds := []Item{{Kind: KindBead, Title: "b", WorkCodename: sp("w-arch"), BeadID: sp("k")}}
	cleanups := []Item{{Kind: KindCleanup, Title: "c", WorkCodename: sp("w-arch")}}
	state := Input{ArchivedOrFinalized: map[string]bool{"w-arch": true}}
	main, _ := AssembleWithWarnings(bds, cleanups, nil, state)
	if len(main) != 0 {
		t.Errorf("archived work should exclude both bead and cleanup; got %+v", main)
	}
}

func TestExclude_WarningsNeverFiltered(t *testing.T) {
	warnings := []Item{{Kind: KindWarning, Title: "w", WorkCodename: sp("w-arch")}}
	state := Input{
		BlockedWorks:        map[string]bool{"w-arch": true},
		ArchivedOrFinalized: map[string]bool{"w-arch": true},
	}
	_, warns := AssembleWithWarnings(nil, nil, warnings, state)
	if len(warns) != 1 {
		t.Errorf("warnings must never be filtered; got %+v", warns)
	}
}

// --- Empty inputs --------------------------------------------------------

func TestAssemble_Empty(t *testing.T) {
	main, warns := AssembleWithWarnings(nil, nil, nil, Input{})
	if len(main) != 0 {
		t.Errorf("empty inputs should yield empty main, got %+v", main)
	}
	if len(warns) != 0 {
		t.Errorf("empty inputs should yield empty warnings, got %+v", warns)
	}
}

// --- Sort: cross-kind --------------------------------------------------

func TestSort_CrossKind(t *testing.T) {
	older := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	items := []Item{
		{Kind: KindCleanup, Score: 100.0, Title: "c-high", WorkCodename: sp("w-old")},
		{Kind: KindBead, Score: 0.1, Title: "b-low", WorkCodename: sp("w-x"), BeadID: sp("k-x")},
		{Kind: KindBead, Score: 5.0, Title: "b-mid", WorkCodename: sp("w-y"), BeadID: sp("k-y")},
		{Kind: KindCleanup, Score: 100.0, Title: "c-high-newer", WorkCodename: sp("w-new")},
		{Kind: KindWarning, Title: "warn"},
	}
	wc := map[string]time.Time{"w-old": older, "w-new": newer}
	Sort(items, wc)

	// Expect: bead(5.0), bead(0.1), cleanup(100, older), cleanup(100, newer), warning.
	if items[0].Title != "b-mid" || items[1].Title != "b-low" {
		t.Errorf("beads should be first by score desc; got %s, %s", items[0].Title, items[1].Title)
	}
	if items[2].Title != "c-high" || items[3].Title != "c-high-newer" {
		t.Errorf("cleanup tie-break by created asc; got %s, %s", items[2].Title, items[3].Title)
	}
	if items[4].Kind != KindWarning {
		t.Errorf("warning should sort last in flat Sort, got %v", items[4].Kind)
	}
}

// --- BeadSource ---------------------------------------------------------

func TestBeadSource_FiltersByStatus(t *testing.T) {
	// Imported lazily via internal/beads in the production path. We construct
	// a minimal Input here using only the fields BeadSource reads.
	in := Input{
		AllBeads: nil,
	}
	if got := BeadSource(in); got != nil {
		t.Errorf("nil beads should yield nil, got %+v", got)
	}
}

// TestBeadSource_WithLabelFilter — a bead with no Epic field but a label
// matching a work's resolved bead_filter must surface with the correct
// work_codename. This is the B:F4 contract: BeadSource consults
// Input.BeadToWork (the caller's resolved-filter join), not bead.Epic.
func TestBeadSource_WithLabelFilter(t *testing.T) {
	in := Input{
		AllBeads: []beads.Bead{
			{ID: "kerf-aaa", Title: "labelled", Status: "open", Epic: "" /* no epic */, Labels: []string{"work:token-refresh"}},
		},
		QueueEntries: []queue.Entry{{Codename: "token-refresh", Score: 7.5}},
		BeadToWork: map[string][]string{
			"kerf-aaa": {"token-refresh"},
		},
	}
	items := BeadSource(in)
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d: %+v", len(items), items)
	}
	it := items[0]
	if it.WorkCodename == nil || *it.WorkCodename != "token-refresh" {
		t.Errorf("WorkCodename: want pointer to %q, got %v", "token-refresh", it.WorkCodename)
	}
	if it.BeadID == nil || *it.BeadID != "kerf-aaa" {
		t.Errorf("BeadID: want pointer to %q, got %v", "kerf-aaa", it.BeadID)
	}
	if it.Score != 7.5 {
		t.Errorf("Score: want 7.5 (from queue entry), got %v", it.Score)
	}
}

// TestBeadSource_MultiMatch — a bead matching multiple works emits one
// item per match, each with a distinct work_codename and the score of
// that work.
func TestBeadSource_MultiMatch(t *testing.T) {
	in := Input{
		AllBeads: []beads.Bead{
			{ID: "kerf-multi", Title: "cross-cutting", Status: "open", Labels: []string{"work:alpha", "work:beta"}},
		},
		QueueEntries: []queue.Entry{
			{Codename: "alpha", Score: 10.0},
			{Codename: "beta", Score: 3.0},
		},
		BeadToWork: map[string][]string{
			"kerf-multi": {"alpha", "beta"},
		},
	}
	items := BeadSource(in)
	if len(items) != 2 {
		t.Fatalf("multi-match: want 2 items, got %d: %+v", len(items), items)
	}
	gotWorks := []string{*items[0].WorkCodename, *items[1].WorkCodename}
	if gotWorks[0] != "alpha" || gotWorks[1] != "beta" {
		t.Errorf("want emit order [alpha, beta], got %v", gotWorks)
	}
	if items[0].Score != 10.0 || items[1].Score != 3.0 {
		t.Errorf("per-match scores should follow the matched work; got %v, %v", items[0].Score, items[1].Score)
	}
	// Both items reference the same bead ID, but pointer identity is not required.
	if *items[0].BeadID != "kerf-multi" || *items[1].BeadID != "kerf-multi" {
		t.Errorf("both items should reference kerf-multi; got %q, %q", *items[0].BeadID, *items[1].BeadID)
	}
}

// TestBeadSource_UnattachedBeadDropped — a ready bead with no entry in
// BeadToWork is not emitted (it surfaces elsewhere as the unmatched
// header count).
func TestBeadSource_UnattachedBeadDropped(t *testing.T) {
	in := Input{
		AllBeads: []beads.Bead{
			{ID: "kerf-orphan", Title: "orphan", Status: "open", Epic: "ghost", Labels: []string{"misc"}},
		},
		// BeadToWork omitted: kerf-orphan matches no work.
	}
	if items := BeadSource(in); len(items) != 0 {
		t.Errorf("unattached bead should not be emitted; got %+v", items)
	}
}

// --- ResolvePins + BeadSource end-to-end (Plan 009 / Bead 5) -----------

// TestResolvePins_OverridesMultiMatch — a bead that matches works A and
// C via filter, pinned to A, ends up attached to A only. BeadSource then
// emits a single item under A.
func TestResolvePins_OverridesMultiMatch(t *testing.T) {
	beadToWork := map[string][]string{
		"kerf-b": {"alpha", "charlie"},
	}
	pins := map[string]string{
		"kerf-b": "alpha",
	}
	resolved := ResolvePins(beadToWork, pins)
	if got := resolved["kerf-b"]; len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("ResolvePins: want [alpha], got %v", got)
	}

	in := Input{
		AllBeads: []beads.Bead{
			{ID: "kerf-b", Title: "pinned", Status: "open"},
		},
		QueueEntries: []queue.Entry{
			{Codename: "alpha", Score: 10.0},
			{Codename: "charlie", Score: 3.0},
		},
		BeadToWork:     resolved,
		PinAssignments: pins,
	}
	items := BeadSource(in)
	if len(items) != 1 {
		t.Fatalf("want 1 item after pin override, got %d: %+v", len(items), items)
	}
	if items[0].WorkCodename == nil || *items[0].WorkCodename != "alpha" {
		t.Errorf("pinned bead should attach only to alpha, got %v", items[0].WorkCodename)
	}
	if items[0].Score != 10.0 {
		t.Errorf("score should reflect the pinning work (alpha=10), got %v", items[0].Score)
	}
}

// TestResolvePins_SurfacesUnmatched — a bead pinned to A but absent from
// the filter-resolved BeadToWork (i.e. no filter caught it) MUST appear
// under A. This is the whole point of pins: catch beads the filter does
// not.
func TestResolvePins_SurfacesUnmatched(t *testing.T) {
	beadToWork := map[string][]string{} // no filter match for kerf-orphan
	pins := map[string]string{
		"kerf-orphan": "alpha",
	}
	resolved := ResolvePins(beadToWork, pins)
	if got := resolved["kerf-orphan"]; len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("ResolvePins: want [alpha] for unmatched-but-pinned bead, got %v", got)
	}

	in := Input{
		AllBeads: []beads.Bead{
			{ID: "kerf-orphan", Title: "orphan", Status: "open"},
		},
		QueueEntries:   []queue.Entry{{Codename: "alpha", Score: 7.0}},
		BeadToWork:     resolved,
		PinAssignments: pins,
	}
	items := BeadSource(in)
	if len(items) != 1 {
		t.Fatalf("pinned-but-unmatched bead should surface under owner, got %d items: %+v", len(items), items)
	}
	if items[0].WorkCodename == nil || *items[0].WorkCodename != "alpha" {
		t.Errorf("WorkCodename: want alpha, got %v", items[0].WorkCodename)
	}
}

// TestResolvePins_PassesThroughUnpinned — beads not in PinAssignments
// keep their filter-resolved attachment unchanged, and ResolvePins must
// return a defensive copy (mutating the result does not mutate the
// input map).
func TestResolvePins_PassesThroughUnpinned(t *testing.T) {
	beadToWork := map[string][]string{
		"kerf-x": {"alpha", "beta"},
		"kerf-y": {"gamma"},
	}
	pins := map[string]string{
		"kerf-x": "alpha", // pinned, narrows
		// kerf-y untouched
	}
	resolved := ResolvePins(beadToWork, pins)
	if got := resolved["kerf-y"]; len(got) != 1 || got[0] != "gamma" {
		t.Errorf("unpinned bead should pass through; got %v", got)
	}
	// Mutating returned slice must not bleed into the original.
	resolved["kerf-y"][0] = "mutated"
	if beadToWork["kerf-y"][0] != "gamma" {
		t.Errorf("ResolvePins should return defensive copies; original was mutated")
	}
}

// TestResolvePins_EmptyInputs — both inputs nil/empty: returns a
// non-nil empty map (caller can use it as feed.Input.BeadToWork).
func TestResolvePins_EmptyInputs(t *testing.T) {
	got := ResolvePins(nil, nil)
	if got == nil {
		t.Fatalf("ResolvePins should return a non-nil map even for nil inputs")
	}
	if len(got) != 0 {
		t.Errorf("want empty map, got %v", got)
	}
}

// TestResolvePins_EmptyOwnerIgnored — defensive: an empty-string owner
// in PinAssignments is meaningless. ResolvePins drops it rather than
// emit a bead attached to an empty codename.
func TestResolvePins_EmptyOwnerIgnored(t *testing.T) {
	beadToWork := map[string][]string{"kerf-a": {"alpha"}}
	pins := map[string]string{"kerf-b": ""}
	resolved := ResolvePins(beadToWork, pins)
	if _, present := resolved["kerf-b"]; present {
		t.Errorf("empty owner should be dropped; got %v", resolved["kerf-b"])
	}
	// Unrelated entries untouched.
	if got := resolved["kerf-a"]; len(got) != 1 || got[0] != "alpha" {
		t.Errorf("unrelated entry mangled; got %v", got)
	}
}

// TestInput_DriftResultPassthrough — a populated DriftResult does not
// affect BeadSource emission. The field is consumed by warning
// detectors (Plan 009 / Bead 4) and the next-headline (Plan 009 /
// Bead 11b), not by the bead/cleanup paths.
func TestInput_DriftResultPassthrough(t *testing.T) {
	in := Input{
		AllBeads: []beads.Bead{
			{ID: "kerf-a", Title: "regular", Status: "open"},
		},
		QueueEntries: []queue.Entry{{Codename: "alpha", Score: 5.0}},
		BeadToWork: map[string][]string{
			"kerf-a": {"alpha"},
		},
		DriftResult: drift.Diff{
			New:              []string{"kerf-new-1", "kerf-new-2"},
			ClosedExternally: []string{"kerf-c-1"},
		},
	}
	items := BeadSource(in)
	if len(items) != 1 || *items[0].BeadID != "kerf-a" {
		t.Errorf("DriftResult should not influence BeadSource; got %+v", items)
	}
}

// TestCleanup_NoSpuriousWorkNoAttachedBeads — a work whose resolved
// filter matches at least one bead must NOT emit the
// work_no_attached_beads cleanup. This guards the B:F4 regression: when
// the bead store has matching beads but BeadSource was joining on
// b.Epic="" (empty), the cleanup detector also using bead.Epic would
// spuriously fire. The detector reads ForWorkWithFilter directly, so the
// bug pre-dated B3, but the contract is sanity-checked here.
func TestCleanup_NoSpuriousWorkNoAttachedBeads(t *testing.T) {
	work := &spec.SpecYAML{
		Codename:   "token-refresh",
		Status:     "implement",
		BeadFilter: &beads.Filter{Label: "work:{codename}"},
	}
	in := Input{
		Works: []*spec.SpecYAML{work},
		AllBeads: []beads.Bead{
			{ID: "kerf-x", Title: "x", Status: "open", Labels: []string{"work:token-refresh"}},
		},
	}
	for _, d := range NewCleanupDetectors(nil) {
		items := d.Detect(in)
		for _, it := range items {
			if it.Title == "no attached beads" {
				t.Errorf("spurious work_no_attached_beads emitted for work with matched bead: %+v", it)
			}
		}
	}
}
