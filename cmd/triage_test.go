package cmd

// Tests for `kerf triage` (Plan 009 / B8).
//
// Coverage matches plans/009_triage/beads.md §B8 "Tests" deliverables:
//   - Each section renders correctly against a seeded fixture.
//   - --resolved exit codes: clean=0, uninitialized=1, stuck=2, progress=3.
//   - --ack rewrites the cache file; subsequent --resolved returns 0.
//   - JSON shape matches the spec (header object + item stream + summary).
//   - --kind filters sections.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/drift"
	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/testutil"
)

// resetTriageFlags restores the package-level flag variables between
// tests so cross-test ordering cannot leak state.
func resetTriageFlags() {
	triageResolved = false
	triageAck = false
	triageKinds = nil
	triageFormat = "text"
	triageLastExitCode = 0
}

// withNoExitHook swaps the process-exit hook for a recorder so tests can
// observe the requested exit code without terminating the test binary.
func withNoExitHook(t *testing.T) *int {
	t.Helper()
	orig := triageExitFn
	got := 0
	triageExitFn = func(code int) { got = code }
	t.Cleanup(func() { triageExitFn = orig })
	return &got
}

// setupTriageProject wires up a bench-mode project with the given works
// (codename + bead_filter) and writes a project.yaml stub so triage does
// not short-circuit on not_initialized. Returns the project ID.
func setupTriageProject(t *testing.T, codenameToFilter map[string]string, pinnedByWork map[string][]string) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projectID := "triage-test-proj"
	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte("jigs: []\n"), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}
	for cn, lbl := range codenameToFilter {
		dir := filepath.Join(projDir, cn)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir work: %v", err)
		}
		s := &spec.SpecYAML{
			Codename:     cn,
			Type:         "feature",
			Project:      spec.Project{ID: projectID},
			Jig:          "implementation",
			JigVersion:   1,
			Status:       "implement",
			StatusValues: []string{"breakdown", "dispatch", "implement", "review", "squared"},
			Created:      time.Now(),
			Updated:      time.Now(),
			BeadFilter:   &beads.Filter{Label: lbl},
			PinnedBeads:  pinnedByWork[cn],
		}
		if err := spec.Write(filepath.Join(dir, "spec.yaml"), s); err != nil {
			t.Fatalf("write spec: %v", err)
		}
	}
	return projectID
}

// runTriageCapturing invokes the triage cobra command and captures
// stdout, returning the buffer contents and any RunE error.
func runTriageCapturing(t *testing.T) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	triageCmd.SetOut(&buf)
	t.Cleanup(func() { triageCmd.SetOut(nil) })
	err := runTriage(triageCmd)
	return buf.String(), err
}

// TestTriage_NotInitialized — project.yaml absent ⇒ exit-1 path + the
// `not_initialized` JSON kind (or a text equivalent).
func TestTriage_NotInitialized(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	resetTriageFlags()
	t.Cleanup(resetTriageFlags)
	projectFlag = "bare-proj"
	t.Cleanup(func() { projectFlag = "" })

	out, err := runTriageCapturing(t)
	if err == nil {
		t.Fatal("expected error when project.yaml is absent")
	}
	testutil.AssertStringContains(t, out, "project not initialized")
}

func TestTriage_NotInitialized_JSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	resetTriageFlags()
	triageFormat = "json"
	t.Cleanup(resetTriageFlags)
	projectFlag = "bare-proj"
	t.Cleanup(func() { projectFlag = "" })

	out, err := runTriageCapturing(t)
	if err == nil {
		t.Fatal("expected error")
	}
	var payload map[string]string
	if jerr := json.Unmarshal([]byte(out), &payload); jerr != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", jerr, out)
	}
	if payload["kind"] != "not_initialized" {
		t.Errorf("kind = %q, want not_initialized", payload["kind"])
	}
}

// TestTriage_ResolvedAndAck_MutuallyExclusive ensures the flag-validation
// check fires before any other work runs.
func TestTriage_ResolvedAndAck_MutuallyExclusive(t *testing.T) {
	resetTriageFlags()
	triageResolved = true
	triageAck = true
	t.Cleanup(resetTriageFlags)

	_, err := runTriageCapturing(t)
	if err == nil || !contains(err.Error(), "mutually exclusive") {
		t.Fatalf("want mutually-exclusive error, got %v", err)
	}
}

