package kerftranscript

import (
	"strings"
	"testing"
)

// TestCompileBeadIDPattern_Empty: an empty pattern string is "not
// configured" — D1 silently no-ops per its existing contract; this is
// NOT a corruption.
func TestCompileBeadIDPattern_Empty(t *testing.T) {
	for _, s := range []string{"", "   ", "\t"} {
		r := CompileBeadIDPattern(s)
		if r.Configured {
			t.Errorf("Configured = true for %q, want false", s)
		}
		if r.Pattern != nil {
			t.Errorf("Pattern != nil for %q", s)
		}
		if r.CompileError != "" {
			t.Errorf("CompileError = %q for %q, want empty", r.CompileError, s)
		}
		if r.Corrupt() {
			t.Errorf("Corrupt() = true for %q, want false", s)
		}
	}
}

// TestCompileBeadIDPattern_Good: a well-formed regex compiles and is
// returned with no error; Corrupt() is false.
func TestCompileBeadIDPattern_Good(t *testing.T) {
	r := CompileBeadIDPattern(`(hk|kerf)-[A-Za-z0-9._]+`)
	if !r.Configured {
		t.Fatalf("Configured = false, want true")
	}
	if r.Pattern == nil {
		t.Fatalf("Pattern is nil for valid regex")
	}
	if r.CompileError != "" {
		t.Errorf("CompileError = %q, want empty", r.CompileError)
	}
	if r.Corrupt() {
		t.Errorf("Corrupt() = true for valid regex")
	}
	// Sanity: the returned pattern actually matches.
	if !r.Pattern.MatchString("kerf-7ozm") {
		t.Errorf("compiled pattern failed to match kerf-7ozm")
	}
}

// TestCompileBeadIDPattern_Corrupt: a malformed regex is reported as
// Corrupt with the compile error captured verbatim for the
// `corrupt_project_config` warning's `reason` field.
func TestCompileBeadIDPattern_Corrupt(t *testing.T) {
	r := CompileBeadIDPattern(`(unterminated`)
	if !r.Configured {
		t.Fatalf("Configured = false, want true (pattern was set)")
	}
	if r.Pattern != nil {
		t.Errorf("Pattern != nil for malformed regex")
	}
	if r.CompileError == "" {
		t.Fatalf("CompileError empty for malformed regex")
	}
	if !r.Corrupt() {
		t.Fatalf("Corrupt() = false for malformed regex")
	}
	// The verbatim Go regexp error mentions the input fragment.
	if !strings.Contains(r.CompileError, "unterminated") &&
		!strings.Contains(r.CompileError, "missing") {
		t.Errorf("CompileError = %q; expected the verbatim Go regexp error", r.CompileError)
	}
}
