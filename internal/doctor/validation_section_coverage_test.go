package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/storage"
)

// newValidationCoverageCase builds a bench-mode Context for the
// validation-section-coverage detector. The test-helper name is bead-specific
// (kerf-ystq) to avoid colliding with newProjectYAMLCtx / newStorageDriftCtx /
// newBeadFilterCovCtx (sibling detectors have collided on generic helper
// names in the past — see plan-025-B4 brief).
func newValidationCoverageCase(t *testing.T) (*Context, *storage.Resolver) {
	t.Helper()
	bench := t.TempDir()
	r, err := storage.NewResolver(bench, "p-test", "")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	if err := os.MkdirAll(r.WorksDir(), 0o755); err != nil {
		t.Fatalf("MkdirAll works: %v", err)
	}
	// Minimal project.yaml — most detectors require it but ours doesn't read
	// it. Written for parity with sibling tests.
	cfg := "jigs:\n  - plan\n  - spec\n  - bug\n  - implementation\n"
	if err := os.WriteFile(r.ProjectConfigPath(), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}
	return &Context{
		ProjectID: "p-test",
		Resolver:  r,
		BenchPath: bench,
	}, r
}

// writeValidationWork writes a minimal spec.yaml plus an artifact at relpath
// with the given body, for a work using the named jig.
func writeValidationWork(t *testing.T, r *storage.Resolver, codename, jig, status, relpath, body string) {
	t.Helper()
	dir := r.WorkDir(codename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	specBody := fmt.Sprintf(`codename: %s
type: implementation
project:
  id: p-test
jig: %s
jig_version: 1
status: %s
status_values:
  - %s
created: %s
updated: %s
sessions: []
depends_on: []
pinned_beads: []
implementation:
  branch: null
  pr: null
  commits: []
`, codename, jig, status, status, now, now)
	if err := os.WriteFile(filepath.Join(dir, "spec.yaml"), []byte(specBody), 0o644); err != nil {
		t.Fatalf("write spec.yaml: %v", err)
	}
	if relpath != "" {
		abs := filepath.Join(dir, filepath.FromSlash(relpath))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir artifact dir: %v", err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
	}
}

const compliantArtifact = `# Tasks

Some prose.

**What done looks like:**

- The thing exists
- Scenario-test item filed with ID ` + "`kerf-aaa`" + ` (see jig-system.md)
- Exploratory-test item filed with ID ` + "`kerf-bbb`" + ` (see jig-system.md)
`

const missingItemsArtifact = `# Tasks

**What done looks like:**

- Tasks listed
- Dependencies form a DAG
`

const emptyIDArtifact = `# Tasks

**What done looks like:**

- Tasks listed
- Scenario-test item filed with ID ` + "`<id>`" + `
- Exploratory-test item filed with ID ` + "`<id>`" + `
`

const missingBlockArtifact = `# Tasks

No checklist heading at all.

- Just a bullet
`

func runValidationDetector(t *testing.T, ctx *Context) []Finding {
	t.Helper()
	findings, err := (validationSectionCoverageDetector{}).Run(ctx)
	if err != nil {
		t.Fatalf("detector run: %v", err)
	}
	return findings
}

func TestValidationCoverage_AllGreen(t *testing.T) {
	ctx, r := newValidationCoverageCase(t)
	// Plan jig: both Change Spec (per-component glob) and Tasks compliant.
	writeValidationWork(t, r, "wing-alpha", "plan", "tasks", "07-tasks.md", compliantArtifact)
	// Add a per-component change spec.
	abs := filepath.Join(r.WorkDir("wing-alpha"), "05-specs", "core-spec.md")
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(compliantArtifact), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	findings := runValidationDetector(t, ctx)
	if len(findings) != 1 || findings[0].Severity != Green {
		t.Fatalf("expected single green finding, got %+v", findings)
	}
}

func TestValidationCoverage_MissingItems(t *testing.T) {
	ctx, r := newValidationCoverageCase(t)
	writeValidationWork(t, r, "wing-beta", "bug", "fix-spec", "05-fix-spec.md", missingItemsArtifact)

	findings := runValidationDetector(t, ctx)
	if len(findings) != 1 || findings[0].Severity != Yellow {
		t.Fatalf("expected yellow, got %+v", findings)
	}
	f := findings[0]
	if !strings.Contains(f.Items[0].Detail, "missing both") {
		t.Errorf("expected 'missing both' detail, got %q", f.Items[0].Detail)
	}
	if !strings.Contains(f.Hint, "05-fix-spec.md") {
		t.Errorf("hint must reference artifact path, got %q", f.Hint)
	}
	if !strings.Contains(f.Hint, "What done looks like") {
		t.Errorf("hint must reference the section, got %q", f.Hint)
	}
}

func TestValidationCoverage_EmptyID(t *testing.T) {
	ctx, r := newValidationCoverageCase(t)
	writeValidationWork(t, r, "wing-gamma", "implementation", "breakdown", "01-breakdown.md", emptyIDArtifact)

	findings := runValidationDetector(t, ctx)
	if len(findings) != 1 || findings[0].Severity != Yellow {
		t.Fatalf("expected yellow, got %+v", findings)
	}
	if !strings.Contains(findings[0].Items[0].Detail, "empty <id>") {
		t.Errorf("expected 'empty <id>' detail, got %q", findings[0].Items[0].Detail)
	}
}

func TestValidationCoverage_MissingBlock(t *testing.T) {
	ctx, r := newValidationCoverageCase(t)
	writeValidationWork(t, r, "wing-delta", "bug", "fix-spec", "05-fix-spec.md", missingBlockArtifact)

	findings := runValidationDetector(t, ctx)
	if len(findings) != 1 || findings[0].Severity != Yellow {
		t.Fatalf("expected yellow, got %+v", findings)
	}
	if !strings.Contains(findings[0].Items[0].Detail, "no 'What done looks like'") {
		t.Errorf("expected missing-block detail, got %q", findings[0].Items[0].Detail)
	}
}

func TestValidationCoverage_RetrofitExcluded(t *testing.T) {
	ctx, r := newValidationCoverageCase(t)
	// retrofit jig is excluded — even with a broken artifact, no finding.
	writeValidationWork(t, r, "wing-retro", "retrofit", "drafting", "05-fix-spec.md", missingItemsArtifact)

	findings := runValidationDetector(t, ctx)
	if len(findings) != 1 || findings[0].Severity != Green {
		t.Fatalf("expected green (retrofit excluded), got %+v", findings)
	}
}

func TestValidationCoverage_SpikeExcluded(t *testing.T) {
	ctx, r := newValidationCoverageCase(t)
	writeValidationWork(t, r, "wing-spike", "spike", "exploring", "05-fix-spec.md", missingItemsArtifact)

	findings := runValidationDetector(t, ctx)
	if len(findings) != 1 || findings[0].Severity != Green {
		t.Fatalf("expected green (spike excluded), got %+v", findings)
	}
}

func TestValidationCoverage_ArchivedExcluded(t *testing.T) {
	ctx, r := newValidationCoverageCase(t)
	writeValidationWork(t, r, "wing-arch", "bug", "archived", "05-fix-spec.md", missingItemsArtifact)

	findings := runValidationDetector(t, ctx)
	if len(findings) != 1 || findings[0].Severity != Green {
		t.Fatalf("expected green (archived excluded), got %+v", findings)
	}
}

func TestValidationCoverage_NoArtifactNoFinding(t *testing.T) {
	ctx, r := newValidationCoverageCase(t)
	// Bug jig with no fix-spec artifact yet — pass hasn't run; no finding.
	writeValidationWork(t, r, "wing-pre", "bug", "reported", "", "")

	findings := runValidationDetector(t, ctx)
	if len(findings) != 1 || findings[0].Severity != Green {
		t.Fatalf("expected green (no artifact yet), got %+v", findings)
	}
}

func TestValidationCoverage_ImplementationVerifyClosurePhrasing(t *testing.T) {
	ctx, r := newValidationCoverageCase(t)
	// implementation jig Verify pass uses "with ID ... is closed" wording —
	// detector must accept the closure-check phrasing too (per
	// internal/jig/builtin/implementation.md Pass 4).
	body := `# Verify

**What done looks like:**

- Acceptance criteria met
- Scenario-test item with ID ` + "`kerf-xyz`" + ` is closed
- Exploratory-test item with ID ` + "`kerf-uvw`" + ` is closed
`
	writeValidationWork(t, r, "wing-impl", "implementation", "verify", "03-verify.md", body)

	findings := runValidationDetector(t, ctx)
	if len(findings) != 1 || findings[0].Severity != Green {
		t.Fatalf("expected green (closure phrasing accepted), got %+v", findings)
	}
}