// TestTriage_UnknownFormat — guard for the --format error path.
func TestTriage_UnknownFormat(t *testing.T) {
	resetTriageFlags()
	triageFormat = "yaml"
	t.Cleanup(resetTriageFlags)

	_, err := runTriageCapturing(t)
	if err == nil || !contains(err.Error(), "unknown format") {
		t.Fatalf("want unknown-format error, got %v", err)
	}
}

// TestTriage_UnknownKind — guard for the --kind error path.
func TestTriage_UnknownKind(t *testing.T) {
	resetTriageFlags()
	triageKinds = []string{"nonsense"}
	t.Cleanup(resetTriageFlags)

	_, err := runTriageCapturing(t)
	if err == nil || !contains(err.Error(), "unknown triage kind") {
		t.Fatalf("want unknown-kind error, got %v", err)
	}
}

// TestTriage_SectionsRender — three open beads exercising all three
// section kinds against a seeded project. Verifies that each section's
// header and per-bead suggestion line appears in the text output.
func TestTriage_SectionsRender(t *testing.T) {
	projectID := setupTriageProject(t,
		map[string]string{
			"alpha": "subsystem:alpha",
			"beta":  "subsystem:beta",
		},
		nil,
	)

	// Seed a drift baseline that contains a bead which is now closed in
	// the current store → drives the external_close path.
	repoRoot := "" // bench mode — sync cache cannot be written; external_drift will be empty
	_ = repoRoot
	_ = projectID

	// One untriaged (no label match), one multi-matched (both filters
	// match), one normal alpha bead.
	stubBr(t, `[
		{"id":"a-1","title":"alpha-only","status":"open","labels":["subsystem:alpha"]},
		{"id":"m-1","title":"shared","status":"open","labels":["subsystem:alpha","subsystem:beta"]},
		{"id":"u-1","title":"orphan","status":"open","labels":["weird:thing"]}
	]`)

	resetTriageFlags()
	t.Cleanup(resetTriageFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	out, err := runTriageCapturing(t)
	if err != nil {
		t.Fatalf("runTriage: %v", err)
	}
	testutil.AssertStringContains(t, out, "Untriaged beads (1):")
	testutil.AssertStringContains(t, out, "u-1")
	testutil.AssertStringContains(t, out, "Multi-matched beads (1):")
	testutil.AssertStringContains(t, out, "m-1")
	testutil.AssertStringContains(t, out, "matches: alpha, beta")
	testutil.AssertStringContains(t, out, "suggest: kerf pin alpha m-1")
	testutil.AssertStringContains(t, out, "Per-work bead health:")
	testutil.AssertStringContains(t, out, "alpha")
	testutil.AssertStringContains(t, out, "beta")
}

// TestTriage_PinOverridesMultiMatch — a multi-matching bead pinned to one
// work disappears from the multi_matched section.
func TestTriage_PinOverridesMultiMatch(t *testing.T) {
	projectID := setupTriageProject(t,
		map[string]string{
			"alpha": "subsystem:shared",
			"beta":  "subsystem:shared",
		},
		map[string][]string{"alpha": {"m-1"}},
	)

	stubBr(t, `[
		{"id":"m-1","title":"shared","status":"open","labels":["subsystem:shared"]}
	]`)

	resetTriageFlags()
	t.Cleanup(resetTriageFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	out, err := runTriageCapturing(t)
	if err != nil {
		t.Fatalf("runTriage: %v", err)
	}
	if contains(out, "Multi-matched beads") {
		t.Errorf("multi_matched section should not appear when bead is pinned\n%s", out)
	}
}

// TestTriage_JSONShape — verifies the spec's header object + items shape
// for --format=json, plus the summary block beads.md §B8 requires.
func TestTriage_JSONShape(t *testing.T) {
	projectID := setupTriageProject(t,
		map[string]string{"alpha": "subsystem:alpha"},
		nil,
	)
	stubBr(t, `[
		{"id":"a-1","title":"in scope","status":"open","labels":["subsystem:alpha"]},
		{"id":"u-1","title":"orphan","status":"open","labels":["weird:thing"]}
	]`)

	resetTriageFlags()
	triageFormat = "json"
	t.Cleanup(resetTriageFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	out, err := runTriageCapturing(t)
	if err != nil {
		t.Fatalf("runTriage: %v", err)
	}
	var report struct {
		BaselineCapturedAt string `json:"baseline_captured_at"`
		Works              []struct {
			Codename string `json:"codename"`
			Filter   string `json:"filter"`
			Open     int    `json:"open"`
			Closed   int    `json:"closed"`
		} `json:"works"`
		Items   []map[string]any `json:"items"`
		Summary struct {
			Untriaged     int `json:"untriaged"`
			MultiMatched  int `json:"multi_matched"`
			ExternalDrift int `json:"external_drift"`
		} `json:"summary"`
	}
	if jerr := json.Unmarshal([]byte(out), &report); jerr != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", jerr, out)
	}
	if report.Summary.Untriaged != 1 {
		t.Errorf("summary.untriaged = %d, want 1", report.Summary.Untriaged)
	}
	if len(report.Items) == 0 {
		t.Fatalf("items must not be empty\n%s", out)
	}
	// At least one item must be the untriaged bead and carry sub_kind=null.
	foundUntriaged := false
	for _, it := range report.Items {
		if it["kind"] == "untriaged" && it["bead_id"] == "u-1" {
			foundUntriaged = true
			if it["sub_kind"] != nil {
				t.Errorf("untriaged item must carry sub_kind=null, got %v", it["sub_kind"])
			}
		}
	}
	if !foundUntriaged {
		t.Errorf("expected an untriaged item for u-1; items=%+v", report.Items)
	}
	// Per-work health: alpha has 1 open bead.
	if len(report.Works) != 1 || report.Works[0].Codename != "alpha" || report.Works[0].Open != 1 {
		t.Errorf("works = %+v, want [alpha open=1]", report.Works)
	}
}

// TestTriage_KindFilter — --kind=multi_matched suppresses other sections.
func TestTriage_KindFilter(t *testing.T) {
	projectID := setupTriageProject(t,
		map[string]string{"alpha": "subsystem:alpha", "beta": "subsystem:beta"},
		nil,
	)
	stubBr(t, `[
		{"id":"m-1","title":"shared","status":"open","labels":["subsystem:alpha","subsystem:beta"]},
		{"id":"u-1","title":"orphan","status":"open","labels":["weird:thing"]}
	]`)

	resetTriageFlags()
	triageKinds = []string{"multi_matched"}
	t.Cleanup(resetTriageFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	out, err := runTriageCapturing(t)
	if err != nil {
		t.Fatalf("runTriage: %v", err)
	}
	testutil.AssertStringContains(t, out, "Multi-matched beads (1):")
	if contains(out, "Untriaged beads") {
		t.Errorf("--kind=multi_matched must suppress untriaged section\n%s", out)
	}
}

// TestTriage_ResolvedClean — no surfaced items ⇒ exit 0.
func TestTriage_ResolvedClean(t *testing.T) {
	projectID := setupTriageProject(t,
		map[string]string{"alpha": "subsystem:alpha"},
		nil,
	)
	// Only well-matched beads, all attached.
	stubBr(t, `[
		{"id":"a-1","title":"clean","status":"open","labels":["subsystem:alpha"]}
	]`)

	resetTriageFlags()
	triageResolved = true
	t.Cleanup(resetTriageFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	gotCode := withNoExitHook(t)
	_, err := runTriageCapturing(t)
	if err != nil {
		t.Fatalf("runTriage: %v", err)
	}
	if *gotCode != 0 {
		t.Errorf("exit code = %d, want 0 (no items)", *gotCode)
	}
}

// TestTriage_ResolvedStuck_ThenProgress — first --resolved run with
// drift records its count; a second run with the SAME count must exit 2
// (stuck); a third run with a STRICTLY-LOWER count must exit 3
// (progress).
func TestTriage_ResolvedStuck_ThenProgress(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)
	projectID := "stuck-proj"

	// Set up bench-mode project (project.yaml lives in HOME/.kerf/...).
	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte("jigs: []\n"), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}
	// Mark this repo as belonging to the project so CachePath resolves.
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatalf("mkdir repo .kerf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(projectID), 0o644); err != nil {
		t.Fatalf("write project-identifier: %v", err)
	}
	// One work with a strict filter so the untriaged beads stand out.
	workDir := filepath.Join(projDir, "alpha")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work: %v", err)
	}
	s := &spec.SpecYAML{
		Codename:     "alpha",
		Type:         "feature",
		Project:      spec.Project{ID: projectID},
		Jig:          "implementation",
		JigVersion:   1,
		Status:       "implement",
		StatusValues: []string{"breakdown", "dispatch", "implement", "review", "squared"},
		Created:      time.Now(),
		Updated:      time.Now(),
		BeadFilter:   &beads.Filter{Label: "subsystem:alpha"},
	}
	if err := spec.Write(filepath.Join(workDir, "spec.yaml"), s); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	// Two untriaged beads — initial drift count = 2.
	stubBr(t, `[
		{"id":"u-1","title":"orphan one","status":"open","labels":["weird:thing"]},
		{"id":"u-2","title":"orphan two","status":"open","labels":["weird:other"]}
	]`)

	resetTriageFlags()
	triageResolved = true
	t.Cleanup(resetTriageFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	// First run: no prior recorded count → exit 2 (we haven't seen progress yet).
	gotCode := withNoExitHook(t)
	if _, err := runTriageCapturing(t); err == nil {
		t.Fatal("first --resolved with drift should surface a non-nil error path")
	}
	if *gotCode != 2 {
		t.Errorf("first --resolved exit code = %d, want 2 (no prior count)", *gotCode)
	}

	// Second run: same drift count → exit 2 (stuck).
	*gotCode = 0
	if _, err := runTriageCapturing(t); err == nil {
		t.Fatal("second --resolved should surface a non-nil error path")
	}
	if *gotCode != 2 {
		t.Errorf("second --resolved exit code = %d, want 2 (stuck)", *gotCode)
	}

	// Third run: drop one bead — progress.
	stubBr(t, `[
		{"id":"u-1","title":"orphan one","status":"open","labels":["weird:thing"]}
	]`)
	*gotCode = 0
	if _, err := runTriageCapturing(t); err == nil {
		t.Fatal("third --resolved should surface a non-nil error path")
	}
	if *gotCode != 3 {
		t.Errorf("third --resolved exit code = %d, want 3 (progress)", *gotCode)
	}
}

// TestTriage_AckWritesCache — running --ack writes a snapshot to
// .kerf/sync-cache.json; a subsequent --resolved (clean) returns 0.
func TestTriage_AckWritesCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)
	projectID := "ack-proj"

	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte("jigs: []\n"), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatalf("mkdir .kerf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(projectID), 0o644); err != nil {
		t.Fatalf("write project-identifier: %v", err)
	}
	// A work that matches all our beads so nothing is untriaged.
	workDir := filepath.Join(projDir, "alpha")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work: %v", err)
	}
	if err := spec.Write(filepath.Join(workDir, "spec.yaml"), &spec.SpecYAML{
		Codename:     "alpha",
		Type:         "feature",
		Project:      spec.Project{ID: projectID},
		Jig:          "implementation",
		JigVersion:   1,
		Status:       "implement",
		StatusValues: []string{"breakdown", "dispatch", "implement", "review", "squared"},
		Created:      time.Now(),
		Updated:      time.Now(),
		BeadFilter:   &beads.Filter{Label: "subsystem:alpha"},
	}); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	stubBr(t, `[
		{"id":"a-1","title":"clean","status":"open","labels":["subsystem:alpha"]}
	]`)

	resetTriageFlags()
	triageAck = true
	t.Cleanup(resetTriageFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	if _, err := runTriageCapturing(t); err != nil {
		t.Fatalf("runTriage --ack: %v", err)
	}
	cachePath := filepath.Join(repo, ".kerf", "sync-cache.json")
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected sync-cache.json to exist after --ack: %v", err)
	}
	snap, ok, rerr := drift.Read(cachePath)
	if rerr != nil || !ok {
		t.Fatalf("drift.Read after --ack: ok=%v err=%v", ok, rerr)
	}
	if _, present := snap.Beads["a-1"]; !present {
		t.Errorf("baseline should include a-1 after --ack; got %+v", snap.Beads)
	}

	// Now run --resolved → exit 0 (clean).
	resetTriageFlags()
	triageResolved = true
	projectFlag = projectID
	gotCode := withNoExitHook(t)
	if _, err := runTriageCapturing(t); err != nil {
		t.Fatalf("runTriage --resolved: %v", err)
	}
	if *gotCode != 0 {
		t.Errorf("clean --resolved exit code = %d, want 0", *gotCode)
	}
}

