package scenariotest

import (
	"os/exec"
	"strings"
	"testing"
)

// TestScenarioA_HappyPath drives the full agent bootstrap-to-finalize flow
// against a real `kerf` binary and a real `bd` store.
//
// Steps:
//  1. Harness `New` runs `bd init` in a tempdir, which also `git init`s.
//  2. We attach a fake `origin` remote so project ID derives from remote.
//  3. `kerf init --yes --jig spec` writes .kerf/project-identifier + project.yaml.
//  4. Seed several beads via `bd create`.
//  5. `kerf setup` prints the agent instruction block, including the
//     .gitignore snippet. We write that snippet to .gitignore and verify
//     `git add .kerf/project-identifier` works — this is the BLOCKER #2
//     regression guard.
//  6. `kerf bootstrap-filters --yes` runs against the seeded store.
//  7. `kerf new <codename>` creates a work.
//  8. `kerf next` is asserted to emit a non-empty feed.
//  9. `kerf work edit <codename> --bead-filter-add 'label=...'` mutates the
//     filter.
// 10. `kerf status <codename> <next-status>` advances through 2–3 statuses
//     of the spec jig.
// 11. `kerf review <codename>` prints the reviewer prompt.
// 12. `kerf doctor` exits cleanly.
//
// Skips when `bd` is not on PATH (via harness `New`).
func TestScenarioA_HappyPath(t *testing.T) {
	t.Parallel()

	r := New(t)

	// --- 2. Fake origin remote so project ID derives deterministically.
	// `bd init` (run by harness New) created a git repo + an initial commit
	// in the project root. We add a fake origin so kerf's project-ID
	// derivation hits the remote branch rather than the dir-name fallback.
	r.AttachFakeRemote("git@github.com:acme/scenario-a.git")

	// --- 3. kerf init --yes --jig spec
	// The harness automatically applies tools.tasks=bd after a successful
	// init (see Runner.UseTaskTool), so no separate config call needed.
	stdout, stderr, code, err := r.Run("init", "--yes", "--jig", "spec")
	mustOK(t, "kerf init", stdout, stderr, code, err)
	if !strings.Contains(stdout, "project-identifier") {
		t.Fatalf("kerf init: stdout missing project-identifier mention\nstdout: %s\nstderr: %s", stdout, stderr)
	}

	// --- 4. Seed beads. Labels include a "codename:" form so kerf's
	// auto-filter / bootstrap-filters logic has something to chew on.
	r.SeedBeads([]BeadSpec{
		{Title: "Implement login flow", Priority: "1", Labels: []string{"codename:auth", "kind:work"}},
		{Title: "Add login tests", Priority: "2", Labels: []string{"codename:auth", "kind:test"}},
		{Title: "Fix session bug", Priority: "1", Labels: []string{"codename:auth", "kind:bug"}},
		{Title: "Other unrelated work", Priority: "3", Labels: []string{"codename:other"}},
	})

	// --- 5. kerf setup — capture and apply the printed .gitignore block,
	// then assert `git add .kerf/project-identifier` succeeds (BLOCKER #2).
	stdout, stderr, code, err = r.Run("setup")
	mustOK(t, "kerf setup", stdout, stderr, code, err)
	if !strings.Contains(stdout, ".kerf/*") || !strings.Contains(stdout, "!.kerf/project-identifier") {
		t.Fatalf("kerf setup: expected .gitignore snippet in stdout\nstdout: %s", stdout)
	}
	// Append the canonical snippet to .gitignore (bd init already wrote one).
	existing := r.ReadFile(".gitignore")
	updated := existing
	if !strings.Contains(existing, ".kerf/*") {
		updated += "\n.kerf/*\n!.kerf/project-identifier\n"
	}
	r.WriteFile(".gitignore", updated)
	// BLOCKER #2 regression guard: git add the project-identifier file
	// should succeed (the negation rule must keep it visible to git).
	addOut, addErr := gitInRootCombined(t, r, "add", ".kerf/project-identifier")
	if addErr != nil {
		t.Fatalf("git add .kerf/project-identifier failed (BLOCKER #2 regression)\nout: %s\nerr: %v", addOut, addErr)
	}
	// Confirm git considers it staged.
	statusOut, _ := gitInRootCombined(t, r, "status", "--porcelain", ".kerf/project-identifier")
	if !strings.Contains(statusOut, ".kerf/project-identifier") {
		t.Fatalf("git did not stage .kerf/project-identifier: %q", statusOut)
	}

	// --- 6. kerf bootstrap-filters --yes
	stdout, stderr, code, err = r.Run("bootstrap-filters", "--yes")
	mustOK(t, "kerf bootstrap-filters", stdout, stderr, code, err)

	// --- 7. kerf new <codename>
	const codename = "auth"
	stdout, stderr, code, err = r.Run("new", codename, "--no-auto-filter")
	mustOK(t, "kerf new", stdout, stderr, code, err)
	if !strings.Contains(stdout, codename) {
		t.Fatalf("kerf new: stdout did not mention codename %q\nstdout: %s", codename, stdout)
	}

	// --- 8. kerf next — must return a ranked feed (non-empty).
	stdout, stderr, code, err = r.Run("next")
	mustOK(t, "kerf next", stdout, stderr, code, err)
	if strings.TrimSpace(stdout) == "" {
		t.Fatalf("kerf next: empty stdout\nstderr: %s", stderr)
	}

	// --- 9. kerf work edit --bead-filter-add 'label=codename:auth'
	stdout, stderr, code, err = r.Run("work", "edit", codename, "--bead-filter-add", "label=codename:auth")
	mustOK(t, "kerf work edit", stdout, stderr, code, err)

	// --- 10. kerf status <codename> <next> — advance through three statuses.
	// spec jig progression: problem-space -> decompose -> research -> ...
	for _, st := range []string{"decompose", "research", "change-design"} {
		stdout, stderr, code, err = r.Run("status", codename, st)
		mustOK(t, "kerf status "+st, stdout, stderr, code, err)
	}
	// Read-back status: confirm the spec.yaml ended on change-design.
	stdout, stderr, code, err = r.Run("status", codename)
	mustOK(t, "kerf status (read)", stdout, stderr, code, err)
	if !strings.Contains(stdout, "change-design") {
		t.Fatalf("kerf status (read): expected status change-design, got\n%s", stdout)
	}

	// --- 11. kerf review <codename>
	stdout, stderr, code, err = r.Run("review", codename)
	mustOK(t, "kerf review", stdout, stderr, code, err)
	// review must emit a reviewer prompt with criteria / artifacts.
	lower := strings.ToLower(stdout)
	if !strings.Contains(lower, "reviewer") && !strings.Contains(lower, "review") {
		t.Fatalf("kerf review: stdout did not look like a review prompt\nstdout: %s", stdout)
	}
	if !strings.Contains(lower, "approved") && !strings.Contains(lower, "criteria") &&
		!strings.Contains(lower, "done when") && !strings.Contains(lower, "artifact") {
		t.Fatalf("kerf review: missing review criteria / artifact markers\nstdout: %s", stdout)
	}

	// --- 12. kerf doctor — must exit cleanly.
	stdout, stderr, code, err = r.Run("doctor")
	mustOK(t, "kerf doctor", stdout, stderr, code, err)
}

// mustOK fails the test if the subprocess errored, did not exit 0, or the
// runner itself failed to start / hit a timeout. Reports stdout + stderr on
// failure so diagnostics survive the test runner.
func mustOK(t *testing.T, label, stdout, stderr string, code int, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: runner error: %v\nstdout: %s\nstderr: %s", label, err, stdout, stderr)
	}
	if code != 0 {
		t.Fatalf("%s: exit code %d (want 0)\nstdout: %s\nstderr: %s", label, code, stdout, stderr)
	}
}

// gitInRootCombined returns the combined output and the underlying error so
// callers can decide whether to fail or inspect.
func gitInRootCombined(t *testing.T, r *Runner, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.ProjectRoot()
	// Re-use the runner's scrubbed env so HOME points at the scenario tempdir.
	cmd.Env = r.Env()
	out, err := cmd.CombinedOutput()
	return string(out), err
}
