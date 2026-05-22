package cmd

// End-to-end tests for D1 (abandoned_dispatch) and D6 (reviewer_absent)
// warning rendering through `kerf next` (Plan 013 / B-E2E — kerf-6iiw).
//
// Spec sentence claimed: specs/commands.md §"`kerf next`" §"Warning kinds"
//   — "verify `abandoned_dispatch` and `reviewer_absent` render correctly
//   in text and JSON" and both are non-fatal (the feed listing still
//   renders). Fatality is mirrored in specs/diagnostics.md §"Severity and
//   fatality" — "Both `abandoned_dispatch` and `reviewer_absent` are
//   non-fatal warnings in `kerf next`. The ranked feed still renders."
//
// Strategy: each test stages
//   1. A temp git repo (real `git log --all` for the D1 indexer),
//   2. A `.kerf/project-identifier` matching a synthetic project ID so
//      cmdutil.Resolver picks up RepoRoot,
//   3. A bench project.yaml carrying `bead.id_pattern` (D1 silently
//      no-ops without it),
//   4. KERF_TRANSCRIPT_DIR pointing at a temp dir containing fixtures
//      derived from internal/kerftranscript/testdata + padding.
//
// The same `runNext` invocation is asserted twice (text + JSON) so the
// renderer paths share one fixture project.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/feed"
)

// fixturePath resolves a path relative to the cmd package directory so
// fixture reads continue to work after a test calls t.Chdir into a
// tempdir. Resolves at init time (before any test changes cwd).
var fixtureDir = func() string {
	wd, err := os.Getwd()
	if err != nil {
		panic("fixtureDir: getwd: " + err.Error())
	}
	return wd
}()

func fixturePath(rel string) string { return filepath.Join(fixtureDir, rel) }

// isolatePATHKeepGit points PATH at a fresh tempdir that contains only a
// symlink to the real `git` binary. This keeps `git log --all`
// (invoked by the D1 indexer) reachable while still hiding any real
// `br` / `bd` bead-tool binary the developer has installed — the same
// hazard isolatePATH guards against.
func isolatePATHKeepGit(t *testing.T) {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	dir := t.TempDir()
	if err := os.Symlink(gitBin, filepath.Join(dir, "git")); err != nil {
		t.Fatalf("symlink git: %v", err)
	}
	t.Setenv("PATH", dir)
}

