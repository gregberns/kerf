package cmd

// Plan 006 / Bead B8 — E2E coordination tests.
//
// Exercises the bead-attachment + actionable-next rollout end-to-end through
// `runNext` and `detectBeadFilter`. These tests are deliberately black-box:
// they install a stub `br` on PATH so `beads.IsAvailable()` returns true and
// `beads.List()` returns the canned JSON, then drive the CLI entry points
// the same way the user would.
//
// Spec references:
//   - specs/commands.md §"kerf next" — flags, item kinds, JSON shape, empty
//     feed line.
//   - specs/commands.md §"kerf init" step 8 — auto-detect of bead_filter.
//   - specs/coordination.md — bead attachment, filter resolution, cleanup
//     and warning detector contracts.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/config"
)

// writeSpecWithStatus writes a minimal spec.yaml at path. Uses the standard
// plan jig status_values so callers can pick any non-terminal or terminal
// stage by name. bead_filter is omitted (per-work override left unset).
func writeSpecWithStatus(t *testing.T, path, codename, projectID, status string) {
	t.Helper()
	content := `codename: ` + codename + `
type: plan
project:
  id: ` + projectID + `
jig: plan
jig_version: 1
status: ` + status + `
status_values: [problem-space, research, spec, implementing, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
areas: []
implementation:
  branch: null
  pr: null
  commits: []
`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	// Ensure the project also has a project.yaml (post-Plan 008 / B10-code,
	// `kerf next` emits a fatal `no_project_yaml` warning otherwise). Tests
	// that explicitly want the warning override this by NOT calling this
	// helper or by writing the file separately first.
	projectYAML := filepath.Join(filepath.Dir(filepath.Dir(path)), "project.yaml")
	if _, err := os.Stat(projectYAML); os.IsNotExist(err) {
		if werr := os.WriteFile(projectYAML, []byte("jigs: []\n"), 0o644); werr != nil {
			t.Fatalf("write project.yaml: %v", werr)
		}
	}
}

// writeProjectFilterYAML writes a minimal project.yaml carrying a label-based
// bead_filter. The kerf list of active jigs is empty — runNext doesn't need
// it.
func writeProjectFilterYAML(t *testing.T, path, label string) {
	t.Helper()
	content := "jigs: []\nbead_filter:\n  label: \"" + label + "\"\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}
}

// runNextCapture invokes runNext with the given flag setup and returns the
// captured output (the writer registered on nextCmd via SetOut, which is
// where runNext directs all its output).
func runNextCapture(t *testing.T, projectID string, setup func()) string {
	t.Helper()
	resetNextFlags()
	t.Cleanup(resetNextFlags)
	if setup != nil {
		setup()
	}
	prevProject := projectFlag
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = prevProject })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	t.Cleanup(func() { nextCmd.SetOut(nil) })

	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext: %v", err)
	}
	return buf.String()
}

// ----------------------------------------------------------------------------
// Scenario 1 — kerf init auto-detect surfaces a subsystem:* filter and the
// resulting filter scopes bead items in the feed.
// ----------------------------------------------------------------------------

