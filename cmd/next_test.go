package cmd

// kerf next tests — Plan 006 / B6.
//
// Coverage:
//   - Help text contains the six-element contract in order and does not
//     mention --area.
//   - Empty feed: text emits the one-liner; JSON emits []. (Tested via the
//     renderer helpers to keep CLI plumbing out of the way.)
//   - JSON shape: work_codename and bead_id emit literal null for non-bead
//     items (no omitempty).
//   - Kind filter precedence: --only / --include / --kinds resolve correctly
//     against feed.ResolveKindSelection (the spec contract).
//   - Unknown kind values produce the spec error message.
//   - --area is rejected as an unknown flag (it was dropped in v1).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/drift"
	"github.com/gberns/kerf/internal/feed"
)

// isolatePATH points PATH at an empty tempdir so that real `br` / `bd`
// binaries installed on the developer's machine are not found during cmd-level
// tests. Without this, plan 021's surfaced BEADS_TOOL_ERROR can leak into
// tests that don't stage a fake binary — the original silent-degrade hid the
// problem. Tests that explicitly want a working bead store stage their own
// fake on PATH (or set HOME-rooted state) and don't call this helper.
func isolatePATH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

// resetNextFlags clears the global flag-backed state between tests so a leak
// from one case cannot poison another.
func resetNextFlags() {
	nextOnly = nil
	nextInclude = nil
	nextKinds = ""
	nextFormat = "text"
}

// --- Help text contract -----------------------------------------------------

func TestNextHelp_SixElementContractInOrder(t *testing.T) {
	h := nextLongHelp
	// The six elements per specs/commands.md §"kerf next" → "Help text".
	wanted := []string{
		"ranked feed of things to act on right now", // 1. what it returns
		"Item kinds:",         // 2. item kinds glossary
		"Default action loop", // 3. default loop
		"Filter flags:",       // 4. filter flags
		"Machine output",      // 5. machine output
		"Scoring",             // 6. scoring + pointer
	}
	idx := 0
	for _, w := range wanted {
		pos := strings.Index(h[idx:], w)
		if pos < 0 {
			t.Fatalf("help text missing fragment %q (or out of order). Full help:\n%s", w, h)
		}
		idx += pos + len(w)
	}
}

func TestNextHelp_DoesNotMentionArea(t *testing.T) {
	if strings.Contains(strings.ToLower(nextLongHelp), "--area") {
		t.Fatalf("help text must not mention --area (dropped in v1); got:\n%s", nextLongHelp)
	}
}

func TestNextHelp_PointsToCoordinationMd(t *testing.T) {
	if !strings.Contains(nextLongHelp, "coordination.md") {
		t.Fatalf("help text must reference coordination.md for scoring detail")
	}
}

// --- Flag handling: --area is not a registered flag -------------------------

func TestNextFlags_AreaIsUnknown(t *testing.T) {
	if nextCmd.Flags().Lookup("area") != nil {
		t.Fatalf("--area must not be a registered flag on kerf next (dropped in v1)")
	}
}

func TestNextFlags_ExpectedFlagsRegistered(t *testing.T) {
	for _, name := range []string{"only", "include", "kinds", "format"} {
		if nextCmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s missing from kerf next", name)
		}
	}
}

// --- Empty feed rendering ---------------------------------------------------

func TestRenderNextText_EmptyFeed(t *testing.T) {
	var buf bytes.Buffer
	if err := renderNextText(&buf, nil, nil, driftSummaryCounts{}, false, nil, 0); err != nil {
		t.Fatalf("renderNextText: %v", err)
	}
	got := strings.TrimRight(buf.String(), "\n")
	if got != nextEmptyText {
		t.Fatalf("empty-feed text\n  got:  %q\n  want: %q", got, nextEmptyText)
	}
}

func TestRenderNextJSON_EmptyFeed(t *testing.T) {
	var buf bytes.Buffer
	if err := renderNextJSON(&buf, nil, nil, driftSummaryCounts{}, false); err != nil {
		t.Fatalf("renderNextJSON: %v", err)
	}
	// json.Encoder.Encode appends a newline.
	got := strings.TrimSpace(buf.String())
	if got != "[]" {
		t.Fatalf("empty-feed JSON\n  got:  %q\n  want: %q", got, "[]")
	}
}

// --- JSON shape: null for non-bead optional fields --------------------------

func TestRenderNextJSON_NonBeadEmitsNullFields(t *testing.T) {
	warn := feed.Item{
		Kind:   feed.KindWarning,
		Title:  "untriaged_beads",
		Action: "check bead_filter",
		Reason: "3 beads match no work",
	}
	var buf bytes.Buffer
	if err := renderNextJSON(&buf, nil, []feed.Item{warn}, driftSummaryCounts{}, false); err != nil {
		t.Fatalf("renderNextJSON: %v", err)
	}
	body := buf.String()
	if !strings.Contains(body, `"work_codename": null`) {
		t.Errorf("warning JSON must contain literal `work_codename: null`; body:\n%s", body)
	}
	if !strings.Contains(body, `"bead_id": null`) {
		t.Errorf("warning JSON must contain literal `bead_id: null`; body:\n%s", body)
	}
	// Snake-case field names enforced via feed.Item JSON tags.
	for _, key := range []string{`"kind"`, `"score"`, `"title"`, `"action"`, `"reason"`} {
		if !strings.Contains(body, key) {
			t.Errorf("JSON missing snake_case key %s; body:\n%s", key, body)
		}
	}
}

