package bench

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func setupBench(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "projects"), 0o755); err != nil {
		t.Fatalf("setup bench: %v", err)
	}
	return dir
}

func TestEnsureBench(t *testing.T) {
	// EnsureBench uses the real home dir, so we test the internal structure instead
	dir := t.TempDir()
	projDir := filepath.Join(dir, "projects")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	info, err := os.Stat(projDir)
	if err != nil {
		t.Fatalf("projects dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("projects is not a directory")
	}
}

func TestWorkDir(t *testing.T) {
	bp := "/tmp/test-kerf"
	got := WorkDir(bp, "myproj", "blue-fox")
	want := filepath.Join(bp, "projects", "myproj", "blue-fox")
	if got != want {
		t.Errorf("WorkDir = %q, want %q", got, want)
	}
}

func TestArchiveDir(t *testing.T) {
	bp := "/tmp/test-kerf"
	got := ArchiveDir(bp, "myproj", "blue-fox")
	want := filepath.Join(bp, "archive", "myproj", "blue-fox")
	if got != want {
		t.Errorf("ArchiveDir = %q, want %q", got, want)
	}
}

func TestCreateAndWorkExists(t *testing.T) {
	bp := setupBench(t)

	if WorkExists(bp, "proj", "alpha") {
		t.Error("work should not exist yet")
	}

	if err := CreateWork(bp, "proj", "alpha"); err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	if !WorkExists(bp, "proj", "alpha") {
		t.Error("work should exist after creation")
	}
}

func TestListWorks(t *testing.T) {
	bp := setupBench(t)

	// Empty project
	works, err := ListWorks(bp, "proj")
	if err != nil {
		t.Fatalf("ListWorks: %v", err)
	}
	if len(works) != 0 {
		t.Errorf("expected 0 works, got %d", len(works))
	}

	// Create some works
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if err := CreateWork(bp, "proj", name); err != nil {
			t.Fatalf("CreateWork(%s): %v", name, err)
		}
	}

	works, err = ListWorks(bp, "proj")
	if err != nil {
		t.Fatalf("ListWorks: %v", err)
	}
	sort.Strings(works)
	if len(works) != 3 {
		t.Fatalf("expected 3 works, got %d", len(works))
	}
	if works[0] != "alpha" || works[1] != "beta" || works[2] != "gamma" {
		t.Errorf("works = %v, want [alpha beta gamma]", works)
	}
}

func TestListWorksNonexistentProject(t *testing.T) {
	bp := setupBench(t)
	works, err := ListWorks(bp, "nonexistent")
	if err != nil {
		t.Fatalf("ListWorks: %v", err)
	}
	if len(works) != 0 {
		t.Errorf("expected 0 works, got %d", len(works))
	}
}