// stageDiagnosticsRepo builds a fresh git repo at a tempdir, drops a
// project.yaml at the bench path with a kerf-style bead.id_pattern, and
// writes .kerf/project-identifier so cmdutil.Resolver finds RepoRoot.
// Returns the repoPath. MUST be called before isolatePATH — git is
// invoked here and needs the real PATH.
func stageDiagnosticsRepo(t *testing.T, projectID string) string {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@kerf.dev"},
		{"config", "user.name", "kerf-test"},
		{"commit", "--allow-empty", "-q", "-m", "initial"},
	} {
		c := exec.Command(gitBin, args...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %s: %v", args, repo, out, err)
		}
	}
	// Drop the project identifier so cmdutil.Resolver returns RepoRoot.
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(projectID+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Bench project.yaml carrying bead.id_pattern so D1 has a regex.
	// Pattern matches hk-…, kerf-… style IDs used in fixtures and synth.
	home, _ := os.UserHomeDir()
	benchProj := filepath.Join(home, ".kerf", "projects", projectID)
	if err := os.MkdirAll(benchProj, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "jigs: []\nbead:\n  id_pattern: '(hk|kerf)-[A-Za-z0-9._]+'\n"
	if err := os.WriteFile(filepath.Join(benchProj, "project.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

// writeTranscript writes lines to dir/name and points KERF_TRANSCRIPT_DIR
// at dir. Lines are concatenated raw — caller controls trailing newlines.
func writeTranscript(t *testing.T, dir, name string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(lines, "")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunNext_E2E_AbandonedDispatch_TextAndJSON exercises the D1 wiring
// end-to-end: a dispatch event for a bead with no commit_ref and no
// matching commit in `git log --all` should surface one
// `abandoned_dispatch` warning. Per specs/commands.md §"Warning kinds"
// → `abandoned_dispatch`: non-fatal. The feed listing still renders.
func TestRunNext_E2E_AbandonedDispatch_TextAndJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("KERF_DOCTOR_FOOTER", "0")

	projectID := "diag-test-d1"
	repo := stageDiagnosticsRepo(t, projectID)
	t.Chdir(repo)
	isolatePATHKeepGit(t)

	// Stage the D1 fixture as the only transcript. The fixture's bead
	// `hk-qo08q.15` is never committed to this fresh repo's git log →
	// indexer.HasCommitFor returns false → D1 fires.
	tDir := filepath.Join(tmp, "transcripts")
	fixture, err := os.ReadFile(fixturePath("../internal/kerftranscript/testdata/d1_abandon_a.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	writeTranscript(t, tDir, "session.jsonl", string(fixture))
	t.Setenv("KERF_TRANSCRIPT_DIR", tDir)

	resetNextFlags()
	t.Cleanup(resetNextFlags)
	projectFlag = ""
	t.Cleanup(func() { projectFlag = "" })

	// --- Text rendering ----------------------------------------------------
	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext text: %v", err)
	}
	body := buf.String()
	// Spec: title is "Abandoned dispatch: {bead-id}".
	if !strings.Contains(body, "Abandoned dispatch: hk-qo08q.15") {
		t.Errorf("text: missing D1 title with bead id; got:\n%s", body)
	}
	// Spec: action is "kerf show {bead-id}".
	if !strings.Contains(body, "kerf show hk-qo08q.15") {
		t.Errorf("text: missing D1 action; got:\n%s", body)
	}
	// Spec reason shape: "Sub-agent dispatched at {dispatched_at} ran
	// {duration}s with no commit; reason: {reason_category}. Session
	// {session_id}." The fixture's last activity is 18:12:02 vs
	// dispatch 18:10:11 ⇒ 111s.
	for _, want := range []string{
		"Sub-agent dispatched at 2026-05-15T18:10:11Z",
		"ran 111s with no commit",
		"reason: appears_completed_no_commit",
		"Session fed61a3d-3aa9-4c8a-91e7-0b1acb4ec1e8",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("text: missing reason fragment %q; got:\n%s", want, body)
		}
	}
	// Non-fatal: warning prefixed `warning:` and the feed footer tip
	// still renders. (Empty-feed-text is also acceptable since no beads
	// exist; key invariant is that we did NOT error out.)
	if !strings.Contains(body, "warning:") {
		t.Errorf("text: D1 must render as a `warning:` row; got:\n%s", body)
	}
	if !strings.Contains(body, nextFooterTip) {
		t.Errorf("text: footer tip must render (non-fatal warning); got:\n%s", body)
	}
	// Cardinality: exactly one D1 row. Guards against duplicate-emission
	// regressions (e.g. detector loop emitting once per session-row).
	if n := strings.Count(body, "Abandoned dispatch:"); n != 1 {
		t.Errorf("text: D1 title count = %d, want 1; got:\n%s", n, body)
	}

	// --- JSON rendering ----------------------------------------------------
	buf.Reset()
	resetNextFlags()
	nextFormat = "json"
	nextCmd.SetOut(&buf)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext json: %v", err)
	}
	var items []feed.Item
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatalf("decode json: %v\nbody=%s", err, buf.String())
	}
	got := findItemByTitlePrefix(items, "Abandoned dispatch:")
	if got == nil {
		t.Fatalf("json: no abandoned_dispatch item; items=%+v", items)
	}
	// Cardinality: exactly one D1 item. Guards duplicate-emission regressions.
	if n := countItemsByTitlePrefix(items, "Abandoned dispatch:"); n != 1 {
		t.Errorf("json: D1 item count = %d, want 1; items=%+v", n, items)
	}
	if got.Kind != feed.KindWarning {
		t.Errorf("json: D1 item kind = %q, want %q", got.Kind, feed.KindWarning)
	}
	if got.Title != "Abandoned dispatch: hk-qo08q.15" {
		t.Errorf("json: D1 title = %q", got.Title)
	}
	if got.Action != "kerf show hk-qo08q.15" {
		t.Errorf("json: D1 action = %q", got.Action)
	}
	if got.BeadID == nil || *got.BeadID != "hk-qo08q.15" {
		t.Errorf("json: D1 bead_id = %v, want hk-qo08q.15", got.BeadID)
	}
	if !strings.Contains(got.Reason, "appears_completed_no_commit") ||
		!strings.Contains(got.Reason, "Session fed61a3d") {
		t.Errorf("json: D1 reason missing spec fragments; got %q", got.Reason)
	}
}

// TestRunNext_E2E_ReviewerAbsent_TextAndJSON exercises the D6 wiring
// end-to-end. The min-history guard (D6MinHistoryBeads = 30) is cleared
// by padding the transcript with 30 dispatch-only beads alongside the
// real reviewer-absent fixture. Per specs/commands.md §"Warning kinds"
// → `reviewer_absent`: non-fatal. The feed listing still renders.
func TestRunNext_E2E_ReviewerAbsent_TextAndJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("KERF_DOCTOR_FOOTER", "0")

	projectID := "diag-test-d6"
	repo := stageDiagnosticsRepo(t, projectID)
	t.Chdir(repo)
	isolatePATHKeepGit(t)

	// Build a transcript with:
	//   - 30 padding dispatches on distinct bead IDs in a separate
	//     session containing a reviewer dispatch (so they cannot trip D6),
	//   - the real d6_reviewer_absent_a fixture (session 801120b5…,
	//     bead hk-iuaed.6, commit dcd7f7e…) with no reviewer dispatch.
	// Together this clears the min-history floor while keeping the
	// reviewer-absent finding as the only D6 hit.
	var lines strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&lines,
			`{"timestamp":"2026-05-14T10:%02d:00Z","kind":"dispatch","session_id":"pad-sess","sub_agent_id":"pad-%d","bead_id":"hk-pad-%03d","role":"implementer","text":"pad"}`+"\n",
			i, i, i)
	}
	// Reviewer dispatch in the padding session (so padding cannot fire D6).
	// Marker per specs/diagnostics.md §"Reviewer dispatch" — the canonical
	// text-format header `kerf review` emits, including the em-dash.
	fmt.Fprintf(&lines,
		`{"timestamp":"2026-05-14T11:00:00Z","kind":"dispatch","session_id":"pad-sess","sub_agent_id":"pad-rev","bead_id":"hk-pad-000","role":"reviewer","text":"Reviewer prompt for hk-pad-000 — pass: review"}`+"\n")

	fixture, err := os.ReadFile(fixturePath("../internal/kerftranscript/testdata/d6_reviewer_absent_a.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	lines.Write(fixture)

	tDir := filepath.Join(tmp, "transcripts")
	writeTranscript(t, tDir, "session.jsonl", lines.String())
	t.Setenv("KERF_TRANSCRIPT_DIR", tDir)

	resetNextFlags()
	t.Cleanup(resetNextFlags)
	projectFlag = ""
	t.Cleanup(func() { projectFlag = "" })

	// --- Text rendering ----------------------------------------------------
	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext text: %v", err)
	}
	body := buf.String()
	if !strings.Contains(body, "Reviewer absent: hk-iuaed.6") {
		t.Errorf("text: missing D6 title; got:\n%s", body)
	}
	// Pin the current action string. cmd/next.go currently emits
	// `kerf show {bead-id}` for D6, a deferred divergence from the spec's
	// `kerf review {codename}` (intentional; documented in cmd/next.go
	// comments). This assertion exists so any future silent change to that
	// line trips the test — it does not assert the spec.
	if !strings.Contains(body, "kerf show hk-iuaed.6") {
		t.Errorf("text: missing D6 action `kerf show hk-iuaed.6`; got:\n%s", body)
	}
	// Cardinality: exactly one D6 row. Guards duplicate-emission regressions.
	if n := strings.Count(body, "Reviewer absent:"); n != 1 {
		t.Errorf("text: D6 title count = %d, want 1; got:\n%s", n, body)
	}
	// Spec: reason shape "Commit {commit_sha} for bead '{bead-id}' landed
	// at {committed_at} with no reviewer dispatch in session {session_id}."
	for _, want := range []string{
		"Commit dcd7f7e5d1a5eb4cf6dc4b292d86a5ea01562c4f",
		"for bead 'hk-iuaed.6'",
		"landed at 2026-05-15T20:23:40Z",
		"no reviewer dispatch in session 801120b5-0000-4000-8000-000000000001",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("text: missing D6 reason fragment %q; got:\n%s", want, body)
		}
	}
	// Non-fatal: footer tip still renders.
	if !strings.Contains(body, nextFooterTip) {
		t.Errorf("text: footer tip must render (non-fatal); got:\n%s", body)
	}

	// --- JSON rendering ----------------------------------------------------
	buf.Reset()
	resetNextFlags()
	nextFormat = "json"
	nextCmd.SetOut(&buf)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext json: %v", err)
	}
	var items []feed.Item
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatalf("decode json: %v\nbody=%s", err, buf.String())
	}
	got := findItemByTitlePrefix(items, "Reviewer absent:")
	if got == nil {
		t.Fatalf("json: no reviewer_absent item; items=%+v", items)
	}
	// Cardinality: exactly one D6 item. Guards duplicate-emission regressions.
	if n := countItemsByTitlePrefix(items, "Reviewer absent:"); n != 1 {
		t.Errorf("json: D6 item count = %d, want 1; items=%+v", n, items)
	}
	if got.Kind != feed.KindWarning {
		t.Errorf("json: D6 kind = %q, want %q", got.Kind, feed.KindWarning)
	}
	if got.Title != "Reviewer absent: hk-iuaed.6" {
		t.Errorf("json: D6 title = %q", got.Title)
	}
	// Pin the current action string — see text-side comment above. This
	// asserts cmd/next.go's deferred behaviour, not the spec.
	if got.Action != "kerf show hk-iuaed.6" {
		t.Errorf("json: D6 action = %q, want %q", got.Action, "kerf show hk-iuaed.6")
	}
	if got.BeadID == nil || *got.BeadID != "hk-iuaed.6" {
		t.Errorf("json: D6 bead_id = %v", got.BeadID)
	}
	if !strings.Contains(got.Reason, "dcd7f7e5d1a5eb4cf6dc4b292d86a5ea01562c4f") ||
		!strings.Contains(got.Reason, "801120b5-0000-4000-8000-000000000001") {
		t.Errorf("json: D6 reason missing fragments; got %q", got.Reason)
	}
}

