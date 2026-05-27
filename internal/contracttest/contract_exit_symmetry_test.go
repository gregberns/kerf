// Plan 023 / B2 (kerf-gro2) — Contract: subprocess exit symmetry.
//
// Invariant. Every kerf cobra leaf command that materially depends on
// the bd/br bead-store subprocess (i.e. would refuse to do its job if
// the store returned an error) must exit non-zero when that subprocess
// exits non-zero. The shape of the regression this protects against:
// `kerf next` was observed (BLOCKER #3, dogfood run 2026-05-18) to
// return exit 0 even when br exited 1 — the per-command unit tests
// stubbed br to succeed, so no single test caught the asymmetry.
//
// What this test does. Walk the cobra tree via the B1 walker
// (Walk), filter out leaves that are registered as exempt
// (IsExempt) under contract id "subprocess-exit-symmetry", then for
// each remaining leaf:
//
//  1. Install a stub `br` on PATH that exits 1 with a diagnostic on
//     stderr (same shape as production bd/br failures).
//  2. Set up a minimal isolated HOME with a bare project dir (no
//     works, no .git) and pass --project=<id> so the command resolves
//     into that empty project.
//  3. Invoke the command end-to-end via rootCmd.Execute() with the
//     command-specific minimal arg fixture.
//  4. Assert err != nil. The exact error text is intentionally not
//     pinned: this is a contract test, not a wording test.
//
// What is exempt. Many kerf commands consult bd/br for best-effort
// enrichment (`kerf show` bead summary, `kerf new` filter detection,
// `kerf map` portfolio annotations, `kerf doctor` without --strict).
// These deliberately swallow subprocess errors and exit 0. Each is
// registered in opt_outs.go with a one-line rationale referencing this
// bead (kerf-gro2). Flipping any exemption to "must fail" is a future
// spec change, not a code-only change.
//
// In-process vs subprocess. The bead body offered a choice: in-process
// via rootCmd.Execute, or out-of-process via the scenariotest harness.
// We picked in-process — it stays inside the unit-test binary, runs
// fast, and exercises the same RunE path the failing dogfood scenario
// hit. The complementary out-of-process assertion lives in plan 022's
// scenariotest bead (kerf-cz2t, "Scenario D — failure-mode shakedown")
// which drives a real kerf binary against a failing bd. Either layer
// landing first is fine; together they pin the invariant from both
// sides.
//
// Spec ref. specs/testing.md §"Property-Based Tests" §"Cross-command
// contracts" — recognised contract id `subprocess-exit-symmetry`.
package contracttest

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gregberns/kerf/cmd"
)

const exitSymmetryContractID = "subprocess-exit-symmetry"

// shellOutLeaves maps the cobra-leaf path (as emitted by Walk) to the
// argv that follows the command name itself when the command is
// invoked. Only commands listed here are asserted by this contract; any
// other leaf is treated as "does not depend on the bead-store
// subprocess" and skipped — silently, because the contract is about
// commands that ARE supposed to fail loudly when bd does.
//
// Adding a new shell-out command: append it here (or, if it
// deliberately degrades silently, register an opt-out in opt_outs.go).
// Removing a shell-out: drop from this map AND from any opt-out entry.
//
// The string slice is "args after the leaf name", excluding --project
// (which is appended uniformly below).
var shellOutLeaves = map[string][]string{
	// kerf next — feed assembly, hard-requires bd (Plan 021,
	// BLOCKER #3 / kerf-gro2 motivating case).
	"kerf.next": nil,

	// kerf triage — drift report, explicitly errors when bd fails
	// (cmd/triage.go: "cannot read bead store: ...").
	"kerf.triage": nil,

	// kerf bootstrap-filters — proposal generator, errors when bd
	// fails (cmd/bootstrap_filters.go).
	"kerf.bootstrap-filters": nil,

	// kerf show <codename> — bead summary + attached-beads block both
	// surface bd subprocess errors (plan 022 / kerf-cz2t; opt-out
	// removed by plan 023 / kerf-61oi). Requires a seeded work.
	"kerf.show": {seededWorkCodename},

	// kerf work edit <codename> — pre/post attached-bead count surfaces
	// bd subprocess errors (plan 022 / kerf-cz2t; opt-out removed by
	// plan 023 / kerf-61oi). Requires a seeded work and at least one
	// --bead-filter-add/-remove flag.
	"kerf.work.edit": {seededWorkCodename, "--bead-filter-add", "label=ct"},
}

// seededWorkCodename is the codename of the minimal work seeded into
// the test's isolated HOME for leaves that take a `<codename>` arg.
// See seedMinimalWork.
const seededWorkCodename = "ct-work"

