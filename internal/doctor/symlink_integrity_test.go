package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gregberns/kerf/internal/storage"
)

// newSymlinkCtx builds a Context for a project at <root> with the
// given mode, creating the bench/projects/<id> path (or symlink in
// local mode) on disk as the test specifies.
func newSymlinkCtx(t *testing.T, mode storage.Mode, root, projectID string) *Context {
	t.Helper()
	benchPath := filepath.Join(root, "bench")
	if err := os.MkdirAll(filepath.Join(benchPath, "projects"), 0o755); err != nil {
		t.Fatalf("mkdir bench: %v", err)
	}
	repoRoot := ""
	if mode == storage.ModeLocal {
		repoRoot = filepath.Join(root, "repo")
		if err := os.MkdirAll(filepath.Join(repoRoot, ".kerf", "works"), 0o755); err != nil {
			t.Fatalf("mkdir repo: %v", err)
		}
	}
	r := &storage.Resolver{
		Mode:      mode,
		BenchPath: benchPath,
		ProjectID: projectID,
		RepoRoot:  repoRoot,
	}
	return &Context{
		ProjectID: projectID,
		Resolver:  r,
		BenchPath: benchPath,
	}
}

func TestSymlinkIntegrity_BenchModeSkipsGreen(t *testing.T) {
	root := t.TempDir()
	ctx := newSymlinkCtx(t, storage.ModeBench, root, "demo")
	d := symlinkIntegrityDetector{}
	fs, err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(fs))
	}
	if fs[0].Severity != Green {
		t.Errorf("bench mode severity = %q, want green", fs[0].Severity)
	}
	if want := "bench storage"; !contains(fs[0].Summary, want) {
		t.Errorf("summary = %q, want it to mention %q", fs[0].Summary, want)
	}
}

func TestSymlinkIntegrity_LocalHealthyGreen(t *testing.T) {
	root := t.TempDir()
	ctx := newSymlinkCtx(t, storage.ModeLocal, root, "demo")
	r := ctx.Resolver
	link := filepath.Join(r.BenchPath, "projects", r.ProjectID)
	target := filepath.Join(r.RepoRoot, ".kerf", "works")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	fs, err := (symlinkIntegrityDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != Green {
		t.Fatalf("expected single green finding, got %+v", fs)
	}
}

func TestSymlinkIntegrity_LocalMissingRed(t *testing.T) {
	root := t.TempDir()
	ctx := newSymlinkCtx(t, storage.ModeLocal, root, "demo")
	// No symlink created.

	fs, err := (symlinkIntegrityDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != Red {
		t.Fatalf("expected red finding, got %+v", fs)
	}
	if !contains(fs[0].Summary, "missing") {
		t.Errorf("summary = %q, want it to mention missing", fs[0].Summary)
	}
}

func TestSymlinkIntegrity_LocalBrokenRed(t *testing.T) {
	root := t.TempDir()
	ctx := newSymlinkCtx(t, storage.ModeLocal, root, "demo")
	r := ctx.Resolver
	link := filepath.Join(r.BenchPath, "projects", r.ProjectID)
	// Point at a path that doesn't exist.
	if err := os.Symlink(filepath.Join(root, "nope"), link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	fs, err := (symlinkIntegrityDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != Red {
		t.Fatalf("expected red finding, got %+v", fs)
	}
	if !contains(fs[0].Summary, "broken") {
		t.Errorf("summary = %q, want it to mention broken", fs[0].Summary)
	}
}

func TestSymlinkIntegrity_RealDirRed(t *testing.T) {
	root := t.TempDir()
	ctx := newSymlinkCtx(t, storage.ModeLocal, root, "demo")
	r := ctx.Resolver
	link := filepath.Join(r.BenchPath, "projects", r.ProjectID)
	if err := os.MkdirAll(link, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	fs, err := (symlinkIntegrityDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != Red {
		t.Fatalf("expected red finding, got %+v", fs)
	}
	if !contains(fs[0].Summary, "real directory") {
		t.Errorf("summary = %q, want it to mention real directory", fs[0].Summary)
	}
}

func TestSymlinkIntegrity_WrongTargetRed(t *testing.T) {
	root := t.TempDir()
	ctx := newSymlinkCtx(t, storage.ModeLocal, root, "demo")
	r := ctx.Resolver
	link := filepath.Join(r.BenchPath, "projects", r.ProjectID)
	otherTarget := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(otherTarget, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(otherTarget, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	fs, err := (symlinkIntegrityDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != Red {
		t.Fatalf("expected red finding, got %+v", fs)
	}
	if !contains(fs[0].Summary, "does not match") {
		t.Errorf("summary = %q, want it to mention mismatch", fs[0].Summary)
	}
}

func TestSymlinkIntegrity_RegisteredByID(t *testing.T) {
	if _, ok := DefaultRegistry.Get("symlink-integrity"); !ok {
		t.Fatal("symlink-integrity not registered in DefaultRegistry")
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
