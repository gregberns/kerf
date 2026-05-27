package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gregberns/kerf/internal/beads"
	"github.com/gregberns/kerf/internal/drift"
	"github.com/gregberns/kerf/internal/spec"
	"github.com/gregberns/kerf/internal/testutil"
)

func TestComputePassStatus(t *testing.T) {
	statusValues := []string{"breakdown", "dispatch", "implement", "review", "squared"}

	tests := []struct {
		name          string
		currentStatus string
		passStatus    string
		want          string
	}{
		{"past pass is done", "implement", "breakdown", "done"},
		{"current pass is active", "implement", "implement", "active"},
		{"future pass is pending", "implement", "review", "pending"},
		{"first pass active", "breakdown", "breakdown", "active"},
		{"last pass active", "squared", "squared", "active"},
		{"all done when past terminal", "finalized", "squared", "done"},
		{"unknown pass status", "implement", "nonexistent", "unknown"},
		{"first pass done when second active", "dispatch", "breakdown", "done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computePassStatus(statusValues, tt.currentStatus, tt.passStatus)
			if got != tt.want {
				t.Errorf("computePassStatus(%v, %q, %q) = %q, want %q",
					statusValues, tt.currentStatus, tt.passStatus, got, tt.want)
			}
		})
	}
}

func TestComputePassStatus_EmptyStatusValues(t *testing.T) {
	got := computePassStatus([]string{}, "anything", "anything")
	if got != "unknown" {
		t.Errorf("got %q, want %q", got, "unknown")
	}
}

func TestShowComposableJig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "show-test-proj"

	// Create a work with the implementation jig
	out := captureOutput(t, func() {
		projectFlag = proj
		newJigFlag = "implementation"
		newTitle = "Build parser"
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = ""; newTitle = "" }()
		newCmd.RunE(newCmd, []string{"build-parser"})
	})
	testutil.AssertStringContains(t, out, "Work created: build-parser")

	workDir := filepath.Join(bp, "projects", proj, "build-parser")
	os.WriteFile(filepath.Join(workDir, "SESSION.md"), []byte("# Session\n"), 0644)

	// Advance to dispatch so breakdown is "done"
	captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		statusCmd.RunE(statusCmd, []string{"build-parser", "dispatch"})
	})

	// Now run show
	out = captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		showCmd.RunE(showCmd, []string{"build-parser"})
	})

	testutil.AssertStringContains(t, out, "Pass status:")
	testutil.AssertStringContains(t, out, "done")
	testutil.AssertStringContains(t, out, "active")
	testutil.AssertStringContains(t, out, "pending")
}

func TestShowNonComposableJig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	proj := "show-nocomp-proj"

	// Create a work with the bug jig (not composable)
	captureOutput(t, func() {
		projectFlag = proj
		newJigFlag = "bug"
		newTitle = "Fix crash"
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = ""; newTitle = "" }()
		newCmd.RunE(newCmd, []string{"fix-crash"})
	})

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		showCmd.RunE(showCmd, []string{"fix-crash"})
	})

	// Should NOT contain pass status section for non-composable jigs
	if containsString(out, "Pass status:") {
		t.Error("non-composable jig should not show Pass status section")
	}
}

func TestGetBeadSummary_NoBr(t *testing.T) {
	// Scrub PATH so `br` cannot be resolved; the function must degrade
	// silently with no error (kerf-cz2t: "tool not on PATH" → silent OK,
	// distinct from "tool on PATH but failed" → surfaced error).
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	got, gbErr := getBeadSummary("any-project", "any-work", nil)
	if gbErr != nil {
		t.Fatalf("getBeadSummary: %v", gbErr)
	}
	if got != "" {
		t.Errorf("expected empty string when beads tool unavailable or no beads, got %q", got)
	}
}

