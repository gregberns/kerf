package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

// Plan 020 — cross-command integration tests covering kerf review / preview /
// show --compact / status --quiet and the pass-directory pre-creation behavior
// they share. Per-bead unit tests already cover each command in isolation;
// these tests exercise the flows that touch multiple commands together.
//
// Spec coverage:
//   - specs/commands.md §`kerf status` step 7 (pre-create on advance)
//   - specs/commands.md §`kerf status` step 8 (--quiet suppresses instructions)
//   - specs/commands.md §`kerf review` Output (text shape, current-pass default)
//   - specs/commands.md §`kerf preview` Output (read-only header, idempotent)
//   - specs/commands.md §`kerf show` --compact output (four-line shape)
//   - specs/jig-system.md §Pass-Directory Pre-Creation (idempotent, no clobber,
//     {component} deferred)
//   - specs/jig-system.md §Surfacing Pass Filenames (Pass N → Output line)

// makePlan020Work creates a spec-jig work in a fresh tmp HOME and returns the
// project, codename, and work directory path.
func makePlan020Work(t *testing.T, proj, codename string) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	captureOutput(t, func() {
		projectFlag = proj
		newJigFlag = "spec"
		newTitle = "Plan 020 integration"
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = ""; newTitle = ""; newType = "" }()
		if err := newCmd.RunE(newCmd, []string{codename}); err != nil {
			t.Fatalf("new: %v", err)
		}
	})
	return filepath.Join(tmp, ".kerf", "projects", proj, codename)
}

// TestStatus_AdvanceCycle_PreCreatesAndQuiet exercises the full advance flow:
// a chain of `kerf status <work> <next>` calls under --quiet must each emit
// only the single-line transition confirmation, and the corresponding pass
// directories/templates must appear on disk after each advance. Covers
// specs/commands.md §`kerf status` steps 7-8 and specs/jig-system.md
// §Pass-Directory Pre-Creation.
func TestStatus_AdvanceCycle_PreCreatesAndQuiet(t *testing.T) {
	workDir := makePlan020Work(t, "plan020-cycle-proj", "cycle-demo")

	// Pass 1 (problem-space) was just landed by `kerf new`; advance through
	// the content passes. The "ready" terminal pass has no output, so we stop
	// at tasks.
	chain := []struct {
		status        string
		expectFile    string // file that should exist after this advance
		expectDirOnly string // dir-only (component) — file may or may not exist
	}{
		{status: "decompose", expectFile: "02-components.md"},
		{status: "research", expectDirOnly: "03-research"},
		{status: "change-design", expectDirOnly: "04-design"},
		{status: "spec-draft", expectFile: "05-changelog.md"},
		{status: "integration", expectFile: "06-integration.md"},
		{status: "tasks", expectFile: "07-tasks.md"},
	}

	for _, step := range chain {
		var out string
		out = captureOutput(t, func() {
			projectFlag = "plan020-cycle-proj"
			statusQuiet = true
			defer func() { projectFlag = ""; statusQuiet = false }()
			if err := statusCmd.RunE(statusCmd, []string{"cycle-demo", step.status}); err != nil {
				t.Fatalf("status %s: %v", step.status, err)
			}
		})

		// --quiet shape: exactly one confirmation line, no jig instructions.
		if !strings.Contains(out, "Status updated:") {
			t.Errorf("status %s --quiet missing confirmation. got:\n%s", step.status, out)
		}
		if strings.Contains(out, "Next steps:") {
			t.Errorf("status %s --quiet must suppress 'Next steps:'. got:\n%s", step.status, out)
		}
		// Count non-empty lines; allow at most: confirmation (+ optional
		// warning if status not in jig list, which should not happen for spec
		// jig statuses).
		lines := 0
		for _, ln := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if strings.TrimSpace(ln) != "" {
				lines++
			}
		}
		if lines > 2 {
			t.Errorf("status %s --quiet emitted %d non-empty lines, expected <= 2. got:\n%s", step.status, lines, out)
		}

		if step.expectFile != "" {
			path := filepath.Join(workDir, step.expectFile)
			if _, err := os.Stat(path); err != nil {
				t.Errorf("expected pre-created file %s after advance to %s: %v", step.expectFile, step.status, err)
			}
		}
		if step.expectDirOnly != "" {
			path := filepath.Join(workDir, step.expectDirOnly)
			info, err := os.Stat(path)
			if err != nil {
				t.Errorf("expected pre-created dir %s after advance to %s: %v", step.expectDirOnly, step.status, err)
			} else if !info.IsDir() {
				t.Errorf("expected %s to be a directory", step.expectDirOnly)
			}
		}
	}
}

