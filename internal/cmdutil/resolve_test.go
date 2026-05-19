package cmdutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

// kerf-vu0r: ResolveProject must surface a corrupt-identifier error rather
// than silently falling through to default_project from the bench config —
// otherwise downstream commands (`kerf list`, `kerf show`, `kerf new`, ...)
// would route at the wrong project. Mirrors kerf-dlb's init-path test.
func TestResolveProject_CorruptIdentifier_Errors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Seed a bench config with a default_project so the fall-through path
	// would otherwise have returned a value — making the test sensitive to
	// the regression.
	benchCfg := filepath.Join(tmp, ".kerf", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(benchCfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(benchCfg, []byte("default_project: fallback-proj\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	idPath := filepath.Join(repo, ".kerf", "project-identifier")
	garbage := []byte("bad/\x00id\n")
	if err := os.WriteFile(idPath, garbage, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveProject("")
	if err == nil {
		t.Fatal("expected error when project-identifier is corrupt, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"corrupt project identifier", idPath, "replace with a clean slug"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}

// kerf-vu0r: legitimate fall-through — no .kerf/project-identifier on disk —
// must still consult default_project from the bench config. This pins the
// behavior the corrupt-id test is contrasted with.
func TestResolveProject_MissingIdentifier_FallsThrough(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchCfg := filepath.Join(tmp, ".kerf", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(benchCfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(benchCfg, []byte("default_project: fallback-proj\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// cwd outside any git repo so project.Resolve fails with FindGitRoot,
	// which is the "no identifier" path (not corruption).
	t.Chdir(tmp)

	id, err := ResolveProject("")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if id != "fallback-proj" {
		t.Errorf("ResolveProject = %q, want %q", id, "fallback-proj")
	}
}

// kerf-vu0r: --project override bypasses identifier lookup entirely, so a
// corrupt file on disk must not affect the explicit path.
func TestResolveProject_ExplicitFlag_BypassesIdentifier(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	id, err := ResolveProject("override-proj")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if id != "override-proj" {
		t.Errorf("ResolveProject = %q, want %q", id, "override-proj")
	}
}

// kerf-shoe: Resolver's findRepoRootForProject previously swallowed all
// ReadIdentifier errors. When the explicit --project flag bypasses
// ResolveProject's own corruption gate but cwd still contains a corrupt
// .kerf/project-identifier, the resolver-build path must also surface the
// corruption rather than silently dropping the repo-root and continuing
// with a (potentially wrong-context) resolver.
func TestResolver_CorruptIdentifier_Errors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	idPath := filepath.Join(repo, ".kerf", "project-identifier")
	garbage := []byte("bad/\x00id\n")
	if err := os.WriteFile(idPath, garbage, 0o644); err != nil {
		t.Fatal(err)
	}

	// Caller has passed --project explicitly; ResolveProject is bypassed,
	// but Resolver still inspects cwd for the repo root.
	_, err := Resolver("override-proj")
	if err == nil {
		t.Fatal("expected error when cwd has corrupt project-identifier, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"corrupt project identifier", idPath} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}