// TestShow_BeadsAttached_RendersCounts verifies that getBeadSummary, when the
// configured beads tool (`br`) is available and returns beads, renders a
// "Beads: N total, C closed, O open" summary line. The implementation must
// reach beads via internal/beads.ListNamed (no direct exec.Command shell-out
// in cmd/show.go) — Plan 008 / Bead 1.
func TestShow_BeadsAttached_RendersCounts(t *testing.T) {
	// Beads carry the default filter label "work:<codename>"; the resolved
	// per-work + project filter (nil/nil here) falls through to the built-in
	// default, so all four match and are counted.
	stubBr(t, `[
		{"id":"x-1","status":"open","labels":["work:demo"]},
		{"id":"x-2","status":"closed","labels":["work:demo"]},
		{"id":"x-3","status":"done","labels":["work:demo"]},
		{"id":"x-4","status":"in-progress","labels":["work:demo"]}
	]`)

	got, gbErr := getBeadSummary("any-project", "demo", nil)
	if gbErr != nil { t.Fatalf("getBeadSummary: %v", gbErr) }
	// 4 total: 2 terminal (closed+done) + 2 non-terminal (open, in-progress)
	want := "Beads: 4 total, 2 closed, 2 open"
	if got != want {
		t.Errorf("getBeadSummary = %q, want %q", got, want)
	}
}

// TestShow_WorkCodename_MultiMatch — JSON contract test asserting that
// when a single bead matches the resolved bead_filters of multiple works,
// `kerf next --format=json` emits one record per (bead, work) pair, each
// with a distinct non-null `work_codename`. Exercises the B:F4 fix at
// the caller boundary (cmd/next.go → feed.BeadSource), not only in the
// internal/feed unit tests. See Plan 008 / Bead 3.
func TestShow_WorkCodename_MultiMatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	projectID := "multimatch-proj"
	// Bench-mode layout: WorksDir() == BenchPath/projects/<id> (works
	// live directly under the project dir, no "works/" subdir).
	worksDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	if err := os.MkdirAll(worksDir, 0o755); err != nil {
		t.Fatalf("works dir: %v", err)
	}

	// Two works, each with a label-based filter that targets a shared
	// label on the same bead. The bead is NOT scoped to either work via
	// Epic (that field is empty) — attachment must come purely from the
	// resolved filter join (Plan 008 / Bead 3).
	mkWork := func(codename, label string) {
		dir := filepath.Join(worksDir, codename)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", codename, err)
		}
		s := &spec.SpecYAML{
			Codename:     codename,
			Type:         "feature",
			Project:      spec.Project{ID: projectID},
			Jig:          "implementation",
			JigVersion:   1,
			Status:       "implement",
			StatusValues: []string{"breakdown", "dispatch", "implement", "review", "squared"},
			Created:      time.Now(),
			Updated:      time.Now(),
			BeadFilter:   &beads.Filter{Label: label},
		}
		if err := spec.Write(filepath.Join(dir, "spec.yaml"), s); err != nil {
			t.Fatalf("write spec %s: %v", codename, err)
		}
	}
	mkWork("alpha", "tag:shared")
	mkWork("beta", "tag:shared")

	// Project.yaml stub (post-Plan 008 / B10-code, `kerf next` emits a
	// fatal `no_project_yaml` warning when absent).
	if werr := os.WriteFile(filepath.Join(worksDir, "project.yaml"), []byte("jigs: []\n"), 0o644); werr != nil {
		t.Fatalf("write project.yaml: %v", werr)
	}

	// Stub `br` so beads.List returns exactly one bead carrying both
	// labels — no Epic, no per-work scoping in the bead itself.
	stubBr(t, `[
		{"id":"kerf-multi","title":"shared bead","status":"open","epic":"","labels":["tag:shared"]}
	]`)

	resetNextFlags()
	nextFormat = "json"
	t.Cleanup(resetNextFlags)
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext: %v", err)
	}

	var items []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", err, buf.String())
	}

	// Collect bead items for kerf-multi and their work_codename values.
	got := map[string]bool{}
	for _, it := range items {
		if it["kind"] != "bead" {
			continue
		}
		if id, _ := it["bead_id"].(string); id != "kerf-multi" {
			continue
		}
		wc, ok := it["work_codename"].(string)
		if !ok {
			t.Errorf("bead item must have non-null work_codename; got %+v", it)
			continue
		}
		got[wc] = true
	}

	if !got["alpha"] || !got["beta"] {
		t.Errorf("kerf-multi must appear under both alpha and beta; got %v\nraw: %s", got, buf.String())
	}
	if len(got) != 2 {
		t.Errorf("multi-match should emit exactly 2 items for kerf-multi; got %d distinct work_codenames: %v", len(got), got)
	}
}