func TestRenderNextJSON_BeadIncludesIDAndCodename(t *testing.T) {
	wc := "alpha"
	id := "hk-001"
	beadItem := feed.Item{
		Kind:         feed.KindBead,
		Score:        12.5,
		Title:        "wire retry",
		WorkCodename: &wc,
		BeadID:       &id,
	}
	var buf bytes.Buffer
	if err := renderNextJSON(&buf, []feed.Item{beadItem}, nil, driftSummaryCounts{}, false); err != nil {
		t.Fatalf("renderNextJSON: %v", err)
	}
	body := buf.String()
	if !strings.Contains(body, `"work_codename": "alpha"`) {
		t.Errorf("bead JSON missing work_codename: body=\n%s", body)
	}
	if !strings.Contains(body, `"bead_id": "hk-001"`) {
		t.Errorf("bead JSON missing bead_id: body=\n%s", body)
	}
	// Validate parsing too.
	var got []feed.Item
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(got) != 1 || got[0].WorkCodename == nil || *got[0].WorkCodename != "alpha" {
		t.Fatalf("round-trip lost work_codename: %+v", got)
	}
}

// --- Render ordering: beads → cleanups → drift footer → warning stanza ------
// Payload-first per specs/commands.md §"kerf next" → "Default kind selection"
// (Plan 019 / B3 — kerf-c1c).

func TestRenderNextText_PayloadAboveWarnings(t *testing.T) {
	wc := "alpha"
	beadID := "hk-001"
	main := []feed.Item{
		{Kind: feed.KindBead, Score: 10, Title: "do X", WorkCodename: &wc, BeadID: &beadID},
		{Kind: feed.KindCleanup, Score: 5, Title: "stale", WorkCodename: &wc, Reason: "all beads closed", Action: "kerf status alpha next"},
	}
	warnings := []feed.Item{
		{Kind: feed.KindWarning, Title: "untriaged_beads", Action: "check bead_filter"},
	}
	var buf bytes.Buffer
	if err := renderNextText(&buf, main, warnings, driftSummaryCounts{}, false, nil, 0); err != nil {
		t.Fatalf("renderNextText: %v", err)
	}
	body := buf.String()
	wi := strings.Index(body, "warning:")
	bi := strings.Index(body, "1. bead")
	ci := strings.Index(body, "2. clean")
	if wi < 0 || bi < 0 || ci < 0 {
		t.Fatalf("expected warning, bead, cleanup markers in text; got:\n%s", body)
	}
	if !(bi < ci && ci < wi) {
		t.Fatalf("expected order bead < cleanup < warning; positions b=%d c=%d w=%d\n%s", bi, ci, wi, body)
	}
	if !strings.Contains(body, "work: alpha") {
		t.Errorf("expected bead row to include `work: alpha`; got:\n%s", body)
	}
	if !strings.Contains(body, nextFooterTip) {
		t.Errorf("expected footer tip; got:\n%s", body)
	}
}

// --- Storage-drift footer (Plan 017 / B11 — kerf-cgb) ----------------------

func TestRenderNextText_StorageDriftFooter_Silent(t *testing.T) {
	wc := "alpha"
	beadID := "hk-001"
	main := []feed.Item{
		{Kind: feed.KindBead, Score: 10, Title: "do X", WorkCodename: &wc, BeadID: &beadID},
	}
	var buf bytes.Buffer
	if err := renderNextText(&buf, main, nil, driftSummaryCounts{}, false, nil, 0); err != nil {
		t.Fatalf("renderNextText: %v", err)
	}
	body := buf.String()
	if strings.Contains(body, "storage finding") {
		t.Fatalf("storage footer must be silent when count=0; got:\n%s", body)
	}
}

func TestRenderNextText_StorageDriftFooter_PluralAndOrder(t *testing.T) {
	wc := "alpha"
	beadID := "hk-001"
	main := []feed.Item{
		{Kind: feed.KindBead, Score: 10, Title: "do X", WorkCodename: &wc, BeadID: &beadID},
	}
	warnings := []feed.Item{
		{Kind: feed.KindWarning, Title: "untriaged_beads", Action: "check bead_filter"},
	}
	var buf bytes.Buffer
	if err := renderNextText(&buf, main, warnings, driftSummaryCounts{}, false, nil, 3); err != nil {
		t.Fatalf("renderNextText: %v", err)
	}
	body := buf.String()
	expected := "note: 3 storage findings — run 'kerf doctor' for details"
	if !strings.Contains(body, expected) {
		t.Fatalf("expected footer %q; got:\n%s", expected, body)
	}
	// Footer follows warnings, precedes tail-tip.
	wi := strings.Index(body, "warning:")
	si := strings.Index(body, "note: 3 storage findings")
	ti := strings.Index(body, nextFooterTip)
	if !(wi < si && si < ti) {
		t.Fatalf("expected warning < storage-footer < tail-tip; w=%d s=%d t=%d\n%s", wi, si, ti, body)
	}
}

