package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/jig"
	"github.com/gberns/kerf/internal/testutil"
)

// ─── Process pass status computation ────────────────────────────────────────

func TestCheckProcessPasses_NoProcessPasses(t *testing.T) {
	// A jig where all passes have output files — no process passes
	jigDefNoProcess := &jig.JigDefinition{
		StatusValues: []string{"draft", "done"},
		Passes: []jig.Pass{
			{Name: "Draft", Status: "draft", Output: []string{"draft.md"}},
			{Name: "Done", Status: "done", Output: []string{"done.md"}},
		},
	}

	result := &squareResult{}
	checkProcessPasses(jigDefNoProcess, "done", result)

	if result.HasProcessPasses {
		t.Error("HasProcessPasses should be false for jig with no empty-output passes")
	}
	if result.ProcessTotal != 0 {
		t.Errorf("ProcessTotal = %d, want 0", result.ProcessTotal)
	}
	// IsSquare should not consider process passes
	result.StatusPass = true
	result.FilesPass = true
	result.DepsPass = true
	if !result.IsSquare() {
		t.Error("IsSquare should be true when no process passes")
	}
}

func TestCheckProcessPasses_AllComplete(t *testing.T) {
	// Implementation-like jig with 2 process passes
	jigDef := &jig.JigDefinition{
		Name:       "test-impl",
		Composable: true,
		StatusValues: []string{"breakdown", "dispatch", "implementing", "verify", "complete"},
		Passes: []jig.Pass{
			{Name: "Breakdown", Status: "breakdown", Output: []string{"01-breakdown.md"}},
			{Name: "Dispatch", Status: "dispatch", Output: []string{"02-dispatch.md"}},
			{Name: "Implement", Status: "implementing", Output: []string{}},
			{Name: "Verify", Status: "verify", Output: []string{"03-verify.md"}},
			{Name: "Complete", Status: "complete", Output: []string{}},
		},
	}

	// Status past all passes (not in status_values — treated as past terminal)
	result := &squareResult{}
	checkProcessPasses(jigDef, "finalized", result)

	if !result.HasProcessPasses {
		t.Fatal("HasProcessPasses should be true")
	}
	if result.ProcessTotal != 2 {
		t.Errorf("ProcessTotal = %d, want 2", result.ProcessTotal)
	}
	if result.ProcessComplete != 2 {
		t.Errorf("ProcessComplete = %d, want 2", result.ProcessComplete)
	}
	if !result.ProcessPass {
		t.Error("ProcessPass should be true")
	}
}

func TestCheckProcessPasses_PartialComplete(t *testing.T) {
	jigDef := &jig.JigDefinition{
		Name:       "test-impl",
		Composable: true,
		StatusValues: []string{"breakdown", "dispatch", "implementing", "verify", "complete"},
		Passes: []jig.Pass{
			{Name: "Breakdown", Status: "breakdown", Output: []string{"01-breakdown.md"}},
			{Name: "Dispatch", Status: "dispatch", Output: []string{"02-dispatch.md"}},
			{Name: "Implement", Status: "implementing", Output: []string{}},
			{Name: "Verify", Status: "verify", Output: []string{"03-verify.md"}},
			{Name: "Complete", Status: "complete", Output: []string{}},
		},
	}

	// Status is "verify" — Implement is past (done), Complete is not yet done
	result := &squareResult{}
	checkProcessPasses(jigDef, "verify", result)

	if !result.HasProcessPasses {
		t.Fatal("HasProcessPasses should be true")
	}
	if result.ProcessTotal != 2 {
		t.Errorf("ProcessTotal = %d, want 2", result.ProcessTotal)
	}
	if result.ProcessComplete != 1 {
		t.Errorf("ProcessComplete = %d, want 1", result.ProcessComplete)
	}
	if result.ProcessPass {
		t.Error("ProcessPass should be false")
	}

	// Check detail labels
	foundDone := false
	foundPending := false
	for _, d := range result.ProcessDetails {
		if strings.Contains(d, "Implement") && strings.Contains(d, "done") {
			foundDone = true
		}
		if strings.Contains(d, "Complete") && strings.Contains(d, "pending") {
			foundPending = true
		}
	}
	if !foundDone {
		t.Errorf("expected Implement: done in details, got %v", result.ProcessDetails)
	}
	if !foundPending {
		t.Errorf("expected Complete: pending in details, got %v", result.ProcessDetails)
	}
}