// TestShow_BeadToolUnavailable_DegradesGracefully verifies that when the
// beads CLI is not on PATH, getBeadSummary returns "" (no error, no panic,
// no partial output). The caller in runShow then simply omits the line.
func TestShow_BeadToolUnavailable_DegradesGracefully(t *testing.T) {
	// Point PATH at an empty dir so `br` cannot be resolved.
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	got, gbErr := getBeadSummary("any-project", "any-work", nil)
	if gbErr != nil { t.Fatalf("getBeadSummary: %v", gbErr) }
	if got != "" {
		t.Errorf("expected empty summary when br unavailable, got %q", got)
	}
}

// TestShow_CaseSensitiveLabelMatching verifies that the spec-conformant
// case-sensitive bead-attachment path is used by `kerf show` (Plan 008 / B5;
// specs/coordination.md L232). A project bead_filter of
// `subsystem:{codename}` must NOT match a label `Subsystem:bridge` — only
// the exact-case `subsystem:bridge`.
func TestShow_CaseSensitiveLabelMatching(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	projectID := "case-proj"
	codename := "bridge"
	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", projectID)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	// Project-level bead_filter forces label "subsystem:{codename}".
	projectYAML := []byte("bead_filter:\n  label: \"subsystem:{codename}\"\n")
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), projectYAML, 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}

	// Stub br with three beads: one with the exact-case label, one with a
	// mixed-case "Subsystem:bridge" label that must be rejected by the
	// spec's case-sensitive rule, and one unrelated.
	stubBr(t, `[
		{"id":"b-match","status":"open","labels":["subsystem:bridge"]},
		{"id":"b-case","status":"open","labels":["Subsystem:bridge"]},
		{"id":"b-other","status":"closed","labels":["subsystem:other"]}
	]`)

	got, gbErr := getBeadSummary(projectID, codename, nil)
	if gbErr != nil { t.Fatalf("getBeadSummary: %v", gbErr) }
	want := "Beads: 1 total, 0 closed, 1 open"
	if got != want {
		t.Errorf("getBeadSummary = %q, want %q (case-sensitive match must reject 'Subsystem:bridge')", got, want)
	}
}

// ─── Attached beads block (Plan 009 / B7) ───────────────────────────────────

// mkShowSpec builds a minimal *spec.SpecYAML sufficient for getAttachedBeadsBlock.
func mkShowSpec(projectID, codename string, perWork *beads.Filter, pinned []string) *spec.SpecYAML {
	return &spec.SpecYAML{
		Codename:    codename,
		Type:        "feature",
		Status:      "implement",
		Project:     spec.Project{ID: projectID},
		Jig:         "implementation",
		JigVersion:  1,
		Created:     time.Now(),
		Updated:     time.Now(),
		BeadFilter:  perWork,
		PinnedBeads: pinned,
	}
}