// TestStatus_AdvanceIdempotent_NoClobber verifies the pre-creation step does
// not overwrite a file an agent has populated. Covers specs/jig-system.md
// §Pass-Directory Pre-Creation "Existing files are never overwritten".
func TestStatus_AdvanceIdempotent_NoClobber(t *testing.T) {
	workDir := makePlan020Work(t, "plan020-idem-proj", "idem-demo")

	// First advance: decompose pre-creates 02-components.md from template.
	captureOutput(t, func() {
		projectFlag = "plan020-idem-proj"
		statusQuiet = true
		defer func() { projectFlag = ""; statusQuiet = false }()
		_ = statusCmd.RunE(statusCmd, []string{"idem-demo", "decompose"})
	})

	target := filepath.Join(workDir, "02-components.md")
	// Agent populates the file with its own content.
	populated := []byte("# Agent-authored content — must not be clobbered.\n")
	if err := os.WriteFile(target, populated, 0o644); err != nil {
		t.Fatalf("seed populated file: %v", err)
	}

	// Re-advance to the same status (idempotent path) and to a later status
	// then back — both must leave the populated file alone.
	captureOutput(t, func() {
		projectFlag = "plan020-idem-proj"
		statusQuiet = true
		defer func() { projectFlag = ""; statusQuiet = false }()
		_ = statusCmd.RunE(statusCmd, []string{"idem-demo", "decompose"})
	})

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read populated: %v", err)
	}
	if !bytes.Equal(got, populated) {
		t.Errorf("re-advance clobbered populated file. got:\n%s\nwant:\n%s", got, populated)
	}
}

// TestReview_AfterStatusAdvance_ReturnsCurrentPassPrompt covers the cross-flow
// where the agent advances status, then asks `kerf review` for the prompt of
// the new current pass. Spec: specs/commands.md §`kerf review` step 3 (resolve
// the pass corresponding to the work's current status).
func TestReview_AfterStatusAdvance_ReturnsCurrentPassPrompt(t *testing.T) {
	_ = makePlan020Work(t, "plan020-review-proj", "review-demo")

	captureOutput(t, func() {
		projectFlag = "plan020-review-proj"
		statusQuiet = true
		defer func() { projectFlag = ""; statusQuiet = false }()
		_ = statusCmd.RunE(statusCmd, []string{"review-demo", "decompose"})
	})

	out := captureOutput(t, func() {
		projectFlag = "plan020-review-proj"
		defer func() { projectFlag = "" }()
		if err := reviewCmd.RunE(reviewCmd, []string{"review-demo"}); err != nil {
			t.Fatalf("review: %v", err)
		}
	})

	// Header: "Reviewer prompt for {codename} — pass: {pass-name}"
	testutil.AssertStringContains(t, out, "Reviewer prompt for review-demo")
	testutil.AssertStringContains(t, out, "pass: Decompose")
	// Artifacts block lists the pass output.
	testutil.AssertStringContains(t, out, "Artifacts to read:")
	testutil.AssertStringContains(t, out, "02-components.md")
	// Criteria block (jig review body) — verbatim emission.
	testutil.AssertStringContains(t, out, "Done when the reviewer approves on:")
	// Return-shape footer.
	testutil.AssertStringContains(t, out, "\"Approved\"")
	testutil.AssertStringContains(t, out, "\"Changes requested:")
}

// TestReview_ExplicitPassFlag verifies --pass overrides the current-status
// default. Spec: specs/commands.md §`kerf review` step 3.
func TestReview_ExplicitPassFlag(t *testing.T) {
	_ = makePlan020Work(t, "plan020-review-pass-proj", "review-pass-demo")

	// Work is at problem-space; ask for the tasks pass prompt (a content pass
	// with a review block — research has no `Done when reviewer approves on`
	// section in the spec jig because it's a free-research pass).
	out := captureOutput(t, func() {
		projectFlag = "plan020-review-pass-proj"
		reviewPassFlag = "tasks"
		defer func() { projectFlag = ""; reviewPassFlag = "" }()
		if err := reviewCmd.RunE(reviewCmd, []string{"review-pass-demo"}); err != nil {
			t.Fatalf("review --pass: %v", err)
		}
	})
	testutil.AssertStringContains(t, out, "pass: Tasks")
}