// TestRunNext_E2E_BothDetectorsNonFatal asserts that when D1 and D6
// findings co-occur, `kerf next` still exits cleanly with no error and
// both warnings render. Mirrors the fatality classification in
// specs/diagnostics.md §"Severity and fatality" — "Both
// `abandoned_dispatch` and `reviewer_absent` are non-fatal warnings".
func TestRunNext_E2E_BothDetectorsNonFatal(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("KERF_DOCTOR_FOOTER", "0")

	projectID := "diag-test-both"
	repo := stageDiagnosticsRepo(t, projectID)
	t.Chdir(repo)
	isolatePATHKeepGit(t)

	d1, err := os.ReadFile(fixturePath("../internal/kerftranscript/testdata/d1_abandon_a.jsonl"))
	if err != nil {
		t.Fatalf("read d1: %v", err)
	}
	d6, err := os.ReadFile(fixturePath("../internal/kerftranscript/testdata/d6_reviewer_absent_a.jsonl"))
	if err != nil {
		t.Fatalf("read d6: %v", err)
	}

	var lines strings.Builder
	// Padding: 30 distinct beads in a separate session with a reviewer
	// dispatch so they neither trip D6 nor (lacking dispatch duration)
	// trip D1.
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&lines,
			`{"timestamp":"2026-05-14T10:%02d:00Z","kind":"dispatch","session_id":"pad-sess","sub_agent_id":"pad-%d","bead_id":"hk-pad-%03d","role":"implementer","text":"pad"}`+"\n",
			i, i, i)
	}
	fmt.Fprintf(&lines,
		`{"timestamp":"2026-05-14T11:00:00Z","kind":"dispatch","session_id":"pad-sess","sub_agent_id":"pad-rev","bead_id":"hk-pad-000","role":"reviewer","text":"Reviewer prompt for hk-pad-000 — pass: review"}`+"\n")
	lines.Write(d1)
	lines.Write(d6)

	tDir := filepath.Join(tmp, "transcripts")
	writeTranscript(t, tDir, "session.jsonl", lines.String())
	t.Setenv("KERF_TRANSCRIPT_DIR", tDir)

	resetNextFlags()
	t.Cleanup(resetNextFlags)
	projectFlag = ""
	t.Cleanup(func() { projectFlag = "" })

	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext (both detectors) should be non-fatal; got error: %v", err)
	}
	body := buf.String()
	if !strings.Contains(body, "Abandoned dispatch: hk-qo08q.15") {
		t.Errorf("expected D1 warning to render alongside D6; got:\n%s", body)
	}
	if !strings.Contains(body, "Reviewer absent: hk-iuaed.6") {
		t.Errorf("expected D6 warning to render alongside D1; got:\n%s", body)
	}
}

