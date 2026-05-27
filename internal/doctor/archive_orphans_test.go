package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gregberns/kerf/internal/storage"
)

// newArchiveOrphansCtx builds a Context rooted at a fresh tempdir for
// the archive-orphans detector. Named uniquely so it doesn't collide
// with the other detectors' per-test helpers at the package level.
func newArchiveOrphansCtx(t *testing.T, root, projectID string) *Context {
	t.Helper()
	benchPath := filepath.Join(root, "bench")
	if err := os.MkdirAll(filepath.Join(benchPath, "projects", projectID), 0o755); err != nil {
		t.Fatalf("mkdir works: %v", err)
	}
	r := &storage.Resolver{
		Mode:      storage.ModeBench,
		BenchPath: benchPath,
		ProjectID: projectID,
	}
	return &Context{
		ProjectID: projectID,
		Resolver:  r,
		BenchPath: benchPath,
	}
}

// makeArchive creates ~/.kerf/archive/<id>/<codename>/ under benchPath.
func makeArchive(t *testing.T, ctx *Context, codename string) {
	t.Helper()
	dir := ctx.Resolver.ArchiveDir(codename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir archive %s: %v", dir, err)
	}
}

// makeLiveWork creates a live work directory under the project's
// canonical works dir.
func makeLiveWork(t *testing.T, ctx *Context, codename string) {
	t.Helper()
	dir := ctx.Resolver.WorkDir(codename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir work %s: %v", dir, err)
	}
}

func TestArchiveOrphans_NoArchiveGreen(t *testing.T) {
	root := t.TempDir()
	ctx := newArchiveOrphansCtx(t, root, "demo")

	fs, err := (archiveOrphansDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != Green {
		t.Fatalf("expected single green finding, got %+v", fs)
	}
	if !contains(fs[0].Summary, "no entries") {
		t.Errorf("summary = %q, want it to mention 'no entries'", fs[0].Summary)
	}
}

func TestArchiveOrphans_ArchiveNoCollisionsGreen(t *testing.T) {
	root := t.TempDir()
	ctx := newArchiveOrphansCtx(t, root, "demo")
	makeArchive(t, ctx, "fern")
	makeArchive(t, ctx, "moss")
	// Live work with a different codename — must not collide.
	makeLiveWork(t, ctx, "ivy")

	fs, err := (archiveOrphansDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != Green {
		t.Fatalf("expected single green finding, got %+v", fs)
	}
	if !contains(fs[0].Summary, "no live collisions") {
		t.Errorf("summary = %q, want it to mention 'no live collisions'", fs[0].Summary)
	}
	if !contains(fs[0].Summary, "2 entries") {
		t.Errorf("summary = %q, want it to count 2 entries", fs[0].Summary)
	}
}

func TestArchiveOrphans_CollisionYellow(t *testing.T) {
	root := t.TempDir()
	ctx := newArchiveOrphansCtx(t, root, "demo")
	makeArchive(t, ctx, "fern")
	makeArchive(t, ctx, "moss")
	// 'fern' exists both as archive and as live work — collision.
	makeLiveWork(t, ctx, "fern")

	fs, err := (archiveOrphansDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != Yellow {
		t.Fatalf("expected single yellow finding, got %+v", fs)
	}
	if !contains(fs[0].Summary, "collision") {
		t.Errorf("summary = %q, want it to mention collision", fs[0].Summary)
	}
	if len(fs[0].Items) != 1 || fs[0].Items[0].Target != "fern" {
		t.Errorf("items = %+v, want one item targeting 'fern'", fs[0].Items)
	}
	if fs[0].Hint == "" {
		t.Errorf("expected non-empty hint for yellow finding")
	}
}

func TestArchiveOrphans_SingularEntryGreen(t *testing.T) {
	root := t.TempDir()
	ctx := newArchiveOrphansCtx(t, root, "demo")
	makeArchive(t, ctx, "fern")

	fs, err := (archiveOrphansDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != Green {
		t.Fatalf("expected single green finding, got %+v", fs)
	}
	if !contains(fs[0].Summary, "1 entry,") {
		t.Errorf("summary = %q, want singular 'entry'", fs[0].Summary)
	}
}

func TestArchiveOrphans_RegisteredByID(t *testing.T) {
	if _, ok := DefaultRegistry.Get("archive-orphans"); !ok {
		t.Fatal("archive-orphans not registered in DefaultRegistry")
	}
}
