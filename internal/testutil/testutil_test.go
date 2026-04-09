package testutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/spec"
)

func TestSetupBench(t *testing.T) {
	bench := SetupBench(t)

	AssertFileExists(t, bench)
	AssertFileExists(t, filepath.Join(bench, "projects"))
}

func TestSetupGitRepo(t *testing.T) {
	repo := SetupGitRepo(t)

	AssertFileExists(t, filepath.Join(repo, ".git"))

	// Verify there is at least one commit.
	logFile := filepath.Join(repo, ".git", "refs", "heads")
	entries, err := os.ReadDir(logFile)
	if err != nil {
		t.Fatalf("reading git heads: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one branch head after initial commit")
	}
}

func TestSetupWork_Defaults(t *testing.T) {
	bench := SetupBench(t)
	workDir := SetupWork(t, bench, "my-project", "blue-bear")

	specPath := filepath.Join(workDir, "spec.yaml")
	AssertFileExists(t, specPath)
	AssertYAMLField(t, specPath, "codename", "blue-bear")
	AssertYAMLField(t, specPath, "status", "problem-space")
	AssertYAMLField(t, specPath, "jig", "feature")
	AssertYAMLField(t, specPath, "jig_version", 1)
}

func TestSetupWork_WithOptions(t *testing.T) {
	bench := SetupBench(t)
	sess := spec.Session{
		ID:      strPtr("sess-1"),
		Started: time.Now().UTC().Truncate(time.Second),
	}
	dep := spec.Dependency{
		Codename:     "other-work",
		Relationship: "must-complete-first",
	}

	workDir := SetupWork(t, bench, "proj", "red-fox",
		WithStatus("research"),
		WithJig("bug"),
		WithSessions(sess),
		WithDeps(dep),
	)

	specPath := filepath.Join(workDir, "spec.yaml")
	AssertFileExists(t, specPath)
	AssertYAMLField(t, specPath, "status", "research")
	AssertYAMLField(t, specPath, "jig", "bug")

	// Verify the spec round-trips correctly.
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if len(s.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(s.Sessions))
	}
	if s.Sessions[0].ID == nil || *s.Sessions[0].ID != "sess-1" {
		t.Errorf("session ID mismatch: got %v", s.Sessions[0].ID)
	}
	if len(s.DependsOn) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(s.DependsOn))
	}
	if s.DependsOn[0].Codename != "other-work" {
		t.Errorf("dependency codename mismatch: got %q", s.DependsOn[0].Codename)
	}
}

func TestFixtureJig_Feature(t *testing.T) {
	data := FixtureJig("feature")
	if len(data) == 0 {
		t.Fatal("FixtureJig(feature) returned empty")
	}
	AssertContainsString(t, string(data), "name: feature")
	AssertContainsString(t, string(data), "status_values:")
}

func TestFixtureJig_Bug(t *testing.T) {
	data := FixtureJig("bug")
	if len(data) == 0 {
		t.Fatal("FixtureJig(bug) returned empty")
	}
	AssertContainsString(t, string(data), "name: bug")
}

func TestFixtureJig_UnknownPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unknown jig name")
		}
	}()
	FixtureJig("nonexistent")
}

func TestAssertFileExists_Missing(t *testing.T) {
	mt := &mockT{}
	AssertFileExists(mt, "/nonexistent/path/file.txt")
	if !mt.errored {
		t.Error("expected AssertFileExists to report error for missing file")
	}
}

func TestAssertFileContains(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.txt")
	os.WriteFile(p, []byte("hello world"), 0644)

	AssertFileContains(t, p, "world")

	mt := &mockT{}
	AssertFileContains(mt, p, "missing")
	if !mt.errored {
		t.Error("expected AssertFileContains to report error for missing substring")
	}
}

func TestAssertYAMLField(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.yaml")
	os.WriteFile(p, []byte("name: kerf\nversion: 1\n"), 0644)

	AssertYAMLField(t, p, "name", "kerf")
	AssertYAMLField(t, p, "version", 1)

	mt := &mockT{}
	AssertYAMLField(mt, p, "name", "wrong")
	if !mt.errored {
		t.Error("expected AssertYAMLField to report error for wrong value")
	}

	mt2 := &mockT{}
	AssertYAMLField(mt2, p, "nonexistent", "x")
	if !mt2.errored {
		t.Error("expected AssertYAMLField to report error for missing field")
	}
}

// helpers

func strPtr(s string) *string { return &s }

// AssertContainsString is used within testutil's own tests.
func AssertContainsString(t *testing.T, s, substr string) {
	t.Helper()
	if !contains(s, substr) {
		t.Errorf("string does not contain %q", substr)
	}
}

// mockT implements testutil.TB to capture failure state without stopping
// the real test.
type mockT struct {
	errored bool
}

func (m *mockT) Helper()                          {}
func (m *mockT) Errorf(format string, args ...any) { m.errored = true }
func (m *mockT) Fatalf(format string, args ...any) { m.errored = true }
