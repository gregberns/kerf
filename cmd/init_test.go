package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/testutil"
)

// resetInitFlags clears package-level init flag state so tests don't leak
// configuration into one another. Tests that flip a flag should call this in
// a defer (or use t.Cleanup) so a subsequent test starts from the documented
// defaults — all bools false, all strings empty.
func resetInitFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		initJigFlag = ""
		initForceFlag = false
		initYesFlag = false
		initNoFlag = false
		initBeadFilterFlag = ""
	})
}

// runInitInRepo bootstraps a fresh git repo with a clean HOME, chdirs in, and
// runs `kerf init` (with the flags already set on the package vars) under
// captureOutput. Returns the captured stdout, the project ID, and the
// resolved project.yaml path.
func runInitInRepo(t *testing.T) (out, projectID, projCfgPath string) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	gitRepo := testutil.SetupGitRepo(t)
	oldWd, _ := os.Getwd()
	if err := os.Chdir(gitRepo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(oldWd) })

	out = captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("init: %v", err)
		}
	})
	pidData, err := os.ReadFile(filepath.Join(gitRepo, ".kerf", "project-identifier"))
	if err != nil {
		t.Fatalf("reading project-identifier: %v", err)
	}
	projectID = strings.TrimSpace(string(pidData))
	projCfgPath = config.ProjectConfigPath(filepath.Join(tmp, ".kerf"), projectID)
	return out, projectID, projCfgPath
}

// runInitAgain re-runs init from the current working directory (assumed to
// be a kerf-initialised git repo) and returns the captured output.
func runInitAgain(t *testing.T) string {
	t.Helper()
	return captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("re-init: %v", err)
		}
	})
}

// --- Fresh init: project.yaml and identifier created. -----------------------

func TestInit_FreshRun_CreatesProjectIdentifierAndConfig(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[]`) // empty store → detector returns no suggestion, but init still succeeds.

	out, projectID, projCfgPath := runInitInRepo(t)

	if projectID == "" {
		t.Fatal("project-identifier should have a non-empty body")
	}
	testutil.AssertFileExists(t, projCfgPath)

	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	if len(cfg.Jigs) == 0 {
		t.Error("project.yaml should declare at least one jig")
	}
	has := func(name string) bool {
		for _, j := range cfg.Jigs {
			if j == name {
				return true
			}
		}
		return false
	}
	if !has("plan") || !has("bug") {
		t.Errorf("expected default jigs to include plan and bug, got %v", cfg.Jigs)
	}
	testutil.AssertStringContains(t, out, "project.yaml")
	testutil.AssertStringContains(t, out, "active jigs")
}

// Spec §kerf init step 11: the AGENT SETUP INSTRUCTIONS block must appear
// exactly once in init's stdout — kerf setup is the single source (kerf-6jw).
func TestInit_OutputContainsExactlyOneSetupBlock(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[]`)
	out, _, _ := runInitInRepo(t)

	if c := strings.Count(out, "START AGENT INSTRUCTIONS"); c != 1 {
		t.Errorf("expected 1 'START AGENT INSTRUCTIONS' block, got %d", c)
	}
	if c := strings.Count(out, "END AGENT INSTRUCTIONS"); c != 1 {
		t.Errorf("expected 1 'END AGENT INSTRUCTIONS' block, got %d", c)
	}
}