// TestTriage_HelpTextOrder — the long help carries the spec-mandated
// fixed-order elements: what triage returns, the three kinds with
// one-liners, the --resolved exit-code matrix including the stuck-loop
// guidance, and --ack as the only baseline-advance command.
func TestTriage_HelpTextOrder(t *testing.T) {
	help := triageLongHelp

	idxReturns := indexOf(help, "What triage returns")
	idxKinds := indexOf(help, "Item kinds:")
	idxUntriaged := indexOf(help, "untriaged")
	idxMulti := indexOf(help, "multi_matched")
	idxExt := indexOf(help, "external_drift")
	idxExit := indexOf(help, "Exit codes")
	idxStuck := indexOf(help, "ask for help")
	idxAck := indexOf(help, "Baseline advancement")

	for _, p := range []struct {
		name string
		v    int
	}{
		{"What triage returns", idxReturns},
		{"Item kinds:", idxKinds},
		{"untriaged", idxUntriaged},
		{"multi_matched", idxMulti},
		{"external_drift", idxExt},
		{"Exit codes", idxExit},
		{"stuck-loop guidance", idxStuck},
		{"Baseline advancement", idxAck},
	} {
		if p.v < 0 {
			t.Errorf("help text missing %q", p.name)
		}
	}
	if !(idxReturns < idxKinds && idxKinds < idxUntriaged && idxUntriaged < idxMulti && idxMulti < idxExt && idxExt < idxExit && idxExit < idxStuck && idxStuck < idxAck) {
		t.Errorf("help text element order violates spec; order indices: returns=%d kinds=%d unt=%d multi=%d ext=%d exit=%d stuck=%d ack=%d",
			idxReturns, idxKinds, idxUntriaged, idxMulti, idxExt, idxExit, idxStuck, idxAck)
	}
}

