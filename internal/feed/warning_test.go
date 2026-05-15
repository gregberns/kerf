package feed

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/drift"
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

// --- untriaged_beads detector (Plan 009 / Bead 4 — renamed from
// unmatched_beads; pin-aware; threshold-free) -----------------------------

// TestUntriagedBeads_FiresOnNonZeroCount — the renamed detector emits a
// single project-level warning whenever the open / unpinned untriaged
// count is > 0. The plan-006 heuristic threshold is gone.
func TestUntriagedBeads_FiresOnNonZeroCount(t *testing.T) {
	bds := makeBeads("subsystem:foo", 3)
	in := Input{
		Works:    []*spec.SpecYAML{workSpec("alpha", nil)},
		AllBeads: bds,
	}
	got := untriagedBeadsDetector(in)
	if len(got) != 1 {
		t.Fatalf("want 1 warning, got %d", len(got))
	}
	w := got[0]
	if w.Kind != KindWarning {
		t.Errorf("kind = %s, want warning", w.Kind)
	}
	if w.Title != WarningKindUntriagedBeads {
		t.Errorf("title = %q, want %q", w.Title, WarningKindUntriagedBeads)
	}
	if w.Title != "untriaged_beads" {
		t.Errorf("title = %q, want literal %q", w.Title, "untriaged_beads")
	}
	if !strings.Contains(w.Action, "subsystem:") {
		t.Errorf("action should surface top prefix 'subsystem:', got %q", w.Action)
	}
	if !strings.Contains(w.Action, "kerf triage") {
		t.Errorf("action should remediate via `kerf triage`, got %q", w.Action)
	}
	if !strings.Contains(w.Reason, "3 beads match no work") {
		t.Errorf("reason = %q", w.Reason)
	}
	if w.WorkCodename != nil || w.BeadID != nil {
		t.Errorf("warning should be project-level (nil work/bead)")
	}
	if w.Score != 0 {
		t.Errorf("score = %v, want 0", w.Score)
	}
}

func TestUntriagedBeads_PinnedBeadIsTriaged(t *testing.T) {
	// One bead matches no work's filter, but it is pinned to "alpha" →
	// untriaged count is 0, no warning.
	in := Input{
		Works:          []*spec.SpecYAML{workSpec("alpha", &beads.Filter{Label: "work:{codename}"})},
		AllBeads:       []beads.Bead{{ID: "leg-1", Status: "open", Labels: []string{"legacy:x"}}},
		PinAssignments: map[string]string{"leg-1": "alpha"},
	}
	if got := untriagedBeadsDetector(in); len(got) != 0 {
		t.Fatalf("pinned bead must not count as untriaged; got %d warnings", len(got))
	}
	// And when the pin is removed, the warning fires again.
	in.PinAssignments = nil
	if got := untriagedBeadsDetector(in); len(got) != 1 {
		t.Fatalf("unpinned + unmatched bead must surface as untriaged; got %d", len(got))
	}
}

func TestUntriagedBeads_QuietWhenAllMatchOrPinned(t *testing.T) {
	in := Input{
		Works:    []*spec.SpecYAML{workSpec("alpha", nil)},
		AllBeads: []beads.Bead{labeled("x", "work:alpha"), labeled("y", "work:alpha")},
	}
	if got := untriagedBeadsDetector(in); len(got) != 0 {
		t.Errorf("all-matched store must be quiet; got %d", len(got))
	}
}

func TestUntriagedBeads_EmptyStoreIsQuiet(t *testing.T) {
	in := Input{Works: []*spec.SpecYAML{workSpec("alpha", nil)}}
	if got := untriagedBeadsDetector(in); len(got) != 0 {
		t.Errorf("expected no warning on empty store, got %d", len(got))
	}
}

func TestUntriagedBeads_OnlyOpenBeadsCount(t *testing.T) {
	// Closed beads are not in the post-open-filter set; they must not
	// inflate the untriaged count.
	bds := []beads.Bead{
		{ID: "a", Status: "open", Labels: []string{"orphan:x"}},
		{ID: "b", Status: "closed", Labels: []string{"orphan:x"}},
	}
	in := Input{
		Works:    []*spec.SpecYAML{workSpec("alpha", nil)},
		AllBeads: bds,
	}
	got := untriagedBeadsDetector(in)
	if len(got) != 1 {
		t.Fatalf("want 1 warning, got %d", len(got))
	}
	if !strings.Contains(got[0].Reason, "1 beads match no work") {
		t.Errorf("only the open bead must count; reason = %q", got[0].Reason)
	}
}

