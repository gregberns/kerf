// Package testutil provides shared test helpers for kerf tests.
package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/gberns/kerf/internal/spec"
)

// SetupBench creates a temporary bench directory and returns its path.
// The directory is automatically cleaned up when the test finishes.
func SetupBench(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	benchDir := filepath.Join(dir, "bench")
	if err := os.MkdirAll(filepath.Join(benchDir, "projects"), 0755); err != nil {
		t.Fatalf("SetupBench: %v", err)
	}
	return benchDir
}

// SetupGitRepo creates a temporary git repo with an initial commit and returns
// the repo path. The directory is automatically cleaned up when the test finishes.
func SetupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@kerf.dev"},
		{"config", "user.name", "kerf-test"},
		{"commit", "--allow-empty", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("SetupGitRepo: git %v: %s: %v", args, out, err)
		}
	}

	return dir
}

// WorkOpt is a functional option for SetupWork.
type WorkOpt func(*workConfig)

type workConfig struct {
	status   string
	jig      string
	sessions []spec.Session
	deps     []spec.Dependency
}

// WithStatus sets the work's status.
func WithStatus(status string) WorkOpt {
	return func(c *workConfig) { c.status = status }
}

// WithJig sets the work's jig name.
func WithJig(jig string) WorkOpt {
	return func(c *workConfig) { c.jig = jig }
}

// WithSessions sets the work's session list.
func WithSessions(sessions ...spec.Session) WorkOpt {
	return func(c *workConfig) { c.sessions = sessions }
}

// WithDeps sets the work's dependencies.
func WithDeps(deps ...spec.Dependency) WorkOpt {
	return func(c *workConfig) { c.deps = deps }
}

// SetupWork creates a work directory with a valid spec.yaml under the given
// bench path. Returns the path to the work directory.
func SetupWork(t *testing.T, benchPath, projectID, codename string, opts ...WorkOpt) string {
	t.Helper()

	cfg := workConfig{
		status: "problem-space",
		jig:    "feature",
	}
	for _, o := range opts {
		o(&cfg)
	}

	projectDir := filepath.Join(benchPath, "projects", projectID)
	workDir := filepath.Join(projectDir, codename)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("SetupWork: create work dir: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	s := &spec.SpecYAML{
		Codename: codename,
		Type:     cfg.jig,
		Project:  spec.Project{ID: projectID},
		Jig:      cfg.jig,
		JigVersion: 1,
		Status:   cfg.status,
		StatusValues: []string{
			"problem-space", "decomposition", "research",
			"detailed-spec", "review", "ready",
		},
		Created:   now,
		Updated:   now,
		Sessions:  cfg.sessions,
		DependsOn: cfg.deps,
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		t.Fatalf("SetupWork: marshal spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "spec.yaml"), data, 0644); err != nil {
		t.Fatalf("SetupWork: write spec.yaml: %v", err)
	}

	return workDir
}

// FixtureJig returns a minimal but valid jig definition for test use.
// Supported names: "feature", "bug". Panics on unknown name.
func FixtureJig(name string) []byte {
	j, ok := fixtureJigs[name]
	if !ok {
		panic(fmt.Sprintf("testutil.FixtureJig: unknown jig %q", name))
	}
	return []byte(j)
}

var fixtureJigs = map[string]string{
	"feature": `---
name: feature
description: Full specification process for new features and subsystems
version: 1
status_values:
  - problem-space
  - decomposition
  - research
  - detailed-spec
  - review
  - ready
passes:
  - name: "Problem Space"
    status: problem-space
    output: ["01-problem-space.md"]
  - name: "Decomposition"
    status: decomposition
    output: ["02-components.md"]
  - name: "Research"
    status: research
    output: ["03-research/{component}/findings.md"]
  - name: "Detailed Spec"
    status: detailed-spec
    output: ["04-plans/{component}-spec.md"]
  - name: "Integration & Review"
    status: review
    output: ["05-integration.md", "06-checklist.md", "SPEC.md"]
file_structure:
  - spec.yaml
  - SESSION.md
  - 01-problem-space.md
  - 02-components.md
  - "03-research/{component}/findings.md"
  - "04-plans/{component}-spec.md"
  - 05-integration.md
  - 06-checklist.md
  - SPEC.md
---

# Feature Jig

Guides an agent through structured feature specification.
`,
	"bug": `---
name: bug
description: Structured investigation and resolution of defects
version: 1
status_values:
  - investigating
  - root-cause
  - fix-spec
  - ready
passes:
  - name: "Investigation"
    status: investigating
    output: ["01-investigation.md"]
  - name: "Root Cause"
    status: root-cause
    output: ["02-root-cause.md"]
  - name: "Fix Spec"
    status: fix-spec
    output: ["03-fix-spec.md"]
file_structure:
  - spec.yaml
  - SESSION.md
  - 01-investigation.md
  - 02-root-cause.md
  - 03-fix-spec.md
---

# Bug Jig

Guides an agent through structured bug investigation and fix specification.
`,
}

// TB is the subset of testing.TB used by assertion helpers, allowing
// both *testing.T and test doubles.
type TB interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// AssertFileExists fails the test if the file at path does not exist.
func AssertFileExists(t TB, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	} else if err != nil {
		t.Errorf("checking file %s: %v", path, err)
	}
}

// AssertFileContains fails the test if the file at path does not contain substr.
func AssertFileContains(t TB, path, substr string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !contains(string(data), substr) {
		t.Errorf("file %s does not contain %q", path, substr)
	}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// AssertYAMLField reads a YAML file and checks that the given top-level field
// matches expected. The field value is compared as a string representation for
// scalar types, or via YAML round-trip for complex types.
func AssertYAMLField(t TB, path, field string, expected any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing YAML from %s: %v", path, err)
	}

	val, ok := raw[field]
	if !ok {
		t.Errorf("field %q not found in %s", field, path)
		return
	}

	// Compare using fmt.Sprint for simple scalar comparison.
	got := fmt.Sprintf("%v", val)
	want := fmt.Sprintf("%v", expected)
	if got != want {
		t.Errorf("field %q in %s: got %v, want %v", field, path, got, want)
	}
}
