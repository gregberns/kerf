package feed

import (
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/spec"
)

// makeBeads constructs n beads, each with a single label. Used to build
// large stores quickly for threshold tests.
func makeBeads(label string, n int) []beads.Bead {
	out := make([]beads.Bead, n)
	for i := 0; i < n; i++ {
		out[i] = beads.Bead{
			ID:     "id-" + label,
			Labels: []string{label},
		}
	}
	return out
}

// labeled is a tiny helper for readability.
func labeled(id string, labels ...string) beads.Bead {
	return beads.Bead{ID: id, Labels: labels}
}

func workSpec(codename string, perWork *beads.Filter) *spec.SpecYAML {
	return &spec.SpecYAML{Codename: codename, BeadFilter: perWork}
}

// --- unmatched_beads detector ---------------------------------------------

func TestUnmatchedBeads_FiresWhenAbsThresholdMet(t *testing.T) {
	// 10 unmatched (subsystem:foo) — equal to abs threshold.
	bds := makeBeads("subsystem:foo", 10)
	in := Input{
		Works:    []*spec.SpecYAML{workSpec("alpha", nil)},
		AllBeads: bds,
	}
	got := unmatchedBeadsDetector(in)
	if len(got) != 1 {
		t.Fatalf("want 1 warning, got %d", len(got))
	}
	w := got[0]
	if w.Kind != KindWarning {
		t.Errorf("kind = %s, want warning", w.Kind)
	}
	if w.Title != "unmatched beads" {
		t.Errorf("title = %q", w.Title)
	}
	if !strings.Contains(w.Action, "subsystem:") {
		t.Errorf("action should surface top prefix 'subsystem:', got %q", w.Action)
	}
	if !strings.Contains(w.Reason, "10 beads match no work") {
		t.Errorf("reason = %q", w.Reason)
	}
	if w.WorkCodename != nil || w.BeadID != nil {
		t.Errorf("warning should be project-level (nil work/bead)")
	}
	if w.Score != 0 {
		t.Errorf("score = %v, want 0", w.Score)
	}
}

func TestUnmatchedBeads_FiresWhenFracThresholdMet(t *testing.T) {
	// 5 unmatched out of 100 = 5% — equals fractional threshold.
	all := append(makeBeads("work:alpha", 95), makeBeads("orphan:x", 5)...)
	in := Input{
		Works:    []*spec.SpecYAML{workSpec("alpha", nil)},
		AllBeads: all,
	}
	got := unmatchedBeadsDetector(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 warning at exactly 5%%, got %d", len(got))
	}
	if !strings.Contains(got[0].Action, "orphan:") {
		t.Errorf("action should mention 'orphan:' prefix, got %q", got[0].Action)
	}
}

func TestUnmatchedBeads_QuietBelowThresholds(t *testing.T) {
	// 4 unmatched out of 100 = 4% — below both thresholds.
	all := append(makeBeads("work:alpha", 96), makeBeads("orphan:x", 4)...)
	in := Input{
		Works:    []*spec.SpecYAML{workSpec("alpha", nil)},
		AllBeads: all,
	}
	got := unmatchedBeadsDetector(in)
	if len(got) != 0 {
		t.Fatalf("expected no warning below thresholds, got %d", len(got))
	}
}

func TestUnmatchedBeads_TopPrefixIsMostCommon(t *testing.T) {
	all := []beads.Bead{
		labeled("a", "subsystem:foo"),
		labeled("b", "subsystem:foo"),
		labeled("c", "subsystem:bar"),
		labeled("d", "work:other"),
		labeled("e", "work:other"),
		labeled("f", "subsystem:baz"),
		labeled("g", "subsystem:baz"),
		labeled("h", "subsystem:baz"),
		labeled("i", "subsystem:baz"),
		labeled("j", "subsystem:baz"),
	}
	// No works → every bead is unmatched.
	in := Input{AllBeads: all}
	got := unmatchedBeadsDetector(in)
	if len(got) != 1 {
		t.Fatalf("want 1 warning, got %d", len(got))
	}
	// subsystem: appears 8 times, work: 2 — subsystem: must win.
	if !strings.Contains(got[0].Action, "subsystem:") {
		t.Errorf("expected top prefix 'subsystem:', action=%q", got[0].Action)
	}
}

func TestUnmatchedBeads_QuietWhenAllMatch(t *testing.T) {
	// 20 beads, all match the default filter for work "alpha".
	bds := make([]beads.Bead, 20)
	for i := range bds {
		bds[i] = labeled("id", "work:alpha")
	}
	in := Input{
		Works:    []*spec.SpecYAML{workSpec("alpha", nil)},
		AllBeads: bds,
	}
	got := unmatchedBeadsDetector(in)
	if len(got) != 0 {
		t.Fatalf("expected no warning, got %d", len(got))
	}
}

func TestUnmatchedBeads_EmptyStoreIsQuiet(t *testing.T) {
	in := Input{Works: []*spec.SpecYAML{workSpec("alpha", nil)}}
	if got := unmatchedBeadsDetector(in); len(got) != 0 {
		t.Errorf("expected no warning on empty store, got %d", len(got))
	}
}