func TestE2E_Plan006_AutoDetectAndBeadFeed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projectID := "auto-detect-proj"

	// Seed two works on the bench.
	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	writeSpecWithStatus(t, filepath.Join(projDir, "auth", "spec.yaml"),
		"auth", projectID, "research")
	writeSpecWithStatus(t, filepath.Join(projDir, "billing", "spec.yaml"),
		"billing", projectID, "research")

	// Seed the bead store with subsystem:* labels matching the codenames.
	stubBr(t, `[
		{"id":"x-1","title":"wire login","status":"open","epic":"auth","labels":["subsystem:auth"]},
		{"id":"x-2","title":"add 2fa","status":"open","epic":"auth","labels":["subsystem:auth"]},
		{"id":"x-3","title":"invoice pdf","status":"open","epic":"billing","labels":["subsystem:billing"]},
		{"id":"x-4","title":"refund flow","status":"open","epic":"billing","labels":["subsystem:billing"]}
	]`)

	// --- Verify the auto-detect heuristic proposes subsystem:{codename}. ---
	// We exercise detectBeadFilter via a Resolver instance pointing at the
	// seeded works dir. The non-interactive path auto-applies the filter.
	r := makeResolverWithWorks(t, []string{"auth", "billing"})
	// Mirror the work-directory layout the real init would have written.
	// makeResolverWithWorks already creates the dirs; the call above doesn't
	// matter for the test — we just need a resolver with ListWorks() output.
	var detectBuf bytes.Buffer
	got := detectBeadFilter(r, beadFilterModeDefault, &detectBuf, nil)
	if got == nil {
		t.Fatalf("detectBeadFilter returned nil; expected subsystem:{codename}; out=%q", detectBuf.String())
	}
	if got.Label != "subsystem:{codename}" {
		t.Fatalf("detectBeadFilter.Label = %q, want subsystem:{codename}", got.Label)
	}

	// --- Apply the filter to the real project and run kerf next. ---
	projCfgPath := config.ProjectConfigPath(filepath.Join(tmp, ".kerf"), projectID)
	writeProjectFilterYAML(t, projCfgPath, "subsystem:{codename}")

	out := runNextCapture(t, projectID, nil)
	// Bead items render as `N. bead <id>  "title"  work: <codename>`.
	if !strings.Contains(out, "bead") {
		t.Fatalf("expected bead items in output; got:\n%s", out)
	}
	if !strings.Contains(out, "work: auth") || !strings.Contains(out, "work: billing") {
		t.Errorf("expected bead rows for both works; got:\n%s", out)
	}
}

// ----------------------------------------------------------------------------
// Scenario 2 — cleanup detector `work_beads_done_status_open` fires when all
// attached beads are closed but the jig status is non-terminal.
// ----------------------------------------------------------------------------

func TestE2E_Plan006_CleanupAllBeadsClosedStatusOpen(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projectID := "cleanup-proj"

	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	// One work, attached beads will all be closed but status is non-terminal.
	writeSpecWithStatus(t, filepath.Join(projDir, "stale", "spec.yaml"),
		"stale", projectID, "research")

	stubBr(t, `[
		{"id":"x-1","title":"a","status":"closed","epic":"stale","labels":["work:stale"]},
		{"id":"x-2","title":"b","status":"closed","epic":"stale","labels":["work:stale"]}
	]`)

	out := runNextCapture(t, projectID, nil)
	// The renderer prints `<n>. clean  <codename>   <reason>` and the
	// action line `kerf status <codename> <next-stage> or kerf shelve ...`
	// for work_beads_done_status_open per cmd/next.go and feed/cleanup.go.
	if !strings.Contains(out, "1. clean") {
		t.Fatalf("expected cleanup row to be the first ranked item; got:\n%s", out)
	}
	if !strings.Contains(out, "stale") {
		t.Errorf("expected cleanup row to mention codename `stale`; got:\n%s", out)
	}
	if !strings.Contains(out, "attached beads closed") {
		t.Errorf("expected reason `N attached beads closed; status: ...` (work_beads_done_status_open); got:\n%s", out)
	}
	if !strings.Contains(out, "kerf status stale") {
		t.Errorf("expected suggested action `kerf status stale <next-stage>`; got:\n%s", out)
	}
}

// ----------------------------------------------------------------------------
// Scenario 3 — warning detector `untriaged_beads` fires when beads in the
// store match no work via the configured filter and are not pinned. The
// warning renders as a header block above the ranked feed. (Renamed from
// the old `unmatched-beads` warning by Plan 009 / Bead 4.)
// ----------------------------------------------------------------------------

