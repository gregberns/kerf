package spec

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/beads"
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

func TestRoundTripBeadFilter_AnyForm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	s := &SpecYAML{
		Codename: "bridge",
		Type:     "plan",
		Project:  Project{ID: "proj"},
		Jig:      "plan",
		Status:   "research",
		Created:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		BeadFilter: &beads.Filter{
			Any: []beads.Filter{
				{Label: "subsystem:bridge"},
				{IDPrefix: "hk-cb"},
			},
		},
	}

	if err := Write(path, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got.BeadFilter == nil {
		t.Fatalf("bead_filter = nil, want non-nil")
	}
	if len(got.BeadFilter.Any) != 2 {
		t.Fatalf("bead_filter.any length = %d, want 2", len(got.BeadFilter.Any))
	}
	if got.BeadFilter.Any[0].Label != "subsystem:bridge" {
		t.Errorf("any[0].label = %q, want %q", got.BeadFilter.Any[0].Label, "subsystem:bridge")
	}
	if got.BeadFilter.Any[1].IDPrefix != "hk-cb" {
		t.Errorf("any[1].id_prefix = %q, want %q", got.BeadFilter.Any[1].IDPrefix, "hk-cb")
	}
	if err := got.BeadFilter.Validate(); err != nil {
		t.Errorf("loaded bead_filter failed Validate: %v", err)
	}
}

// TestWrite_BeadFilterAlwaysEmitted verifies the Plan 019 invariant: every
// spec.yaml written by Write contains a top-level `bead_filter:` key, even
// when the SpecYAML's BeadFilter pointer is nil. See specs/works.md row
// for bead_filter and specs/commands.md §`kerf new` step 6.
func TestWrite_BeadFilterAlwaysEmitted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	s := &SpecYAML{
		Codename:   "unwired",
		Type:       "plan",
		Project:    Project{ID: "proj"},
		Jig:        "plan",
		Status:     "research",
		Created:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		BeadFilter: nil,
	}
	if err := Write(path, s); err != nil {
		t.Fatalf("Write: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !regexp.MustCompile(`(?m)^bead_filter:`).Match(raw) {
		t.Errorf("expected top-level bead_filter: key in output, got:\n%s", raw)
	}
	// Round-trip: reading the file back must yield a nil BeadFilter (absent /
	// empty resolve identically).
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.BeadFilter != nil {
		t.Errorf("expected BeadFilter to be nil after round-trip of empty key, got %+v", got.BeadFilter)
	}
}

func TestRoundTripBeadFilter_DirectLabel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	s := &SpecYAML{
		Codename:   "alpha",
		Type:       "plan",
		Project:    Project{ID: "proj"},
		Jig:        "plan",
		Status:     "research",
		Created:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		BeadFilter: &beads.Filter{Label: "work:{codename}"},
	}

	if err := Write(path, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got.BeadFilter == nil || got.BeadFilter.Label != "work:{codename}" {
		t.Errorf("bead_filter.label = %+v, want label=work:{codename}", got.BeadFilter)
	}
}

func TestBackwardCompatibility_NoBeadFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	content := []byte(`codename: legacy
type: plan
project:
    id: proj
jig: plan
status: research
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got.BeadFilter != nil {
		t.Errorf("BeadFilter = %+v, want nil for spec without bead_filter", got.BeadFilter)
	}
}

func TestBeadFilter_InvalidEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	// Empty bead_filter (no label, id_prefix, or any) is invalid.
	content := []byte(`codename: bad
type: plan
project:
    id: proj
jig: plan
status: research
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
bead_filter: {}
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Read(path)
	if err == nil {
		t.Fatal("expected error for empty bead_filter, got nil")
	}
}

