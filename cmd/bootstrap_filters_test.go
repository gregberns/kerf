package cmd

// Plan 019 / B5 (kerf-a7t) — kerf bootstrap-filters command tests.
//
// Spec references:
//   - specs/commands.md §"kerf bootstrap-filters" — full section.
//
// Behaviors under test:
//   - Dry-run prints proposals but does not mutate spec.yaml.
//   - --yes writes proposals via the spec.AddBeadFilterClause path, matching
//     the wire format of `kerf work edit --bead-filter-add`.
//   - Idempotent: re-running after a successful apply finds no eligible works.
//   - --codename restricts the run; an unknown codename errors.
//   - No-proposal reports surface a 'reason' explanation.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetBootstrapFlags restores the package-level flag vars between tests.
func resetBootstrapFlags(t *testing.T) {
	t.Helper()
	prevProject := projectFlag
	prevApply := bootstrapFiltersApply
	prevYes := bootstrapFiltersYes
	prevCN := bootstrapFiltersCodename
	prevFmt := bootstrapFiltersFormat
	t.Cleanup(func() {
		projectFlag = prevProject
		bootstrapFiltersApply = prevApply
		bootstrapFiltersYes = prevYes
		bootstrapFiltersCodename = prevCN
		bootstrapFiltersFormat = prevFmt
	})
	bootstrapFiltersApply = false
	bootstrapFiltersYes = false
	bootstrapFiltersCodename = nil
	bootstrapFiltersFormat = "text"
}

// bootstrapTestProject lays down a project under $HOME/.kerf/projects/<id>
// containing one spec.yaml per supplied codename. Each spec starts with no
// bead_filter, so the default filter applies and (in the synthetic store)
// matches nothing — making every work eligible.
func bootstrapTestProject(t *testing.T, projectID string, codenames []string) {
	t.Helper()
	home := os.Getenv("HOME")
	projDir := filepath.Join(home, ".kerf", "projects", projectID)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir proj: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte("jigs: []\n"), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}
	for _, cn := range codenames {
		writeWorkEditTestSpec(t, projectID, cn, baseSpecYAML(cn, projectID))
	}
}

// --- core behaviors ---------------------------------------------------------