func TestRenderNextText_StorageDriftFooter_Singular(t *testing.T) {
	var buf bytes.Buffer
	if err := renderNextText(&buf, nil, nil, driftSummaryCounts{}, false, nil, 1); err != nil {
		t.Fatalf("renderNextText: %v", err)
	}
	body := buf.String()
	if !strings.Contains(body, "note: 1 storage finding — run 'kerf doctor' for details") {
		t.Fatalf("expected singular form; got:\n%s", body)
	}
	if strings.Contains(body, "1 storage findings") {
		t.Fatalf("singular form regressed to plural; got:\n%s", body)
	}
}

// --- Kind selection precedence (the flag contract) --------------------------

func TestKindSelection_DefaultIncludesAll(t *testing.T) {
	sel, err := feed.ResolveKindSelection(nil, nil, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	for _, k := range feed.KnownKinds() {
		if !sel.Has(k) {
			t.Errorf("default selection missing kind %q", k)
		}
	}
}

func TestKindSelection_OnlyBead(t *testing.T) {
	sel, err := feed.ResolveKindSelection(nil, []string{"bead"}, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !sel.Has(feed.KindBead) {
		t.Errorf("expected bead in selection")
	}
	if sel.Has(feed.KindCleanup) || sel.Has(feed.KindWarning) {
		t.Errorf("--only=bead must exclude other kinds: %+v", sel)
	}
}

func TestKindSelection_OnlyMultipleUnion(t *testing.T) {
	sel, err := feed.ResolveKindSelection(nil, []string{"bead", "cleanup"}, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !sel.Has(feed.KindBead) || !sel.Has(feed.KindCleanup) {
		t.Errorf("--only=bead --only=cleanup must keep both: %+v", sel)
	}
	if sel.Has(feed.KindWarning) {
		t.Errorf("--only must not include warning: %+v", sel)
	}
}

func TestKindSelection_IncludeAddsKind(t *testing.T) {
	sel, err := feed.ResolveKindSelection([]string{"bead", "cleanup"}, nil, []string{"warning"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	for _, k := range feed.KnownKinds() {
		if !sel.Has(k) {
			t.Errorf("--include=warning must add warning back: missing %q", k)
		}
	}
}

func TestKindSelection_KindsReplacesBase(t *testing.T) {
	sel, err := feed.ResolveKindSelection([]string{"bead", "cleanup"}, nil, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !sel.Has(feed.KindBead) || !sel.Has(feed.KindCleanup) {
		t.Errorf("--kinds=bead,cleanup must include both")
	}
	if sel.Has(feed.KindWarning) {
		t.Errorf("--kinds=bead,cleanup must exclude warning")
	}
}

func TestKindSelection_IdempotentRepeats(t *testing.T) {
	sel, err := feed.ResolveKindSelection(nil, []string{"bead", "bead", "bead"}, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !sel.Has(feed.KindBead) || sel.Has(feed.KindCleanup) || sel.Has(feed.KindWarning) {
		t.Errorf("repeats must be idempotent; got: %+v", sel)
	}
}

func TestKindSelection_OnlyWarningProducesOnlyHeader(t *testing.T) {
	sel, err := feed.ResolveKindSelection(nil, []string{"warning"}, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !sel.Has(feed.KindWarning) {
		t.Errorf("--only=warning must include warning")
	}
	if sel.Has(feed.KindBead) || sel.Has(feed.KindCleanup) {
		t.Errorf("--only=warning must exclude bead/cleanup: %+v", sel)
	}
}

func TestKindSelection_UnknownKindErrors(t *testing.T) {
	_, err := feed.ResolveKindSelection([]string{"frog"}, nil, nil)
	if err == nil {
		t.Fatalf("expected error for unknown kind")
	}
	if !strings.Contains(err.Error(), "frog") {
		t.Errorf("error should name the bad kind; got: %v", err)
	}
}

// --- runNext integration: empty project emits the empty-feed one-liner -----

func TestRunNext_EmptyProjectTextOneLiner(t *testing.T) {
	isolatePATH(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Create the bench project dir with no works.
	if err := mkdirp(filepath.Join(tmp, ".kerf", "projects", "test-proj")); err != nil {
		t.Fatal(err)
	}
	resetNextFlags()
	t.Cleanup(resetNextFlags)
	projectFlag = "test-proj"
	t.Cleanup(func() { projectFlag = "" })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext: %v", err)
	}
	got := strings.TrimRight(buf.String(), "\n")
	if got != nextEmptyText {
		t.Fatalf("empty-project text\n  got:  %q\n  want: %q", got, nextEmptyText)
	}
}

func TestRunNext_EmptyProjectJSONIsEmptyArray(t *testing.T) {
	isolatePATH(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := mkdirp(filepath.Join(tmp, ".kerf", "projects", "test-proj")); err != nil {
		t.Fatal(err)
	}
	resetNextFlags()
	nextFormat = "json"
	t.Cleanup(resetNextFlags)
	projectFlag = "test-proj"
	t.Cleanup(func() { projectFlag = "" })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "[]" {
		t.Fatalf("empty-project JSON\n  got:  %q\n  want: %q", got, "[]")
	}
}

// --- runNext: bad --format errors with the spec message --------------------

func TestRunNext_UnknownFormatErrors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := mkdirp(filepath.Join(tmp, ".kerf", "projects", "test-proj")); err != nil {
		t.Fatal(err)
	}
	resetNextFlags()
	nextFormat = "yaml"
	t.Cleanup(resetNextFlags)
	projectFlag = "test-proj"
	t.Cleanup(func() { projectFlag = "" })

	err := runNext(nextCmd)
	if err == nil {
		t.Fatalf("expected error for --format=yaml")
	}
	if !strings.Contains(err.Error(), "unknown format") || !strings.Contains(err.Error(), "yaml") {
		t.Errorf("expected spec message; got: %v", err)
	}
}

func TestRunNext_UnknownKindErrors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := mkdirp(filepath.Join(tmp, ".kerf", "projects", "test-proj")); err != nil {
		t.Fatal(err)
	}
	resetNextFlags()
	nextOnly = []string{"frog"}
	t.Cleanup(resetNextFlags)
	projectFlag = "test-proj"
	t.Cleanup(func() { projectFlag = "" })

	err := runNext(nextCmd)
	if err == nil {
		t.Fatalf("expected error for --only=frog")
	}
	if !strings.Contains(err.Error(), "unknown item kind") || !strings.Contains(err.Error(), "frog") {
		t.Errorf("expected spec unknown-kind message; got: %v", err)
	}
	if !strings.Contains(err.Error(), "bead") || !strings.Contains(err.Error(), "cleanup") || !strings.Contains(err.Error(), "warning") {
		t.Errorf("error should list known kinds; got: %v", err)
	}
}

// --- runNext with a single active work: bead path renders some content -----

func TestRunNext_SingleWorkTextProducesRanking(t *testing.T) {
	isolatePATH(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projDir := filepath.Join(tmp, ".kerf", "projects", "test-proj")
	writeSpecWithAreas(t,
		filepath.Join(projDir, "blue-fox", "spec.yaml"),
		"blue-fox", "test-proj", "research", "Auth rewrite", nil)

	resetNextFlags()
	t.Cleanup(resetNextFlags)
	projectFlag = "test-proj"
	t.Cleanup(func() { projectFlag = "" })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext: %v", err)
	}
	body := buf.String()
	// With no beads in the store, the bead-source produces no items. We may
	// also see a cleanup ("no attached beads") for the work. Either way the
	// output should contain either the cleanup row or the empty-feed line.
	if strings.Contains(body, nextEmptyText) {
		return // valid: no detectors fired and no beads present
	}
	if !strings.Contains(body, "blue-fox") && !strings.Contains(body, "1.") {
		t.Errorf("expected output to mention blue-fox or be empty; got:\n%s", body)
	}
}

// --- runNext: unknown status remains visible -------------------------------

// TestNext_UnknownStatus_RemainsVisible locks Invariant 5 (specs/_index.md
// L75): status is an open string. A work whose status is outside the
// known set must NOT be dropped from the feed. Plan 008 / Bead 4 (kerf-1dm).
func TestNext_UnknownStatus_RemainsVisible(t *testing.T) {
	isolatePATH(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projDir := filepath.Join(tmp, ".kerf", "projects", "test-proj")
	// "wibble-status" is not in the jig's status_values list and is also
	// not the terminal sentinel "finalized". The work must remain visible.
	writeSpecWithAreas(t,
		filepath.Join(projDir, "blue-fox", "spec.yaml"),
		"blue-fox", "test-proj", "wibble-status", "Auth rewrite", nil)

	resetNextFlags()
	nextFormat = "json"
	t.Cleanup(resetNextFlags)
	projectFlag = "test-proj"
	t.Cleanup(func() { projectFlag = "" })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext: %v", err)
	}

	var items []feed.Item
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatalf("decode feed json: %v\nbody=%s", err, buf.String())
	}

	// The work must surface in at least one feed item (typically a cleanup
	// such as "no attached beads") — the bug we are guarding against is
	// silent suppression.
	found := false
	for _, it := range items {
		if it.WorkCodename != nil && *it.WorkCodename == "blue-fox" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("work with unknown status was dropped from the feed; items=%+v", items)
	}
}

// TestNext_FinalizedStatus_IsExcluded is the negative pair for
// TestNext_UnknownStatus_RemainsVisible: the literal terminal sentinel
// "finalized" still suppresses the work. Lock both halves of the contract
// so a future refactor of the status-exclusion block cannot regress either.
func TestNext_FinalizedStatus_IsExcluded(t *testing.T) {
	isolatePATH(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projDir := filepath.Join(tmp, ".kerf", "projects", "test-proj")
	writeSpecWithAreas(t,
		filepath.Join(projDir, "blue-fox", "spec.yaml"),
		"blue-fox", "test-proj", "finalized", "Auth rewrite", nil)

	resetNextFlags()
	nextFormat = "json"
	t.Cleanup(resetNextFlags)
	projectFlag = "test-proj"
	t.Cleanup(func() { projectFlag = "" })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext: %v", err)
	}

	var items []feed.Item
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatalf("decode feed json: %v\nbody=%s", err, buf.String())
	}

	for _, it := range items {
		if it.WorkCodename != nil && *it.WorkCodename == "blue-fox" {
			t.Fatalf("finalized work should be excluded from the feed; items=%+v", items)
		}
	}
}

// --- Plan 009 / Bead 11b — drift-summary headline counters -----------------
//
// Coverage:
//   - Three non-zero categories render the exact spec format.
//   - Zero drift → no line.
//   - JSON drift_summary shape (object form, present only when baseline
//     exists).
//   - Sync cache absent → empty baseline → command does not crash and the
//     bare JSON array contract is preserved.

// TestRenderNextText_DriftSummary_AllThreeCategories asserts the headline
// renders with the spec-exact phrasing for three non-zero counts when a
// baseline is recorded. See specs/commands.md §"kerf next" drift summary.
func TestRenderNextText_DriftSummary_AllThreeCategories(t *testing.T) {
	var buf bytes.Buffer
	summary := driftSummaryCounts{
		Untriaged:     6,
		MultiMatched:  2,
		ExternalDrift: 1,
	}
	if err := renderNextText(&buf, nil, nil, summary, true, nil, 0); err != nil {
		t.Fatalf("renderNextText: %v", err)
	}
	body := buf.String()
	want := "! 6 untriaged beads · ! 2 beads multi-matched · ! 1 bead changed externally — run 'kerf triage'"
	if !strings.Contains(body, want) {
		t.Fatalf("expected drift summary headline\n  want: %q\n  got:  %q", want, body)
	}
}

// TestRenderNextText_DriftSummary_OmitsZeroSegments asserts segments with
// zero counts are dropped from the headline; the surviving segments still
// render in the canonical order untriaged → multi-matched → external.
func TestRenderNextText_DriftSummary_OmitsZeroSegments(t *testing.T) {
	var buf bytes.Buffer
	summary := driftSummaryCounts{Untriaged: 0, MultiMatched: 1, ExternalDrift: 0}
	if err := renderNextText(&buf, nil, nil, summary, true, nil, 0); err != nil {
		t.Fatalf("renderNextText: %v", err)
	}
	body := buf.String()
	if !strings.Contains(body, "! 1 bead multi-matched") {
		t.Fatalf("missing multi-matched segment; got:\n%s", body)
	}
	if strings.Contains(body, "untriaged") {
		t.Fatalf("zero-count untriaged segment must be omitted; got:\n%s", body)
	}
	if strings.Contains(body, "changed externally") {
		t.Fatalf("zero-count external-drift segment must be omitted; got:\n%s", body)
	}
}

// TestRenderNextText_DriftSummary_ZeroDriftNoLine asserts the whole line
// is omitted when all three counts are zero (or when no baseline exists).
func TestRenderNextText_DriftSummary_ZeroDriftNoLine(t *testing.T) {
	var buf bytes.Buffer
	if err := renderNextText(&buf, nil, nil, driftSummaryCounts{}, true, nil, 0); err != nil {
		t.Fatalf("renderNextText: %v", err)
	}
	if strings.Contains(buf.String(), "kerf triage") {
		t.Fatalf("zero drift must not render the summary line; got:\n%s", buf.String())
	}

	buf.Reset()
	// Even with non-zero counters, absent baseline suppresses the headline.
	summary := driftSummaryCounts{Untriaged: 3}
	if err := renderNextText(&buf, nil, nil, summary, false, nil, 0); err != nil {
		t.Fatalf("renderNextText: %v", err)
	}
	if strings.Contains(buf.String(), "untriaged") {
		t.Fatalf("no-baseline must suppress the headline; got:\n%s", buf.String())
	}
}

// TestRenderNextJSON_DriftSummaryShape asserts that JSON output emits a
// top-level `drift_summary` object alongside an `items` array when a
// baseline is recorded.
func TestRenderNextJSON_DriftSummaryShape(t *testing.T) {
	var buf bytes.Buffer
	summary := driftSummaryCounts{Untriaged: 2, MultiMatched: 1, ExternalDrift: 3}
	if err := renderNextJSON(&buf, nil, nil, summary, true); err != nil {
		t.Fatalf("renderNextJSON: %v", err)
	}
	var got struct {
		DriftSummary driftSummaryCounts `json:"drift_summary"`
		Items        []feed.Item        `json:"items"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\nbody=%s", err, buf.String())
	}
	if got.DriftSummary != summary {
		t.Fatalf("drift_summary mismatch\n  got:  %+v\n  want: %+v", got.DriftSummary, summary)
	}
	// Field names must be snake_case.
	for _, key := range []string{`"untriaged"`, `"multi_matched"`, `"external_drift"`} {
		if !strings.Contains(buf.String(), key) {
			t.Errorf("expected snake_case key %s in JSON; body:\n%s", key, buf.String())
		}
	}
}

// TestRenderNextJSON_NoBaselineKeepsArrayShape asserts that without a
// recorded baseline the JSON output remains a bare array — the existing
// contract that `[]` denotes an empty feed is preserved.
func TestRenderNextJSON_NoBaselineKeepsArrayShape(t *testing.T) {
	var buf bytes.Buffer
	if err := renderNextJSON(&buf, nil, nil, driftSummaryCounts{}, false); err != nil {
		t.Fatalf("renderNextJSON: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("no-baseline JSON must be bare array; got:\n%s", buf.String())
	}
}

// TestComputeDriftSummary_FromWarnings asserts the counter derivation:
// untriaged from the untriaged_beads warning's Reason, multi_matched as
// the number of `multi_matched:` warning items, external_drift summing
// the four drift.Diff categories (Changed intentionally excluded).
func TestComputeDriftSummary_FromWarnings(t *testing.T) {
	warnings := []feed.Item{
		{Kind: feed.KindWarning, Title: feed.WarningKindUntriagedBeads, Reason: "5 beads match no work via current filter and are not pinned"},
		{Kind: feed.KindWarning, Title: feed.WarningKindMultiMatchedBead + ": kerf-a"},
		{Kind: feed.KindWarning, Title: feed.WarningKindMultiMatchedBead + ": kerf-b"},
		{Kind: feed.KindWarning, Title: feed.WarningKindExternalDrift + "/external_close"}, // not counted directly
	}
	d := drift.Diff{
		New:                []string{"kerf-n1"},
		Deleted:            []string{"kerf-d1", "kerf-d2"},
		ClosedExternally:   []string{"kerf-c1"},
		ReopenedExternally: nil,
		Changed:            []string{"kerf-x1"}, // must NOT be counted in external_drift
	}
	got := computeDriftSummary(warnings, d)
	want := driftSummaryCounts{Untriaged: 5, MultiMatched: 2, ExternalDrift: 4}
	if got != want {
		t.Fatalf("counters mismatch\n  got:  %+v\n  want: %+v", got, want)
	}
}

// TestRunNext_CacheAbsent_FirstRunDoesNotCrash exercises the end-to-end
// path: with no sync-cache file, the command must still render the
// existing warning block and main feed, and JSON must remain a bare
// array. Bead 11b spec: "Sync cache absent → empty baseline → first-run
// shows everything as new; text rendering still works."
func TestRunNext_CacheAbsent_FirstRunDoesNotCrash(t *testing.T) {
	isolatePATH(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := mkdirp(filepath.Join(tmp, ".kerf", "projects", "test-proj")); err != nil {
		t.Fatal(err)
	}
	resetNextFlags()
	nextFormat = "json"
	t.Cleanup(resetNextFlags)
	projectFlag = "test-proj"
	t.Cleanup(func() { projectFlag = "" })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext: %v", err)
	}
	// No baseline → bare array shape.
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("expected bare array JSON when cache is absent; got:\n%s", buf.String())
	}
}

// --- Near-match advisor (Plan 019 / B7 — kerf-d9f) -------------------------
//
// computeNearMatchHints + renderNextText together implement the
// specs/commands.md §"kerf next" → "Near-match advisor" surface: an `empty`
// cleanup row gains an inline `try: ...` hint when one and only one
// alternate label-shape would lift it out of `empty`.

func TestNearMatchHints_DominantProposalProducesHint(t *testing.T) {
	cn := "bridge"
	wc := cn
	cleanup := feed.Item{
		Kind:         feed.KindCleanup,
		Title:        "no attached beads",
		WorkCodename: &wc,
		Reason:       "resolved bead_filter matches zero beads in the store",
		RankLabel:    "empty",
	}
	// Five beads carrying `subsystem:bridge` (well above the sampler's
	// absolute floor of 3 and ≥ 80% of the candidate matches) → dominant.
	store := []beads.Bead{
		{ID: "b-1", Labels: []string{"subsystem:bridge"}},
		{ID: "b-2", Labels: []string{"subsystem:bridge"}},
		{ID: "b-3", Labels: []string{"subsystem:bridge"}},
		{ID: "b-4", Labels: []string{"subsystem:bridge"}},
		{ID: "b-5", Labels: []string{"subsystem:bridge"}},
	}
	hints := computeNearMatchHints([]feed.Item{cleanup}, store)
	got, ok := hints[cn]
	if !ok {
		t.Fatalf("expected a hint for %q; got map=%v", cn, hints)
	}
	want := "try: kerf work edit bridge --bead-filter 'label=subsystem:bridge'"
	if got != want {
		t.Fatalf("hint mismatch\n  got:  %q\n  want: %q", got, want)
	}
}

func TestNearMatchHints_NoMatchSilent(t *testing.T) {
	cn := "ghost"
	wc := cn
	cleanup := feed.Item{
		Kind:         feed.KindCleanup,
		WorkCodename: &wc,
		RankLabel:    "empty",
	}
	// Store has beads, but none carry any shape matching `ghost`.
	store := []beads.Bead{
		{ID: "b-1", Labels: []string{"subsystem:something-else"}},
		{ID: "b-2", Labels: []string{"area:other"}},
	}
	hints := computeNearMatchHints([]feed.Item{cleanup}, store)
	if _, ok := hints[cn]; ok {
		t.Fatalf("expected no hint when no candidate matches; got %v", hints)
	}
}

func TestNearMatchHints_AmbiguousUnionSilent(t *testing.T) {
	cn := "bridge"
	wc := cn
	cleanup := feed.Item{
		Kind:         feed.KindCleanup,
		WorkCodename: &wc,
		RankLabel:    "empty",
	}
	// Two label shapes carry equal weight (3 each) so the sampler returns
	// ReasonUnion, not ReasonDominant. The advisor must stay silent —
	// ambiguous cases would otherwise present multiple "right answers"
	// to the agent and force a guess.
	store := []beads.Bead{
		{ID: "b-1", Labels: []string{"subsystem:bridge"}},
		{ID: "b-2", Labels: []string{"subsystem:bridge"}},
		{ID: "b-3", Labels: []string{"subsystem:bridge"}},
		{ID: "b-4", Labels: []string{"codename:bridge"}},
		{ID: "b-5", Labels: []string{"codename:bridge"}},
		{ID: "b-6", Labels: []string{"codename:bridge"}},
	}
	hints := computeNearMatchHints([]feed.Item{cleanup}, store)
	if _, ok := hints[cn]; ok {
		t.Fatalf("expected no hint on ambiguous union; got %v", hints)
	}
}

func TestNearMatchHints_UnwiredRowsIgnored(t *testing.T) {
	// `unwired` rows already carry a bootstrap-oriented Action; the
	// advisor is scoped to `empty` per the spec sentence.
	cn := "bridge"
	wc := cn
	cleanup := feed.Item{
		Kind:         feed.KindCleanup,
		WorkCodename: &wc,
		RankLabel:    "unwired",
	}
	store := []beads.Bead{
		{ID: "b-1", Labels: []string{"subsystem:bridge"}},
		{ID: "b-2", Labels: []string{"subsystem:bridge"}},
		{ID: "b-3", Labels: []string{"subsystem:bridge"}},
		{ID: "b-4", Labels: []string{"subsystem:bridge"}},
	}
	hints := computeNearMatchHints([]feed.Item{cleanup}, store)
	if len(hints) != 0 {
		t.Fatalf("advisor must skip unwired rows; got %v", hints)
	}
}

func TestRenderNextText_EmptyRowEmbedsHintInline(t *testing.T) {
	cn := "bridge"
	wc := cn
	main := []feed.Item{{
		Kind:         feed.KindCleanup,
		Title:        "no attached beads",
		WorkCodename: &wc,
		Reason:       "resolved bead_filter matches zero beads in the store",
		Action:       "edit spec.yaml bead_filter or check the project filter",
		RankLabel:    "empty",
	}}
	hints := map[string]string{
		cn: "try: kerf work edit bridge --bead-filter 'label=subsystem:bridge'",
	}
	var buf bytes.Buffer
	if err := renderNextText(&buf, main, nil, driftSummaryCounts{}, false, hints, 0); err != nil {
		t.Fatalf("renderNextText: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "— try: kerf work edit bridge --bead-filter 'label=subsystem:bridge'") {
		t.Fatalf("expected inline `try:` hint on the empty row; got:\n%s", out)
	}
	// Hint replaces the indented action line — only one mention of the
	// suggested command, not duplicated.
	if strings.Contains(out, "edit spec.yaml bead_filter or check the project filter") {
		t.Fatalf("indented action line should be suppressed when a hint is present; got:\n%s", out)
	}
}

func TestRenderNextText_EmptyRowWithoutHintKeepsActionLine(t *testing.T) {
	cn := "bridge"
	wc := cn
	main := []feed.Item{{
		Kind:         feed.KindCleanup,
		Title:        "no attached beads",
		WorkCodename: &wc,
		Reason:       "resolved bead_filter matches zero beads in the store",
		Action:       "edit spec.yaml bead_filter or check the project filter",
		RankLabel:    "empty",
	}}
	var buf bytes.Buffer
	if err := renderNextText(&buf, main, nil, driftSummaryCounts{}, false, nil, 0); err != nil {
		t.Fatalf("renderNextText: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "try:") {
		t.Fatalf("no hint expected when hints map is nil; got:\n%s", out)
	}
	if !strings.Contains(out, "edit spec.yaml bead_filter or check the project filter") {
		t.Fatalf("indented action line must remain when no hint is present; got:\n%s", out)
	}
}

// --- kerf-1d6: bead-tool subprocess failure surfaces as a returned error ----

// TestRunNext_BeadsToolError_ReturnsError exercises kerf-1d6: when the
// configured `tools.tasks` subprocess (here a fake `br` that always fails)
// produces a BEADS_TOOL_ERROR, runNext must return a non-nil error so cobra
// exits 1. Prior to kerf-1d6 the error path was reachable (line 200-206 of
// cmd/next.go), but the bug surfaced as `kerf next` dumping the usage block
// alongside the error — symptom of SilenceUsage being unset. Asserting the
// returned-error contract pins the exit-code path; the SilenceUsage flag is
// covered by the sibling test below.
func TestRunNext_BeadsToolError_ReturnsError(t *testing.T) {
	// Stage a `br` stub that exits non-zero with a JSON-shape error — the
	// real-world `bd`-vs-`br` schema mismatch the dogfood log captured.
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"echo 'JSON error: missing field jsonl_export at line 7 column 1' 1>&2\n" +
		"exit 2\n"
	if err := os.WriteFile(filepath.Join(dir, "br"), []byte(script), 0o755); err != nil {
		t.Fatalf("write stub br: %v", err)
	}
	t.Setenv("PATH", dir)

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := mkdirp(filepath.Join(tmp, ".kerf", "projects", "test-proj")); err != nil {
		t.Fatal(err)
	}
	resetNextFlags()
	t.Cleanup(resetNextFlags)
	projectFlag = "test-proj"
	t.Cleanup(func() { projectFlag = "" })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	err := runNext(nextCmd)
	if err == nil {
		t.Fatalf("expected non-nil error when bead tool subprocess fails; got nil")
	}
	msg := err.Error()
	// Must reference the tool name so scripts/users can identify the
	// misconfiguration source (per AC: "single-line error referencing the
	// configured tool name").
	if !strings.Contains(msg, "br") {
		t.Errorf("error must name the configured tool; got: %v", err)
	}
	// Must surface the BEADS_TOOL_ERROR prefix so the diagnostic trail
	// matches the plan-021 contract (internal/beads/beads.go).
	if !strings.Contains(msg, "BEADS_TOOL_ERROR") {
		t.Errorf("error must include BEADS_TOOL_ERROR diagnostic; got: %v", err)
	}
}

// TestNextCmd_SilenceUsageOnError pins kerf-1d6's secondary fix: when runNext
// returns an error, cobra must NOT dump the usage block. SilenceUsage is the
// flag that prevents that; assert it on the command definition so a future
// refactor cannot silently regress it (the usage dump is what made the
// dogfood report describe the failure as "kerf next dumps help on br
// failure").
func TestNextCmd_SilenceUsageOnError(t *testing.T) {
	if !nextCmd.SilenceUsage {
		t.Fatalf("nextCmd.SilenceUsage must be true so subprocess errors do not trigger a usage dump (kerf-1d6)")
	}
}

// --- kerf-fx5: near-match advisor fires on realistic dogfood corpus -------

// TestRunNext_Fx5_AdvisorFiresOnDogfoodCorpus exercises the dogfood-2026-05-18
// repro that kerf-fx5 was opened for: work `gama` with `bead_filter:
// label=gama` against a store carrying open beads tagged `codename:gama`
// must produce the inline `try:` hint pointing at the prefix-swap clause.
//
// Prior to the kerf-fx5 fix the test failed even at the dogfood-observed
// scale (2 beads): labelsample.ProposeFilter's absolute floor was 3, and the
// 2-bead match produced ReasonBelowFloor / no proposal. The fix introduces a
// caller-tunable floor (ProposeFilterWithFloor) and lowers it to 2 on the
// advisor path while leaving bootstrap-filters strict.
//
// This test exercises the cmd-level integration (not the labelsample unit)
// because the AC explicitly calls out "a test that exercises the repro
// shape, not the unit-test stub data kerf-d9f originally used".
func TestRunNext_Fx5_AdvisorFiresOnDogfoodCorpus(t *testing.T) {
	// Stage two open beads tagged `codename:gama` — the exact corpus the
	// dogfood log captured. The advisor must surface a hint despite the
	// 2-bead count being below the bootstrap-filters floor.
	stubBr(t, `[
		{"id":"x-1","labels":["codename:gama"],"status":"open"},
		{"id":"x-2","labels":["codename:gama"],"status":"open"}
	]`)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projDir := filepath.Join(tmp, ".kerf", "projects", "test-proj")
	if err := mkdirp(projDir); err != nil {
		t.Fatal(err)
	}
	// Work `gama` with `bead_filter: label=gama` — the exact spec shape
	// the dogfood-2026-05-18 repro used.
	specPath := filepath.Join(projDir, "gama", "spec.yaml")
	content := `codename: gama
title: Gama
type: plan
project:
  id: test-proj
jig: plan
jig_version: 1
status: research
status_values: [research, spec, implementing]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
areas: []
bead_filter:
  label: gama
implementation:
  branch: null
  pr: null
  commits: []
`
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	resetNextFlags()
	t.Cleanup(resetNextFlags)
	projectFlag = "test-proj"
	t.Cleanup(func() { projectFlag = "" })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext: %v", err)
	}
	body := buf.String()
	wantClause := "try: kerf work edit gama --bead-filter 'label=codename:gama'"
	if !strings.Contains(body, wantClause) {
		t.Fatalf("expected advisor hint %q in output; got:\n%s", wantClause, body)
	}
}

// TestRunNext_Fx5_NoSpuriousHintOnUnrelatedStore is the negative pair: a
// store containing no labels resembling the work's codename must NOT
// produce a `try:` line. Locks the AC's "no-near-match case still produces
// silence" requirement against floor-relaxation overreach.
func TestRunNext_Fx5_NoSpuriousHintOnUnrelatedStore(t *testing.T) {
	stubBr(t, `[
		{"id":"x-1","labels":["subsystem:auth"],"status":"open"},
		{"id":"x-2","labels":["area:storage"],"status":"open"}
	]`)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projDir := filepath.Join(tmp, ".kerf", "projects", "test-proj")
	if err := mkdirp(projDir); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(projDir, "ghost", "spec.yaml")
	content := `codename: ghost
title: Ghost
type: plan
project:
  id: test-proj
jig: plan
jig_version: 1
status: research
status_values: [research, spec, implementing]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
areas: []
bead_filter:
  label: ghost
implementation:
  branch: null
  pr: null
  commits: []
`
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	resetNextFlags()
	t.Cleanup(resetNextFlags)
	projectFlag = "test-proj"
	t.Cleanup(func() { projectFlag = "" })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext: %v", err)
	}
	body := buf.String()
	if strings.Contains(body, "try:") {
		t.Fatalf("expected no advisor hint when no candidate matches; got:\n%s", body)
	}
}

// --- helpers ---------------------------------------------------------------

func mkdirp(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	// Ensure the project dir always has a project.yaml so kerf next does
	// not emit the fatal `no_project_yaml` warning (Plan 008 / B10-code;
	// specs/commands.md §"Warning kinds"). Tests that want to exercise
	// the warning create the project dir without using this helper.
	if filepath.Base(filepath.Dir(path)) == "projects" {
		py := filepath.Join(path, "project.yaml")
		if _, err := os.Stat(py); os.IsNotExist(err) {
			if werr := os.WriteFile(py, []byte("jigs: []\n"), 0o644); werr != nil {
				return werr
			}
		}
	}
	return nil
}
