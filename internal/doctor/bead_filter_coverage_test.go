package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/storage"
)

// withBeadLoader swaps in a static bead set for the duration of t. The
// restoration runs via t.Cleanup so parallel tests in this package don't
// see each other's stubs (each test is non-parallel).
func withBeadLoader(t *testing.T, bds []beads.Bead) {
	t.Helper()
	saved := beadFilterCoverageLoader
	beadFilterCoverageLoader = func(string) ([]beads.Bead, error) { return bds, nil }
	t.Cleanup(func() { beadFilterCoverageLoader = saved })
}

// newBeadFilterCovCtx builds a bench-mode Context for the bead-filter-coverage
// detector. The bench TempDir is layered as:
//
//	<bench>/projects/p-test/
//	    project.yaml        (declares tools.tasks → a non-existent binary so
//	                         beads.ListNamed returns (nil, nil) and the test
//	                         is independent of the real `br` tool)
//	    <codename>/spec.yaml per work
//
// Test-helper name is detector-specific (kerf-7lq) to avoid colliding with
// newProjectYAMLCtx / newStorageDriftCtx in this package.
func newBeadFilterCovCtx(t *testing.T) (*Context, *storage.Resolver) {
	t.Helper()
	bench := t.TempDir()
	r, err := storage.NewResolver(bench, "p-test", "")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	// Bench-mode works dir = <bench>/projects/p-test/.
	if err := os.MkdirAll(r.WorksDir(), 0o755); err != nil {
		t.Fatalf("MkdirAll works: %v", err)
	}
	// Stub project.yaml: declare jigs (cosmetic — only used by other
	// detectors) and point tools.tasks at a binary that doesn't exist
	// so beads.ListNamed reports the tool absent and returns (nil, nil).
	cfg := `jigs:
  - implementation
tools:
  tasks: kerf-7lq-nonexistent-binary
`
	if err := os.WriteFile(r.ProjectConfigPath(), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}
	return &Context{
		ProjectID: "p-test",
		Resolver:  r,
		BenchPath: bench,
	}, r
}

// writeSpec writes a minimal spec.yaml for codename under r.WorksDir().
// When filterBlock is empty the `bead_filter` key is omitted entirely
// (modeling an `unwired` work). When non-empty, filterBlock is inlined
// verbatim — callers control whether it's `bead_filter: label=...` or a
// clause that resolves to zero beads.
func writeSpec(t *testing.T, r *storage.Resolver, codename, filterBlock string) {
	t.Helper()
	dir := r.WorkDir(codename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`codename: %s
type: implementation
project:
  id: p-test
jig: implementation
jig_version: 1
status: open
status_values:
  - open
  - done
created: %s
updated: %s
sessions: []
depends_on: []
pinned_beads: []
implementation:
  branch: null
  pr: null
  commits: []
`, codename, now, now)
	if filterBlock != "" {
		body += filterBlock
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write spec.yaml: %v", err)
	}
}

func TestBeadFilterCoverageDetector_ID(t *testing.T) {
	if got := (beadFilterCoverageDetector{}).ID(); got != "bead-filter-coverage" {
		t.Errorf("ID() = %q, want %q", got, "bead-filter-coverage")
	}
}

func TestBeadFilterCoverageDetector_RegisteredByDefault(t *testing.T) {
	if _, ok := DefaultRegistry.Get("bead-filter-coverage"); !ok {
		t.Fatal("bead-filter-coverage detector not registered in DefaultRegistry")
	}
}