// --- multi_matched detector (Plan 009 / Bead 4) --------------------------

func TestMultiMatched_FiresPerBeadMatchingTwoWorks(t *testing.T) {
	// Two works, both with a filter that matches the same bead.
	bds := []beads.Bead{{ID: "shared", Status: "open", Labels: []string{"work:alpha", "work:beta"}}}
	in := Input{
		Works: []*spec.SpecYAML{
			workSpec("alpha", &beads.Filter{Label: "work:alpha"}),
			workSpec("beta", &beads.Filter{Label: "work:beta"}),
		},
		AllBeads: bds,
	}
	got := multiMatchedBeadDetector(in)
	if len(got) != 1 {
		t.Fatalf("want 1 warning, got %d", len(got))
	}
	w := got[0]
	if !strings.Contains(w.Title, "multi_matched") || !strings.Contains(w.Title, "shared") {
		t.Errorf("title should name kind + bead, got %q", w.Title)
	}
	if !strings.Contains(w.Reason, "alpha") || !strings.Contains(w.Reason, "beta") {
		t.Errorf("reason should list both matching works, got %q", w.Reason)
	}
	if !strings.HasPrefix(w.Action, "kerf pin alpha shared") {
		t.Errorf("action should pin to lex-earliest codename, got %q", w.Action)
	}
}

func TestMultiMatched_PinSuppresses(t *testing.T) {
	bds := []beads.Bead{{ID: "shared", Status: "open", Labels: []string{"work:alpha", "work:beta"}}}
	in := Input{
		Works: []*spec.SpecYAML{
			workSpec("alpha", &beads.Filter{Label: "work:alpha"}),
			workSpec("beta", &beads.Filter{Label: "work:beta"}),
		},
		AllBeads:       bds,
		PinAssignments: map[string]string{"shared": "alpha"},
	}
	if got := multiMatchedBeadDetector(in); len(got) != 0 {
		t.Errorf("pinned bead must not surface as multi_matched; got %d", len(got))
	}
}

func TestMultiMatched_QuietOnSingleWork(t *testing.T) {
	in := Input{
		Works:    []*spec.SpecYAML{workSpec("alpha", &beads.Filter{Label: "work:alpha"})},
		AllBeads: []beads.Bead{labeled("x", "work:alpha")},
	}
	if got := multiMatchedBeadDetector(in); len(got) != 0 {
		t.Errorf("single-work project cannot have multi-match; got %d", len(got))
	}
}

func TestMultiMatched_DeterministicOrder(t *testing.T) {
	bds := []beads.Bead{
		{ID: "z-bead", Status: "open", Labels: []string{"work:alpha", "work:beta"}},
		{ID: "a-bead", Status: "open", Labels: []string{"work:alpha", "work:beta"}},
	}
	in := Input{
		Works: []*spec.SpecYAML{
			workSpec("alpha", &beads.Filter{Label: "work:alpha"}),
			workSpec("beta", &beads.Filter{Label: "work:beta"}),
		},
		AllBeads: bds,
	}
	got := multiMatchedBeadDetector(in)
	if len(got) != 2 {
		t.Fatalf("want 2 warnings, got %d", len(got))
	}
	if !strings.Contains(got[0].Title, "a-bead") || !strings.Contains(got[1].Title, "z-bead") {
		t.Errorf("multi-match warnings must be ordered by bead ID; got titles %q, %q",
			got[0].Title, got[1].Title)
	}
}

// --- external_drift detector (Plan 009 / Bead 4) --------------------------