// Dry-run: with a synthetic bead store of three beads labeled `subsystem:bridge`,
// the `bridge` work should be proposed; the `phantom` work should report
// no-proposal. No spec.yaml is mutated.
func TestBootstrapFilters_DryRun(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	resetBootstrapFlags(t)

	proj := "bootstrap-dryrun"
	bootstrapTestProject(t, proj, []string{"bridge", "phantom"})
	stubBr(t, `[
		{"id":"b-1","title":"x","status":"open","labels":["subsystem:bridge"]},
		{"id":"b-2","title":"y","status":"open","labels":["subsystem:bridge"]},
		{"id":"b-3","title":"z","status":"open","labels":["subsystem:bridge"]}
	]`)

	projectFlag = proj
	var stdout bytes.Buffer
	if err := runBootstrapFilters(&stdout, strings.NewReader("")); err != nil {
		t.Fatalf("runBootstrapFilters: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "bridge") || !strings.Contains(out, "label=subsystem:bridge") {
		t.Errorf("missing bridge proposal in:\n%s", out)
	}
	if !strings.Contains(out, "phantom") || !strings.Contains(out, "no proposal") {
		t.Errorf("missing phantom no-proposal in:\n%s", out)
	}
	if !strings.Contains(out, "Dry-run") {
		t.Errorf("expected dry-run hint, got:\n%s", out)
	}

	// spec.yaml for bridge should be unchanged (no bead_filter key).
	home := os.Getenv("HOME")
	body, err := os.ReadFile(filepath.Join(home, ".kerf", "projects", proj, "bridge", "spec.yaml"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	if strings.Contains(string(body), "bead_filter:") {
		t.Errorf("dry-run wrote bead_filter to spec.yaml:\n%s", body)
	}
}

// --yes applies proposals and writes via the same mutator path as `kerf work edit`.
// Re-running afterwards reports nothing to do (idempotent).
func TestBootstrapFilters_ApplyAndIdempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	resetBootstrapFlags(t)

	proj := "bootstrap-apply"
	bootstrapTestProject(t, proj, []string{"bridge", "auth"})
	stubBr(t, `[
		{"id":"b-1","title":"x","status":"open","labels":["subsystem:bridge"]},
		{"id":"b-2","title":"y","status":"open","labels":["subsystem:bridge"]},
		{"id":"b-3","title":"z","status":"open","labels":["subsystem:bridge"]},
		{"id":"a-1","title":"x","status":"open","labels":["codename:auth"]},
		{"id":"a-2","title":"y","status":"open","labels":["codename:auth"]},
		{"id":"a-3","title":"z","status":"open","labels":["codename:auth"]}
	]`)

	projectFlag = proj
	bootstrapFiltersYes = true
	var stdout bytes.Buffer
	if err := runBootstrapFilters(&stdout, strings.NewReader("")); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !strings.Contains(stdout.String(), "applied ") {
		t.Errorf("expected 'applied' lines, got:\n%s", stdout.String())
	}

	home := os.Getenv("HOME")
	for _, cn := range []string{"bridge", "auth"} {
		body, err := os.ReadFile(filepath.Join(home, ".kerf", "projects", proj, cn, "spec.yaml"))
		if err != nil {
			t.Fatalf("read %s spec: %v", cn, err)
		}
		if !strings.Contains(string(body), "bead_filter:") {
			t.Errorf("%s: bead_filter not written:\n%s", cn, body)
		}
	}

	// Idempotency: re-run; both works now resolve to non-empty filters, so
	// nothing should be eligible.
	stdout.Reset()
	bootstrapFiltersYes = false // dry-run path
	if err := runBootstrapFilters(&stdout, strings.NewReader("")); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !strings.Contains(stdout.String(), "No works in") {
		t.Errorf("expected 'No works in ...' on idempotent re-run, got:\n%s", stdout.String())
	}
}

// --codename restricts the run; an unknown codename errors with the spec
// message shape.
func TestBootstrapFilters_CodenameFilter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	resetBootstrapFlags(t)

	proj := "bootstrap-cn"
	bootstrapTestProject(t, proj, []string{"bridge", "auth"})
	stubBr(t, `[
		{"id":"b-1","title":"x","status":"open","labels":["subsystem:bridge"]},
		{"id":"b-2","title":"y","status":"open","labels":["subsystem:bridge"]},
		{"id":"b-3","title":"z","status":"open","labels":["subsystem:bridge"]}
	]`)

	projectFlag = proj
	bootstrapFiltersCodename = []string{"bridge"}
	var stdout bytes.Buffer
	if err := runBootstrapFilters(&stdout, strings.NewReader("")); err != nil {
		t.Fatalf("filtered run: %v", err)
	}
	if strings.Contains(stdout.String(), "auth") {
		t.Errorf("auth should not appear when filtered to bridge, got:\n%s", stdout.String())
	}

	// Unknown codename surfaces the documented error message.
	bootstrapFiltersCodename = []string{"nope"}
	stdout.Reset()
	err := runBootstrapFilters(&stdout, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for unknown codename")
	}
	if !strings.Contains(err.Error(), "work 'nope' not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// Closed beads do not count as evidence — the bridge work in this store has
// three closed beads with the dominant label and one open bead with a
// different label. Result: no proposal (the open signal is too weak).
func TestBootstrapFilters_IgnoresClosedBeads(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	resetBootstrapFlags(t)

	proj := "bootstrap-closed"
	bootstrapTestProject(t, proj, []string{"bridge"})
	stubBr(t, `[
		{"id":"b-1","title":"x","status":"closed","labels":["subsystem:bridge"]},
		{"id":"b-2","title":"y","status":"closed","labels":["subsystem:bridge"]},
		{"id":"b-3","title":"z","status":"closed","labels":["subsystem:bridge"]}
	]`)

	projectFlag = proj
	var stdout bytes.Buffer
	if err := runBootstrapFilters(&stdout, strings.NewReader("")); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "no proposal") {
		t.Errorf("expected no-proposal when only closed beads carry the label, got:\n%s", stdout.String())
	}
}