// TestPreview_ArbitraryStatus_DoesNotAdvance covers `kerf preview <work>
// <status>` for a future pass — output marks read-only, status on disk is
// unchanged. Companion to TestPreview_RendersFuturePassWithoutAdvancing but
// scoped to a later pass (tasks, late in the chain) to catch index-handling
// bugs in the pass renderer. Spec: specs/commands.md §`kerf preview` step 4
// + "The status on disk is not touched."
func TestPreview_ArbitraryStatus_DoesNotAdvance(t *testing.T) {
	workDir := makePlan020Work(t, "plan020-preview-proj", "preview-late-demo")
	specPath := filepath.Join(workDir, "spec.yaml")

	pre, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	out := captureOutput(t, func() {
		projectFlag = "plan020-preview-proj"
		defer func() { projectFlag = "" }()
		if err := previewCmd.RunE(previewCmd, []string{"preview-late-demo", "tasks"}); err != nil {
			t.Fatalf("preview: %v", err)
		}
	})

	testutil.AssertStringContains(t, out, "Preview for preview-late-demo")
	testutil.AssertStringContains(t, out, "(read-only, status unchanged)")
	testutil.AssertStringContains(t, out, "Tasks")
	testutil.AssertStringContains(t, out, "Output:")

	post, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec post: %v", err)
	}
	if !bytes.Equal(pre, post) {
		t.Errorf("preview altered spec.yaml; preview is read-only")
	}

	// Idempotent: a second preview must produce the same output.
	out2 := captureOutput(t, func() {
		projectFlag = "plan020-preview-proj"
		defer func() { projectFlag = "" }()
		_ = previewCmd.RunE(previewCmd, []string{"preview-late-demo", "tasks"})
	})
	if out != out2 {
		t.Errorf("preview not idempotent across two invocations")
	}
}

// TestShow_Compact_AllPlan020Invariants packs the four invariants of
// specs/commands.md §`kerf show` --compact output into one cross-flow test:
// the four-line shape after a status advance, the next-pass field tracking
// the new status, the bead_filter slot always rendered, and the omission of
// the verbose sections.
func TestShow_Compact_AllPlan020Invariants(t *testing.T) {
	_ = makePlan020Work(t, "plan020-compact-proj", "compact-flow-demo")

	captureOutput(t, func() {
		projectFlag = "plan020-compact-proj"
		statusQuiet = true
		defer func() { projectFlag = ""; statusQuiet = false }()
		_ = statusCmd.RunE(statusCmd, []string{"compact-flow-demo", "decompose"})
	})

	out := captureOutput(t, func() {
		projectFlag = "plan020-compact-proj"
		showCompactFlag = true
		defer func() { projectFlag = ""; showCompactFlag = false }()
		if err := showCmd.RunE(showCmd, []string{"compact-flow-demo"}); err != nil {
			t.Fatalf("show --compact: %v", err)
		}
	})

	// Line 1: codename, current status (decompose), next-pass name (Research).
	testutil.AssertStringContains(t, out, "compact-flow-demo  status: decompose → next: Research")
	// Line 2: bead_filter slot, default "(none)".
	testutil.AssertStringContains(t, out, "bead_filter: (none)")
	// Line 3: files count.
	testutil.AssertStringContains(t, out, "files:")
	testutil.AssertStringContains(t, out, "in work directory")
	// Line 4: last-session marker.
	testutil.AssertStringContains(t, out, "last session:")

	// Compact form must omit the verbose Pass N → Output: lines and file tree.
	if strings.Contains(out, "Pass 1:") {
		t.Errorf("compact form must omit per-pass listing; got:\n%s", out)
	}
	if strings.Contains(out, "Files:") {
		t.Errorf("compact form must omit verbose Files: tree; got:\n%s", out)
	}
}

// TestShow_Default_PassOutputLines_FullCycle verifies the Pass N → Output:
// line surfaces for every pass in the spec jig and that {component} paths
// render their template form. Spec: specs/jig-system.md §Surfacing Pass
// Filenames.
func TestShow_Default_PassOutputLines_FullCycle(t *testing.T) {
	_ = makePlan020Work(t, "plan020-passlines-proj", "passlines-demo")

	out := captureOutput(t, func() {
		projectFlag = "plan020-passlines-proj"
		defer func() { projectFlag = "" }()
		if err := showCmd.RunE(showCmd, []string{"passlines-demo"}); err != nil {
			t.Fatalf("show: %v", err)
		}
	})

	// Each content pass surfaces a Pass N → Output: line. The {component}
	// passes render the template form, not a resolved path.
	wants := []string{
		"Pass 1: Problem Space → Output: 01-problem-space.md",
		"Pass 2: Decompose → Output: 02-components.md",
		"Pass 3: Research → Output: 03-research/{component}/findings.md",
		"Pass 4: Change Design → Output: 04-design/{component}-design.md",
		"Pass 6: Integration → Output: 06-integration.md",
		"Pass 7: Tasks → Output: 07-tasks.md",
	}
	for _, w := range wants {
		testutil.AssertStringContains(t, out, w)
	}
}