// TestShow_AttachedBeads_BlockRenders verifies the basic block: three attached
// beads (two open, one closed) → header counts match, all three lines present.
// Plan 009 / B7.
func TestShow_AttachedBeads_BlockRenders(t *testing.T) {
	stubBr(t, `[
		{"id":"hk-001","title":"wire retry","status":"open","labels":["work:demo"]},
		{"id":"hk-002","title":"extract parser","status":"open","labels":["work:demo"]},
		{"id":"hk-003","title":"scaffold adapter","status":"closed","labels":["work:demo"]}
	]`)

	s := mkShowSpec("show-attached-proj", "demo", nil, nil)
	got, gbErr := getAttachedBeadsBlock(s)
	if gbErr != nil { t.Fatalf("getAttachedBeadsBlock: %v", gbErr) }

	testutil.AssertStringContains(t, got, "Attached beads (2 open / 1 closed):")
	testutil.AssertStringContains(t, got, "hk-001")
	testutil.AssertStringContains(t, got, "wire retry")
	testutil.AssertStringContains(t, got, "hk-002")
	testutil.AssertStringContains(t, got, "extract parser")
	testutil.AssertStringContains(t, got, "hk-003")
	testutil.AssertStringContains(t, got, "scaffold adapter")
	// Open before closed: hk-001 appears before hk-003.
	if i1, i3 := strings.Index(got, "hk-001"), strings.Index(got, "hk-003"); i1 < 0 || i3 < 0 || i1 > i3 {
		t.Errorf("open beads should sort before closed; got:\n%s", got)
	}
}

// TestShow_AttachedBeads_PinnedAnnotated verifies that a pinned bead NOT
// matching the filter still appears in the block, annotated "(pinned)".
// Plan 009 / B7.
func TestShow_AttachedBeads_PinnedAnnotated(t *testing.T) {
	stubBr(t, `[
		{"id":"hk-100","title":"matches filter","status":"open","labels":["work:pin-test"]},
		{"id":"hk-200","title":"pinned but unmatched","status":"open","labels":["unrelated:label"]}
	]`)

	s := mkShowSpec("show-pinned-proj", "pin-test", nil, []string{"hk-200"})
	got, gbErr := getAttachedBeadsBlock(s)
	if gbErr != nil { t.Fatalf("getAttachedBeadsBlock: %v", gbErr) }

	testutil.AssertStringContains(t, got, "Attached beads (2 open / 0 closed):")
	testutil.AssertStringContains(t, got, "hk-100")
	testutil.AssertStringContains(t, got, "hk-200")
	testutil.AssertStringContains(t, got, "pinned but unmatched")
	testutil.AssertStringContains(t, got, "(pinned)")
	// The pinned annotation must be on the hk-200 line.
	lines := strings.Split(got, "\n")
	for _, ln := range lines {
		if strings.Contains(ln, "hk-200") && !strings.Contains(ln, "(pinned)") {
			t.Errorf("hk-200 line must carry (pinned); got: %q", ln)
		}
		if strings.Contains(ln, "hk-100") && strings.Contains(ln, "(pinned)") {
			t.Errorf("hk-100 (filter-matched, not in PinnedBeads) must NOT carry (pinned); got: %q", ln)
		}
	}
}

// TestShow_AttachedBeads_BeadToolUnavailable verifies that when `br` is not on
// PATH, the block is omitted silently (no error, no panic, empty string).
// Plan 009 / B7.
func TestShow_AttachedBeads_BeadToolUnavailable(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	s := mkShowSpec("show-nobr-proj", "any-work", nil, []string{"would-pin"})
	got, gbErr := getAttachedBeadsBlock(s)
	if gbErr != nil { t.Fatalf("getAttachedBeadsBlock: %v", gbErr) }
	if got != "" {
		t.Errorf("expected empty block when br unavailable, got %q", got)
	}
}