// TestContract_SubprocessExitSymmetry walks the cobra tree and asserts
// the invariant. See file-header comment for full design notes.
func TestContract_SubprocessExitSymmetry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("contract test uses a POSIX shell stub for bd/br")
	}

	leaves := Walk(t)
	if len(leaves) == 0 {
		t.Fatal("Walk returned zero leaves; cmd.Root() is empty or the walker is broken")
	}

	// Sanity: every entry in shellOutLeaves must correspond to a real
	// cobra leaf — catches drift between this maintained set and the
	// cobra tree (e.g. a command was renamed or removed).
	leafPathSet := make(map[string]struct{}, len(leaves))
	for _, l := range leaves {
		leafPathSet[l.Path] = struct{}{}
	}
	for path := range shellOutLeaves {
		if _, ok := leafPathSet[path]; !ok {
			t.Errorf("shellOutLeaves references %q but the cobra walker did not return it; update this set or restore the command", path)
		}
	}
	// Sanity: every opt-out under this contract id must also map to a
	// real leaf (typo / dead exemption catch).
	for key := range optOuts {
		path, cid, ok := splitExemptKey(key)
		if !ok || cid != exitSymmetryContractID {
			continue
		}
		if _, found := leafPathSet[path]; !found {
			t.Errorf("opt_outs.go references %q under %q but the cobra walker did not return that leaf", path, exitSymmetryContractID)
		}
	}

	for _, leaf := range leaves {
		extra, isShellOut := shellOutLeaves[leaf.Path]
		if !isShellOut {
			// Not in the maintained shell-out set — contract does not
			// apply (the command does not depend on bd for its core
			// function). Skip silently.
			continue
		}
		if IsExempt(leaf.Path, exitSymmetryContractID) {
			// Should not normally happen — a command is either in the
			// shell-out set OR exempt, not both. If it does, the
			// exemption wins and the bead naming the exemption must
			// rationalise the omission.
			continue
		}
		t.Run(leaf.Path, func(t *testing.T) {
			assertNonZeroOnSubprocessFailure(t, leaf.Path, extra)
		})
	}
}

// assertNonZeroOnSubprocessFailure runs `kerf <leaf...> [extra...]
// --project <id>` against a stub br that exits 1, and fails the test
// if the command returns a nil error from rootCmd.Execute.
func assertNonZeroOnSubprocessFailure(t *testing.T, leafPath string, extraArgs []string) {
	t.Helper()

	// --- Isolate HOME so cmdutil.Resolver builds inside this test. ---
	home := t.TempDir()
	t.Setenv("HOME", home)

	const projectID = "kerf-gro2-contract"
	projectDir := filepath.Join(home, ".kerf", "projects", projectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("seeding project dir: %v", err)
	}

	// Seed a minimal work for leaves that take a `<codename>` arg (kerf
	// show, kerf work edit). Other leaves ignore it. The spec.yaml is
	// the smallest doc that satisfies spec.Read / Validate; without a
	// work the command would error before reaching the bd subprocess
	// and the contract assertion would not actually exercise exit
	// symmetry. Added for plan 023 / kerf-61oi.
	seedMinimalWork(t, projectDir, projectID, seededWorkCodename)

	// --- Install a failing `br` stub on a fresh PATH. ----------------
	// We deliberately replace PATH (rather than prepend) so no real bd
	// installation on the developer's machine can leak in and mask the
	// failure. The stub matches the contract bd/br is expected to obey:
	// non-zero exit + a stderr diagnostic.
	stubDir := t.TempDir()
	stubPath := filepath.Join(stubDir, "br")
	stubScript := "#!/bin/sh\necho 'contract stub: simulated bd failure' >&2\nexit 1\n"
	if err := os.WriteFile(stubPath, []byte(stubScript), 0o755); err != nil {
		t.Fatalf("writing stub br: %v", err)
	}
	t.Setenv("PATH", stubDir)

	// --- Translate the dotted leaf path back into argv tokens. -------
	// "kerf.work.edit" → []string{"work", "edit"}; the leading "kerf"
	// is the rootCmd itself and is not part of SetArgs.
	argv := append([]string{}, leafToArgv(leafPath)...)
	argv = append(argv, extraArgs...)
	argv = append(argv, "--project", projectID)

	// --- Invoke through the assembled cobra tree. --------------------
	// Each Execute() call may scribble on package-global flags (e.g.
	// nextFormat, projectFlag). We don't bother to reset them: the
	// only flag this test sets is --project, and t.Cleanup-running
	// future tests inside the same `go test` binary will reset HOME &
	// PATH before they look at storage.
	root := cmd.Root()
	root.SetArgs(argv)
	var stdout, stderr bytes.Buffer
	root.SetOut(io.MultiWriter(&stdout, io.Discard))
	root.SetErr(io.MultiWriter(&stderr, io.Discard))
	t.Cleanup(func() {
		root.SetArgs(nil)
		root.SetOut(nil)
		root.SetErr(nil)
	})

	err := root.Execute()
	if err == nil {
		t.Fatalf(
			"contract violated: `kerf %v` returned nil error despite bd subprocess exit 1.\n"+
				"  stdout: %s\n"+
				"  stderr: %s\n"+
				"  expected: a non-nil error so the process exits non-zero (specs/testing.md §subprocess-exit-symmetry).\n"+
				"  fix: surface the subprocess error from the command's RunE — do not swallow it. If the command is intentionally best-effort, register an exemption in internal/contracttest/opt_outs.go.",
			argv, stdout.String(), stderr.String(),
		)
	}
}