func TestBeadFilter_InvalidMixedClauses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	// Mixing direct leaf (label) with any: is invalid.
	content := []byte(`codename: bad
type: plan
project:
    id: proj
jig: plan
status: research
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
bead_filter:
  label: "subsystem:foo"
  any:
    - id_prefix: "hk-"
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Read(path)
	if err == nil {
		t.Fatal("expected error for mixed direct/any bead_filter, got nil")
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

// --- PinnedBeads (Plan 009 / B3) -----------------------------------------

// TestRoundTripPinnedBeads covers the populated-list case from beads.md.
func TestRoundTripPinnedBeads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	s := &SpecYAML{
		Codename:    "bridge",
		Type:        "plan",
		Project:     Project{ID: "proj"},
		Jig:         "plan",
		Status:      "research",
		Created:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PinnedBeads: []string{"hk-cb-001", "hk-cb-099"},
	}

	if err := Write(path, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(got.PinnedBeads) != 2 {
		t.Fatalf("pinned_beads length = %d, want 2", len(got.PinnedBeads))
	}
	if got.PinnedBeads[0] != "hk-cb-001" || got.PinnedBeads[1] != "hk-cb-099" {
		t.Errorf("PinnedBeads = %v, want [hk-cb-001 hk-cb-099]", got.PinnedBeads)
	}
}

// TestPinnedBeadsEmptyRendersFlowList asserts the empty list is serialized
// as `pinned_beads: []` rather than `pinned_beads: null` or omitted (per
// works.md row: required-on-write, default `[]`).
func TestPinnedBeadsEmptyRendersFlowList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	s := &SpecYAML{
		Codename:    "alpha",
		Type:        "plan",
		Project:     Project{ID: "proj"},
		Jig:         "plan",
		Status:      "research",
		Created:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PinnedBeads: []string{},
	}

	if err := Write(path, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// go-yaml renders an empty slice as the flow form `[]`; an omitted or
	// nil slice would render as `pinned_beads: null` or be absent. Assert
	// the key is present and renders as the empty flow list.
	if !bytesContains(raw, []byte("pinned_beads: []")) {
		t.Errorf("written spec.yaml does not contain `pinned_beads: []`:\n%s", raw)
	}
}

// TestPinnedBeadsNilCoercedOnWrite asserts that a nil PinnedBeads slice is
// coerced to an empty slice on Write (since the field has no `omitempty`,
// a nil slice would otherwise render as `pinned_beads: null`).
func TestPinnedBeadsNilCoercedOnWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	s := &SpecYAML{
		Codename: "gamma",
		Type:     "plan",
		Project:  Project{ID: "proj"},
		Jig:      "plan",
		Status:   "research",
		Created:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		// PinnedBeads left nil intentionally.
	}

	if err := Write(path, s); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytesContains(raw, []byte("pinned_beads: []")) {
		t.Errorf("nil PinnedBeads should be coerced to []; output:\n%s", raw)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.PinnedBeads == nil {
		t.Errorf("Read returned nil PinnedBeads; expected non-nil empty slice")
	}
	if len(got.PinnedBeads) != 0 {
		t.Errorf("PinnedBeads length = %d, want 0", len(got.PinnedBeads))
	}
}

// TestPinnedBeadsDuplicateRejected asserts Validate (and therefore Read /
// Write) reject duplicate bead IDs within a single work's pinned_beads.
func TestPinnedBeadsDuplicateRejected(t *testing.T) {
	s := &SpecYAML{
		Codename:    "delta",
		Type:        "plan",
		Project:     Project{ID: "proj"},
		Jig:         "plan",
		Status:      "research",
		Created:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PinnedBeads: []string{"hk-cb-001", "hk-cb-002", "hk-cb-001"},
	}

	if err := s.Validate(); err == nil {
		t.Fatal("expected duplicate-ID error from Validate, got nil")
	}

	// Write must also surface the error.
	dir := t.TempDir()
	if err := Write(filepath.Join(dir, "spec.yaml"), s); err == nil {
		t.Fatal("expected duplicate-ID error from Write, got nil")
	}
}

// TestBackwardCompatibility_NoPinnedBeads asserts that reading a legacy
// spec.yaml (no `pinned_beads:` key) returns a non-nil empty slice, so
// downstream code can iterate without a nil check.
func TestBackwardCompatibility_NoPinnedBeads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	content := []byte(`codename: legacy
type: plan
project:
    id: proj
jig: plan
status: research
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.PinnedBeads == nil {
		t.Error("expected non-nil empty PinnedBeads for legacy spec, got nil")
	}
	if len(got.PinnedBeads) != 0 {
		t.Errorf("PinnedBeads length = %d, want 0", len(got.PinnedBeads))
	}
}

// bytesContains is a tiny helper to avoid pulling in `bytes` for one
// substring check in a file that already imports nothing from there.
func bytesContains(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