// Spec §kerf init Output bullet 7 (kerf-yl1): the final block is the
// state-change summary. Per-artifact verbs are one of created / updated /
// unchanged, and the artifacts init touched on a fresh run include
// project-identifier, project.yaml, and bead_filter.
func TestInit_StateChangeSummary_FreshRunShape(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[]`)
	out, _, _ := runInitInRepo(t)

	if !strings.Contains(out, "State changes:") {
		t.Fatalf("missing state-change block; got:\n%s", out)
	}
	rows := parseStateChanges(t, out)
	wantVerbs := map[string]string{
		".kerf/project-identifier": "created",
		"project.yaml":             "created",
		"bead_filter":              "unchanged", // empty store → no confident suggestion.
	}
	for artifact, want := range wantVerbs {
		got, ok := rows[artifact]
		if !ok {
			t.Errorf("state-change summary missing artifact %q; rows=%v", artifact, rows)
			continue
		}
		if got != want {
			t.Errorf("artifact %q: want verb %q, got %q", artifact, want, got)
		}
	}
	// Allowed verbs only.
	allowed := map[string]bool{"created": true, "updated": true, "unchanged": true}
	for artifact, verb := range rows {
		if !allowed[verb] {
			t.Errorf("artifact %q reported unknown verb %q", artifact, verb)
		}
	}
}

// --jig persists default_jig in project.yaml (kerf-q5l) and reports it in the
// state-change summary.
func TestInit_JigFlag_PersistsDefaultJigInProjectYAML(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[]`)
	initJigFlag = "spec"

	out, _, projCfgPath := runInitInRepo(t)

	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	if cfg.DefaultJig != "spec" {
		t.Errorf("project.yaml default_jig: want %q, got %q", "spec", cfg.DefaultJig)
	}
	rows := parseStateChanges(t, out)
	if v, ok := rows["default_jig"]; !ok || v != "created" {
		t.Errorf("expected default_jig 'created' row, got %q (rows=%v)", v, rows)
	}
}

// --jig with an invalid value errors before writing anything.
func TestInit_JigFlag_RejectsInvalidValue(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[]`)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	gitRepo := testutil.SetupGitRepo(t)
	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	t.Cleanup(func() { os.Chdir(oldWd) })

	initJigFlag = "nonsense"
	err := captureErr(func() error { return initCmd.RunE(initCmd, []string{}) })
	if err == nil || !strings.Contains(err.Error(), "--jig must be 'plan' or 'spec'") {
		t.Fatalf("expected --jig validation error, got %v", err)
	}
}

// --- Flag matrix: --yes / --no / --bead-filter / default. -------------------

// --bead-filter sets the literal verbatim and bypasses the detector
// (kerf-pjs precedence: --bead-filter > --no > --yes > default).
func TestInit_BeadFilterFlag_SetsExplicitLiteralAndBypassesDetector(t *testing.T) {
	resetInitFlags(t)
	// br store would otherwise produce a confident "subsystem:*" suggestion;
	// --bead-filter must win.
	stubBr(t, `[
		{"id":"x-1","labels":["subsystem:auth"]},
		{"id":"x-2","labels":["subsystem:db"]},
		{"id":"x-3","labels":["subsystem:api"]}
	]`)
	initBeadFilterFlag = "label=team:billing"

	out, _, projCfgPath := runInitInRepo(t)

	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	if cfg.BeadFilter == nil || cfg.BeadFilter.Label != "team:billing" {
		t.Errorf("bead_filter: want label=team:billing, got %+v", cfg.BeadFilter)
	}
	// Detector output must not appear — explicit flag bypasses detection.
	if strings.Contains(out, "Detected:") {
		t.Errorf("detector output leaked despite --bead-filter; out=%q", out)
	}
}

// --bead-filter with a malformed literal errors before any write.
func TestInit_BeadFilterFlag_RejectsMalformedLiteral(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[]`)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	gitRepo := testutil.SetupGitRepo(t)
	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	t.Cleanup(func() { os.Chdir(oldWd) })

	initBeadFilterFlag = "this is not a clause"
	err := captureErr(func() error { return initCmd.RunE(initCmd, []string{}) })
	if err == nil || !strings.Contains(err.Error(), "--bead-filter expects") {
		t.Fatalf("expected --bead-filter validation error, got %v", err)
	}
}

