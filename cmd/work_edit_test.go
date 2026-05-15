package cmd

// Plan 009 / B10 — kerf work edit command tests.
//
// Spec references:
//   - specs/commands.md §"kerf work edit" — full section, including step 7
//     ("do not advance the drift baseline").
//   - specs/coordination.md §"Baseline advancement" — `kerf work edit` MUST NOT
//     advance the baseline.
//
// Deliverables under test (per beads.md B10):
//   - Adding two clauses produces an `any:` list.
//   - Removing the last clause removes the `bead_filter` key entirely.
//   - Round-trip preserves comments on the target `spec.yaml`.
//   - Baseline unchanged after invocation.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeWorkEditTestSpec lays down $HOME/.kerf/projects/<proj>/<codename>/spec.yaml
// containing the supplied YAML body. The body is written verbatim (no
// templating) so callers can include comments / unusual formatting that
// the comment-preserving mutators must round-trip.
func writeWorkEditTestSpec(t *testing.T, projectID, codename, body string) string {
	t.Helper()
	home := os.Getenv("HOME")
	workDir := filepath.Join(home, ".kerf", "projects", projectID, codename)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work dir: %v", err)
	}
	specPath := filepath.Join(workDir, "spec.yaml")
	if err := os.WriteFile(specPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	// Drop a minimal project.yaml so helpers that try to read it don't bail.
	projectYAML := filepath.Join(filepath.Dir(workDir), "project.yaml")
	if _, err := os.Stat(projectYAML); os.IsNotExist(err) {
		_ = os.WriteFile(projectYAML, []byte("jigs: []\n"), 0o644)
	}
	return specPath
}

// baseSpecYAML returns a minimal spec.yaml body suitable for round-tripping
// through spec.Read, with no bead_filter set. Tests that want a starting
// filter can append it themselves.
func baseSpecYAML(codename, projectID string) string {
	return "codename: " + codename + "\n" +
		"type: plan\n" +
		"project:\n  id: " + projectID + "\n" +
		"jig: plan\n" +
		"jig_version: 1\n" +
		"status: problem-space\n" +
		"status_values: [problem-space, ready]\n" +
		"created: 2026-04-09T00:00:00Z\n" +
		"updated: 2026-04-09T00:00:00Z\n" +
		"sessions: []\n" +
		"depends_on: []\n" +
		"areas: []\n" +
		"pinned_beads: []\n" +
		"implementation:\n  branch: null\n  pr: null\n  commits: []\n"
}

// runWorkEditWithFlags sets the package-level flag vars, invokes runWorkEdit,
// then restores them. Returns the error from runWorkEdit. Output goes to
// stdout (the command writes via fmt.Printf directly); we don't assert on it
// here — the YAML changes are the primary observable.
func runWorkEditWithFlags(t *testing.T, projectID, codename string, adds, removes []string) error {
	t.Helper()
	prevProject := projectFlag
	prevAdd := workEditBeadFilterAdd
	prevRemove := workEditBeadFilterRemove
	projectFlag = projectID
	workEditBeadFilterAdd = adds
	workEditBeadFilterRemove = removes
	t.Cleanup(func() {
		projectFlag = prevProject
		workEditBeadFilterAdd = prevAdd
		workEditBeadFilterRemove = prevRemove
	})
	return runWorkEdit(codename)
}