// TestRunNext_E2E_CorruptProjectConfig_TextAndJSON exercises the
// `corrupt_project_config` warning end-to-end. A malformed
// `bead.id_pattern` regex in project.yaml must:
//
//  1. Surface exactly one `corrupt_project_config` warning per
//     invocation (cardinality, per specs/commands.md
//     §`corrupt_project_config`).
//  2. Carry the spec's title, action ("-"), and a reason embedding the
//     verbatim compile error plus the D1-disabled clause.
//  3. Disable D1 only: a transcript that would otherwise produce a D1
//     finding yields none. D6 is unaffected and still emits its findings
//     (specs/commands.md §`corrupt_project_config`, kerf-x91o).
//  4. Be non-fatal: the feed still renders (footer tip still prints).
//
// Bead: kerf-7ozm.
func TestRunNext_E2E_CorruptProjectConfig_TextAndJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("KERF_DOCTOR_FOOTER", "0")

	projectID := "diag-test-corrupt"
	repo := stageDiagnosticsRepo(t, projectID)

	// Overwrite the bench project.yaml with a malformed regex. The
	// project.yaml dropped by stageDiagnosticsRepo carries a valid
	// kerf-style pattern; the malformed pattern here is what triggers
	// the `corrupt_project_config` warning.
	home, _ := os.UserHomeDir()
	benchProj := filepath.Join(home, ".kerf", "projects", projectID, "project.yaml")
	bad := "jigs: []\nbead:\n  id_pattern: '(unterminated'\n"
	if err := os.WriteFile(benchProj, []byte(bad), 0o644); err != nil {
		t.Fatalf("overwrite project.yaml with malformed pattern: %v", err)
	}

	t.Chdir(repo)
	isolatePATHKeepGit(t)

	// Build a transcript that would surface both D1 and D6 findings if
	// the pattern were valid — D1 must be disabled (no findings), D6
	// must still run (kerf-x91o decoupling).
	d1, err := os.ReadFile(fixturePath("../internal/kerftranscript/testdata/d1_abandon_a.jsonl"))
	if err != nil {
		t.Fatalf("read d1: %v", err)
	}
	d6, err := os.ReadFile(fixturePath("../internal/kerftranscript/testdata/d6_reviewer_absent_a.jsonl"))
	if err != nil {
		t.Fatalf("read d6: %v", err)
	}
	var lines strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&lines,
			`{"timestamp":"2026-05-14T10:%02d:00Z","kind":"dispatch","session_id":"pad-sess","sub_agent_id":"pad-%d","bead_id":"hk-pad-%03d","role":"implementer","text":"pad"}`+"\n",
			i, i, i)
	}
	fmt.Fprintf(&lines,
		`{"timestamp":"2026-05-14T11:00:00Z","kind":"dispatch","session_id":"pad-sess","sub_agent_id":"pad-rev","bead_id":"hk-pad-000","role":"reviewer","text":"Reviewer prompt for hk-pad-000 — pass: review"}`+"\n")
	lines.Write(d1)
	lines.Write(d6)

	tDir := filepath.Join(tmp, "transcripts")
	writeTranscript(t, tDir, "session.jsonl", lines.String())
	t.Setenv("KERF_TRANSCRIPT_DIR", tDir)

	resetNextFlags()
	t.Cleanup(resetNextFlags)
	projectFlag = ""
	t.Cleanup(func() { projectFlag = "" })

	// --- Text rendering ----------------------------------------------------
	var buf bytes.Buffer
	nextCmd.SetOut(&buf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext text (non-fatal warning) should not error; got: %v", err)
	}
	body := buf.String()
	// Spec: title.
	if !strings.Contains(body, "Corrupt project config: bead.id_pattern") {
		t.Errorf("text: missing corrupt_project_config title; got:\n%s", body)
	}
	// Reason fragments — verbatim Go regexp error + D1/D6-disabled clause.
	if !strings.Contains(body, "bead.id_pattern failed to compile:") {
		t.Errorf("text: missing reason prefix; got:\n%s", body)
	}
	if !strings.Contains(body, "D1 diagnostics disabled until fixed") {
		t.Errorf("text: missing D1-disabled clause; got:\n%s", body)
	}
	// Bead kerf-x91o: the reason field must not advertise D6 as disabled
	// (D6 does not consume bead.id_pattern).
	if strings.Contains(body, "D1 and D6 diagnostics disabled") {
		t.Errorf("text: reason must not couple D6 to bead.id_pattern (kerf-x91o); got:\n%s", body)
	}
	// Cardinality: exactly one corrupt_project_config row, even with
	// transcripts staged for both D1 and D6 (shared emission contract).
	if n := strings.Count(body, "Corrupt project config: bead.id_pattern"); n != 1 {
		t.Errorf("text: corrupt_project_config title count = %d, want 1; got:\n%s", n, body)
	}
	// D1 must be disabled: no per-bead warnings about hk-qo08q.15 from
	// this invocation. D6 is unaffected (kerf-x91o): "Reviewer absent:"
	// findings should still surface for the staged d6 fixture.
	if strings.Contains(body, "Abandoned dispatch:") {
		t.Errorf("text: D1 must be disabled when bead.id_pattern is corrupt; got:\n%s", body)
	}
	if !strings.Contains(body, "Reviewer absent:") {
		t.Errorf("text: D6 must still emit findings when bead.id_pattern is corrupt (kerf-x91o); got:\n%s", body)
	}
	// Non-fatal: footer tip still renders.
	if !strings.Contains(body, nextFooterTip) {
		t.Errorf("text: footer tip must render (non-fatal warning); got:\n%s", body)
	}

	// --- JSON rendering ----------------------------------------------------
	buf.Reset()
	resetNextFlags()
	nextFormat = "json"
	nextCmd.SetOut(&buf)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("runNext json: %v", err)
	}
	var items []feed.Item
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatalf("decode json: %v\nbody=%s", err, buf.String())
	}
	got := findItemByTitlePrefix(items, "Corrupt project config:")
	if got == nil {
		t.Fatalf("json: no corrupt_project_config item; items=%+v", items)
	}
	if n := countItemsByTitlePrefix(items, "Corrupt project config:"); n != 1 {
		t.Errorf("json: corrupt_project_config item count = %d, want 1; items=%+v", n, items)
	}
	if got.Kind != feed.KindWarning {
		t.Errorf("json: kind = %q, want %q", got.Kind, feed.KindWarning)
	}
	if got.Title != "Corrupt project config: bead.id_pattern" {
		t.Errorf("json: title = %q", got.Title)
	}
	if got.Action != "-" {
		t.Errorf("json: action = %q, want %q", got.Action, "-")
	}
	if !strings.Contains(got.Reason, "bead.id_pattern failed to compile:") ||
		!strings.Contains(got.Reason, "D1 diagnostics disabled until fixed") {
		t.Errorf("json: reason missing spec fragments; got %q", got.Reason)
	}
	if strings.Contains(got.Reason, "D1 and D6 diagnostics disabled") {
		t.Errorf("json: reason must not couple D6 to bead.id_pattern (kerf-x91o); got %q", got.Reason)
	}
	if got.BeadID != nil {
		t.Errorf("json: bead_id = %v, want nil for project-level warning", got.BeadID)
	}
	// D1 disabled, D6 still runs (kerf-x91o decoupling).
	if countItemsByTitlePrefix(items, "Abandoned dispatch:") != 0 {
		t.Errorf("json: D1 must be disabled; items=%+v", items)
	}
	if countItemsByTitlePrefix(items, "Reviewer absent:") == 0 {
		t.Errorf("json: D6 must still emit findings when bead.id_pattern is corrupt (kerf-x91o); items=%+v", items)
	}
}

// findItemByTitlePrefix returns the first item whose Title begins with
// prefix, or nil if none. Used to locate diagnostic warning items in
// JSON output where ordering across detectors is not load-bearing.
func findItemByTitlePrefix(items []feed.Item, prefix string) *feed.Item {
	for i := range items {
		if strings.HasPrefix(items[i].Title, prefix) {
			return &items[i]
		}
	}
	return nil
}

// countItemsByTitlePrefix returns the number of items whose Title begins
// with prefix. Used to assert that a given diagnostic warning is emitted
// exactly once (duplicate-emission regression guard).
func countItemsByTitlePrefix(items []feed.Item, prefix string) int {
	n := 0
	for i := range items {
		if strings.HasPrefix(items[i].Title, prefix) {
			n++
		}
	}
	return n
}