// Zero active works → single green "no active works" finding.
func TestBeadFilterCoverageDetector_Green_NoWorks(t *testing.T) {
	ctx, _ := newBeadFilterCovCtx(t)
	withBeadLoader(t, nil)

	got, err := (beadFilterCoverageDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 1 || got[0].Severity != Green {
		t.Fatalf("want one green finding; got %+v", got)
	}
	if !strings.Contains(got[0].Summary, "no active works") {
		t.Errorf("summary missing 'no active works': %q", got[0].Summary)
	}
}

// All works wired AND match at least one bead → single green finding.
func TestBeadFilterCoverageDetector_Green_AllWired(t *testing.T) {
	ctx, r := newBeadFilterCovCtx(t)
	writeSpec(t, r, "alpha", "bead_filter:\n  label: \"work:alpha\"\n")
	writeSpec(t, r, "bravo", "bead_filter:\n  label: \"work:bravo\"\n")
	withBeadLoader(t, []beads.Bead{
		{ID: "x1", Status: "open", Labels: []string{"work:alpha"}},
		{ID: "x2", Status: "open", Labels: []string{"work:bravo"}},
	})

	got, err := (beadFilterCoverageDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 1 || got[0].Severity != Green {
		t.Fatalf("want one green finding; got %+v", got)
	}
	if !strings.Contains(got[0].Summary, "2 of 2 works wired") {
		t.Errorf("summary missing wired count: %q", got[0].Summary)
	}
}

// All works carry a parsing-valid bead_filter that nonetheless resolves
// to zero beads. The detector emits a single "empty" yellow row.
func TestBeadFilterCoverageDetector_Yellow_AllWired_AllEmpty(t *testing.T) {
	ctx, r := newBeadFilterCovCtx(t)
	writeSpec(t, r, "alpha", "bead_filter:\n  label: \"work:alpha\"\n")
	writeSpec(t, r, "bravo", "bead_filter:\n  label: \"work:bravo\"\n")
	withBeadLoader(t, nil) // empty bead set → all filters resolve to zero.

	got, err := (beadFilterCoverageDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 finding; got %d: %+v", len(got), got)
	}
	if got[0].Severity != Yellow {
		t.Errorf("severity = %q, want yellow", got[0].Severity)
	}
	if !strings.Contains(got[0].Summary, "empty filter") {
		t.Errorf("summary missing 'empty filter': %q", got[0].Summary)
	}
	if got[0].Hint == "" {
		t.Error("missing hint")
	}
	if len(got[0].Items) != 2 {
		t.Errorf("want 2 items; got %d", len(got[0].Items))
	}
}

// One work unwired (no bead_filter key) — emits the "unwired" yellow row.
// Other works are wired with literal filters that resolve to zero (which
// would normally produce an "empty" row too — this test isolates the
// unwired branch by using a single work).
func TestBeadFilterCoverageDetector_Yellow_OneUnwired(t *testing.T) {
	ctx, r := newBeadFilterCovCtx(t)
	withBeadLoader(t, nil)
	// One unwired work, no other works.
	writeSpec(t, r, "alpha", "")

	got, err := (beadFilterCoverageDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 finding; got %d: %+v", len(got), got)
	}
	if got[0].Severity != Yellow {
		t.Errorf("severity = %q, want yellow", got[0].Severity)
	}
	if !strings.Contains(got[0].Summary, "unwired") {
		t.Errorf("summary missing 'unwired': %q", got[0].Summary)
	}
	if got[0].Hint == "" {
		t.Error("missing hint")
	}
	if len(got[0].Items) != 1 || got[0].Items[0].Target != "alpha" {
		t.Errorf("want one item targeting 'alpha'; got %+v", got[0].Items)
	}
	if !strings.Contains(got[0].Items[0].Detail, "no bead_filter") {
		t.Errorf("detail missing 'no bead_filter': %q", got[0].Items[0].Detail)
	}
}

// Mixed: one unwired + one empty → two yellow rows (unwired first).
func TestBeadFilterCoverageDetector_Yellow_MixedUnwiredAndEmpty(t *testing.T) {
	ctx, r := newBeadFilterCovCtx(t)
	withBeadLoader(t, nil)
	writeSpec(t, r, "alpha", "")                                        // unwired
	writeSpec(t, r, "bravo", "bead_filter:\n  label: \"work:bravo\"\n") // empty

	got, err := (beadFilterCoverageDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 findings (unwired + empty); got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Summary, "unwired") {
		t.Errorf("first finding summary should mention 'unwired'; got %q", got[0].Summary)
	}
	if !strings.Contains(got[1].Summary, "empty filter") {
		t.Errorf("second finding summary should mention 'empty filter'; got %q", got[1].Summary)
	}
	for _, f := range got {
		if f.Severity != Yellow {
			t.Errorf("finding %q: severity = %q, want yellow", f.Summary, f.Severity)
		}
	}
}