// TestTriage_HeaderBeadCounts_LabeledByStatus verifies Plan 018 / B6:
// the canonical header line surfaces both 'open' and 'total' counts with
// explicit labels so the previously-ambiguous discrepancy (e.g. 163 vs
// 168 in the harmonik dogfood) is no longer silent. Seeds a store with a
// mix of open, closed, blocked, and in_progress beads so all three
// counts (open, ready, total) carry distinct values.
func TestTriage_HeaderBeadCounts_LabeledByStatus(t *testing.T) {
	projectID := setupTriageProject(t,
		map[string]string{"alpha": "subsystem:alpha"},
		nil,
	)
	// Status mix: 2 open ready, 1 in_progress (open but not ready),
	// 1 blocked (open but not ready), 2 closed (neither open nor ready).
	// Expected: open=4, ready=2, total=6.
	stubBr(t, `[
		{"id":"a-1","title":"o1","status":"open","labels":["subsystem:alpha"]},
		{"id":"a-2","title":"o2","status":"open","labels":["subsystem:alpha"]},
		{"id":"a-3","title":"ip","status":"in_progress","labels":["subsystem:alpha"]},
		{"id":"a-4","title":"bl","status":"blocked","labels":["subsystem:alpha"]},
		{"id":"a-5","title":"c1","status":"closed","labels":["subsystem:alpha"]},
		{"id":"a-6","title":"c2","status":"done","labels":["subsystem:alpha"]}
	]`)

	resetTriageFlags()
	t.Cleanup(resetTriageFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	out, err := runTriageCapturing(t)
	if err != nil {
		t.Fatalf("runTriage: %v", err)
	}
	testutil.AssertStringContains(t, out, "Beads: 4 open · 2 ready · 6 total")
}

// TestTriage_HeaderBeadCounts_JSON verifies the parallel JSON shape from
// Plan 018 / B6.
func TestTriage_HeaderBeadCounts_JSON(t *testing.T) {
	projectID := setupTriageProject(t,
		map[string]string{"alpha": "subsystem:alpha"},
		nil,
	)
	stubBr(t, `[
		{"id":"a-1","title":"o1","status":"open","labels":["subsystem:alpha"]},
		{"id":"a-2","title":"ip","status":"in_progress","labels":["subsystem:alpha"]},
		{"id":"a-3","title":"c1","status":"closed","labels":["subsystem:alpha"]}
	]`)

	resetTriageFlags()
	triageFormat = "json"
	t.Cleanup(resetTriageFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	out, err := runTriageCapturing(t)
	if err != nil {
		t.Fatalf("runTriage: %v", err)
	}
	var report struct {
		BeadCounts struct {
			Open  int `json:"open"`
			Ready int `json:"ready"`
			Total int `json:"total"`
		} `json:"bead_counts"`
	}
	if jerr := json.Unmarshal([]byte(out), &report); jerr != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", jerr, out)
	}
	if report.BeadCounts.Open != 2 || report.BeadCounts.Ready != 1 || report.BeadCounts.Total != 3 {
		t.Errorf("bead_counts = %+v, want {Open:2 Ready:1 Total:3}", report.BeadCounts)
	}
}

// TestTriage_SuggestUntriaged_TierRouting exercises Plan 018 / B1's
// tier-1 vs tier-2 prefix routing:
//   - tier-1 only (codename: / spec:): seeds kerf new / kerf work edit.
//   - tier-2 only (axis:, tag:, subsystem:, …): falls back to kerf pin
//     against the lexicographically-earliest active codename.
//   - mixed: tier-1 wins.
//   - no labels: pin fallback (or "no auto-suggestion" with empty works).
func TestTriage_SuggestUntriaged_TierRouting(t *testing.T) {
	cases := []struct {
		name      string
		labels    []string
		codenames []string
		archived  map[string]bool
		want      string
	}{
		{
			name:      "tier1_codename_no_existing_match_seeds_new",
			labels:    []string{"codename:newfeat"},
			codenames: []string{"alpha", "beta"},
			want:      "kerf new newfeat --bead-filter 'label=codename:newfeat'",
		},
		{
			name:      "tier1_codename_existing_match_extends_filter",
			labels:    []string{"codename:alpha"},
			codenames: []string{"alpha", "beta"},
			want:      "kerf work edit alpha --bead-filter-add 'label=codename:alpha'",
		},
		{
			name:      "tier1_spec_seeds_new",
			labels:    []string{"spec:storage"},
			codenames: []string{"alpha", "beta"},
			want:      "kerf new storage --bead-filter 'label=spec:storage'",
		},
		{
			name:      "tier2_axis_falls_back_to_pin",
			labels:    []string{"axis:perf"},
			codenames: []string{"zulu", "alpha"},
			want:      "kerf pin alpha bead-1",
		},
		{
			name:      "tier2_subsystem_falls_back_to_pin",
			labels:    []string{"subsystem:bridge"},
			codenames: []string{"zulu", "alpha"},
			want:      "kerf pin alpha bead-1",
		},
		{
			name:      "mixed_tier1_wins_over_tier2",
			labels:    []string{"axis:perf", "spec:storage"},
			codenames: []string{"alpha"},
			want:      "kerf new storage --bead-filter 'label=spec:storage'",
		},
		{
			name:      "no_labels_pins_against_first_work",
			labels:    nil,
			codenames: []string{"zulu", "alpha"},
			want:      "kerf pin alpha bead-1",
		},
		{
			name:      "no_labels_no_works_no_auto_suggestion",
			labels:    nil,
			codenames: nil,
			want:      "no auto-suggestion; investigate manually (bead bead-1)",
		},
		{
			name:      "all_tier2_no_works_no_auto_suggestion",
			labels:    []string{"tag:foo", "scope:bar"},
			codenames: nil,
			want:      "no auto-suggestion; investigate manually (bead bead-1)",
		},
		// Plan 018 / B2 — archive-aware codename check. When the proposed
		// `kerf new <codename>` would collide with an archived codename,
		// emit a restore/pin hint instead.
		{
			name:      "tier1_codename_archived_emits_restore_hint",
			labels:    []string{"codename:oldfeat"},
			codenames: []string{"alpha", "beta"},
			archived:  map[string]bool{"oldfeat": true},
			want:      "codename 'oldfeat' is archived — consider 'kerf restore oldfeat' to unarchive, or 'kerf pin <codename> bead-1' to attach this bead to a different live work.",
		},
		{
			name:      "tier1_spec_value_archived_emits_restore_hint",
			labels:    []string{"spec:retired"},
			codenames: []string{"alpha"},
			archived:  map[string]bool{"retired": true},
			want:      "codename 'retired' is archived — consider 'kerf restore retired' to unarchive, or 'kerf pin <codename> bead-1' to attach this bead to a different live work.",
		},
		{
			// Existing live codename takes precedence over archive
			// membership (live works win — extend the filter rather than
			// suggesting restore of an unrelated archive entry).
			name:      "tier1_existing_live_codename_wins_over_archive",
			labels:    []string{"codename:alpha"},
			codenames: []string{"alpha"},
			archived:  map[string]bool{"alpha": true},
			want:      "kerf work edit alpha --bead-filter-add 'label=codename:alpha'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := beads.Bead{ID: "bead-1", Labels: tc.labels}
			got := triageSuggestUntriaged(b, tc.codenames, tc.archived)
			if got != tc.want {
				t.Errorf("suggest: got %q, want %q", got, tc.want)
			}
		})
	}
}

