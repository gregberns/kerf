package scenariotest

// Scenario B (kerf-fyze / Plan 022): kerf doctor must degrade to a RED
// finding — not crash — when the configured bead-store tool exits non-zero.
//
// This locks the post-kerf-pq5 contract end-to-end as a subprocess test:
// the canonical dogfood BLOCKER #1 (2026-05-18) was that `kerf doctor`
// panicked on a `br` JSON_ERROR. The detector layer now degrades to a
// `bead store unavailable` RED finding; this scenario exercises the
// detector through the real CLI binary against a deliberately failing
// shim and asserts:
//
//   1. default `kerf doctor` exits 0 with a RED finding present
//      naming the failing tool ("bead store unavailable" + tool name).
//   2. `kerf doctor --strict` exits 1 against the same substrate.
//   3. Neither invocation produces a Go runtime panic stack.
//
// Approach choice (a) vs (b): we use (a) — install a deliberately
// failing shim and point `tools.tasks` at it. Rationale: (a) is
// hermetic (no dependence on which `br` or `bd` version happens to be
// on PATH, no fragility around how `bd init` writes its store), and
// the shim's exit-status / stderr are fully under test control. (b)
// — corrupting `.beads/` — would couple the test to bd's internal
// storage layout, which is out of scope for kerf and changes
// independently of kerf.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScenarioB_DoctorDegradesOnBeadToolFailure is the kerf-fyze scenario.
func TestScenarioB_DoctorDegradesOnBeadToolFailure(t *testing.T) {
	r := New(t)

	// --- Step 1: run `kerf init` to create project.yaml and the
	// project-identifier under the scenario's pinned HOME / bench. ----
	stdout, stderr, code, err := r.Run("init", "--no", "--jig", "spec")
	if err != nil {
		t.Fatalf("kerf init: %v\nstderr: %s", err, stderr)
	}
	if code != 0 {
		t.Fatalf("kerf init exited %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// --- Step 2: read the project identifier kerf wrote into
	// .kerf/project-identifier so we know where project.yaml lives. ---
	projectID := strings.TrimSpace(r.ReadFile(".kerf/project-identifier"))
	if projectID == "" {
		t.Fatalf("project-identifier empty after kerf init")
	}

	// --- Step 3: install a deliberately failing bead-tool shim. ------
	// The shim exits 8 (matching the `br` JSON_ERROR exit code from the
	// dogfood transcript) and prints a JSON-shaped error payload on
	// stderr. Its name is deliberately scenario-scoped — `kerf-fyze-
	// failshim` — so it can't be confused with a real tool.
	shimDir := filepath.Join(r.HomeDir(), "shimbin")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatalf("mkdir shimDir: %v", err)
	}
	shimName := "kerf-fyze-failshim"
	shimPath := filepath.Join(shimDir, shimName)
	shimBody := `#!/bin/sh
# Scenario B (kerf-fyze): deliberately failing bead-tool shim.
# Exits 8 with a JSON-shaped error payload on stderr — modeled on the
# canonical br JSON_ERROR seen in dogfood test 2026-05-18.
printf '{"error":{"code":"JSON_ERROR","message":"missing field jsonl_export"}}\n' 1>&2
exit 8
`
	if err := os.WriteFile(shimPath, []byte(shimBody), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}

	// Prepend shimDir to PATH so the shim wins over any real binary.
	// Find and rewrite PATH= in the runner env (it was inherited from
	// the parent and may already contain useful entries for `bd`,
	// `git`, etc.).
	for i, kv := range r.env {
		if strings.HasPrefix(kv, "PATH=") {
			r.env[i] = "PATH=" + shimDir + string(os.PathListSeparator) + kv[len("PATH="):]
			break
		}
	}

	// --- Step 4: point tools.tasks at the shim by writing project.yaml
	// directly under the scenario's bench. We use `kerf config` to
	// avoid hard-coding the bench layout — keeps the test honest about
	// the public-config-API contract. -----------------------------------
	stdout, stderr, code, err = r.Run("config", "tools.tasks", shimName)
	if err != nil {
		t.Fatalf("kerf config tools.tasks: %v\nstderr: %s", err, stderr)
	}
	if code != 0 {
		t.Fatalf("kerf config tools.tasks exited %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// --- Step 5: default `kerf doctor` — exit 0, RED finding present. -
	stdout, stderr, code, err = r.Run("doctor")
	if err != nil {
		t.Fatalf("kerf doctor: %v\nstderr: %s", err, stderr)
	}
	combined := stdout + stderr
	if code != 0 {
		t.Fatalf("kerf doctor default exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(combined, "[red]") && !strings.Contains(combined, "RED") {
		t.Errorf("expected a RED finding in doctor output; got:\n%s", combined)
	}
	if !strings.Contains(combined, "bead store unavailable") {
		t.Errorf("expected 'bead store unavailable' summary in doctor output; got:\n%s", combined)
	}
	if !strings.Contains(combined, shimName) {
		t.Errorf("expected failing tool name %q in doctor output; got:\n%s", shimName, combined)
	}
	// Negative assertion: no Go runtime panic stack.
	if strings.Contains(combined, "panic:") || strings.Contains(combined, "goroutine 1 [running]") {
		t.Fatalf("kerf doctor produced a panic stack (regression of BLOCKER #1):\n%s", combined)
	}

	// --- Step 6: `kerf doctor --strict` — exit 1 against same substrate.
	stdout, stderr, code, err = r.Run("doctor", "--strict")
	if err != nil {
		t.Fatalf("kerf doctor --strict: %v\nstderr: %s", err, stderr)
	}
	combined = stdout + stderr
	if code != 1 {
		t.Fatalf("kerf doctor --strict exit = %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(combined, "bead store unavailable") {
		t.Errorf("--strict: expected 'bead store unavailable' in output; got:\n%s", combined)
	}
	if strings.Contains(combined, "panic:") || strings.Contains(combined, "goroutine 1 [running]") {
		t.Fatalf("kerf doctor --strict produced a panic stack:\n%s", combined)
	}

	// Silence unused-var lint if we ever drop the projectID read.
	_ = projectID
}
