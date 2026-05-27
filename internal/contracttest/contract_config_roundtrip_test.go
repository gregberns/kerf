package contracttest

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/kerf/cmd"
	"github.com/gregberns/kerf/internal/config"
	"github.com/gregberns/kerf/internal/testutil"
)

// TestContract_ConfigRoundTrip asserts the "documented config-key
// round-trip" contract from specs/testing.md (Cross-command contracts):
// for every documented configuration key, `kerf config K V` followed by
// `kerf config K` returns V.
//
// Documented-key source of truth (plan 023 OQ1): the bench-scope keys
// come from config.ValidKeys() and the project-scope keys come from the
// hardcoded set in cmd/config.go's isProjectScopedKey. specs/commands.md
// is treated as derived from these two sources.
//
// Exemption: bead_filter is documented as read-only via `kerf config`
// (per cmd/config.go and specs/commands.md §kerf config). It is set via
// `kerf init`, `kerf bootstrap-filters`, or `kerf work edit
// --bead-filter-add` and therefore cannot round-trip through the
// `kerf config K V` mutator. It is skipped from the contract with a
// recorded rationale below.
//
// Caveat on typed values: `kerf config K V` is string-only at the CLI
// boundary. Non-string keys (bool, int) parse the string value via the
// Set() switch in internal/config/config.go and via parseBool in
// setProjectScoped. The round-trip is then "input string V → parsed and
// stored typed value → string-formatted readback equal to V". This test
// chooses V to be canonical for the type (e.g. "true", "42") so the
// readback is byte-identical to the input.
func TestContract_ConfigRoundTrip(t *testing.T) {
	const contractID = "config-key-roundtrip"

	// Per-key opt-outs for this contract. Keyed by config key (not
	// command path, since this contract is per-key). Each value must
	// cite the bead or spec note that justifies the exemption.
	configOptOuts := map[string]string{
		"bead_filter": "read-only via `kerf config`; spec'd as set by `kerf init` / `kerf bootstrap-filters` / `kerf work edit --bead-filter-add` (see cmd/config.go and specs/commands.md §kerf config)",
	}

	// Sentinel test values per key. Chosen so the canonical string
	// formatting after parse-and-store equals the input verbatim.
	values := map[string]string{
		"default_jig":                    "spec",
		"default_project":                "demo-project",
		"spec_path":                      "docs/specs/",
		"snapshots.enabled":              "false",
		"snapshots.interval_enabled":     "true",
		"snapshots.interval_seconds":     "600",
		"snapshots.max_snapshots":        "42",
		"sessions.stale_threshold_hours": "48",
		"finalize.repo_spec_path":        ".kerf/{codename}/",
		"tools.tasks":                    "bd",
		"doctor.footer":                  "false",
	}

	keys := documentedKeys()

	for _, key := range keys {
		key := key
		t.Run(key, func(t *testing.T) {
			if reason, exempt := configOptOuts[key]; exempt {
				t.Skipf("opt-out (%s): %s", contractID, reason)
			}
			value, ok := values[key]
			if !ok {
				t.Fatalf("contract test has no sentinel value defined for documented key %q; add one to the `values` map", key)
			}

			// Each subtest gets a fresh HOME and (if needed) a fresh
			// project context. Project-scoped keys are routed to
			// project.yaml, which requires a git repo with a
			// .kerf/project-identifier in cwd.
			setupConfigHome(t, isProjectScopedConfigKey(key))

			// Write.
			if _, err := runConfig(t, key, value); err != nil {
				t.Fatalf("set %s=%s: %v", key, value, err)
			}

			// Read.
			out, err := runConfig(t, key)
			if err != nil {
				t.Fatalf("get %s: %v", key, err)
			}
			got := parseConfigGetOutput(t, key, out)
			if got != value {
				t.Errorf("round-trip mismatch for %q: set=%q, get=%q (full output: %q)", key, value, got, out)
			}
		})
	}
}

// documentedKeys returns the union of bench-scope (config.ValidKeys) and
// project-scope (cmd/config.go isProjectScopedKey) documented keys,
// deduped and order-preserving (bench first, then project-only).
func documentedKeys() []string {
	seen := make(map[string]struct{})
	var out []string
	for _, k := range config.ValidKeys() {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	// Mirrors cmd/config.go projectScopedKeys, minus tools.<name>
	// prefix-matched entries (we list tools.tasks explicitly because it
	// is the only documented tools.* key today).
	for _, k := range []string{"tools.tasks", "default_jig", "doctor.footer", "bead_filter"} {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// isProjectScopedConfigKey mirrors cmd/config.go's isProjectScopedKey
// without depending on package cmd's unexported helper.
func isProjectScopedConfigKey(key string) bool {
	if strings.HasPrefix(key, "tools.") {
		return true
	}
	switch key {
	case "default_jig", "doctor.footer", "bead_filter":
		return true
	}
	return false
}

// setupConfigHome primes a fresh, isolated HOME for the subtest. When
// projectScoped is true it also constructs a tempdir git repo with a
// .kerf/project-identifier and chdir's into it so project-scoped writes
// resolve a project.
func setupConfigHome(t *testing.T, projectScoped bool) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := os.MkdirAll(filepath.Join(tmp, ".kerf"), 0o755); err != nil {
		t.Fatalf("mkdir bench: %v", err)
	}
	if !projectScoped {
		return
	}
	repo := testutil.SetupGitRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatalf("mkdir repo .kerf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte("contract-roundtrip\n"), 0o644); err != nil {
		t.Fatalf("write project-identifier: %v", err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
}

// runConfig invokes `kerf config <args...>` against the assembled cobra
// root and returns the captured stdout. cobra's Execute() walks the same
// dispatch path the real binary uses, so this is a faithful proxy for an
// out-of-process `kerf config` invocation while keeping the test in the
// same process for speed.
func runConfig(t *testing.T, args ...string) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, perr := os.Pipe()
	if perr != nil {
		return "", fmt.Errorf("pipe: %w", perr)
	}
	os.Stdout = w

	root := cmd.Root()
	root.SetArgs(append([]string{"config"}, args...))
	// Suppress cobra's own usage spew on error so the captured stream is
	// just the command's output.
	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	root.SetOut(w)
	execErr := root.Execute()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		return "", fmt.Errorf("copy stdout: %w", copyErr)
	}
	return buf.String(), execErr
}

// parseConfigGetOutput extracts the value from a `kerf config K`
// readback. The format is "K: V\n"; we trim and parse defensively so
// trailing newlines or whitespace differences do not break the assert.
func parseConfigGetOutput(t *testing.T, key, out string) string {
	t.Helper()
	line := strings.TrimSpace(out)
	prefix := key + ":"
	idx := strings.Index(line, prefix)
	if idx < 0 {
		t.Fatalf("readback for %q does not contain expected prefix %q; output=%q", key, prefix, out)
	}
	return strings.TrimSpace(line[idx+len(prefix):])
}
