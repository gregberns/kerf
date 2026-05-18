package cmd

// Help-text snapshot tests — Plan 008 / Bead 7 (kerf-mgg).
//
// Locks the user-facing help-text contracts so future drift gets caught.
// Spec references:
//   - specs/commands.md §"kerf next" §"Help text" (six-element contract).
//   - specs/commands.md §"kerf triage" §"Help text".
//   - specs/commands.md §"Bare `kerf` invocation" → Output bullet list.
//
// Per the bead's deliverables, assertions are regex-per-contract-element,
// not exact byte match: a byte match would either lock in implementation
// drift or require a "Reference output" block in the spec. The
// content-contract shape is what the spec specifies; that is what these
// tests enforce.

import (
	"bytes"
	"regexp"
	"testing"
)

// --- kerf next help — six-element contract --------------------------------

// TestHelpSnapshot_KerfNext locks the six-element contract from
// specs/commands.md §"kerf next" §"Help text" — each element must be
// present, in order, in nextLongHelp.
func TestHelpSnapshot_KerfNext(t *testing.T) {
	// One regex per contract element. Order is enforced via Index walk.
	elements := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"1. what it returns", regexp.MustCompile(`(?i)ranked feed of things to act on`)},
		{"2a. bead kind", regexp.MustCompile(`(?m)^\s*bead\s+—`)},
		{"2b. cleanup kind", regexp.MustCompile(`(?m)^\s*cleanup\s+—`)},
		{"2c. warning kind", regexp.MustCompile(`(?m)^\s*warning\s+—`)},
		{"3. default action loop", regexp.MustCompile(`(?i)Default action loop`)},
		{"4a. --only flag", regexp.MustCompile(`--only`)},
		{"4b. --include flag", regexp.MustCompile(`--include`)},
		{"4c. --kinds flag", regexp.MustCompile(`--kinds`)},
		{"5. machine output", regexp.MustCompile(`(?i)--format=json`)},
		{"6a. scoring", regexp.MustCompile(`(?i)Scoring`)},
		{"6b. pointer to coordination.md", regexp.MustCompile(`coordination\.md`)},
	}

	h := nextLongHelp
	cursor := 0
	for _, el := range elements {
		loc := el.pattern.FindStringIndex(h[cursor:])
		if loc == nil {
			t.Fatalf("kerf next help is missing required element %q (or out of order).\nRemaining text from cursor:\n%s",
				el.name, h[cursor:])
		}
		cursor += loc[1]
	}
}

// --- kerf triage help — fixed-order contract ------------------------------

// TestHelpSnapshot_KerfTriage locks the fixed-order contract from
// specs/commands.md §"kerf triage" §"Help text".
func TestHelpSnapshot_KerfTriage(t *testing.T) {
	elements := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"1. what triage returns (drift report)", regexp.MustCompile(`(?i)drift report`)},
		{"2a. untriaged kind", regexp.MustCompile(`(?m)^\s*untriaged\s+—`)},
		{"2b. multi_matched kind", regexp.MustCompile(`(?m)^\s*multi_matched\s+—`)},
		{"2c. external_drift kind", regexp.MustCompile(`(?m)^\s*external_drift\s+—`)},
		{"2d. external_drift sub-kinds", regexp.MustCompile(`external_close.*external_reopen.*external_delete.*external_new`)},
		{"3a. --resolved exit codes", regexp.MustCompile(`(?i)Exit codes \(--resolved\)`)},
		{"3b. exit 0", regexp.MustCompile(`(?m)^\s*0\s+—`)},
		{"3c. exit 1", regexp.MustCompile(`(?m)^\s*1\s+—`)},
		{"3d. exit 2", regexp.MustCompile(`(?m)^\s*2\s+—`)},
		{"3e. exit 3", regexp.MustCompile(`(?m)^\s*3\s+—`)},
		{"3f. stuck-loop guidance", regexp.MustCompile(`(?i)(stop and ask|ask for help|not converging|converging)`)},
		{"4. --ack as only baseline advancer", regexp.MustCompile(`(?i)--ack.*baseline`)},
	}

	h := triageLongHelp
	cursor := 0
	for _, el := range elements {
		loc := el.pattern.FindStringIndex(h[cursor:])
		if loc == nil {
			t.Fatalf("kerf triage help is missing required element %q (or out of order).\nRemaining text from cursor:\n%s",
				el.name, h[cursor:])
		}
		cursor += loc[1]
	}
}