// TestShow_AttachedBeads_ClosedExternallyDriftMarker verifies that a bead which
// is open in the baseline snapshot but closed in the current bead store renders
// with `! closed externally since last triage`. Plan 009 / B7.
//
// Requires a git repo + project-identifier so drift.CachePath resolves to a
// real path that drift.Read can find.
func TestShow_AttachedBeads_ClosedExternallyDriftMarker(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	projectID := "drift-proj"
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatalf("mkdir .kerf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(projectID), 0o644); err != nil {
		t.Fatalf("write project-identifier: %v", err)
	}

	// Seed a baseline snapshot in which hk-901 is OPEN.
	baseline := drift.Capture(
		[]beads.Bead{
			{ID: "hk-901", Title: "guard idempotency", Status: "open", Labels: []string{"work:bridge"}},
		},
		nil,
	)
	cachePath := filepath.Join(repo, ".kerf", "sync-cache.json")
	if err := drift.Write(cachePath, baseline); err != nil {
		t.Fatalf("drift.Write: %v", err)
	}

	// Current bead store: same bead but now CLOSED.
	stubBr(t, `[
		{"id":"hk-901","title":"guard idempotency","status":"closed","labels":["work:bridge"]}
	]`)

	s := mkShowSpec(projectID, "bridge", nil, nil)
	got, gbErr := getAttachedBeadsBlock(s)
	if gbErr != nil { t.Fatalf("getAttachedBeadsBlock: %v", gbErr) }

	testutil.AssertStringContains(t, got, "hk-901")
	testutil.AssertStringContains(t, got, "! closed externally since last triage")
	// Counts: closed=1, open=0.
	testutil.AssertStringContains(t, got, "Attached beads (0 open / 1 closed):")
}

// TestShow_AttachedBeads_DeletedBeadStillRenders verifies that a bead present
// in the baseline but absent from the current bead store still appears in the
// block — using the baseline title — with a `! deleted since last triage`
// marker. Beads only disappear from this block when --ack advances the
// baseline. Plan 009 / B7.
func TestShow_AttachedBeads_DeletedBeadStillRenders(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	projectID := "drift-del-proj"
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatalf("mkdir .kerf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(projectID), 0o644); err != nil {
		t.Fatalf("write project-identifier: %v", err)
	}

	// Baseline: two beads attached to work "gateway"; one will be deleted.
	baseline := drift.Capture(
		[]beads.Bead{
			{ID: "hk-700", Title: "still here", Status: "open", Labels: []string{"work:gateway"}},
			{ID: "hk-701", Title: "vanished bead", Status: "open", Labels: []string{"work:gateway"}},
		},
		map[string][]string{
			"hk-700": {"gateway"},
			"hk-701": {"gateway"},
		},
	)
	if err := drift.Write(filepath.Join(repo, ".kerf", "sync-cache.json"), baseline); err != nil {
		t.Fatalf("drift.Write: %v", err)
	}

	// Current store: hk-701 is gone.
	stubBr(t, `[
		{"id":"hk-700","title":"still here","status":"open","labels":["work:gateway"]}
	]`)

	s := mkShowSpec(projectID, "gateway", nil, nil)
	got, gbErr := getAttachedBeadsBlock(s)
	if gbErr != nil { t.Fatalf("getAttachedBeadsBlock: %v", gbErr) }

	testutil.AssertStringContains(t, got, "hk-700")
	testutil.AssertStringContains(t, got, "hk-701")
	testutil.AssertStringContains(t, got, "vanished bead")
	testutil.AssertStringContains(t, got, "! deleted since last triage")
}

// ─── --compact + Pass N → Output rendering (Plan 020 / kerf-85a) ──────────

// TestShow_PassOutputLines_RenderedInDefault verifies that the default render
// emits one `Pass N: <name> → Output: NN-<file>.md` line per declared pass,
// per specs/jig-system.md §"Surfacing Pass Filenames" and specs/commands.md
// §`kerf show`. Process passes with no `output:` declared render `(none)`.
func TestShow_PassOutputLines_RenderedInDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	proj := "show-pass-lines-proj"

	captureOutput(t, func() {
		projectFlag = proj
		newJigFlag = "implementation"
		newTitle = "Pass lines render"
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = ""; newTitle = "" }()
		newCmd.RunE(newCmd, []string{"pass-lines"})
	})

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		showCmd.RunE(showCmd, []string{"pass-lines"})
	})

	// One line per pass in the implementation jig.
	testutil.AssertStringContains(t, out, "Passes:")
	testutil.AssertStringContains(t, out, "Pass 1: Breakdown → Output: 01-breakdown.md")
	testutil.AssertStringContains(t, out, "Pass 2: Dispatch → Output: 02-dispatch.md")
	// Process pass with no declared output → (none).
	testutil.AssertStringContains(t, out, "Pass 3: Implement → Output: (none)")
	testutil.AssertStringContains(t, out, "Pass 4: Verify → Output: 03-verify.md")
}