// pointPATHEmpty hides `br` from the work-edit count helper so the optional
// "Now matches" line is suppressed (and so attachedBeadCount returns false).
// The mutators themselves do not depend on `br`.
func pointPATHEmpty(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

// ---------------------------------------------------------------------------
// Spec deliverable: "Adding two clauses produces an `any:` list."
// ---------------------------------------------------------------------------

func TestWorkEdit_AddTwoClauses_ProducesAny(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	pointPATHEmpty(t)

	proj := "work-edit-proj-add2"
	specPath := writeWorkEditTestSpec(t, proj, "auth", baseSpecYAML("auth", proj))

	if err := runWorkEditWithFlags(t, proj, "auth",
		[]string{"label=subsystem:auth", "label=subsystem:authz"}, nil); err != nil {
		t.Fatalf("runWorkEdit: %v", err)
	}

	got, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	body := string(got)
	if !strings.Contains(body, "bead_filter:") {
		t.Fatalf("expected bead_filter key in output, got:\n%s", body)
	}
	if !strings.Contains(body, "any:") {
		t.Fatalf("expected any: union when two clauses are added, got:\n%s", body)
	}
	if !strings.Contains(body, "subsystem:auth") || !strings.Contains(body, "subsystem:authz") {
		t.Fatalf("missing one of the added clauses, got:\n%s", body)
	}
}

// ---------------------------------------------------------------------------
// Spec deliverable: "Removing the last clause removes the `bead_filter` key
// entirely."
// ---------------------------------------------------------------------------

func TestWorkEdit_RemoveLastClause_RemovesKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	pointPATHEmpty(t)

	proj := "work-edit-proj-remove"
	body := baseSpecYAML("api", proj) +
		"bead_filter:\n  label: subsystem:api\n"
	specPath := writeWorkEditTestSpec(t, proj, "api", body)

	if err := runWorkEditWithFlags(t, proj, "api",
		nil, []string{"label=subsystem:api"}); err != nil {
		t.Fatalf("runWorkEdit: %v", err)
	}

	out, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	if strings.Contains(string(out), "bead_filter") {
		t.Fatalf("expected bead_filter key removed, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Spec deliverable: "Round-trip preserves comments on the target `spec.yaml`."
// ---------------------------------------------------------------------------

func TestWorkEdit_PreservesComments(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	pointPATHEmpty(t)

	proj := "work-edit-proj-comments"
	body := "# This is a heartfelt head comment.\n" +
		baseSpecYAML("storage", proj) +
		"# Pre-filter comment.\n" +
		"bead_filter:\n  label: subsystem:storage\n"
	specPath := writeWorkEditTestSpec(t, proj, "storage", body)

	if err := runWorkEditWithFlags(t, proj, "storage",
		[]string{"label=subsystem:cache"}, nil); err != nil {
		t.Fatalf("runWorkEdit: %v", err)
	}

	got, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	gotStr := string(got)
	if !strings.Contains(gotStr, "# This is a heartfelt head comment.") {
		t.Errorf("head comment lost, got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "# Pre-filter comment.") {
		t.Errorf("pre-filter comment lost, got:\n%s", gotStr)
	}
	// Sanity: the new clause is there.
	if !strings.Contains(gotStr, "subsystem:cache") {
		t.Errorf("new clause missing, got:\n%s", gotStr)
	}
}

// ---------------------------------------------------------------------------
// Spec deliverable: "Baseline unchanged after invocation."
//
// Per specs/coordination.md §"Baseline advancement", `kerf work edit` MUST
// NOT advance the drift baseline. We seed a baseline and verify it is
// byte-identical after the command runs.
// ---------------------------------------------------------------------------

func TestWorkEdit_DoesNotAdvanceBaseline(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	pointPATHEmpty(t)

	proj := "work-edit-proj-baseline"
	writeWorkEditTestSpec(t, proj, "ledger", baseSpecYAML("ledger", proj))

	// Seed a synthetic drift baseline file under .kerf/projects/<proj>/.
	// `kerf work edit` MUST NOT touch it. (drift.Advance is the only
	// legitimate writer; this test asserts work-edit does not call into
	// drift at all by way of byte-equality on a sentinel file.)
	driftDir := filepath.Join(tmp, ".kerf", "projects", proj)
	cachePath := filepath.Join(driftDir, "drift-baseline.sentinel")
	if err := os.MkdirAll(driftDir, 0o755); err != nil {
		t.Fatalf("mkdir cache parent: %v", err)
	}
	const seed = "BASELINE-DO-NOT-TOUCH\n"
	if err := os.WriteFile(cachePath, []byte(seed), 0o644); err != nil {
		t.Fatalf("seed drift baseline: %v", err)
	}

	if err := runWorkEditWithFlags(t, proj, "ledger",
		[]string{"label=subsystem:ledger"}, nil); err != nil {
		t.Fatalf("runWorkEdit: %v", err)
	}

	got, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read drift baseline after: %v", err)
	}
	if string(got) != seed {
		t.Fatalf("drift baseline changed; expected unchanged.\n  before: %q\n  after:  %q", seed, string(got))
	}
}

// ---------------------------------------------------------------------------
// Defensive: "At least one of --bead-filter-add or --bead-filter-remove is
// required."
// ---------------------------------------------------------------------------

func TestWorkEdit_RequiresAtLeastOneFlag(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	pointPATHEmpty(t)

	proj := "work-edit-proj-noflags"
	writeWorkEditTestSpec(t, proj, "auth", baseSpecYAML("auth", proj))

	err := runWorkEditWithFlags(t, proj, "auth", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "at least one of") {
		t.Fatalf("expected 'at least one of' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Defensive: codename miss errors with a non-zero exit.
// ---------------------------------------------------------------------------

func TestWorkEdit_CodenameMiss_Errors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	pointPATHEmpty(t)

	proj := "work-edit-proj-miss"
	// Seed a sibling work so the project dir exists but the target is absent.
	writeWorkEditTestSpec(t, proj, "alpha", baseSpecYAML("alpha", proj))

	err := runWorkEditWithFlags(t, proj, "beta",
		[]string{"label=subsystem:beta"}, nil)
	if err == nil {
		t.Fatalf("expected error for missing work, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Defensive: bad clause syntax errors before any disk mutation.
// ---------------------------------------------------------------------------

func TestWorkEdit_BadClauseSyntax_Errors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	pointPATHEmpty(t)

	proj := "work-edit-proj-badclause"
	specPath := writeWorkEditTestSpec(t, proj, "auth", baseSpecYAML("auth", proj))
	before, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	err = runWorkEditWithFlags(t, proj, "auth",
		[]string{"all=nope"}, nil)
	if err == nil {
		t.Fatalf("expected error for bad clause, got nil")
	}
	if !strings.Contains(err.Error(), "does not parse") {
		t.Errorf("expected 'does not parse' error, got: %v", err)
	}

	after, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("spec.yaml mutated despite bad clause; expected fail-fast")
	}
}
