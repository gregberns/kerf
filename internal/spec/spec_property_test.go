package spec

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProperty_YAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Create a fully-populated SpecYAML.
	title := "Test Title"
	sessionID := "sess-123"
	depProject := "other-proj"
	branch := "feat/test"
	pr := "https://github.com/acme/app/pull/42"

	original := &SpecYAML{
		Codename: "test-work",
		Title:    &title,
		Type:     "feature",
		Project:  Project{ID: "test-proj"},
		Jig:      "feature",
		JigVersion:   1,
		Status:       "research",
		StatusValues: []string{"problem-space", "decomposition", "research", "detailed-spec", "review", "ready"},
		Created:      time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
		Updated:      time.Date(2026, 4, 8, 14, 30, 0, 0, time.UTC),
		Sessions: []Session{
			{
				ID:      &sessionID,
				Started: time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
				Ended:   timePtr(time.Date(2026, 4, 7, 16, 30, 0, 0, time.UTC)),
				Notes:   strPtr("Completed problem space"),
			},
		},
		ActiveSession: nil,
		DependsOn: []Dependency{
			{
				Codename:     "database-migration",
				Project:      &depProject,
				Relationship: "must-complete-first",
			},
			{
				Codename:     "auth-service",
				Relationship: "inform",
			},
		},
		Implementation: Implementation{
			Branch:  &branch,
			PR:      &pr,
			Commits: []string{"abc123", "def456"},
		},
	}

	specPath := filepath.Join(dir, "spec.yaml")

	// Write
	if err := Write(specPath, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Read back
	roundTripped, err := Read(specPath)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	// Verify all fields (Write auto-sets Updated, so compare everything else).
	if roundTripped.Codename != original.Codename {
		t.Errorf("Codename = %q, want %q", roundTripped.Codename, original.Codename)
	}
	if roundTripped.Title == nil || *roundTripped.Title != *original.Title {
		t.Errorf("Title mismatch")
	}
	if roundTripped.Type != original.Type {
		t.Errorf("Type = %q, want %q", roundTripped.Type, original.Type)
	}
	if roundTripped.Project.ID != original.Project.ID {
		t.Errorf("Project.ID = %q, want %q", roundTripped.Project.ID, original.Project.ID)
	}
	if roundTripped.Jig != original.Jig {
		t.Errorf("Jig = %q, want %q", roundTripped.Jig, original.Jig)
	}
	if roundTripped.JigVersion != original.JigVersion {
		t.Errorf("JigVersion = %d, want %d", roundTripped.JigVersion, original.JigVersion)
	}
	if roundTripped.Status != original.Status {
		t.Errorf("Status = %q, want %q", roundTripped.Status, original.Status)
	}
	if len(roundTripped.StatusValues) != len(original.StatusValues) {
		t.Errorf("StatusValues len = %d, want %d", len(roundTripped.StatusValues), len(original.StatusValues))
	}
	for i, v := range roundTripped.StatusValues {
		if v != original.StatusValues[i] {
			t.Errorf("StatusValues[%d] = %q, want %q", i, v, original.StatusValues[i])
		}
	}
	if !roundTripped.Created.Equal(original.Created) {
		t.Errorf("Created = %v, want %v", roundTripped.Created, original.Created)
	}
	// Sessions
	if len(roundTripped.Sessions) != len(original.Sessions) {
		t.Fatalf("Sessions len = %d, want %d", len(roundTripped.Sessions), len(original.Sessions))
	}
	if *roundTripped.Sessions[0].ID != *original.Sessions[0].ID {
		t.Error("Session ID mismatch")
	}
	if !roundTripped.Sessions[0].Started.Equal(original.Sessions[0].Started) {
		t.Error("Session Started mismatch")
	}
	if roundTripped.Sessions[0].Ended == nil || !roundTripped.Sessions[0].Ended.Equal(*original.Sessions[0].Ended) {
		t.Error("Session Ended mismatch")
	}
	if roundTripped.Sessions[0].Notes == nil || *roundTripped.Sessions[0].Notes != *original.Sessions[0].Notes {
		t.Error("Session Notes mismatch")
	}
	// Dependencies
	if len(roundTripped.DependsOn) != 2 {
		t.Fatalf("DependsOn len = %d, want 2", len(roundTripped.DependsOn))
	}
	if roundTripped.DependsOn[0].Codename != "database-migration" {
		t.Error("DependsOn[0].Codename mismatch")
	}
	if roundTripped.DependsOn[0].Project == nil || *roundTripped.DependsOn[0].Project != "other-proj" {
		t.Error("DependsOn[0].Project mismatch")
	}
	if roundTripped.DependsOn[1].Project != nil {
		t.Error("DependsOn[1].Project should be nil for same-project dep")
	}
	// Implementation
	if roundTripped.Implementation.Branch == nil || *roundTripped.Implementation.Branch != *original.Implementation.Branch {
		t.Error("Implementation.Branch mismatch")
	}
	if roundTripped.Implementation.PR == nil || *roundTripped.Implementation.PR != *original.Implementation.PR {
		t.Error("Implementation.PR mismatch")
	}
	if len(roundTripped.Implementation.Commits) != 2 {
		t.Error("Implementation.Commits mismatch")
	}
}

func TestProperty_YAMLRoundTrip_NilOptionalFields(t *testing.T) {
	dir := t.TempDir()

	original := &SpecYAML{
		Codename:     "minimal-work",
		Type:         "bug",
		Project:      Project{ID: "proj"},
		Jig:          "bug",
		JigVersion:   1,
		Status:       "triaging",
		StatusValues: []string{"triaging", "ready"},
		Created:      time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
		Updated:      time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
	}

	specPath := filepath.Join(dir, "spec.yaml")
	if err := Write(specPath, original); err != nil {
		t.Fatal(err)
	}

	roundTripped, err := Read(specPath)
	if err != nil {
		t.Fatal(err)
	}

	if roundTripped.Title != nil {
		t.Error("Title should be nil")
	}
	if roundTripped.ActiveSession != nil {
		t.Error("ActiveSession should be nil")
	}
}

func TestProperty_MalformedYAML(t *testing.T) {
	dir := t.TempDir()

	malformed := []string{
		"{{{{",
		"codename: [invalid",
		": : : :",
		"\x00\x01\x02",
		"codename: test\nstatus: [",
	}

	for _, content := range malformed {
		path := filepath.Join(dir, "bad.yaml")
		os.WriteFile(path, []byte(content), 0o644)
		_, err := Read(path)
		if err == nil {
			t.Errorf("Read should error for malformed YAML: %q", content)
		}
	}
}

func strPtr(s string) *string { return &s }
func timePtr(t time.Time) *time.Time { return &t }
