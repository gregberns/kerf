// Plan 021 — verifies that ListNamed honors the configured tool name and
// surfaces concrete errors instead of silently degrading to (nil, nil) on
// exec failure. Each test stages a fake binary in a temp directory and
// points PATH at it so the test never depends on host br/bd installations.
package beads

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeFakeTool stages an executable shell script that emits the given stdout
// and stderr and exits with exitCode. Returns the directory containing it
// (suitable for prepending to PATH).
func writeFakeTool(t *testing.T, name, stdout, stderr string, exitCode int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-tool tests use POSIX shell scripts")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\n"
	if stdout != "" {
		// printf to avoid trailing-newline surprises and to handle JSON braces.
		script += "printf '%s' '" + strings.ReplaceAll(stdout, "'", "'\\''") + "'\n"
	}
	if stderr != "" {
		script += "printf '%s' '" + strings.ReplaceAll(stderr, "'", "'\\''") + "' >&2\n"
	}
	if exitCode != 0 {
		script += "exit " + itoa(exitCode) + "\n"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tool: %v", err)
	}
	return dir
}

func itoa(n int) string {
	// Avoid importing strconv just for this; small positive ints only.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func withPATH(t *testing.T, dir string) {
	t.Helper()
	old := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", old) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}
}

// TestListNamed_HonorsConfiguredToolBr — tools.tasks=br invokes the `br`
// binary (and not, say, `bd`).
func TestListNamed_HonorsConfiguredToolBr(t *testing.T) {
	stdout := `[{"id":"br-1","title":"from-br","status":"open"}]`
	dir := writeFakeTool(t, "br", stdout, "", 0)
	withPATH(t, dir)

	tool := ResolveToolName(map[string]string{"tasks": "br"})
	if tool != "br" {
		t.Fatalf("resolve: got %q want br", tool)
	}
	got, err := ListNamed(tool)
	if err != nil {
		t.Fatalf("ListNamed: %v", err)
	}
	if len(got) != 1 || got[0].ID != "br-1" {
		t.Fatalf("got %+v, want one bead br-1", got)
	}
}

// TestListNamed_HonorsConfiguredToolBd — tools.tasks=bd invokes `bd`.
// If only `br` were called (the pre-plan-021 bug), the test would see no
// beads because no `br` is staged.
func TestListNamed_HonorsConfiguredToolBd(t *testing.T) {
	stdout := `{"issues":[{"id":"bd-1","title":"from-bd","status":"open"}]}`
	dir := writeFakeTool(t, "bd", stdout, "", 0)
	withPATH(t, dir)

	tool := ResolveToolName(map[string]string{"tasks": "bd"})
	if tool != "bd" {
		t.Fatalf("resolve: got %q want bd", tool)
	}
	got, err := ListNamed(tool)
	if err != nil {
		t.Fatalf("ListNamed: %v", err)
	}
	if len(got) != 1 || got[0].ID != "bd-1" {
		t.Fatalf("got %+v, want one bead bd-1", got)
	}
}

// TestListNamed_NonexistentTool — tools.tasks names a binary that is not on
// PATH. Per the documented contract, this degrades to (nil, nil) so kerf
// works on machines without a bead store installed. (Callers that need to
// distinguish use IsAvailableNamed first; see cmd/triage.go.)
func TestListNamed_NonexistentTool(t *testing.T) {
	dir := t.TempDir()
	withPATH(t, dir)

	tool := ResolveToolName(map[string]string{"tasks": "nonexistent-bead-tool"})
	if IsAvailableNamed(tool) {
		t.Fatalf("IsAvailableNamed(%q) = true, want false", tool)
	}
	got, err := ListNamed(tool)
	if err != nil || got != nil {
		t.Fatalf("ListNamed(%q) = (%v, %v), want (nil, nil)", tool, got, err)
	}
}

// TestListNamed_ToolFails — tool is on PATH but exits non-zero with a JSON
// error envelope on stderr (exactly the `br` JSON_ERROR shape that was
// silently swallowed before plan 021). Must surface a *ToolError tagged
// "BEADS_TOOL_ERROR" with the tool name and a stderr snippet.
func TestListNamed_ToolFails(t *testing.T) {
	stderr := `{"error":{"code":"JSON_ERROR","message":"missing field jsonl_export"}}`
	dir := writeFakeTool(t, "br", "", stderr, 8)
	withPATH(t, dir)

	got, err := ListNamed("br")
	if err == nil {
		t.Fatalf("expected error, got beads=%v", got)
	}
	var te *ToolError
	if !errors.As(err, &te) {
		t.Fatalf("err is not *ToolError: %T %v", err, err)
	}
	if te.Tool != "br" {
		t.Errorf("Tool = %q, want br", te.Tool)
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "BEADS_TOOL_ERROR:") {
		t.Errorf("message missing BEADS_TOOL_ERROR prefix: %s", msg)
	}
	if !strings.Contains(msg, "JSON_ERROR") {
		t.Errorf("message missing stderr snippet: %s", msg)
	}
}

// TestListNamed_DefaultsToBr — empty toolName falls back to DefaultToolName.
func TestListNamed_DefaultsToBr(t *testing.T) {
	stdout := `[{"id":"x","title":"t","status":"open"}]`
	dir := writeFakeTool(t, "br", stdout, "", 0)
	withPATH(t, dir)

	got, err := ListNamed("")
	if err != nil {
		t.Fatalf("ListNamed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d beads, want 1", len(got))
	}
}
