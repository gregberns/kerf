package kerftranscript_test

import (
	"testing"

	"github.com/gberns/kerf/internal/kerftranscript"
)

func TestIsValidKind_parser(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"dispatch", true},
		{"tool_result", true},
		{"commit_ref", true},
		{"bead_close", true},
		{"", false},
		{"DISPATCH", false},
		{"assistant", false},
		{"summary", false},
	}
	for _, tc := range cases {
		if got := kerftranscript.IsValidKind(tc.in); got != tc.want {
			t.Errorf("IsValidKind(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestEventKindConstants_parser(t *testing.T) {
	// Guard against accidental string drift. The spec table in
	// specs/diagnostics.md §"Diagnostic input vocabulary" lists exactly
	// these four kinds; the JSONL fixtures use these literals.
	if string(kerftranscript.EventDispatch) != "dispatch" {
		t.Fatalf("EventDispatch = %q", kerftranscript.EventDispatch)
	}
	if string(kerftranscript.EventToolResult) != "tool_result" {
		t.Fatalf("EventToolResult = %q", kerftranscript.EventToolResult)
	}
	if string(kerftranscript.EventCommitRef) != "commit_ref" {
		t.Fatalf("EventCommitRef = %q", kerftranscript.EventCommitRef)
	}
	if string(kerftranscript.EventBeadClose) != "bead_close" {
		t.Fatalf("EventBeadClose = %q", kerftranscript.EventBeadClose)
	}
}
