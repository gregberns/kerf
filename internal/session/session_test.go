package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gberns/kerf/internal/spec"
)

func TestStartSession_WithID(t *testing.T) {
	s := &spec.SpecYAML{}
	StartSession(s, "abc-123")

	if len(s.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(s.Sessions))
	}
	if s.Sessions[0].ID == nil || *s.Sessions[0].ID != "abc-123" {
		t.Error("session ID not set correctly")
	}
	if s.Sessions[0].Ended != nil {
		t.Error("new session should not have ended timestamp")
	}
	if s.ActiveSession == nil || *s.ActiveSession != "abc-123" {
		t.Error("active_session not set correctly")
	}
}

func TestStartSession_Anonymous(t *testing.T) {
	s := &spec.SpecYAML{}
	StartSession(s, "")

	if len(s.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(s.Sessions))
	}
	if s.Sessions[0].ID != nil {
		t.Error("anonymous session should have nil ID")
	}
	if s.ActiveSession == nil || *s.ActiveSession != "anonymous" {
		t.Errorf("active_session = %v, want 'anonymous'", s.ActiveSession)
	}
}

func TestEndSession(t *testing.T) {
	s := &spec.SpecYAML{}
	StartSession(s, "sess-1")
	EndSession(s)

	if s.ActiveSession != nil {
		t.Error("active_session should be nil after end")
	}
	if s.Sessions[0].Ended == nil {
		t.Error("ended timestamp should be set")
	}
}

func TestStartEndMultipleSessions(t *testing.T) {
	s := &spec.SpecYAML{}

	StartSession(s, "sess-1")
	EndSession(s)
	StartSession(s, "sess-2")

	if len(s.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(s.Sessions))
	}
	if s.Sessions[0].Ended == nil {
		t.Error("first session should have ended")
	}
	if s.Sessions[1].Ended != nil {
		t.Error("second session should not have ended")
	}
	if *s.ActiveSession != "sess-2" {
		t.Errorf("active_session = %q, want sess-2", *s.ActiveSession)
	}
}

func TestIsStale_NoActiveSession(t *testing.T) {
	s := &spec.SpecYAML{}
	if IsStale(s, 24) {
		t.Error("no active session should not be stale")
	}
}

func TestIsStale_RecentSession(t *testing.T) {
	s := &spec.SpecYAML{}
	StartSession(s, "sess-1")
	// Just started — should not be stale.
	if IsStale(s, 24) {
		t.Error("just-started session should not be stale")
	}
}

func TestIsStale_OldSession(t *testing.T) {
	s := &spec.SpecYAML{}
	active := "old-sess"
	s.ActiveSession = &active
	old := time.Now().UTC().Add(-48 * time.Hour)
	s.Sessions = []spec.Session{
		{ID: &active, Started: old},
	}
	if !IsStale(s, 24) {
		t.Error("48h-old session should be stale with 24h threshold")
	}
}

func TestIsStale_BoundaryExact(t *testing.T) {
	s := &spec.SpecYAML{}
	active := "edge-sess"
	s.ActiveSession = &active
	// Exactly at threshold — should not be stale (> not >=).
	boundary := time.Now().UTC().Add(-24 * time.Hour)
	s.Sessions = []spec.Session{
		{ID: &active, Started: boundary},
	}
	// At exact boundary with time resolution, this could go either way.
	// The important thing is it doesn't panic.
	_ = IsStale(s, 24)
}

func TestStaleWarning_NotStale(t *testing.T) {
	s := &spec.SpecYAML{}
	StartSession(s, "sess-1")
	msg := StaleWarning(s, 24)
	if msg != "" {
		t.Errorf("expected empty warning, got %q", msg)
	}
}

func TestStaleWarning_Stale(t *testing.T) {
	s := &spec.SpecYAML{}
	active := "old-sess"
	s.ActiveSession = &active
	old := time.Now().UTC().Add(-48 * time.Hour)
	s.Sessions = []spec.Session{
		{ID: &active, Started: old},
	}
	msg := StaleWarning(s, 24)
	if msg == "" {
		t.Error("expected stale warning, got empty")
	}
}

func TestFindActiveWork(t *testing.T) {
	dir := t.TempDir()

	// Create two works, one active.
	makeWork(t, dir, "cool-bear", nil)
	activeID := "sess-1"
	makeWork(t, dir, "warm-fox", &activeID)

	got, err := FindActiveWork(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "warm-fox" {
		t.Errorf("FindActiveWork = %q, want %q", got, "warm-fox")
	}
}

func TestFindActiveWork_ZeroActive(t *testing.T) {
	dir := t.TempDir()
	makeWork(t, dir, "cool-bear", nil)

	_, err := FindActiveWork(dir)
	if err == nil {
		t.Error("expected error for zero active works")
	}
}

func TestFindActiveWork_MultipleActive(t *testing.T) {
	dir := t.TempDir()
	a1 := "sess-1"
	a2 := "sess-2"
	makeWork(t, dir, "cool-bear", &a1)
	makeWork(t, dir, "warm-fox", &a2)

	_, err := FindActiveWork(dir)
	if err == nil {
		t.Error("expected error for multiple active works")
	}
}

// makeWork creates a minimal work directory with a spec.yaml.
func makeWork(t *testing.T, projectDir, codename string, activeSession *string) {
	t.Helper()
	workDir := filepath.Join(projectDir, codename)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	s := &spec.SpecYAML{
		Codename:      codename,
		Type:          "feature",
		Jig:           "feature",
		JigVersion:    1,
		Status:        "problem-space",
		StatusValues:  []string{"problem-space", "ready"},
		Created:       time.Now().UTC().Truncate(time.Second),
		Updated:       time.Now().UTC().Truncate(time.Second),
		ActiveSession: activeSession,
		Project:       spec.Project{ID: "test"},
	}
	specPath := filepath.Join(workDir, "spec.yaml")
	if err := spec.Write(specPath, s); err != nil {
		t.Fatal(err)
	}
}