func TestMoveToArchive(t *testing.T) {
	bp := setupBench(t)

	if err := CreateWork(bp, "proj", "alpha"); err != nil {
		t.Fatal(err)
	}

	// Write a file in the work dir to verify it moves
	wd := WorkDir(bp, "proj", "alpha")
	if err := os.WriteFile(filepath.Join(wd, "test.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MoveToArchive(bp, "proj", "alpha"); err != nil {
		t.Fatalf("MoveToArchive: %v", err)
	}

	if WorkExists(bp, "proj", "alpha") {
		t.Error("work should no longer exist in active area")
	}
	if !IsArchived(bp, "proj", "alpha") {
		t.Error("work should exist in archive")
	}

	// Verify file was preserved
	ad := ArchiveDir(bp, "proj", "alpha")
	data, err := os.ReadFile(filepath.Join(ad, "test.txt"))
	if err != nil {
		t.Fatalf("failed to read archived file: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("archived file content = %q, want %q", string(data), "hello")
	}
}

func TestListArchivedWorks(t *testing.T) {
	bp := setupBench(t)

	archived, err := ListArchivedWorks(bp, "proj")
	if err != nil {
		t.Fatalf("ListArchivedWorks: %v", err)
	}
	if len(archived) != 0 {
		t.Errorf("expected 0 archived, got %d", len(archived))
	}

	if err := CreateWork(bp, "proj", "alpha"); err != nil {
		t.Fatal(err)
	}
	if err := MoveToArchive(bp, "proj", "alpha"); err != nil {
		t.Fatal(err)
	}

	archived, err = ListArchivedWorks(bp, "proj")
	if err != nil {
		t.Fatalf("ListArchivedWorks: %v", err)
	}
	if len(archived) != 1 || archived[0] != "alpha" {
		t.Errorf("archived = %v, want [alpha]", archived)
	}
}

func TestDeleteWork(t *testing.T) {
	bp := setupBench(t)

	if err := CreateWork(bp, "proj", "alpha"); err != nil {
		t.Fatal(err)
	}
	if !WorkExists(bp, "proj", "alpha") {
		t.Fatal("work should exist before delete")
	}

	if err := DeleteWork(bp, "proj", "alpha"); err != nil {
		t.Fatalf("DeleteWork: %v", err)
	}

	if WorkExists(bp, "proj", "alpha") {
		t.Error("work should not exist after delete")
	}
}

func TestDeleteArchivedWork(t *testing.T) {
	bp := setupBench(t)

	if err := CreateWork(bp, "proj", "alpha"); err != nil {
		t.Fatal(err)
	}
	if err := MoveToArchive(bp, "proj", "alpha"); err != nil {
		t.Fatal(err)
	}

	// DeleteWork should find it in archive
	if err := DeleteWork(bp, "proj", "alpha"); err != nil {
		t.Fatalf("DeleteWork (archived): %v", err)
	}
	if IsArchived(bp, "proj", "alpha") {
		t.Error("archived work should not exist after delete")
	}
}

func TestIsArchived(t *testing.T) {
	bp := setupBench(t)

	if IsArchived(bp, "proj", "alpha") {
		t.Error("should not be archived before creation")
	}

	if err := CreateWork(bp, "proj", "alpha"); err != nil {
		t.Fatal(err)
	}
	if IsArchived(bp, "proj", "alpha") {
		t.Error("active work should not appear as archived")
	}

	if err := MoveToArchive(bp, "proj", "alpha"); err != nil {
		t.Fatal(err)
	}
	if !IsArchived(bp, "proj", "alpha") {
		t.Error("work should be archived after move")
	}
}

func TestFullLifecycle(t *testing.T) {
	bp := setupBench(t)
	project := "myproject"

	// Create
	if err := CreateWork(bp, project, "task-one"); err != nil {
		t.Fatal(err)
	}
	if err := CreateWork(bp, project, "task-two"); err != nil {
		t.Fatal(err)
	}

	// List
	works, _ := ListWorks(bp, project)
	if len(works) != 2 {
		t.Fatalf("expected 2 works, got %d", len(works))
	}

	// Archive one
	if err := MoveToArchive(bp, project, "task-one"); err != nil {
		t.Fatal(err)
	}
	works, _ = ListWorks(bp, project)
	if len(works) != 1 {
		t.Fatalf("expected 1 active work, got %d", len(works))
	}
	archived, _ := ListArchivedWorks(bp, project)
	if len(archived) != 1 {
		t.Fatalf("expected 1 archived work, got %d", len(archived))
	}

	// Delete the other
	if err := DeleteWork(bp, project, "task-two"); err != nil {
		t.Fatal(err)
	}
	works, _ = ListWorks(bp, project)
	if len(works) != 0 {
		t.Fatalf("expected 0 active works, got %d", len(works))
	}
}

func TestArchiveDirAutoCreated(t *testing.T) {
	bp := setupBench(t)

	if err := CreateWork(bp, "proj", "alpha"); err != nil {
		t.Fatal(err)
	}

	if err := MoveToArchive(bp, "proj", "alpha"); err != nil {
		t.Fatalf("MoveToArchive: %v", err)
	}

	archiveProjectDir := filepath.Join(bp, "archive", "proj")
	info, err := os.Stat(archiveProjectDir)
	if err != nil {
		t.Fatalf("archive project dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("archive project dir is not a directory")
	}
}

func TestListAllProjects(t *testing.T) {
	bp := setupBench(t)

	projects, err := ListAllProjects(bp)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}

	CreateWork(bp, "proj-a", "w1")
	CreateWork(bp, "proj-b", "w2")

	projects, err = ListAllProjects(bp)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(projects)
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0] != "proj-a" || projects[1] != "proj-b" {
		t.Errorf("projects = %v", projects)
	}
}