// --no skips detection entirely; bead_filter stays unset even when the store
// has a confident candidate (kerf-pjs).
func TestInit_NoFlag_SkipsDetectionLeavingBeadFilterUnset(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[
		{"id":"x-1","labels":["subsystem:auth"]},
		{"id":"x-2","labels":["subsystem:db"]},
		{"id":"x-3","labels":["subsystem:api"]}
	]`)
	initNoFlag = true

	out, _, projCfgPath := runInitInRepo(t)

	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	if cfg.BeadFilter != nil {
		t.Errorf("--no must leave bead_filter unset, got %+v", cfg.BeadFilter)
	}
	if strings.Contains(out, "Detected:") {
		t.Errorf("--no must keep detector silent; out=%q", out)
	}
}

// --yes accepts a confident detector suggestion (the default path), but —
// because the worktree has no codenames yet on a first init — the detector
// stays silent and bead_filter remains unset. The flag is then a no-op
// relative to the default. This test pins behavior: --yes never errors.
func TestInit_YesFlag_OnFirstInit_LeavesBeadFilterUnset(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[
		{"id":"x-1","labels":["subsystem:foo"]},
		{"id":"x-2","labels":["subsystem:bar"]},
		{"id":"x-3","labels":["subsystem:baz"]}
	]`)
	initYesFlag = true

	_, _, projCfgPath := runInitInRepo(t)
	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	// No work codenames seeded yet → detector silent → bead_filter unset.
	if cfg.BeadFilter != nil {
		t.Errorf("expected bead_filter unset on first init with --yes, got %+v", cfg.BeadFilter)
	}
}

// --yes and --no together is a flag error.
func TestInit_YesAndNoFlags_MutuallyExclusive(t *testing.T) {
	resetInitFlags(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	gitRepo := testutil.SetupGitRepo(t)
	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	t.Cleanup(func() { os.Chdir(oldWd) })

	initYesFlag = true
	initNoFlag = true
	err := captureErr(func() error { return initCmd.RunE(initCmd, []string{}) })
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutual-exclusion error, got %v", err)
	}
}

// --- Detector tri-state confidence (kerf-yxl) routed through init. ---------

// A dominant prefix above both floors is auto-applied on --force re-run.
func TestInit_DetectorConfident_AutoAppliesPrefix(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[]`)

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	gitRepo := testutil.SetupGitRepo(t)
	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	t.Cleanup(func() { os.Chdir(oldWd) })

	captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("first init: %v", err)
		}
	})
	pidData, _ := os.ReadFile(filepath.Join(gitRepo, ".kerf", "project-identifier"))
	projectID := strings.TrimSpace(string(pidData))
	worksDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	for _, cn := range []string{"foo", "bar", "baz", "qux"} {
		if err := os.MkdirAll(filepath.Join(worksDir, cn), 0o755); err != nil {
			t.Fatalf("seed work %s: %v", cn, err)
		}
	}
	// 5/5 work:* match codenames → score 1.0, ConfidenceConfident.
	stubBr(t, `[
		{"id":"b-1","labels":["work:foo"]},
		{"id":"b-2","labels":["work:bar"]},
		{"id":"b-3","labels":["work:baz"]},
		{"id":"b-4","labels":["work:qux"]},
		{"id":"b-5","labels":["work:foo"]}
	]`)

	initForceFlag = true
	out := runInitAgain(t)

	if !strings.Contains(out, "Detected:") {
		t.Errorf("expected 'Detected:' line for confident suggestion; out=%q", out)
	}
	projCfgPath := config.ProjectConfigPath(filepath.Join(tmp, ".kerf"), projectID)
	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	if cfg.BeadFilter == nil || cfg.BeadFilter.Label != "work:{codename}" {
		t.Errorf("expected bead_filter work:{codename}, got %+v", cfg.BeadFilter)
	}
}

// A 1-bead corpus produces ConfidenceNone (kerf-yxl) — detector stays silent
// and bead_filter remains unset. No 'Detected:' line.
func TestInit_DetectorNone_OnTinyCorpus_SilentNoFilter(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[{"id":"b-1","labels":["subsystem:auth"]}]`)

	out, _, projCfgPath := runInitInRepo(t)

	if strings.Contains(out, "Detected:") {
		t.Errorf("1-bead corpus must stay silent; out=%q", out)
	}
	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	if cfg.BeadFilter != nil {
		t.Errorf("expected bead_filter unset on tiny corpus, got %+v", cfg.BeadFilter)
	}
}

