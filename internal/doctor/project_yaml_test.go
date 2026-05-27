package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/kerf/internal/storage"
)

// newProjectYAMLCtx builds a Context whose Resolver points at a bench-mode
// project under tmp. Returns the context and the resolved
// project.yaml path so each test can stage its own fixture content.
func newProjectYAMLCtx(t *testing.T) (*Context, string) {
	t.Helper()
	bench := t.TempDir()
	r, err := storage.NewResolver(bench, "p-test", "")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	// Ensure parent dirs exist so tests that write a fixture can
	// create the file without extra ceremony.
	cfgPath := r.ProjectConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return &Context{
		ProjectID: "p-test",
		Resolver:  r,
		BenchPath: bench,
	}, cfgPath
}

func TestProjectYAMLDetector_ID(t *testing.T) {
	if got := (projectYAMLDetector{}).ID(); got != "project-yaml" {
		t.Errorf("ID() = %q, want %q", got, "project-yaml")
	}
}

func TestProjectYAMLDetector_Green_Valid(t *testing.T) {
	ctx, path := newProjectYAMLCtx(t)
	body := "jigs:\n  - implementation\n  - feature\ndefault_jig: implementation\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := (projectYAMLDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 1 || got[0].Severity != Green {
		t.Fatalf("want one green finding; got %+v", got)
	}
	if !strings.Contains(got[0].Summary, "default_jig=implementation") {
		t.Errorf("summary missing default_jig: %q", got[0].Summary)
	}
	if !strings.Contains(got[0].Summary, "2 jigs") {
		t.Errorf("summary missing jig count: %q", got[0].Summary)
	}
}

func TestProjectYAMLDetector_Red_Missing(t *testing.T) {
	ctx, path := newProjectYAMLCtx(t)
	// File deliberately not written.
	got, err := (projectYAMLDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 1 || got[0].Severity != Red {
		t.Fatalf("want one red finding; got %+v", got)
	}
	if !strings.Contains(strings.ToLower(got[0].Summary), "missing") {
		t.Errorf("expected 'missing' in summary, got %q", got[0].Summary)
	}
	if got[0].Hint == "" {
		t.Errorf("missing-finding lacks hint")
	}
	if len(got[0].Items) != 1 || got[0].Items[0].Target != path {
		t.Errorf("expected item to name canonical path %q; got %+v", path, got[0].Items)
	}
}

func TestProjectYAMLDetector_Red_InvalidYAML(t *testing.T) {
	ctx, path := newProjectYAMLCtx(t)
	if err := os.WriteFile(path, []byte("jigs: [unterminated\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := (projectYAMLDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 1 || got[0].Severity != Red {
		t.Fatalf("want one red finding; got %+v", got)
	}
	if !strings.Contains(strings.ToLower(got[0].Summary), "invalid yaml") {
		t.Errorf("expected 'invalid YAML' in summary, got %q", got[0].Summary)
	}
	if got[0].Hint == "" {
		t.Errorf("invalid-YAML finding lacks hint")
	}
}

func TestProjectYAMLDetector_Red_NoJigs(t *testing.T) {
	ctx, path := newProjectYAMLCtx(t)
	// Parses cleanly but declares no jigs.
	if err := os.WriteFile(path, []byte("default_jig: implementation\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := (projectYAMLDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 1 || got[0].Severity != Red {
		t.Fatalf("want one red finding; got %+v", got)
	}
	if !strings.Contains(strings.ToLower(got[0].Summary), "no jigs") {
		t.Errorf("expected 'no jigs' in summary, got %q", got[0].Summary)
	}
}

func TestProjectYAMLDetector_RegisteredByDefault(t *testing.T) {
	if _, ok := DefaultRegistry.Get("project-yaml"); !ok {
		t.Fatal("project-yaml detector not registered in DefaultRegistry")
	}
}
