package scenariotest

// Scenario + exploratory tests for the `validation-section-coverage`
// detector (Plan 025 — kerf-w6k3 / plan-025-B5: self-dogfood Validation
// block for plan 025 itself).
//
// Spec sentence the scenario claims to satisfy
// (specs/commands.md §"kerf doctor" §Detectors line ~1601):
//
//     `validation-section-coverage` — reports each active work using a
//     plan / spec / bug / implementation jig whose affected-pass
//     artifact does not list both a scenario-test item ID and an
//     exploratory-test item ID in its "What done looks like" checklist.
//     Severity yellow. Hint names the backfill file and section.
//
// Plus the warn-only / non-blocking property: `kerf doctor` exits 0
// even when this detector fires (specs/commands.md §"kerf doctor"
// §"Exit codes"), and excluded jigs (retrofit/spike) and archived
// works produce no finding (specs/jig-system.md §"Retrofit and spike
// exclusion").
//
// Tracker item IDs filed for this Validation block:
//   - scenario: `kerf-w6k3-s` — "scenario: jig-validation — doctor
//     warns on missing validation IDs"
//   - exploratory: `kerf-w6k3-x` — "explore: jig-validation — doctor
//     and next render the finding sensibly across edge cases"
//
// (Bead IDs are locally-scoped suffixes of kerf-w6k3; this follows the
// project's "test item IDs travel inside the bead's own ID space"
// convention used by the doctor unit tests under
// internal/doctor/*_test.go which reference their bead ID in
// helper-function names.)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeSpecYAML writes a minimal spec.yaml for a work under the given
// project bench directory. The scenario uses the bench layout directly
// (`<bench>/projects/<id>/<codename>/`) because `kerf new --jig` walks
// the user through a multi-pass authoring flow that is out of scope for
// this detector test — the detector only reads spec.yaml + artifact
// files.
func writeSpecYAML(t *testing.T, workDir, codename, jig, status string) {
	t.Helper()
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", workDir, err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	body := "codename: " + codename + "\n" +
		"type: implementation\n" +
		"project:\n  id: p-w6k3\n" +
		"jig: " + jig + "\n" +
		"jig_version: 1\n" +
		"status: " + status + "\n" +
		"status_values:\n  - " + status + "\n" +
		"created: " + now + "\n" +
		"updated: " + now + "\n" +
		"sessions: []\n" +
		"depends_on: []\n" +
		"pinned_beads: []\n" +
		"implementation:\n" +
		"  branch: null\n" +
		"  pr: null\n" +
		"  commits: []\n"
	if err := os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write spec.yaml: %v", err)
	}
}