func TestCheckProcessPasses_ActiveStatus(t *testing.T) {
	jigDef := &jig.JigDefinition{
		Name:       "test-impl",
		Composable: true,
		StatusValues: []string{"breakdown", "dispatch", "implementing", "verify", "complete"},
		Passes: []jig.Pass{
			{Name: "Implement", Status: "implementing", Output: []string{}},
			{Name: "Complete", Status: "complete", Output: []string{}},
		},
	}

	// Status is "implementing" — currently active on Implement pass
	result := &squareResult{}
	checkProcessPasses(jigDef, "implementing", result)

	if result.ProcessComplete != 0 {
		t.Errorf("ProcessComplete = %d, want 0", result.ProcessComplete)
	}

	foundActive := false
	for _, d := range result.ProcessDetails {
		if strings.Contains(d, "Implement") && strings.Contains(d, "active") {
			foundActive = true
		}
	}
	if !foundActive {
		t.Errorf("expected Implement: active in details, got %v", result.ProcessDetails)
	}
}

func TestCheckProcessPasses_IsSquareIncludesProcess(t *testing.T) {
	result := &squareResult{
		StatusPass:       true,
		FilesPass:        true,
		DepsPass:         true,
		HasProcessPasses: true,
		ProcessPass:      false,
	}
	if result.IsSquare() {
		t.Error("IsSquare should be false when ProcessPass is false and HasProcessPasses is true")
	}

	result.ProcessPass = true
	if !result.IsSquare() {
		t.Error("IsSquare should be true when all checks pass including process")
	}
}

// ─── Square check with composable jig (implementation) ──────────────────────

func TestSquare_ImplementationJig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "impl-proj"
	codename := "api-refactor"

	// Create implementation work at "complete" status with all files
	workDir := filepath.Join(bp, "projects", proj, codename)
	os.MkdirAll(workDir, 0755)

	workSpec := `codename: ` + codename + `
type: implementation
project:
  id: ` + proj + `
jig: implementation
jig_version: 1
status: complete
status_values: [breakdown, dispatch, implementing, verify, complete]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(workSpec), 0644)
	os.WriteFile(filepath.Join(workDir, "SESSION.md"), []byte("# Session"), 0644)
	os.WriteFile(filepath.Join(workDir, "01-breakdown.md"), []byte("breakdown"), 0644)
	os.WriteFile(filepath.Join(workDir, "02-dispatch.md"), []byte("dispatch"), 0644)
	os.WriteFile(filepath.Join(workDir, "03-verify.md"), []byte("verify"), 0644)

	result, err := checkSquare(proj, codename)
	if err != nil {
		t.Fatalf("checkSquare error: %v", err)
	}

	if !result.HasProcessPasses {
		t.Fatal("expected HasProcessPasses=true for implementation jig")
	}
	if result.ProcessTotal != 2 {
		t.Errorf("ProcessTotal = %d, want 2", result.ProcessTotal)
	}
	// "complete" is the terminal status. When at terminal, all process passes are complete.
	// This matches the verification.md example: status=complete, process=pass, result=SQUARE.
	if result.ProcessComplete != 2 {
		t.Errorf("ProcessComplete = %d, want 2 (all done at terminal status)", result.ProcessComplete)
	}
	if !result.ProcessPass {
		t.Error("ProcessPass should be true at terminal status")
	}
	if !result.IsSquare() {
		t.Error("expected SQUARE for complete implementation work")
	}
}

func TestSquare_ImplementationJig_InProgress(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "impl-proj2"
	codename := "in-progress"

	workDir := filepath.Join(bp, "projects", proj, codename)
	os.MkdirAll(workDir, 0755)

	workSpec := `codename: ` + codename + `
type: implementation
project:
  id: ` + proj + `