// --- tiny helpers --------------------------------------------------------

func contains(s, sub string) bool {
	return indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	// Simple substring search; avoids pulling strings into the test file's
	// import list when only a couple of tests need it.
	n, m := len(s), len(sub)
	if m == 0 {
		return 0
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}

// TestTriage_AckQuietMode_Text — with --ack, stdout is a single
// `Baseline advanced to <timestamp>.` line; no report sections.
// Spec: specs/commands.md §"kerf triage" steps 7-8 (Plan 018 / B3).
func TestTriage_AckQuietMode_Text(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)
	projectID := "ack-quiet-text"

	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte("jigs: []\n"), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatalf("mkdir .kerf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(projectID), 0o644); err != nil {
		t.Fatalf("write project-identifier: %v", err)
	}
	// Seed an untriaged bead so we can prove its section is suppressed.
	stubBr(t, `[
		{"id":"u-1","title":"orphan","status":"open","labels":["subsystem:nothing"]}
	]`)

	resetTriageFlags()
	triageAck = true
	t.Cleanup(resetTriageFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	out, err := runTriageCapturing(t)
	if err != nil {
		t.Fatalf("runTriage --ack: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected single-line --ack output; got %d lines:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "Baseline advanced to ") || !strings.HasSuffix(lines[0], ".") {
		t.Errorf("expected `Baseline advanced to <ts>.`; got %q", lines[0])
	}
	for _, banned := range []string{"Untriaged beads", "Multi-matched beads", "External changes", "Per-work bead health", "Triage for"} {
		if strings.Contains(out, banned) {
			t.Errorf("--ack output unexpectedly contains %q:\n%s", banned, out)
		}
	}
}

// TestTriage_AckQuietMode_JSON — with --ack --format=json, stdout is
// the one-record summary `{baseline_advanced_at, items_captured}`.
// Spec: specs/commands.md §"kerf triage" step 8 (OQ4 — summary record).
func TestTriage_AckQuietMode_JSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)
	projectID := "ack-quiet-json"

	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte("jigs: []\n"), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatalf("mkdir .kerf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(projectID), 0o644); err != nil {
		t.Fatalf("write project-identifier: %v", err)
	}
	stubBr(t, `[
		{"id":"a-1","title":"one","status":"open","labels":["x"]},
		{"id":"a-2","title":"two","status":"open","labels":["x"]}
	]`)

	resetTriageFlags()
	triageAck = true
	triageFormat = "json"
	t.Cleanup(resetTriageFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	out, err := runTriageCapturing(t)
	if err != nil {
		t.Fatalf("runTriage --ack --format=json: %v", err)
	}
	var got struct {
		BaselineAdvancedAt string `json:"baseline_advanced_at"`
		ItemsCaptured      int    `json:"items_captured"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode --ack --format=json: %v\n%s", err, out)
	}
	if got.BaselineAdvancedAt == "" {
		t.Errorf("baseline_advanced_at empty; got %+v", got)
	}
	if got.ItemsCaptured != 2 {
		t.Errorf("items_captured = %d, want 2", got.ItemsCaptured)
	}
	// And: no embedded `items` / `summary` arrays — purely the summary
	// record, no full report stream.
	if strings.Contains(out, "\"items\":") || strings.Contains(out, "\"summary\":") {
		t.Errorf("--ack JSON should be a summary record only; got:\n%s", out)
	}
}
