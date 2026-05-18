package duration

import (
	"os"
	"testing"
)

// TestLoadDefault_CwdIndependent verifies that LoadDefault can locate
// the fitted-distributions YAML regardless of the process cwd. This
// guards against the original bug where DefaultRegistryPath was a
// repo-root-relative constant and only worked when kerfsim was run
// from the repo root.
func TestLoadDefault_CwdIndependent(t *testing.T) {
	// Save and restore cwd around the test.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	// First, sanity-check that resolution works from the original cwd
	// (the package directory inside the repo).
	if p := ResolveDefaultPath(); p == "" {
		t.Fatalf("ResolveDefaultPath returned empty from %s; data file should be reachable via upward search", orig)
	}

	// Change to a directory that is not inside the kerf repo so a
	// pure cwd-relative resolution would fail. The executable-based
	// search root still points at the test binary inside the repo's
	// build cache, so resolution must continue to succeed.
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}

	p := ResolveDefaultPath()
	if p == "" {
		t.Fatalf("ResolveDefaultPath returned empty after chdir away from repo; upward search from executable should still find the data file")
	}

	reg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault after chdir: %v", err)
	}
	if reg == nil {
		t.Fatalf("LoadDefault returned nil registry after chdir; expected fitted distributions to load")
	}
	// Spot-check a known phase from the corpus.
	if _, ok := reg.Lookup("task_work"); !ok {
		t.Fatalf("registry missing task_work after cwd-independent load")
	}
}