jig: implementation
jig_version: 1
status: implementing
status_values: [breakdown, dispatch, implementing, verify, complete]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(workSpec), 0644)
	os.WriteFile(filepath.Join(workDir, "SESSION.md"), []byte("# Session"), 0644)
	os.WriteFile(filepath.Join(workDir, "01-breakdown.md"), []byte("breakdown"), 0644)
	os.WriteFile(filepath.Join(workDir, "02-dispatch.md"), []byte("dispatch"), 0644)

	result, err := checkSquare(proj, codename)
	if err != nil {
		t.Fatalf("checkSquare error: %v", err)
	}

	if !result.HasProcessPasses {
		t.Fatal("expected HasProcessPasses=true")
	}
	// "implementing" is at index 2, Implement pass is also at index 2 (active, not past)
	// Complete pass is at index 4 (pending)
	if result.ProcessComplete != 0 {
		t.Errorf("ProcessComplete = %d, want 0", result.ProcessComplete)
	}
	if result.ProcessPass {
		t.Error("ProcessPass should be false while implementing")
	}
	if result.IsSquare() {
		t.Error("should NOT be square while implementing")
	}
}

// ─── Output format includes Process section ─────────────────────────────────

func TestSquareOutput_IncludesProcessSection(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "output-proj"
	codename := "format-check"

	workDir := filepath.Join(bp, "projects", proj, codename)
	os.MkdirAll(workDir, 0755)

	workSpec := `codename: ` + codename + `
type: implementation
project:
  id: ` + proj + `
jig: implementation
jig_version: 1
status: complete
status_values: [breakdown, dispatch, implementing, verify, complete]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(workSpec), 0644)
	os.WriteFile(filepath.Join(workDir, "SESSION.md"), []byte("# Session"), 0644)
	os.WriteFile(filepath.Join(workDir, "01-breakdown.md"), []byte("breakdown"), 0644)
	os.WriteFile(filepath.Join(workDir, "02-dispatch.md"), []byte("dispatch"), 0644)
	os.WriteFile(filepath.Join(workDir, "03-verify.md"), []byte("verify"), 0644)

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		squareCmd.RunE(squareCmd, []string{codename})
	})

	testutil.AssertStringContains(t, out, "Process:")
	testutil.AssertStringContains(t, out, "pass — 2/2 process passes complete")
	testutil.AssertStringContains(t, out, "Result: SQUARE")
}

func TestSquareOutput_NoProcessForBugJig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "noprocess-proj"
	codename := "no-process"

	createSquareBugWork(t, bp, proj, codename)

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		squareCmd.RunE(squareCmd, []string{codename})
	})

	if strings.Contains(out, "Process:") {
		t.Errorf("bug jig output should not contain Process section, got:\n%s", out)
	}
	testutil.AssertStringContains(t, out, "Result: SQUARE")
}

func TestSquareOutput_FailedProcess(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "fail-proc-proj"
	codename := "fail-process"

	workDir := filepath.Join(bp, "projects", proj, codename)
	os.MkdirAll(workDir, 0755)

	workSpec := `codename: ` + codename + `
type: implementation
project:
  id: ` + proj + `
jig: implementation
jig_version: 1
status: implementing
status_values: [breakdown, dispatch, implementing, verify, complete]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(workSpec), 0644)
	os.WriteFile(filepath.Join(workDir, "SESSION.md"), []byte("# Session"), 0644)
	os.WriteFile(filepath.Join(workDir, "01-breakdown.md"), []byte("breakdown"), 0644)
	os.WriteFile(filepath.Join(workDir, "02-dispatch.md"), []byte("dispatch"), 0644)

	out := captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		squareCmd.RunE(squareCmd, []string{codename})
	})

	testutil.AssertStringContains(t, out, "Process:")
	testutil.AssertStringContains(t, out, "fail — 0/2 process passes complete")
	testutil.AssertStringContains(t, out, "Result: NOT SQUARE")
}

// ─── Bead output parsing ────────────────────────────────────────────────────

func TestParseBeadOutput(t *testing.T) {
	output := "37 total\n22 closed\n15 open\n"
	total, closed, open := parseBeadOutput(output)
	if total != 37 {
		t.Errorf("total = %d, want 37", total)
	}
	if closed != 22 {
		t.Errorf("closed = %d, want 22", closed)
	}
	if open != 15 {
		t.Errorf("open = %d, want 15", open)
	}
}

func TestParseBeadOutput_Empty(t *testing.T) {
	total, closed, open := parseBeadOutput("")
	if total != 0 || closed != 0 || open != 0 {
		t.Errorf("expected all zeros, got total=%d closed=%d open=%d", total, closed, open)
	}
}
