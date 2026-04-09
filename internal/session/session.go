package session

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gberns/kerf/internal/spec"
)

// StartSession appends a session entry and sets active_session.
// If sessionID is empty, active_session is set to "anonymous".
func StartSession(s *spec.SpecYAML, sessionID string) {
	now := time.Now().UTC().Truncate(time.Second)

	var id *string
	activeVal := "anonymous"
	if sessionID != "" {
		id = &sessionID
		activeVal = sessionID
	}

	entry := spec.Session{
		ID:      id,
		Started: now,
	}
	s.Sessions = append(s.Sessions, entry)
	s.ActiveSession = &activeVal
}

// EndSession sets the ended timestamp on the active session entry and clears active_session.
func EndSession(s *spec.SpecYAML) {
	now := time.Now().UTC().Truncate(time.Second)

	// Find the active (un-ended) session and set its ended timestamp.
	for i := len(s.Sessions) - 1; i >= 0; i-- {
		if s.Sessions[i].Ended == nil {
			s.Sessions[i].Ended = &now
			break
		}
	}
	s.ActiveSession = nil
}

// IsStale returns true if the active session started more than thresholdHours ago.
func IsStale(s *spec.SpecYAML, thresholdHours int) bool {
	if s.ActiveSession == nil {
		return false
	}

	// Find the active session entry (last un-ended session).
	for i := len(s.Sessions) - 1; i >= 0; i-- {
		if s.Sessions[i].Ended == nil {
			threshold := time.Duration(thresholdHours) * time.Hour
			return time.Since(s.Sessions[i].Started) > threshold
		}
	}
	return false
}

// StaleWarning returns a warning message if the session is stale, or empty string if not.
func StaleWarning(s *spec.SpecYAML, thresholdHours int) string {
	if !IsStale(s, thresholdHours) {
		return ""
	}

	// Find the active session's start time.
	for i := len(s.Sessions) - 1; i >= 0; i-- {
		if s.Sessions[i].Ended == nil {
			return fmt.Sprintf(
				"Warning: active session started %s appears stale\n"+
					"(threshold: %dh). The previous session may have ended without running\n"+
					"`kerf shelve`. Run `kerf shelve --force <codename>` to clear it.",
				s.Sessions[i].Started.Format(time.RFC3339),
				thresholdHours,
			)
		}
	}
	return ""
}

// FindActiveWork scans all work directories in projectDir for one with an active session.
// Returns the codename. Errors on 0 or >1 matches.
func FindActiveWork(projectDir string) (string, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", fmt.Errorf("reading project directory: %w", err)
	}

	var active []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		specPath := filepath.Join(projectDir, entry.Name(), "spec.yaml")
		s, err := spec.Read(specPath)
		if err != nil {
			continue // skip unreadable works
		}
		if s.ActiveSession != nil {
			active = append(active, entry.Name())
		}
	}

	switch len(active) {
	case 0:
		return "", fmt.Errorf("no work with an active session found in project")
	case 1:
		return active[0], nil
	default:
		return "", fmt.Errorf("multiple works with active sessions: %v", active)
	}
}
