package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gregberns/kerf/internal/storage"
)

// newStorageDriftCtx builds a Context against temp bench + repo paths.
// repoRoot may be "" to model bench-only mode.
func newStorageDriftCtx(t *testing.T, mode storage.Mode) (*Context, string, string) {
	t.Helper()
	bench := t.TempDir()
	repo := t.TempDir()
	// Pre-create the repo .kerf so the resolver picks the right mode.
	repoKerf := filepath.Join(repo, ".kerf")
	if err := os.MkdirAll(repoKerf, 0o755); err != nil {
		t.Fatalf("mkdir .kerf: %v", err)
	}
	if mode == storage.ModeLocal {
		cfg := &storage.RepoConfig{Storage: string(storage.ModeLocal)}
		if err := storage.SaveRepoConfig(repo, cfg); err != nil {
			t.Fatalf("save repo config: %v", err)
		}
	}
	r, err := storage.NewResolver(bench, "p1", repo)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}
	return &Context{ProjectID: "p1", Resolver: r, BenchPath: bench}, bench, repo
}

func TestStorageDrift_Clean_LocalMode(t *testing.T) {
	ctx, bench, repo := newStorageDriftCtx(t, storage.ModeLocal)
	// Canonical: works under repo/.kerf/works, bench symlink to repo works.
	worksDir := filepath.Join(repo, ".kerf", "works")
	if err := os.MkdirAll(filepath.Join(worksDir, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir alpha: %v", err)
	}
	benchLink := filepath.Join(bench, "projects", "p1")
	if err := os.MkdirAll(filepath.Dir(benchLink), 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}
	if err := os.Symlink(worksDir, benchLink); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	d := storageDriftDetector{}
	findings, err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != Green {
		t.Fatalf("expected single green finding, got %#v", findings)
	}
}

func TestStorageDrift_Clean_BenchMode(t *testing.T) {
	ctx, bench, _ := newStorageDriftCtx(t, storage.ModeBench)
	// Canonical: works under bench/projects/p1/alpha.
	if err := os.MkdirAll(filepath.Join(bench, "projects", "p1", "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir alpha: %v", err)
	}

	d := storageDriftDetector{}
	findings, err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != Green {
		t.Fatalf("expected single green, got %#v", findings)
	}
}

func TestStorageDrift_WrongLocation_BenchMode(t *testing.T) {
	ctx, _, repo := newStorageDriftCtx(t, storage.ModeBench)
	// Bench mode active, but a work dir sits in repo .kerf/works/ — drift.
	wrong := filepath.Join(repo, ".kerf", "works", "stray")
	if err := os.MkdirAll(wrong, 0o755); err != nil {
		t.Fatalf("mkdir stray: %v", err)
	}

	d := storageDriftDetector{}
	findings, err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d: %#v", len(findings), findings)
	}
	if findings[0].Severity != Yellow {
		t.Fatalf("expected yellow, got %s", findings[0].Severity)
	}
	if len(findings[0].Items) != 1 || findings[0].Items[0].Target != "stray" {
		t.Fatalf("expected item target 'stray', got %#v", findings[0].Items)
	}
}

func TestStorageDrift_DoublePresence_LocalMode(t *testing.T) {
	ctx, bench, repo := newStorageDriftCtx(t, storage.ModeLocal)
	// Local mode canonical: repo .kerf/works/alpha
	if err := os.MkdirAll(filepath.Join(repo, ".kerf", "works", "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir canonical: %v", err)
	}
	// Non-canonical real dir on bench (NOT a symlink) shadows it.
	if err := os.MkdirAll(filepath.Join(bench, "projects", "p1", "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir non-canonical: %v", err)
	}

	d := storageDriftDetector{}
	findings, err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Expect: red double-presence, plus red "bench is a real directory".
	hasDouble := false
	hasBenchRealDir := false
	for _, f := range findings {
		if f.Severity == Red {
			for _, it := range f.Items {
				if it.Target == "alpha" {
					hasDouble = true
				}
			}
			if f.Summary == "bench path is a real directory, expected symlink (local mode)" {
				hasBenchRealDir = true
			}
		}
	}
	if !hasDouble {
		t.Fatalf("expected red double-presence finding for 'alpha', got %#v", findings)
	}
	if !hasBenchRealDir {
		t.Fatalf("expected red bench-real-directory finding, got %#v", findings)
	}
}

func TestStorageDrift_DoubleConfigFile(t *testing.T) {
	ctx, bench, repo := newStorageDriftCtx(t, storage.ModeLocal)
	// Both repo and bench have project.yaml.
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project.yaml"), []byte("a: 1\n"), 0o644); err != nil {
		t.Fatalf("write repo project.yaml: %v", err)
	}
	projDir := filepath.Join(bench, "projects", "p1")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir projDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte("a: 2\n"), 0o644); err != nil {
		t.Fatalf("write bench project.yaml: %v", err)
	}

	d := storageDriftDetector{}
	findings, err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		for _, it := range f.Items {
			if it.Target == "project.yaml" && f.Severity == Red {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected red finding listing project.yaml double-presence, got %#v", findings)
	}
}

func TestStorageDrift_ArchiveLiveCollision(t *testing.T) {
	ctx, bench, _ := newStorageDriftCtx(t, storage.ModeBench)
	// Live work on bench (canonical for bench mode).
	if err := os.MkdirAll(filepath.Join(bench, "projects", "p1", "gamma"), 0o755); err != nil {
		t.Fatalf("mkdir live: %v", err)
	}
	// Archive entry with same codename.
	if err := os.MkdirAll(filepath.Join(bench, "archive", "p1", "gamma"), 0o755); err != nil {
		t.Fatalf("mkdir archive: %v", err)
	}

	d := storageDriftDetector{}
	findings, err := d.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.Severity == Yellow {
			for _, it := range f.Items {
				if it.Target == "gamma" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected yellow archive/live collision for 'gamma', got %#v", findings)
	}
}

func TestStorageDrift_RegisteredInDefault(t *testing.T) {
	if _, ok := DefaultRegistry.Get("storage-drift"); !ok {
		t.Fatalf("storage-drift detector not registered in DefaultRegistry")
	}
}
