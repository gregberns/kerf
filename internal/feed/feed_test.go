package feed

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
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