func TestE2E_Plan006_WarningUntriagedBeads(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projectID := "warn-proj"

	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	writeSpecWithStatus(t, filepath.Join(projDir, "alpha", "spec.yaml"),
		"alpha", projectID, "research")

	// Seed 12 beads with a prefix `legacy:*` that no work's filter matches.
	// Plan 009 / Bead 4: the renamed `untriaged_beads` detector fires on
	// any non-zero count (the plan-006 abs/frac thresholds are gone).
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 12; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		// Status "open" — Plan 008 / Bead 6 (kerf-ohp): the detector
		// counts the post-open-filter set, so untriaged beads must be
		// open to register in the warning header. These beads have no
		// matching work, so BeadSource does not list them; the header
		// is the only signal.
		sb.WriteString(`{"id":"leg-`)
		sb.WriteByte(byte('a' + i))
		sb.WriteString(`","title":"x","status":"open","epic":"","labels":["legacy:thing"]}`)
	}
	sb.WriteString("]")
	stubBr(t, sb.String())

	out := runNextCapture(t, projectID, nil)
	if !strings.Contains(out, "warning:") {
		t.Fatalf("expected warning header for untriaged beads; got:\n%s", out)
	}
	if !strings.Contains(out, "untriaged_beads") {
		t.Errorf("expected warning title `untriaged_beads`; got:\n%s", out)
	}
	// Payload-first ordering (Plan 019 / B3 — kerf-c1c): ranked items
	// precede the warning stanza per specs/commands.md §"kerf next" →
	// "Default kind selection". There may be no ranked items at all — that's
	// fine, the warning stanza should still be present.
	wi := strings.Index(out, "warning:")
	if wi < 0 {
		t.Fatalf("warning stanza missing; out:\n%s", out)
	}
	for _, marker := range []string{"1. bead", "1. clean"} {
		if idx := strings.Index(out, marker); idx >= 0 && idx > wi {
			t.Errorf("ranked items should precede warning stanza; %s at %d, warning at %d",
				marker, idx, wi)
		}
	}
}

// ----------------------------------------------------------------------------
// Scenario 4 — flag precedence in the CLI: `--only=bead` restricts the feed
// to bead items only; `--include=warning` adds the warning header back to a
// kinds-restricted feed.
// ----------------------------------------------------------------------------

func TestE2E_Plan006_FilterFlagOnlyBead(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projectID := "flag-proj"

	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	writeSpecWithStatus(t, filepath.Join(projDir, "alpha", "spec.yaml"),
		"alpha", projectID, "research")
	writeSpecWithStatus(t, filepath.Join(projDir, "stale", "spec.yaml"),
		"stale", projectID, "research")

	// Mix of ready beads (for alpha), closed beads (for stale → cleanup),
	// plus 12 unmatched (for warning).
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(`{"id":"x-1","title":"do","status":"open","epic":"alpha","labels":["work:alpha"]}`)
	sb.WriteString(`,{"id":"x-2","title":"old1","status":"closed","epic":"stale","labels":["work:stale"]}`)
	sb.WriteString(`,{"id":"x-3","title":"old2","status":"closed","epic":"stale","labels":["work:stale"]}`)
	for i := 0; i < 12; i++ {
		sb.WriteString(`,{"id":"leg-`)
		sb.WriteByte(byte('a' + i))
		sb.WriteString(`","title":"x","status":"closed","epic":"","labels":["legacy:thing"]}`)
	}
	sb.WriteString("]")
	stubBr(t, sb.String())

	// --only=bead: no cleanup, no warning header.
	out := runNextCapture(t, projectID, func() {
		nextOnly = []string{"bead"}
	})
	if !strings.Contains(out, "bead") {
		t.Errorf("--only=bead should include bead items; got:\n%s", out)
	}
	if strings.Contains(out, "warning:") {
		t.Errorf("--only=bead must not render warning header; got:\n%s", out)
	}
	if strings.Contains(out, "beads done, jig walk owed") {
		t.Errorf("--only=bead must not render cleanup items; got:\n%s", out)
	}
}

func TestE2E_Plan006_FilterFlagIncludeWarning(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projectID := "flag-warn-proj"

	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	writeSpecWithStatus(t, filepath.Join(projDir, "alpha", "spec.yaml"),
		"alpha", projectID, "research")

	// One ready bead + 12 unmatched (warning fires). Unmatched beads
	// must be open per Plan 008 / Bead 6 (post-open-filter count).
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(`{"id":"x-1","title":"do","status":"open","epic":"alpha","labels":["work:alpha"]}`)
	for i := 0; i < 12; i++ {
		sb.WriteString(`,{"id":"leg-`)
		sb.WriteByte(byte('a' + i))
		sb.WriteString(`","title":"x","status":"open","epic":"","labels":["legacy:thing"]}`)
	}
	sb.WriteString("]")
	stubBr(t, sb.String())

	// --kinds=bead replaces default → warning excluded.
	// --include=warning adds warning back.
	out := runNextCapture(t, projectID, func() {
		nextKinds = "bead"
		nextInclude = []string{"warning"}
	})
	if !strings.Contains(out, "warning:") {
		t.Errorf("--include=warning should restore warning header; got:\n%s", out)
	}
	if !strings.Contains(out, "1. bead") {
		t.Errorf("--kinds=bead --include=warning should still render the bead item; got:\n%s", out)
	}
}