// TestShow_Compact_RendersFourLinesPlusBeadFilter verifies the --compact
// rendering per specs/commands.md §`kerf show` "--compact output". Output:
//
//	{codename}  status: {current} → next: {next-pass}
//	bead_filter: {value or (none)}
//	files:       {n} in work directory
//	last session: ...
func TestShow_Compact_RendersFourLinesPlusBeadFilter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	proj := "show-compact-proj"

	captureOutput(t, func() {
		projectFlag = proj
		newJigFlag = "implementation"
		newTitle = "Compact render"
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = ""; newTitle = "" }()
		newCmd.RunE(newCmd, []string{"compact-demo"})
	})

	out := captureOutput(t, func() {
		projectFlag = proj
		showCompactFlag = true
		defer func() { projectFlag = ""; showCompactFlag = false }()
		showCmd.RunE(showCmd, []string{"compact-demo"})
	})

	// First line: codename, current status, next-pass name.
	testutil.AssertStringContains(t, out, "compact-demo  status: breakdown → next: Dispatch")
	// bead_filter slot must always be rendered (kerf-3ac contract); no
	// bead_filter was set on creation, so the literal reads "(none)".
	testutil.AssertStringContains(t, out, "bead_filter: (none)")
	testutil.AssertStringContains(t, out, "files:")
	testutil.AssertStringContains(t, out, "in work directory")
	testutil.AssertStringContains(t, out, "last session:")

	// Compact form must omit the verbose sections.
	if strings.Contains(out, "Passes:") {
		t.Errorf("compact form must omit per-pass output list; got:\n%s", out)
	}
	if strings.Contains(out, "Files:") {
		t.Errorf("compact form must omit the verbose Files: tree; got:\n%s", out)
	}
	if strings.Contains(out, "Pass status:") {
		t.Errorf("compact form must omit Pass status block; got:\n%s", out)
	}
}

// TestShow_Compact_PreservesBeadFilterLine verifies that when a work has an
// explicit bead_filter, the --compact rendering surfaces the literal value
// (not "(none)"). Guards against regressing kerf-3ac's always-emit contract.
func TestShow_Compact_PreservesBeadFilterLine(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	proj := "show-compact-filter-proj"

	captureOutput(t, func() {
		projectFlag = proj
		newJigFlag = "implementation"
		newTitle = "Filter render"
		newType = ""
		newBeadFilter = "label=subsystem:bridge"
		defer func() {
			projectFlag = ""
			newJigFlag = ""
			newTitle = ""
			newBeadFilter = ""
		}()
		newCmd.RunE(newCmd, []string{"filter-demo"})
	})

	out := captureOutput(t, func() {
		projectFlag = proj
		showCompactFlag = true
		defer func() { projectFlag = ""; showCompactFlag = false }()
		showCmd.RunE(showCmd, []string{"filter-demo"})
	})

	testutil.AssertStringContains(t, out, "bead_filter: label=subsystem:bridge")
}

// TestShow_AttachedBeads_NoAttachments_OmitsBlock verifies that when the work
// has zero attached beads and zero pinned beads, the block is omitted (the
// `Bead status` line above already covers the empty case). Plan 009 / B7.
func TestShow_AttachedBeads_NoAttachments_OmitsBlock(t *testing.T) {
	stubBr(t, `[
		{"id":"hk-x","title":"unrelated","status":"open","labels":["unrelated:label"]}
	]`)

	s := mkShowSpec("show-empty-proj", "lonely", nil, nil)
	got, gbErr := getAttachedBeadsBlock(s)
	if gbErr != nil { t.Fatalf("getAttachedBeadsBlock: %v", gbErr) }
	if got != "" {
		t.Errorf("expected empty block when no beads attach to work, got %q", got)
	}
}
