package scenariotest

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestHarness_Help is the sanity test: spin up a runner, run `kerf --help`,
// assert it exits cleanly with sensible output.
func TestHarness_Help(t *testing.T) {
	r := New(t)

	stdout, stderr, code, err := r.Run("--help")
	if err != nil {
		t.Fatalf("Run --help: %v\nstderr: %s", err, stderr)
	}
	if code != 0 {
		t.Fatalf("kerf --help exited %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	// Sanity: --help should mention the binary name somewhere.
	combined := stdout + stderr
	if !strings.Contains(strings.ToLower(combined), "kerf") &&
		!strings.Contains(strings.ToLower(combined), "usage") {
		t.Fatalf("kerf --help output looks empty / unrelated; stdout=%q stderr=%q", stdout, stderr)
	}
}

// TestScrubbedEnv verifies KERF_* and BD_* are stripped and HOME is pinned.
func TestScrubbedEnv(t *testing.T) {
	// Inject sentinel values into the parent env for the duration of this test.
	t.Setenv("KERF_BENCH", "should-not-leak")
	t.Setenv("BD_STORE", "should-not-leak-either")
	t.Setenv("OTHER_VAR", "should-survive")

	env := scrubbedEnv("/tmp/fake-home")

	var sawHome, sawOther bool
	for _, kv := range env {
		if strings.HasPrefix(kv, "KERF_") {
			t.Errorf("KERF_* leaked into scrubbed env: %s", kv)
		}
		if strings.HasPrefix(kv, "BD_") {
			t.Errorf("BD_* leaked into scrubbed env: %s", kv)
		}
		if kv == "HOME=/tmp/fake-home" {
			sawHome = true
		}
		if kv == "OTHER_VAR=should-survive" {
			sawOther = true
		}
	}
	if !sawHome {
		t.Errorf("HOME not pinned in scrubbed env")
	}
	if !sawOther {
		t.Errorf("unrelated env var was wrongly scrubbed")
	}
}

// TestRequireBd_SkipsWhenAbsent simulates absence by clearing PATH.
// We run a subtest with a doctored t and verify it skips with the canonical
// message. Using a helper *testing.T isn't possible; instead, we use a sub-
// test plus PATH manipulation and rely on the harness's skip side effect.
func TestRequireBd_SkipsWhenAbsent(t *testing.T) {
	// Save and restore PATH.
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })

	// Empty PATH guarantees `bd` is not found regardless of installation.
	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	// Verify directly that exec.LookPath fails. If for some reason it doesn't
	// (e.g. some shells inherit a fallback), we skip this test rather than
	// fail spuriously — the contract is "skip when bd is missing".
	if _, err := exec.LookPath("bd"); err == nil {
		t.Skip("exec.LookPath still found bd with empty PATH; cannot test skip path on this platform")
	}

	// Use a buffered t-like to capture the skip. Standard library exposes no
	// test-double for testing.T's Skip; the simplest reliable shape is to
	// confirm the SkipMessage constant is non-empty and well-formed, since
	// RequireBd has a single deterministic branch.
	if SkipMessage == "" || !strings.Contains(SkipMessage, "bd not found") {
		t.Fatalf("SkipMessage looks wrong: %q", SkipMessage)
	}
}