// ----------------------------------------------------------------------------
// Scenario 5 — JSON output null contract. Non-bead items emit literal `null`
// for work_codename and/or bead_id depending on the item kind.
// ----------------------------------------------------------------------------

func TestE2E_Plan006_JSONNullContract(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projectID := "json-proj"

	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	writeSpecWithStatus(t, filepath.Join(projDir, "alpha", "spec.yaml"),
		"alpha", projectID, "research")

	// One ready bead + 12 unmatched legacy beads → produces both a bead item
	// (work_codename + bead_id set) and a warning item (both null).
	// Unmatched legacy beads must be open per Plan 008 / Bead 6.
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(`{"id":"x-1","title":"do","status":"open","epic":"alpha","labels":["work:alpha"]}`)
	for i := 0; i < 12; i++ {
		sb.WriteString(`,{"id":"leg-`)
		sb.WriteByte(byte('a' + i))
		sb.WriteString(`","title":"x","status":"open","epic":"","labels":["legacy:thing"]}`)
	}
	sb.WriteString("]")
	stubBr(t, sb.String())

	out := runNextCapture(t, projectID, func() {
		nextFormat = "json"
	})

	// Raw text contract: literal null tokens must be present for the
	// warning item.
	if !strings.Contains(out, `"work_codename": null`) {
		t.Errorf("expected literal `work_codename: null` for warning; got:\n%s", out)
	}
	if !strings.Contains(out, `"bead_id": null`) {
		t.Errorf("expected literal `bead_id: null` for warning; got:\n%s", out)
	}

	// Parse as JSON and assert per-kind shape.
	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("unmarshal json output: %v; body:\n%s", err, out)
	}
	if len(items) == 0 {
		t.Fatalf("expected at least one item in JSON output; body:\n%s", out)
	}
	sawBead, sawWarning := false, false
	for _, it := range items {
		kind, _ := it["kind"].(string)
		switch kind {
		case "bead":
			sawBead = true
			if it["work_codename"] == nil {
				t.Errorf("bead item must carry non-null work_codename; got: %v", it)
			}
			if it["bead_id"] == nil {
				t.Errorf("bead item must carry non-null bead_id; got: %v", it)
			}
		case "warning":
			sawWarning = true
			if v, present := it["work_codename"]; !present || v != nil {
				t.Errorf("warning item must serialize work_codename as null; got: %v (present=%v)", v, present)
			}
			if v, present := it["bead_id"]; !present || v != nil {
				t.Errorf("warning item must serialize bead_id as null; got: %v (present=%v)", v, present)
			}
		}
	}
	if !sawBead {
		t.Errorf("expected a bead item in JSON output; items=%v", items)
	}
	if !sawWarning {
		t.Errorf("expected a warning item in JSON output; items=%v", items)
	}
}

// ----------------------------------------------------------------------------
// Scenario 6 — empty feed message. A project with no beads, no cleanups and
// no warnings emits the exact spec line in text mode.
// ----------------------------------------------------------------------------

func TestE2E_Plan006_EmptyFeedMessage(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projectID := "empty-proj"

	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte("jigs: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// stubBr with empty array — no beads anywhere; no works → no detectors fire.
	stubBr(t, `[]`)

	out := runNextCapture(t, projectID, nil)
	got := strings.TrimRight(out, "\n")
	if got != nextEmptyText {
		t.Fatalf("empty-feed text\n  got:  %q\n  want: %q", got, nextEmptyText)
	}
}

// Sanity: the beads package types compile against the test fixtures above.
// Keeps the import live in case future refactors trim direct usage.
var _ = beads.Filter{}