// --- kerf init help — flag-surface contract -------------------------------

// TestHelpSnapshot_KerfInit locks the post-Plan-016 `kerf init` flag surface
// from specs/commands.md §"kerf init" — every documented flag must appear in
// `kerf init --help`, and the command is non-interactive (no prompt).
//
// Plan 016 / B8 (kerf-3lz) added --yes, --no, --bead-filter and removed the
// y/N prompt; this test catches regressions if any of those flags are dropped
// or renamed, and asserts the pre-existing --jig / --force flags survive.
func TestHelpSnapshot_KerfInit(t *testing.T) {
	var buf bytes.Buffer
	initCmd.SetOut(&buf)
	initCmd.SetErr(&buf)
	defer func() {
		initCmd.SetOut(nil)
		initCmd.SetErr(nil)
	}()

	if err := initCmd.Help(); err != nil {
		t.Fatalf("initCmd.Help() returned error: %v", err)
	}
	out := buf.String()

	// Presence-only (flag layout is alphabetical via cobra; order is not a
	// spec contract). Each flag's long form plus a snippet of its
	// description is asserted so a silent rename of either side fails the
	// test.
	flags := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"--jig flag present", regexp.MustCompile(`--jig\b.*(?i)workflow`)},
		{"--force flag present", regexp.MustCompile(`--force\b.*(?i)project\.yaml`)},
		{"--yes flag present", regexp.MustCompile(`--yes\b.*(?i)bead-filter`)},
		{"--no flag present", regexp.MustCompile(`--no\b.*(?i)bead-filter`)},
		{"--bead-filter flag present", regexp.MustCompile(`--bead-filter\b.*(?i)literal`)},
	}
	for _, f := range flags {
		if !f.pattern.MatchString(out) {
			t.Fatalf("kerf init help missing element %q.\nFull output:\n%s", f.name, out)
		}
	}

	// Non-interactive: no leftover y/N prompt mention from the pre-Plan-016
	// interactive flow.
	if regexp.MustCompile(`\[y/N\]|\(y/N\)`).MatchString(out) {
		t.Fatalf("kerf init help still mentions a y/N prompt; init is non-interactive post-Plan-016.\nFull output:\n%s", out)
	}
}

// --- bare `kerf` invocation — output-contract elements --------------------

// TestHelpSnapshot_KerfBare locks the bare-`kerf` output contract from
// specs/commands.md §"Bare `kerf` invocation" §"Output". The five required
// output elements are: (1) one-line description, (2) available commands
// list, (3) standard workflow, (4) bench summary (or, if no bench, getting
// started instructions referencing `kerf new`), (5) the per-command list
// includes every command we ship.
//
// This test exercises the no-bench branch (simpler, deterministic) plus
// asserts the post-Plan-009 commands are surfaced.
func TestHelpSnapshot_KerfBare(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	out := captureOutput(t, func() {
		rootCmd.SetArgs([]string{})
		rootCmd.Run(rootCmd, []string{})
	})

	elements := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"1. one-line description", regexp.MustCompile(`(?i)spec-writing CLI for AI agents`)},
		{"4. getting-started (no-bench branch)", regexp.MustCompile(`(?i)No bench found`)},
		{"4b. getting-started suggests kerf new", regexp.MustCompile(`kerf new`)},
		{"2. Available commands header", regexp.MustCompile(`(?i)Available commands:`)},
		// Post-Plan-009 commands must be present.
		{"5a. kerf next listed", regexp.MustCompile(`kerf next\b`)},
		{"5b. kerf triage listed", regexp.MustCompile(`kerf triage\b`)},
		{"5c. kerf pin listed", regexp.MustCompile(`kerf pin\b`)},
		{"5d. kerf work edit listed", regexp.MustCompile(`kerf work edit\b`)},
		{"5e. kerf new listed", regexp.MustCompile(`kerf new\b`)},
		{"5f. per-command --help pointer", regexp.MustCompile(`kerf <command> --help`)},
	}

	cursor := 0
	for _, el := range elements {
		loc := el.pattern.FindStringIndex(out[cursor:])
		if loc == nil {
			// For these we don't strictly enforce order (the output
			// layout is fixed but agent-readability requires presence,
			// not position, for the per-command lines). Re-search from
			// the start before failing.
			if el.pattern.FindStringIndex(out) == nil {
				t.Fatalf("bare kerf output is missing required element %q.\nFull output:\n%s", el.name, out)
			}
			continue
		}
		cursor += loc[1]
	}
}