func TestExternalDrift_FiresPerNonEmptyCategory(t *testing.T) {
	in := Input{
		DriftResult: drift.Diff{
			New:                []string{"a", "b"},
			Deleted:            []string{"c"},
			ClosedExternally:   []string{"d", "e", "f"},
			ReopenedExternally: []string{"g"},
		},
	}
	got := externalDriftDetector(in)
	if len(got) != 4 {
		t.Fatalf("want 4 warnings (one per category), got %d", len(got))
	}
	// Spec-ordered: close → reopen → delete → new.
	wantSubKinds := []string{"external_close", "external_reopen", "external_delete", "external_new"}
	for i, sk := range wantSubKinds {
		if !strings.Contains(got[i].Title, sk) {
			t.Errorf("warning[%d] title = %q, want sub-kind %q", i, got[i].Title, sk)
		}
		if !strings.HasPrefix(got[i].Title, WarningKindExternalDrift+"/") {
			t.Errorf("warning[%d] title should start with %q/, got %q",
				i, WarningKindExternalDrift, got[i].Title)
		}
		if got[i].Action != "kerf triage" {
			t.Errorf("warning[%d] action = %q", i, got[i].Action)
		}
		if got[i].WorkCodename != nil || got[i].BeadID != nil {
			t.Errorf("warning[%d] should be project-level", i)
		}
	}
	if !strings.Contains(got[0].Reason, "3 bead(s)") {
		t.Errorf("close-count reason should say `3 bead(s)`, got %q", got[0].Reason)
	}
}

func TestExternalDrift_QuietOnZeroDiff(t *testing.T) {
	// Zero-value DriftResult — cache absent or first run.
	if got := externalDriftDetector(Input{}); len(got) != 0 {
		t.Errorf("zero DriftResult must be silent; got %d warnings", len(got))
	}
	// Only Changed populated → no external_* warning (Changed belongs to
	// relabelDriftDetector, intentionally not duplicated here).
	in := Input{DriftResult: drift.Diff{Changed: []string{"kerf-a"}}}
	if got := externalDriftDetector(in); len(got) != 0 {
		t.Errorf("Changed-only drift must be silent; got %d", len(got))
	}
}

// --- pin_conflict warning factory (Plan 009 / Bead 4) ---------------------

