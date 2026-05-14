package spec

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	title := "Test Work"
	sessionID := "abc-123"
	depProject := "other-project"

	original := &SpecYAML{
		Codename: "test-work",
		Title:    &title,
		Type:     "feature",
		Project:  Project{ID: "my-project"},
		Jig:      "feature",
		JigVersion: 1,
		Status:   "research",
		StatusValues: []string{"problem-space", "decomposition", "research", "detailed-spec", "review", "ready"},
		Created:  time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
		Updated:  time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
		Sessions: []Session{
			{
				ID:      &sessionID,
				Started: time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
				Ended:   nil,
			},
		},
		ActiveSession: &sessionID,
		DependsOn: []Dependency{
			{
				Codename:     "database-migration",
				Project:      &depProject,
				Relationship: "must-complete-first",
			},
		},
		Implementation: Implementation{
			Branch:  nil,
			PR:      nil,
			Commits: []string{},
		},
	}

	if err := Write(path, original); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Check key fields survive round-trip
	if got.Codename != "test-work" {
		t.Errorf("codename = %q, want %q", got.Codename, "test-work")
	}
	if got.Title == nil || *got.Title != "Test Work" {
		t.Errorf("title = %v, want %q", got.Title, "Test Work")
	}
	if got.Type != "feature" {
		t.Errorf("type = %q, want %q", got.Type, "feature")
	}
	if got.Project.ID != "my-project" {
		t.Errorf("project.id = %q, want %q", got.Project.ID, "my-project")
	}
	if got.Jig != "feature" {
		t.Errorf("jig = %q, want %q", got.Jig, "feature")
	}
	if got.JigVersion != 1 {
		t.Errorf("jig_version = %d, want %d", got.JigVersion, 1)
	}
	if got.Status != "research" {
		t.Errorf("status = %q, want %q", got.Status, "research")
	}
	if len(got.StatusValues) != 6 {
		t.Errorf("status_values length = %d, want 6", len(got.StatusValues))
	}
	if got.Created != time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC) {
		t.Errorf("created = %v, want 2026-04-07T10:00:00Z", got.Created)
	}
	// Updated is auto-set by Write, so it will differ from original
	if got.Updated.IsZero() {
		t.Error("updated should not be zero")
	}
	if len(got.Sessions) != 1 {
		t.Fatalf("sessions length = %d, want 1", len(got.Sessions))
	}
	if got.Sessions[0].ID == nil || *got.Sessions[0].ID != "abc-123" {
		t.Errorf("session id = %v, want %q", got.Sessions[0].ID, "abc-123")
	}
	if got.Sessions[0].Ended != nil {
		t.Errorf("session ended = %v, want nil", got.Sessions[0].Ended)
	}
	if got.ActiveSession == nil || *got.ActiveSession != "abc-123" {
		t.Errorf("active_session = %v, want %q", got.ActiveSession, "abc-123")
	}
	if len(got.DependsOn) != 1 {
		t.Fatalf("depends_on length = %d, want 1", len(got.DependsOn))
	}
	if got.DependsOn[0].Codename != "database-migration" {
		t.Errorf("dep codename = %q, want %q", got.DependsOn[0].Codename, "database-migration")
	}
	if got.DependsOn[0].Project == nil || *got.DependsOn[0].Project != "other-project" {
		t.Errorf("dep project = %v, want %q", got.DependsOn[0].Project, "other-project")
	}
	if got.Implementation.Branch != nil {
		t.Errorf("implementation.branch = %v, want nil", got.Implementation.Branch)
	}
	if got.Implementation.PR != nil {
		t.Errorf("implementation.pr = %v, want nil", got.Implementation.PR)
	}
}

func TestReadNonexistent(t *testing.T) {
	_, err := Read("/nonexistent/spec.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")
	os.WriteFile(path, []byte(":::not valid yaml[[["), 0644)

	_, err := Read(path)
	if err == nil {
		t.Error("expected error for malformed YAML")
	}
}

func TestWriteAutoSetsUpdated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	before := time.Now().UTC().Truncate(time.Second)

	s := &SpecYAML{
		Codename: "test",
		Type:     "feature",
		Project:  Project{ID: "proj"},
		Jig:      "feature",
		Status:   "research",
		Created:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := Write(path, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	after := time.Now().UTC().Truncate(time.Second).Add(time.Second)

	if s.Updated.Before(before) || s.Updated.After(after) {
		t.Errorf("updated = %v, expected between %v and %v", s.Updated, before, after)
	}
}

func TestNullTitle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	s := &SpecYAML{
		Codename: "test",
		Title:    nil,
		Type:     "feature",
		Project:  Project{ID: "proj"},
		Jig:      "feature",
		Status:   "research",
		Created:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := Write(path, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got.Title != nil {
		t.Errorf("title = %v, want nil", got.Title)
	}
}

func TestEmptySessionsAndDeps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	s := &SpecYAML{
		Codename:       "test",
		Type:           "feature",
		Project:        Project{ID: "proj"},
		Jig:            "feature",
		Status:         "research",
		Created:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Sessions:       []Session{},
		ActiveSession:  nil,
		DependsOn:      []Dependency{},
		Implementation: Implementation{Commits: []string{}},
	}

	if err := Write(path, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got.ActiveSession != nil {
		t.Errorf("active_session = %v, want nil", got.ActiveSession)
	}
}

func TestRoundTripAreas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	s := &SpecYAML{
		Codename: "test",
		Type:     "feature",
		Project:  Project{ID: "proj"},
		Jig:      "feature",
		Status:   "research",
		Created:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Areas:    []string{"auth", "api", "database"},
	}

	if err := Write(path, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(got.Areas) != 3 {
		t.Fatalf("areas length = %d, want 3", len(got.Areas))
	}
	for i, want := range []string{"auth", "api", "database"} {
		if got.Areas[i] != want {
			t.Errorf("Areas[%d] = %q, want %q", i, got.Areas[i], want)
		}
	}
}

func TestRoundTripRelatedTo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	s := &SpecYAML{
		Codename: "test",
		Type:     "feature",
		Project:  Project{ID: "proj"},
		Jig:      "feature",
		Status:   "research",
		Created:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		RelatedTo: []RelatedWork{
			{Codename: "other-work", Relationship: "informs"},
			{Codename: "third-work", Relationship: "supersedes"},
		},
	}

	if err := Write(path, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(got.RelatedTo) != 2 {
		t.Fatalf("related_to length = %d, want 2", len(got.RelatedTo))
	}
	if got.RelatedTo[0].Codename != "other-work" || got.RelatedTo[0].Relationship != "informs" {
		t.Errorf("RelatedTo[0] = %+v, want {other-work informs}", got.RelatedTo[0])
	}
	if got.RelatedTo[1].Codename != "third-work" || got.RelatedTo[1].Relationship != "supersedes" {
		t.Errorf("RelatedTo[1] = %+v, want {third-work supersedes}", got.RelatedTo[1])
	}
}

func TestBackwardCompatibility_NoAreasOrRelatedTo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	// Write a YAML file without areas or related_to fields.
	content := []byte(`codename: legacy-work
type: bug
project:
    id: proj
jig: plan
status: done
created: 2025-01-01T00:00:00Z
updated: 2025-01-01T00:00:00Z
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got.Areas != nil {
		t.Errorf("expected nil Areas for legacy spec, got %v", got.Areas)
	}
	if got.RelatedTo != nil {
		t.Errorf("expected nil RelatedTo for legacy spec, got %v", got.RelatedTo)
	}
}
