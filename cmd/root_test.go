package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

func TestRootCommand_NoBench(t *testing.T) {
	// Point HOME to a temp dir so ~/.kerf doesn't exist.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	out := captureOutput(t, func() {
		rootCmd.SetArgs([]string{})
		rootCmd.Run(rootCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "kerf")
	testutil.AssertStringContains(t, out, "No bench found")
	testutil.AssertStringContains(t, out, "kerf new")
}

func TestRootCommand_WithBench(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create bench with a project and work.
	benchDir := filepath.Join(tmp, ".kerf")
	os.MkdirAll(filepath.Join(benchDir, "projects", "my-proj", "blue-bear"), 0755)
	writeMinimalSpec(t, filepath.Join(benchDir, "projects", "my-proj", "blue-bear", "spec.yaml"), "blue-bear", "my-proj")

	out := captureOutput(t, func() {
		rootCmd.SetArgs([]string{})
		rootCmd.Run(rootCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Bench summary")
	testutil.AssertStringContains(t, out, "Total active works: 1")
	testutil.AssertStringContains(t, out, "Standard workflow")
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func writeMinimalSpec(t *testing.T, path, codename, projectID string) {
	t.Helper()
	content := `codename: ` + codename + `
type: plan
project:
  id: ` + projectID + `
jig: plan
jig_version: 1
status: problem-space
status_values: [problem-space, analyze, decompose, research, change-spec, integration, tasks, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeMinimalSpec: %v", err)
	}
}