// A corpus that clears the count floor but not the score floor produces
// ConfidenceLow (kerf-yxl). Init stays silent — no auto-applied filter.
func TestInit_DetectorLow_NoConfidentSuggestion_SilentNoFilter(t *testing.T) {
	resetInitFlags(t)
	// 3 beads under one prefix (count floor met) but none of the tail
	// segments match codenames → score 0, below score floor.
	stubBr(t, `[
		{"id":"x-1","labels":["subsystem:foo"]},
		{"id":"x-2","labels":["subsystem:bar"]},
		{"id":"x-3","labels":["subsystem:baz"]}
	]`)

	// runInitInRepo creates no work codenames, so no tail can match.
	out, _, projCfgPath := runInitInRepo(t)
	if strings.Contains(out, "Detected:") {
		t.Errorf("low-confidence corpus must stay silent; out=%q", out)
	}
	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	if cfg.BeadFilter != nil {
		t.Errorf("expected bead_filter unset on low-confidence corpus, got %+v", cfg.BeadFilter)
	}
}

// --- Idempotency (spec §"Re-running on an existing project"). --------------

// Re-run without --force prints the skip-with-informative-output summary,
// keeps the existing project.yaml byte-identical, and preserves the
// hand-set bead_filter.
func TestInit_RerunWithoutForce_PreservesProjectYAMLAndBeadFilter(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[]`)

	_, projectID, projCfgPath := runInitInRepo(t)

	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	cfg.BeadFilter = &beads.Filter{Label: "team:{codename}"}
	if err := config.SaveProjectConfig(projCfgPath, cfg); err != nil {
		t.Fatalf("saving project config: %v", err)
	}
	originalContent, _ := os.ReadFile(projCfgPath)

	out := runInitAgain(t)
	testutil.AssertStringContains(t, out, "already exists at")
	testutil.AssertStringContains(t, out, "skipping re-initialisation")
	testutil.AssertStringContains(t, out, "Active jigs")
	testutil.AssertStringContains(t, out, "team:{codename}")
	testutil.AssertStringContains(t, out, "kerf init --force")

	after, _ := os.ReadFile(projCfgPath)
	if string(after) != string(originalContent) {
		t.Errorf("project.yaml was overwritten on re-init without --force")
	}
	cfg2, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("reloading project config: %v", err)
	}
	if cfg2.BeadFilter == nil || cfg2.BeadFilter.Label != "team:{codename}" {
		t.Errorf("bead_filter not preserved: got %+v", cfg2.BeadFilter)
	}
	_ = projectID
}

// --force on an existing project.yaml warns, rewrites the file, and still
// preserves a hand-set bead_filter when the detector has no new suggestion.
func TestInit_RerunWithForce_RewritesAndPreservesPriorBeadFilter(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[]`)

	_, _, projCfgPath := runInitInRepo(t)

	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	cfg.BeadFilter = &beads.Filter{Label: "team:{codename}"}
	if err := config.SaveProjectConfig(projCfgPath, cfg); err != nil {
		t.Fatalf("saving project config: %v", err)
	}
	if err := os.WriteFile(projCfgPath, []byte("jigs: []\n# sentinel\n"+`bead_filter:
  label: team:{codename}
`), 0o644); err != nil {
		t.Fatalf("seed sentinel: %v", err)
	}

	initForceFlag = true
	out := runInitAgain(t)

	testutil.AssertStringContains(t, out, "overwriting existing project.yaml")
	after, _ := os.ReadFile(projCfgPath)
	if strings.Contains(string(after), "sentinel") {
		t.Errorf("--force did not rewrite project.yaml; sentinel remains")
	}
	cfg2, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("reloading after force: %v", err)
	}
	if len(cfg2.Jigs) == 0 {
		t.Errorf("expected jigs repopulated by --force")
	}
	if cfg2.BeadFilter == nil || cfg2.BeadFilter.Label != "team:{codename}" {
		t.Errorf("--force must preserve prior bead_filter when detector has nothing; got %+v", cfg2.BeadFilter)
	}
}

// --- Audit/util ------------------------------------------------------------

// captureErr runs fn under captured stdout (so the test log stays clean) and
// returns the error fn produced.
func captureErr(fn func() error) error {
	var err error
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = fn()
	w.Close()
	os.Stdout = old
	// Drain the pipe so the goroutine doesn't leak.
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, e := r.Read(buf); e != nil {
				return
			}
		}
	}()
	return err
}