func TestUnmatchedBeads_NineIsBelowAbsAndFrac(t *testing.T) {
	// 9 unmatched, total 9 → frac 100% so fires via frac path.
	// Construct 9 unmatched + 91 matched → 9% > 5%, fires.
	all := append(makeBeads("work:alpha", 91), makeBeads("orphan:x", 9)...)
	in := Input{
		Works:    []*spec.SpecYAML{workSpec("alpha", nil)},
		AllBeads: all,
	}
	if got := unmatchedBeadsDetector(in); len(got) != 1 {
		t.Errorf("want 1 warning at 9%%, got %d", len(got))
	}
	// Now drop to 9 unmatched out of 200 → 4.5% and abs<10: should NOT fire.
	all2 := append(makeBeads("work:alpha", 191), makeBeads("orphan:x", 9)...)
	in2 := Input{
		Works:    []*spec.SpecYAML{workSpec("alpha", nil)},
		AllBeads: all2,
	}
	if got := unmatchedBeadsDetector(in2); len(got) != 0 {
		t.Errorf("want no warning at 4.5%% and abs=9, got %d", len(got))
	}
}

// --- filter_case_mismatch detector ----------------------------------------

func TestFilterCaseMismatch_FiresWhenLowerCaseWouldMatch(t *testing.T) {
	// Project filter expects "Subsystem:" (uppercase) but beads use "subsystem:".
	pf := &beads.Filter{Label: "Subsystem:{codename}"}
	all := []beads.Bead{
		labeled("a", "subsystem:foo"),
		labeled("b", "subsystem:bar"),
	}
	in := Input{
		Works:    []*spec.SpecYAML{workSpec("foo", nil)},
		AllBeads: all,
	}
	det := filterCaseMismatchDetector(pf)
	got := det(in)
	if len(got) != 1 {
		t.Fatalf("want 1 warning, got %d", len(got))
	}
	w := got[0]
	if w.Kind != KindWarning {
		t.Errorf("kind = %s, want warning", w.Kind)
	}
	if w.Title != "bead_filter case-mismatch" {
		t.Errorf("title = %q", w.Title)
	}
	if !strings.Contains(w.Action, "case") {
		t.Errorf("action should mention case, got %q", w.Action)
	}
	if !strings.Contains(w.Reason, "subsystem:") {
		t.Errorf("reason should suggest lower-case prefix, got %q", w.Reason)
	}
	if w.WorkCodename != nil || w.BeadID != nil {
		t.Errorf("warning must be project-level")
	}
}

func TestFilterCaseMismatch_QuietWhenCaseMatchesAlready(t *testing.T) {
	pf := &beads.Filter{Label: "subsystem:{codename}"}
	all := []beads.Bead{labeled("a", "subsystem:foo")}
	in := Input{AllBeads: all}
	det := filterCaseMismatchDetector(pf)
	if got := det(in); len(got) != 0 {
		t.Errorf("expected no warning when prefix already matches, got %d", len(got))
	}
}

func TestFilterCaseMismatch_QuietWhenLowerCaseAlsoMisses(t *testing.T) {
	// "Frob:" → no case-sensitive match AND no case-insensitive match.
	pf := &beads.Filter{Label: "Frob:{codename}"}
	all := []beads.Bead{labeled("a", "subsystem:foo")}
	in := Input{AllBeads: all}
	det := filterCaseMismatchDetector(pf)
	if got := det(in); len(got) != 0 {
		t.Errorf("expected no warning when lower-case also misses, got %d", len(got))
	}
}

func TestFilterCaseMismatch_QuietWithoutProjectFilter(t *testing.T) {
	det := filterCaseMismatchDetector(nil)
	if got := det(Input{AllBeads: []beads.Bead{labeled("a", "x:y")}}); len(got) != 0 {
		t.Errorf("expected no warning with nil project filter")
	}
}

func TestFilterCaseMismatch_QuietOnEmptyStore(t *testing.T) {
	pf := &beads.Filter{Label: "Anything:{codename}"}
	det := filterCaseMismatchDetector(pf)
	if got := det(Input{}); len(got) != 0 {
		t.Errorf("expected no warning on empty store")
	}
}

func TestFilterCaseMismatch_HandlesAnyUnion(t *testing.T) {
	// any: — if ANY clause has a case-sensitive match, no warning.
	pf := &beads.Filter{Any: []beads.Filter{
		{Label: "Subsystem:{codename}"},
		{Label: "work:{codename}"},
	}}
	all := []beads.Bead{labeled("a", "work:foo")}
	in := Input{AllBeads: all}
	det := filterCaseMismatchDetector(pf)
	if got := det(in); len(got) != 0 {
		t.Errorf("expected no warning when one any: clause matches, got %d", len(got))
	}
}

// --- constructor ----------------------------------------------------------

func TestNewWarningDetectors_ReturnsBoth(t *testing.T) {
	ds := NewWarningDetectors(&beads.Filter{Label: "work:{codename}"})
	if len(ds) != 2 {
		t.Fatalf("want 2 detectors, got %d", len(ds))
	}
}

func TestNewWarningDetectors_HealthyStateProducesNoWarnings(t *testing.T) {
	pf := &beads.Filter{Label: "work:{codename}"}
	all := []beads.Bead{
		labeled("a", "work:alpha"),
		labeled("b", "work:alpha"),
	}
	in := Input{
		Works:         []*spec.SpecYAML{workSpec("alpha", nil)},
		AllBeads:      all,
		ProjectFilter: pf,
	}
	for _, d := range NewWarningDetectors(pf) {
		if got := d.Detect(in); len(got) != 0 {
			t.Errorf("healthy state produced warning: %+v", got)
		}
	}
}