func TestPinConflictWarning_Shape(t *testing.T) {
	w := PinConflictWarning("kerf-xyz", "alpha", "beta")
	if w.Kind != KindWarning {
		t.Errorf("kind = %s, want warning", w.Kind)
	}
	if !strings.HasPrefix(w.Title, WarningKindPinConflict) {
		t.Errorf("title should start with %q, got %q", WarningKindPinConflict, w.Title)
	}
	if !strings.Contains(w.Title, "kerf-xyz") {
		t.Errorf("title should name the bead, got %q", w.Title)
	}
	if !strings.Contains(w.Reason, "alpha") || !strings.Contains(w.Reason, "beta") {
		t.Errorf("reason should name both works, got %q", w.Reason)
	}
	if w.Action != "kerf pin alpha kerf-xyz" {
		t.Errorf("action should pin to winner, got %q", w.Action)
	}
	if w.WorkCodename != nil || w.BeadID != nil {
		t.Errorf("pin_conflict is project-level")
	}
	if w.Score != 0 {
		t.Errorf("score = %v, want 0", w.Score)
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

func TestNewWarningDetectors_ReturnsAll(t *testing.T) {
	// v1 detectors (Plan 006/B5): unmatched_beads (renamed to
	// untriaged_beads in Plan 009/B4), filter_case_mismatch.
	// Plan 008/B10-code adds corrupt_spec and no_project_yaml.
	// Plan 008/B11-code adds relabel_drift.
	// Plan 009/B4 adds multi_matched and external_drift; renames
	// unmatched_beads → untriaged_beads. Total = 7.
	ds := NewWarningDetectors(&beads.Filter{Label: "work:{codename}"})
	if len(ds) != 7 {
		t.Fatalf("want 7 detectors, got %d", len(ds))
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

// --- corrupt_spec detector (Plan 008 / B10-code) --------------------------

// TestWarning_CorruptSpec_Surfaces verifies that the corrupt_spec detector
// emits one warning per CorruptSpec entry, with the spec-defined field
// shapes (specs/commands.md §"Warning kinds" → `corrupt_spec`).
func TestWarning_CorruptSpec_Surfaces(t *testing.T) {
	in := Input{
		CorruptSpecs: []CorruptSpec{
			{Codename: "bridge", ParseError: "yaml: line 3: mapping values are not allowed in this context"},
			{Codename: "tunnel", ParseError: "invalid created_at timestamp"},
		},
	}
	got := corruptSpecDetector(in)
	if len(got) != 2 {
		t.Fatalf("want 2 warnings, got %d", len(got))
	}
	w := got[0]
	if w.Kind != KindWarning {
		t.Errorf("kind = %s, want warning", w.Kind)
	}
	if w.Score != 0 {
		t.Errorf("score = %v, want 0", w.Score)
	}
	if w.Title != "Corrupt spec: bridge" {
		t.Errorf("title = %q, want %q", w.Title, "Corrupt spec: bridge")
	}
	if w.Action != "kerf show bridge" {
		t.Errorf("action = %q, want %q", w.Action, "kerf show bridge")
	}
	if !strings.Contains(w.Reason, "bridge") {
		t.Errorf("reason missing codename: %q", w.Reason)
	}
	if !strings.Contains(w.Reason, "yaml: line 3") {
		t.Errorf("reason missing parse-error: %q", w.Reason)
	}
	if !strings.Contains(w.Reason, "excluded from this feed") {
		t.Errorf("reason missing exclusion note: %q", w.Reason)
	}
	if w.WorkCodename != nil {
		t.Errorf("WorkCodename = %v, want nil (project-level)", *w.WorkCodename)
	}
	if w.BeadID != nil {
		t.Errorf("BeadID = %v, want nil (project-level)", *w.BeadID)
	}
	// Second entry — independent shape.
	if got[1].Title != "Corrupt spec: tunnel" {
		t.Errorf("title[1] = %q, want %q", got[1].Title, "Corrupt spec: tunnel")
	}
}

// TestWarning_CorruptSpec_NoOpWhenEmpty: no CorruptSpec entries → no warnings.
func TestWarning_CorruptSpec_NoOpWhenEmpty(t *testing.T) {
	in := Input{CorruptSpecs: nil}
	if got := corruptSpecDetector(in); len(got) != 0 {
		t.Errorf("want 0 warnings, got %d", len(got))
	}
}

// --- no_project_yaml detector (Plan 008 / B10-code) -----------------------

// TestWarning_NoProjectYaml_Surfaces verifies that when Input.NoProjectYAML
// is true, the detector emits exactly one warning with the spec-defined
// fields (specs/commands.md §"Warning kinds" → `no_project_yaml`).
func TestWarning_NoProjectYaml_Surfaces(t *testing.T) {
	in := Input{
		ProjectID:     "kerf-demo",
		NoProjectYAML: true,
	}
	got := noProjectYAMLDetector(in)
	if len(got) != 1 {
		t.Fatalf("want 1 warning, got %d", len(got))
	}
	w := got[0]
	if w.Kind != KindWarning {
		t.Errorf("kind = %s, want warning", w.Kind)
	}
	if w.Title != "No project.yaml for 'kerf-demo'" {
		t.Errorf("title = %q", w.Title)
	}
	if w.Action != "kerf init" {
		t.Errorf("action = %q, want %q", w.Action, "kerf init")
	}
	if !strings.Contains(w.Reason, "kerf-demo") || !strings.Contains(w.Reason, "kerf init") {
		t.Errorf("reason = %q (missing project id or remedy)", w.Reason)
	}
	if w.WorkCodename != nil || w.BeadID != nil {
		t.Errorf("expected project-level warning (no WorkCodename/BeadID)")
	}
}

// TestWarning_NoProjectYaml_NoOpWhenFalse: NoProjectYAML false → no warning.
func TestWarning_NoProjectYaml_NoOpWhenFalse(t *testing.T) {
	in := Input{ProjectID: "kerf-demo", NoProjectYAML: false}
	if got := noProjectYAMLDetector(in); len(got) != 0 {
		t.Errorf("want 0 warnings, got %d", len(got))
	}
}

// --- relabel_drift detector (Plan 008 / B11-code) -------------------------

// TestWarning_RelabelDrift_Fires verifies the detector emits one warning
// per bead whose canonical hash differs between baseline and current
// snapshots. The fixture pair below changes one bead's labels between
// snapshots; drift.Compute classifies it under Changed, and the detector
// surfaces it.
func TestWarning_RelabelDrift_Fires(t *testing.T) {
	// Baseline: two beads, both open with their original labels.
	baselineBeads := []beads.Bead{
		{ID: "kerf-a", Status: "open", Title: "first", Labels: []string{"phase-1"}},
		{ID: "kerf-b", Status: "open", Title: "second", Labels: []string{"phase-1"}},
	}
	// Current: kerf-a got a new label (relabeled externally); kerf-b is
	// unchanged. Same IDs, same statuses, same titles, same deps — only
	// the labels on kerf-a differ.
	currentBeads := []beads.Bead{
		{ID: "kerf-a", Status: "open", Title: "first", Labels: []string{"phase-1", "plan-008"}},
		{ID: "kerf-b", Status: "open", Title: "second", Labels: []string{"phase-1"}},
	}
	baseline := drift.Capture(baselineBeads, nil)
	current := drift.Capture(currentBeads, nil)
	closed := map[string]bool{"closed": true, "done": true}
	diff := drift.Compute(baseline, current, closed)

	if len(diff.Changed) != 1 || diff.Changed[0] != "kerf-a" {
		t.Fatalf("fixture invariant: want Changed=[kerf-a], got %v", diff.Changed)
	}

	in := Input{DriftResult: diff}
	got := relabelDriftDetector(in)
	if len(got) != 1 {
		t.Fatalf("want 1 warning, got %d", len(got))
	}
	w := got[0]
	if w.Kind != KindWarning {
		t.Errorf("kind = %s, want warning", w.Kind)
	}
	if w.Score != 0 {
		t.Errorf("score = %v, want 0", w.Score)
	}
	if !strings.Contains(w.Title, "kerf-a") {
		t.Errorf("title should name the drifted bead, got %q", w.Title)
	}
	if !strings.Contains(w.Reason, "kerf-a") {
		t.Errorf("reason should name the drifted bead, got %q", w.Reason)
	}
	if w.Action != "kerf triage" {
		t.Errorf("action = %q, want %q", w.Action, "kerf triage")
	}
	if w.WorkCodename != nil || w.BeadID != nil {
		t.Errorf("expected project-level warning (no WorkCodename/BeadID)")
	}
}

// TestWarning_RelabelDrift_QuietOnEmptyDrift: zero-value DriftResult
// (no in-memory last-seen yet — plan 009 wires the cache) → no warning.
func TestWarning_RelabelDrift_QuietOnEmptyDrift(t *testing.T) {
	in := Input{} // zero-value DriftResult, Changed is nil
	if got := relabelDriftDetector(in); len(got) != 0 {
		t.Errorf("want 0 warnings on empty DriftResult, got %d", len(got))
	}
}

// --- rename audit (Plan 009 / Bead 4) ------------------------------------

// TestRenameAudit_NoUnmatchedBeadsLiteral asserts the plan-006 kind string
// "unmatched_beads" / "unmatched beads" and symbol "unmatchedBeadsDetector"
// no longer appear anywhere in the kerf module tree (Go sources). The
// rename is part of B4's deliverable; this test is the regression gate.
//
// The audit is scoped to the working module checkout (rooted via
// runtime.Caller); .claude/worktrees/ scratch trees and external $GOPATH
// caches are excluded by walking from the repo root and skipping
// directories whose basename starts with "." (which includes
// .claude/worktrees as well as .git).
func TestRenameAudit_NoUnmatchedBeadsLiteral(t *testing.T) {
	// Self-permitted strings: this test file mentions them in commentary
	// and in the needle slice below. We allow ONE file (this one) to
	// contain the literal — every other Go file must be clean.
	const selfBase = "warning_test.go"

	// Walk up from this file to find the module root (look for go.mod).
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("could not find go.mod walking up from " + thisFile)
		}
		root = parent
	}

	needles := []string{
		"unmatched_beads",
		"unmatchedBeadsDetector",
		"UnmatchedBeads",
		"unmatched beads",
		"UnmatchedAbsThreshold",
		"UnmatchedFracThreshold",
	}

	var offenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			// Skip hidden dirs (.git, .claude/worktrees, etc.).
			if base != "." && strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		if d.Name() == selfBase {
			return nil // self-permitted
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		body := string(data)
		for _, n := range needles {
			if strings.Contains(body, n) {
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, fmt.Sprintf("%s contains %q", rel, n))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(offenders) > 0 {
		t.Fatalf("rename audit failed — plan-006 kind/symbol still present in:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}
