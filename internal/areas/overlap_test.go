package areas

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gregberns/kerf/internal/storage"
)

func benchResolver(bp, projectID string) *storage.Resolver {
	r, _ := storage.NewResolver(bp, projectID, "")
	return r
}

// writeSpecWithAreas writes a minimal spec.yaml with the given areas.
func writeSpecWithAreas(t *testing.T, benchPath, projectID, codename, status string, areas []string) {
	t.Helper()
	workDir := filepath.Join(benchPath, "projects", projectID, codename)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := "codename: " + codename + "\n"
	content += "type: plan\n"
	content += "project:\n  id: " + projectID + "\n"
	content += "jig: plan\njig_version: 1\n"
	content += "status: " + status + "\n"
	content += "status_values: [problem-space, tasks, ready]\n"
	content += "created: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n"
	content += "sessions: []\ndepends_on: []\n"
	content += "implementation:\n  branch: null\n  pr: null\n  commits: []\n"

	if len(areas) > 0 {
		content += "areas:\n"
		for _, a := range areas {
			content += "  - " + a + "\n"
		}
	}

	specPath := filepath.Join(workDir, "spec.yaml")
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindOverlappingWorks_NoOverlap(t *testing.T) {
	bp := t.TempDir()
	proj := "test-proj"

	writeSpecWithAreas(t, bp, proj, "work-a", "tasks", []string{"auth"})
	writeSpecWithAreas(t, bp, proj, "work-b", "tasks", []string{"api"})

	entries, err := FindOverlappingWorks(benchResolver(bp, proj), []string{"database"}, "new-work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no overlaps, got %d", len(entries))
	}
}

func TestFindOverlappingWorks_SingleOverlap(t *testing.T) {
	bp := t.TempDir()
	proj := "test-proj"

	writeSpecWithAreas(t, bp, proj, "work-a", "research", []string{"auth", "api"})
	writeSpecWithAreas(t, bp, proj, "work-b", "tasks", []string{"database"})

	entries, err := FindOverlappingWorks(benchResolver(bp, proj), []string{"auth"}, "new-work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(entries))
	}
	if entries[0].Codename != "work-a" {
		t.Errorf("codename = %q, want %q", entries[0].Codename, "work-a")
	}
	if entries[0].Status != "research" {
		t.Errorf("status = %q, want %q", entries[0].Status, "research")
	}
	if len(entries[0].SharedAreas) != 1 || entries[0].SharedAreas[0] != "auth" {
		t.Errorf("shared areas = %v, want [auth]", entries[0].SharedAreas)
	}
}

func TestFindOverlappingWorks_MultipleOverlaps(t *testing.T) {
	bp := t.TempDir()
	proj := "test-proj"

	writeSpecWithAreas(t, bp, proj, "work-a", "research", []string{"auth", "api"})
	writeSpecWithAreas(t, bp, proj, "work-b", "tasks", []string{"auth", "database"})

	entries, err := FindOverlappingWorks(benchResolver(bp, proj), []string{"auth", "api"}, "new-work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 overlaps, got %d", len(entries))
	}

	// work-a shares auth and api.
	if entries[0].Codename != "work-a" {
		t.Errorf("entries[0].Codename = %q, want %q", entries[0].Codename, "work-a")
	}
	if len(entries[0].SharedAreas) != 2 {
		t.Errorf("entries[0].SharedAreas = %v, want [api, auth]", entries[0].SharedAreas)
	}

	// work-b shares auth only.
	if entries[1].Codename != "work-b" {
		t.Errorf("entries[1].Codename = %q, want %q", entries[1].Codename, "work-b")
	}
	if len(entries[1].SharedAreas) != 1 || entries[1].SharedAreas[0] != "auth" {
		t.Errorf("entries[1].SharedAreas = %v, want [auth]", entries[1].SharedAreas)
	}
}

func TestFindOverlappingWorks_ExcludesSelf(t *testing.T) {
	bp := t.TempDir()
	proj := "test-proj"

	writeSpecWithAreas(t, bp, proj, "self-work", "tasks", []string{"auth"})

	entries, err := FindOverlappingWorks(benchResolver(bp, proj), []string{"auth"}, "self-work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected self to be excluded, got %d entries", len(entries))
	}
}

func TestFindOverlappingWorks_EmptyTargetAreas(t *testing.T) {
	bp := t.TempDir()
	proj := "test-proj"

	writeSpecWithAreas(t, bp, proj, "work-a", "tasks", []string{"auth"})

	entries, err := FindOverlappingWorks(benchResolver(bp, proj), nil, "new-work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no overlaps for empty target, got %d", len(entries))
	}
}

func TestFindOverlappingWorks_NoWorks(t *testing.T) {
	bp := t.TempDir()
	proj := "test-proj"

	// Create the project dir but no works.
	os.MkdirAll(filepath.Join(bp, "projects", proj), 0o755)

	entries, err := FindOverlappingWorks(benchResolver(bp, proj), []string{"auth"}, "new-work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no overlaps, got %d", len(entries))
	}
}

func TestFindOverlappingWorks_SkipsWorksWithNoAreas(t *testing.T) {
	bp := t.TempDir()
	proj := "test-proj"

	writeSpecWithAreas(t, bp, proj, "no-areas", "tasks", nil)
	writeSpecWithAreas(t, bp, proj, "has-areas", "tasks", []string{"auth"})

	entries, err := FindOverlappingWorks(benchResolver(bp, proj), []string{"auth"}, "new-work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(entries))
	}
	if entries[0].Codename != "has-areas" {
		t.Errorf("codename = %q, want %q", entries[0].Codename, "has-areas")
	}
}