// leafToArgv converts a dotted leaf path (e.g. "kerf.work.edit") into
// the argv tokens cobra.Execute expects (e.g. {"work", "edit"}). The
// leading "kerf" segment is the root and is dropped.
func leafToArgv(path string) []string {
	tokens := splitDots(path)
	if len(tokens) > 0 && tokens[0] == "kerf" {
		tokens = tokens[1:]
	}
	return tokens
}

func splitDots(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// splitExemptKey reverses exemptKey: "kerf.foo::contract-id" → ("kerf.foo", "contract-id", true).
// Returns ok=false for malformed keys (no "::" separator).
func splitExemptKey(key string) (path, contractID string, ok bool) {
	for i := 0; i+1 < len(key); i++ {
		if key[i] == ':' && key[i+1] == ':' {
			return key[:i], key[i+2:], true
		}
	}
	return "", "", false
}

// seedMinimalWork writes a minimal spec.yaml into projectDir/<codename>/
// so that contract leaves which take a `<codename>` arg can reach the bd
// subprocess. Without this, the command would error at the "work not
// found" step and exit-symmetry would not be exercised.
//
// Schema: minimum fields required by spec.Read / SpecYAML.Validate plus a
// bead_filter and a non-empty status_values list so `kerf work edit`'s
// add-clause flow has something to operate on. PinnedBeads is rendered
// explicitly to satisfy the required-on-write rule. Created/Updated are
// fixed RFC3339 strings — the contract doesn't care about their values.
func seedMinimalWork(t *testing.T, projectDir, projectID, codename string) {
	t.Helper()
	workDir := filepath.Join(projectDir, codename)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("seeding work dir: %v", err)
	}
	specYAML := "" +
		"codename: " + codename + "\n" +
		"type: feature\n" +
		"project:\n" +
		"  id: " + projectID + "\n" +
		"jig: default\n" +
		"jig_version: 1\n" +
		"status: open\n" +
		"status_values:\n" +
		"  - open\n" +
		"  - closed\n" +
		"created: 2026-01-01T00:00:00Z\n" +
		"updated: 2026-01-01T00:00:00Z\n" +
		"sessions: []\n" +
		"active_session: null\n" +
		"depends_on: []\n" +
		"pinned_beads: []\n" +
		"implementation:\n" +
		"  branch: null\n" +
		"  pr: null\n" +
		"  commits: []\n"
	if err := os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(specYAML), 0o644); err != nil {
		t.Fatalf("writing seed spec.yaml: %v", err)
	}
}

// --- Self-tests for the helpers above (so a refactor of splitDots /
// leafToArgv / splitExemptKey doesn't silently break the main contract).

func TestExitSymmetryHelpers(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		{"kerf.next", []string{"next"}},
		{"kerf.bootstrap-filters", []string{"bootstrap-filters"}},
		{"kerf.work.edit", []string{"work", "edit"}},
		{"kerf", []string{}},
	}
	for _, c := range cases {
		got := leafToArgv(c.path)
		if !equalStrings(got, c.want) {
			t.Errorf("leafToArgv(%q) = %v; want %v", c.path, got, c.want)
		}
	}

	path, cid, ok := splitExemptKey("kerf.next::subprocess-exit-symmetry")
	if !ok || path != "kerf.next" || cid != "subprocess-exit-symmetry" {
		t.Errorf("splitExemptKey: got (%q, %q, %v); want (kerf.next, subprocess-exit-symmetry, true)", path, cid, ok)
	}
	if _, _, ok := splitExemptKey("malformed-no-separator"); ok {
		t.Error("splitExemptKey accepted a malformed key")
	}
}