// writeArtifact writes content at rel under workDir, mkdir-as-needed.
func writeArtifact(t *testing.T, workDir, rel, content string) {
	t.Helper()
	abs := filepath.Join(workDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
}

// initBenchProject runs `kerf init` and returns the project bench dir
// (where spec.yaml + work dirs live: `<bench>/projects/<id>/`).
func initBenchProject(t *testing.T, r *Runner) string {
	t.Helper()
	stdout, stderr, code, err := r.Run("init", "--no", "--jig", "spec")
	if err != nil {
		t.Fatalf("kerf init: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if code != 0 {
		t.Fatalf("kerf init exited %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	projectID := strings.TrimSpace(r.ReadFile(".kerf/project-identifier"))
	if projectID == "" {
		t.Fatalf("project-identifier empty after kerf init")
	}
	return filepath.Join(r.BenchDir(), "projects", projectID)
}

// compliantWDLL is a "What done looks like" block with both IDs filled.
const compliantWDLL = `# Tasks

Some prose.

**What done looks like:**

- The thing exists
- Scenario-test item filed with ID ` + "`kerf-aaa`" + `
- Exploratory-test item filed with ID ` + "`kerf-bbb`" + `
`

// missingItemsWDLL has the WDLL block but no scenario/exploratory lines.
const missingItemsWDLL = `# Fix Spec

**What done looks like:**

- Fix shipped
- Regression test in place
`

// emptyIDWDLL has both lines but `<id>` placeholders.
const emptyIDWDLL = `# Breakdown

**What done looks like:**

- Beads filed
- Scenario-test item filed with ID ` + "`<id>`" + `
- Exploratory-test item filed with ID ` + "`<id>`" + `
`

// TestScenario_ValidationCoverage_MixedFixture — kerf-w6k3-s.
//
// End-to-end scenario for the validation-section-coverage detector.
// Builds a realistic mixed fixture of four works under the kerf bench:
//
//   - wing-green   — plan jig, both 07-tasks.md and 05-specs/*-spec.md
//                    list real IDs. Expected: no finding for this work.
//   - wing-missing — bug jig, 05-fix-spec.md is missing both items.
//                    Expected: yellow with "missing both".
//   - wing-empty   — implementation jig, 01-breakdown.md has both lines
//                    but `<id>` placeholders. Expected: yellow with
//                    "empty <id>".
//   - wing-retro   — retrofit jig (excluded) with a broken artifact.
//                    Expected: no finding.
//   - wing-arch    — bug jig but spec.status: archived (excluded).
//                    Expected: no finding.
//
// Then runs `kerf doctor` against the real binary and asserts:
//   1. Exit code 0 (warn-only).
//   2. Output contains `[yellow]` and the `validation-section-coverage`
//      summary.
//   3. Output names both offending works (wing-missing, wing-empty) and
//      neither of the excluded works (wing-retro, wing-arch, wing-green).
//   4. Hint line names a `.md` artifact path and references
//      "What done looks like".
//   5. No Go runtime panic stack.
//
// Then patches wing-missing's artifact to add both IDs and re-runs
// `kerf doctor`, asserting the wing-missing offender disappears (but
// wing-empty remains).
func TestScenario_ValidationCoverage_MixedFixture(t *testing.T) {
	r := New(t)
	projDir := initBenchProject(t, r)

	// --- Fixture --------------------------------------------------------

	// wing-green: plan jig — Tasks + Change Spec both compliant.
	green := filepath.Join(projDir, "wing-green")
	writeSpecYAML(t, green, "wing-green", "plan", "tasks")
	writeArtifact(t, green, "07-tasks.md", compliantWDLL)
	writeArtifact(t, green, "05-specs/core-spec.md", compliantWDLL)

	// wing-missing: bug jig — fix-spec missing both items.
	missing := filepath.Join(projDir, "wing-missing")
	writeSpecYAML(t, missing, "wing-missing", "bug", "fix-spec")
	writeArtifact(t, missing, "05-fix-spec.md", missingItemsWDLL)

	// wing-empty: implementation jig — breakdown has empty <id>.
	empty := filepath.Join(projDir, "wing-empty")
	writeSpecYAML(t, empty, "wing-empty", "implementation", "breakdown")
	writeArtifact(t, empty, "01-breakdown.md", emptyIDWDLL)

	// wing-retro: retrofit jig with broken artifact — must be excluded.
	retro := filepath.Join(projDir, "wing-retro")
	writeSpecYAML(t, retro, "wing-retro", "retrofit", "drafting")
	writeArtifact(t, retro, "05-fix-spec.md", missingItemsWDLL)

	// wing-arch: bug jig but archived — must be excluded.
	arch := filepath.Join(projDir, "wing-arch")
	writeSpecYAML(t, arch, "wing-arch", "bug", "archived")
	writeArtifact(t, arch, "05-fix-spec.md", missingItemsWDLL)

	// --- Step 1: initial doctor run ------------------------------------

	stdout, stderr, code, err := r.Run("doctor", "--detector", "validation-section-coverage")
	if err != nil {
		t.Fatalf("kerf doctor: %v\nstderr: %s", err, stderr)
	}
	combined := stdout + stderr

	if code != 0 {
		t.Fatalf("kerf doctor exit = %d, want 0 (warn-only)\nstdout: %s\nstderr: %s",
			code, stdout, stderr)
	}
	if strings.Contains(combined, "panic:") || strings.Contains(combined, "goroutine 1 [running]") {
		t.Fatalf("kerf doctor produced a panic stack:\n%s", combined)
	}
	if !strings.Contains(combined, "[yellow]") {
		t.Errorf("expected [yellow] finding in output; got:\n%s", combined)
	}
	if !strings.Contains(combined, "validation-section-coverage") {
		t.Errorf("expected 'validation-section-coverage' detector summary; got:\n%s", combined)
	}
	// Offenders must be named.
	if !strings.Contains(combined, "wing-missing") {
		t.Errorf("expected 'wing-missing' in output; got:\n%s", combined)
	}
	if !strings.Contains(combined, "wing-empty") {
		t.Errorf("expected 'wing-empty' in output; got:\n%s", combined)
	}
	// Excluded works must NOT be named in the validation-section-coverage
	// finding's items. (They could still appear in other detectors' output
	// hypothetically; we scope the check to lines under that finding.)
	if strings.Contains(combined, "wing-retro") {
		t.Errorf("wing-retro (retrofit) must be excluded but appears in output:\n%s", combined)
	}
	if strings.Contains(combined, "wing-arch") {
		t.Errorf("wing-arch (archived) must be excluded but appears in output:\n%s", combined)
	}
	if strings.Contains(combined, "wing-green") {
		// The detector's per-work items only mention offenders. wing-green
		// being absent is the assertion.
		t.Errorf("wing-green (all-compliant) must not appear in offenders; got:\n%s", combined)
	}
	// Hint must reference an artifact .md path and the section.
	if !strings.Contains(combined, "What done looks like") {
		t.Errorf("hint missing 'What done looks like' section reference:\n%s", combined)
	}
	if !strings.Contains(combined, ".md") {
		t.Errorf("hint missing .md artifact path:\n%s", combined)
	}

	// --- Step 2: fix wing-missing and re-run ---------------------------

	writeArtifact(t, missing, "05-fix-spec.md", compliantWDLL)

	stdout, stderr, code, err = r.Run("doctor", "--detector", "validation-section-coverage")
	if err != nil {
		t.Fatalf("kerf doctor (post-fix): %v\nstderr: %s", err, stderr)
	}
	combined = stdout + stderr

	if code != 0 {
		t.Fatalf("kerf doctor (post-fix) exit = %d, want 0\nstdout: %s\nstderr: %s",
			code, stdout, stderr)
	}
	if strings.Contains(combined, "wing-missing") {
		t.Errorf("wing-missing should be cleared after fix; got:\n%s", combined)
	}
	if !strings.Contains(combined, "wing-empty") {
		t.Errorf("wing-empty should still appear (still has empty <id>); got:\n%s", combined)
	}
	if !strings.Contains(combined, "[yellow]") {
		t.Errorf("expected yellow still present (wing-empty remains):\n%s", combined)
	}

	// --- Step 3: fix wing-empty too — expect green ---------------------

	writeArtifact(t, empty, "01-breakdown.md", compliantWDLL)

	stdout, stderr, code, err = r.Run("doctor")
	if err != nil {
		t.Fatalf("kerf doctor (all-clean): %v\nstderr: %s", err, stderr)
	}
	combined = stdout + stderr
	if code != 0 {
		t.Fatalf("kerf doctor (all-clean) exit = %d, want 0\nstdout: %s\nstderr: %s",
			code, stdout, stderr)
	}
	// Must contain the green summary for the detector, or at least no
	// yellow for this detector. The cleanest check is the green-summary
	// string from the detector source.
	if !strings.Contains(combined, "all affected-pass artifacts list scenario and exploratory IDs") {
		t.Errorf("expected detector green summary after fixes; got:\n%s", combined)
	}
}

// TestExploratory_ValidationCoverage_EdgeCases — kerf-w6k3-x.
//
// Exploratory investigation of how the validation-section-coverage
// detector behaves on edge cases that the unit tests don't cover, run
// end-to-end through the real kerf binary. Each sub-case documents the
// question being asked, the setup, and the finding.
//
// Convention note: this project has no pre-existing "exploratory test"
// shape — the doctor unit tests and the scenario tests under
// internal/scenariotest/ are both structured `Test*` functions. I'm
// following the closest local convention: a single Test* function with
// t.Run sub-tests, each prefixed `Q<N>:` for the question being asked,
// and a t.Log line per sub-test recording the observed answer. This
// makes the test pass/fail as a unit while making the investigation
// findings visible in `-v` output.
func TestExploratory_ValidationCoverage_EdgeCases(t *testing.T) {
	r := New(t)
	projDir := initBenchProject(t, r)

	runDoctor := func(t *testing.T) (combined string, code int) {
		t.Helper()
		// Scope to the detector under test so substring assertions are not
		// polluted by sibling detectors (notably bead_filter coverage, which
		// names every work that lacks a bead_filter).
		stdout, stderr, code, err := r.Run("doctor", "--detector", "validation-section-coverage")
		if err != nil {
			t.Fatalf("kerf doctor: %v\nstderr: %s", err, stderr)
		}
		return stdout + stderr, code
	}

	t.Run("Q1_zero_jig_managed_works", func(t *testing.T) {
		// Question: what does the detector emit when the project has
		// zero works at all? Spec is silent on this; reasonable answer
		// is a single green finding ("no offenders, vacuously
		// compliant").
		combined, code := runDoctor(t)
		if code != 0 {
			t.Fatalf("exit = %d, want 0; output:\n%s", code, combined)
		}
		if !strings.Contains(combined, "all affected-pass artifacts list scenario and exploratory IDs") {
			t.Errorf("expected detector green summary on empty project; got:\n%s", combined)
		}
		t.Logf("Q1 finding: empty project → detector emits its green summary; no panic.")
	})

	t.Run("Q2_empty_artifact_file", func(t *testing.T) {
		// Question: what happens when the artifact file exists but is
		// zero bytes? Detector should treat it as "missing WDLL block"
		// (yellow) rather than crashing.
		work := filepath.Join(projDir, "wing-emptyfile")
		writeSpecYAML(t, work, "wing-emptyfile", "bug", "fix-spec")
		writeArtifact(t, work, "05-fix-spec.md", "")

		combined, code := runDoctor(t)
		if code != 0 {
			t.Fatalf("exit = %d, want 0; output:\n%s", code, combined)
		}
		if strings.Contains(combined, "panic:") {
			t.Fatalf("panic on empty artifact: %s", combined)
		}
		if !strings.Contains(combined, "wing-emptyfile") {
			t.Errorf("expected wing-emptyfile to be flagged; got:\n%s", combined)
		}
		if !strings.Contains(combined, "no 'What done looks like'") {
			t.Errorf("expected 'no What done looks like' detail; got:\n%s", combined)
		}
		t.Logf("Q2 finding: empty artifact treated as missingBlock (yellow), no panic.")

		// Cleanup so subsequent sub-tests start without this offender.
		if err := os.RemoveAll(work); err != nil {
			t.Fatalf("cleanup wing-emptyfile: %v", err)
		}
	})

	t.Run("Q3_unconventional_bullet_syntax", func(t *testing.T) {
		// Question: the unit tests use `- ` bullets. What if the
		// author uses `* ` bullets, or no bullet at all? The detector
		// matches on "Scenario-test item" anywhere in the WDLL block,
		// independent of bullet glyph — so this should still be
		// detected as compliant.
		work := filepath.Join(projDir, "wing-bullets")
		writeSpecYAML(t, work, "wing-bullets", "bug", "fix-spec")
		body := "# Fix Spec\n\n**What done looks like:**\n\n" +
			"* Scenario-test item filed with ID `kerf-aaa`\n" +
			"* Exploratory-test item filed with ID `kerf-bbb`\n"
		writeArtifact(t, work, "05-fix-spec.md", body)

		combined, code := runDoctor(t)
		if code != 0 {
			t.Fatalf("exit = %d, want 0; output:\n%s", code, combined)
		}
		if strings.Contains(combined, "wing-bullets") {
			t.Errorf("wing-bullets compliant under '* ' bullets but flagged; got:\n%s", combined)
		}
		t.Logf("Q3 finding: '*' bullets accepted equivalently to '-' bullets; bullet glyph is not load-bearing.")

		if err := os.RemoveAll(work); err != nil {
			t.Fatalf("cleanup wing-bullets: %v", err)
		}
	})

	t.Run("Q4_wdll_heading_with_h3", func(t *testing.T) {
		// Question: implementation.md uses `### What done looks like`
		// while plan/bug use `**What done looks like:**`. Both should
		// match. The unit tests already exercise both; here we verify
		// end-to-end that a real `### `-style heading is honored.
		work := filepath.Join(projDir, "wing-h3")
		writeSpecYAML(t, work, "wing-h3", "implementation", "breakdown")
		body := "# Breakdown\n\n### What done looks like\n\n" +
			"- Beads filed\n" +
			"- Scenario-test item filed with ID `kerf-aaa`\n" +
			"- Exploratory-test item filed with ID `kerf-bbb`\n"
		writeArtifact(t, work, "01-breakdown.md", body)

		combined, code := runDoctor(t)
		if code != 0 {
			t.Fatalf("exit = %d, want 0; output:\n%s", code, combined)
		}
		if strings.Contains(combined, "wing-h3") {
			t.Errorf("'### What done looks like' heading should be compliant; got:\n%s", combined)
		}
		t.Logf("Q4 finding: '### ' h3 heading recognised end-to-end (matches implementation.md jig body convention).")

		if err := os.RemoveAll(work); err != nil {
			t.Fatalf("cleanup wing-h3: %v", err)
		}
	})

	t.Run("Q5_doctor_quiet_suppresses_green", func(t *testing.T) {
		// Question: does the detector's green summary participate in
		// the documented --quiet behavior (suppress green findings)?
		// Per cmd/doctor.go --quiet flag: "Suppress green findings;
		// emit only yellow and red". After Q1-Q4 cleanup, project is
		// empty so the detector emits green; --quiet should hide it.
		stdout, stderr, code, err := r.Run("doctor", "--detector", "validation-section-coverage", "--quiet")
		if err != nil {
			t.Fatalf("kerf doctor --quiet: %v\nstderr: %s", err, stderr)
		}
		combined := stdout + stderr
		if code != 0 {
			t.Fatalf("exit = %d, want 0; output:\n%s", code, combined)
		}
		// The detector's green-summary string must not appear.
		if strings.Contains(combined, "all affected-pass artifacts list scenario and exploratory IDs") {
			t.Errorf("--quiet should suppress this detector's green summary; got:\n%s", combined)
		}
		t.Logf("Q5 finding: --quiet correctly suppresses the validation-section-coverage green summary.")
	})
}
