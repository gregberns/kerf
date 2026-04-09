package dep

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/spec"
)

func setupBench(t *testing.T) string {
	t.Helper()
	bench := t.TempDir()
	return bench
}

func createWork(t *testing.T, bench, project, codename, status string, statusValues []string) {
	t.Helper()
	dir := filepath.Join(bench, "projects", project, codename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	s := &spec.SpecYAML{
		Codename:     codename,
		Type:         "feature",
		Jig:          "feature",
		JigVersion:   1,
		Status:       status,
		StatusValues: statusValues,
		Created:      time.Now().UTC().Truncate(time.Second),
		Updated:      time.Now().UTC().Truncate(time.Second),
		Project:      spec.Project{ID: project},
	}
	if err := spec.Write(filepath.Join(dir, "spec.yaml"), s); err != nil {
		t.Fatal(err)
	}
}

func TestIsComplete_AtTerminal(t *testing.T) {
	vals := []string{"draft", "review", "ready"}
	if !IsComplete("ready", vals) {
		t.Error("terminal status should be complete")
	}
}

func TestIsComplete_BeforeTerminal(t *testing.T) {
	vals := []string{"draft", "review", "ready"}
	if IsComplete("draft", vals) {
		t.Error("pre-terminal status should not be complete")
	}
	if IsComplete("review", vals) {
		t.Error("pre-terminal status should not be complete")
	}
}

func TestIsComplete_PastTerminal(t *testing.T) {
	vals := []string{"draft", "review", "ready"}
	if !IsComplete("finalized", vals) {
		t.Error("status not in list should be considered past terminal")
	}
}

func TestIsComplete_EmptyValues(t *testing.T) {
	if IsComplete("anything", nil) {
		t.Error("empty status values should not be complete")
	}
}

func TestResolve_SameProject(t *testing.T) {
	bench := setupBench(t)
	createWork(t, bench, "myproj", "dep-work", "ready", []string{"draft", "ready"})

	d := spec.Dependency{
		Codename:     "dep-work",
		Relationship: "must-complete-first",
	}
	result := Resolve(d, bench, "myproj")

	if result.Unresolvable {
		t.Error("should be resolvable")
	}
	if result.Status != "ready" {
		t.Errorf("status = %q, want ready", result.Status)
	}
	if !result.Complete {
		t.Error("should be complete")
	}
	if result.Project != "myproj" {
		t.Errorf("project = %q, want myproj", result.Project)
	}
}

func TestResolve_CrossProject(t *testing.T) {
	bench := setupBench(t)
	createWork(t, bench, "other-proj", "dep-work", "draft", []string{"draft", "ready"})

	otherProj := "other-proj"
	d := spec.Dependency{
		Codename:     "dep-work",
		Project:      &otherProj,
		Relationship: "must-complete-first",
	}
	result := Resolve(d, bench, "myproj")

	if result.Unresolvable {
		t.Error("should be resolvable")
	}
	if result.Status != "draft" {
		t.Errorf("status = %q, want draft", result.Status)
	}
	if result.Complete {
		t.Error("should not be complete")
	}
	if result.Project != "other-proj" {
		t.Errorf("project = %q, want other-proj", result.Project)
	}
}

func TestResolve_Unresolvable(t *testing.T) {
	bench := setupBench(t)
	d := spec.Dependency{
		Codename:     "nonexistent",
		Relationship: "must-complete-first",
	}
	result := Resolve(d, bench, "myproj")

	if !result.Unresolvable {
		t.Error("should be unresolvable")
	}
}

func TestCheckBlocking(t *testing.T) {
	bench := setupBench(t)
	createWork(t, bench, "myproj", "blocking-dep", "draft", []string{"draft", "ready"})
	createWork(t, bench, "myproj", "info-dep", "draft", []string{"draft", "ready"})

	deps := []spec.Dependency{
		{Codename: "blocking-dep", Relationship: "must-complete-first"},
		{Codename: "info-dep", Relationship: "inform"},
	}
	results := CheckBlocking(deps, bench, "myproj")

	if len(results) != 1 {
		t.Fatalf("CheckBlocking should return only must-complete-first deps, got %d", len(results))
	}
	if results[0].Codename != "blocking-dep" {
		t.Errorf("codename = %q, want blocking-dep", results[0].Codename)
	}
}

func TestCheckBlocking_SkipsInform(t *testing.T) {
	bench := setupBench(t)
	deps := []spec.Dependency{
		{Codename: "info-only", Relationship: "inform"},
	}
	results := CheckBlocking(deps, bench, "myproj")
	if len(results) != 0 {
		t.Errorf("inform relationships should be skipped, got %d results", len(results))
	}
}