// parseStateChanges extracts the fenced state-change block from init output
// and returns a map of artifact → verb. The block format is fixed per spec:
//
//	```
//	State changes:
//	  <artifact>   <verb> [(detail)]
//	  ...
//	```
//
// Detail (in parentheses) is intentionally discarded — tests assert verbs.
func parseStateChanges(t *testing.T, out string) map[string]string {
	t.Helper()
	rows := map[string]string{}
	idx := strings.Index(out, "State changes:")
	if idx < 0 {
		return rows
	}
	// The header is the first line inside the fenced block. Body is from
	// the line after the header to the next closing fence.
	tail := out[idx:]
	nl := strings.Index(tail, "\n")
	if nl < 0 {
		return rows
	}
	rest := tail[nl+1:]
	fenceEnd := strings.Index(rest, "```")
	if fenceEnd < 0 {
		return rows
	}
	body := rest[:fenceEnd]
	// Each line: "  <artifact>   <verb>[ (detail)]"
	lineRe := regexp.MustCompile(`^\s*([\S]+(?:\s+[\S]+)*?)\s{2,}(created|updated|unchanged)\b`)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" {
			continue
		}
		m := lineRe.FindStringSubmatch(line)
		if m == nil {
			t.Logf("state-change line did not parse: %q", line)
			continue
		}
		rows[m[1]] = m[2]
	}
	return rows
}

// kerf-dlb: a corrupt .kerf/project-identifier (garbage bytes / control chars
// / path separators) used to flow through to mkdir(2) and surface a low-level
// Go error. The fix validates the identifier on read in internal/project and
// surfaces a clear, actionable message at the kerf init layer.
func TestInit_CorruptProjectIdentifier_RejectedWithClearError(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[]`)

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	gitRepo := testutil.SetupGitRepo(t)
	oldWd, _ := os.Getwd()
	if err := os.Chdir(gitRepo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(oldWd) })

	if err := os.MkdirAll(filepath.Join(gitRepo, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	garbage := []byte("bad/\x00id\n")
	idPath := filepath.Join(gitRepo, ".kerf", "project-identifier")
	if err := os.WriteFile(idPath, garbage, 0o644); err != nil {
		t.Fatal(err)
	}

	err := captureErr(func() error {
		return initCmd.RunE(initCmd, []string{})
	})
	if err == nil {
		t.Fatal("expected error when project-identifier is corrupt, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"corrupt project identifier", idPath, "replace with a clean slug"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}

	// Bytes on disk are unchanged — kerf must not paper over the corruption.
	after, readErr := os.ReadFile(idPath)
	if readErr != nil {
		t.Fatalf("reading project-identifier after refused init: %v", readErr)
	}
	if string(after) != string(garbage) {
		t.Errorf("corrupt project-identifier was modified; want %q got %q", garbage, after)
	}
}

// kerf-45x: malformed project.yaml on rerun must NOT be silently overwritten.
// The fix in cmd/init.go distinguishes "parse failed" from "missing file" and
// errors out, preserving the original bytes on disk so a user can recover by
// hand (or re-run with --force).
func TestInit_RerunWithMalformedProjectYAML_RefusesToOverwrite(t *testing.T) {
	resetInitFlags(t)
	stubBr(t, `[]`)

	_, _, projCfgPath := runInitInRepo(t)

	// Corrupt project.yaml with syntactically invalid YAML.
	garbage := []byte("jigs: [unclosed\n: : :\n\tnot yaml at all\n")
	if err := os.WriteFile(projCfgPath, garbage, 0o644); err != nil {
		t.Fatalf("writing malformed project.yaml: %v", err)
	}

	err := captureErr(func() error {
		return initCmd.RunE(initCmd, []string{})
	})
	if err == nil {
		t.Fatal("expected error from re-init with malformed project.yaml, got nil")
	}
	msg := err.Error()
	for _, want := range []string{projCfgPath, "could not be parsed", "--force"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}

	// Original bytes must be preserved on disk.
	after, readErr := os.ReadFile(projCfgPath)
	if readErr != nil {
		t.Fatalf("reading project.yaml after refused re-init: %v", readErr)
	}
	if string(after) != string(garbage) {
		t.Errorf("malformed project.yaml was modified; want %q got %q", garbage, after)
	}
}
