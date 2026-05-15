package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

	// Bead 8: every command in the Available-commands list must render.
	for _, cmdName := range expectedCommandNames() {
		testutil.AssertStringContains(t, out, "kerf "+cmdName)
	}
}

// expectedCommandNames returns the command names that must appear in the
// rendered "Available commands" help output. Mirrors specs/commands.md.
func expectedCommandNames() []string {
	return []string{
		"init",
		"setup",
		"localize",
		"new",
		"list",
		"show",
		"status",
		"resume",
		"shelve",
		"finalize",
		"square",
		"next",
		"triage",
		"pin",
		"work",
		"map",
		"areas",
		"snapshot",
		"history",
		"restore",
		"archive",
		"delete",
		"config",
		"jig",
	}
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

	// Bead 8: every command in the Available-commands list must render.
	for _, cmdName := range expectedCommandNames() {
		testutil.AssertStringContains(t, out, "kerf "+cmdName)
	}
}

func TestRootCommand_WithJigChain(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "my-proj")
	os.MkdirAll(filepath.Join(projDir, "blue-bear"), 0755)
	writeMinimalSpec(t, filepath.Join(projDir, "blue-bear", "spec.yaml"), "blue-bear", "my-proj")

	// Create project.yaml with a jig chain
	projectYAML := "jigs:\n  - plan\n  - bug\n"
	os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte(projectYAML), 0644)

	// Set up git repo so project resolves
	gitRepo := testutil.SetupGitRepo(t)
	os.MkdirAll(filepath.Join(gitRepo, ".kerf"), 0755)
	os.WriteFile(filepath.Join(gitRepo, ".kerf", "project-identifier"), []byte("my-proj\n"), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	out := captureOutput(t, func() {
		rootCmd.SetArgs([]string{})
		rootCmd.Run(rootCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Bench summary")
	testutil.AssertStringContains(t, out, "This project uses:")
	testutil.AssertStringContains(t, out, "plan")
	testutil.AssertStringContains(t, out, "bug")
}

func TestRootCommand_NoJigChainWithoutProjectYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	os.MkdirAll(filepath.Join(benchDir, "projects", "my-proj", "blue-bear"), 0755)
	writeMinimalSpec(t, filepath.Join(benchDir, "projects", "my-proj", "blue-bear", "spec.yaml"), "blue-bear", "my-proj")

	// No project.yaml — jig chain should not appear

	gitRepo := testutil.SetupGitRepo(t)
	os.MkdirAll(filepath.Join(gitRepo, ".kerf"), 0755)
	os.WriteFile(filepath.Join(gitRepo, ".kerf", "project-identifier"), []byte("my-proj\n"), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	out := captureOutput(t, func() {
		rootCmd.SetArgs([]string{})
		rootCmd.Run(rootCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Bench summary")
	if strings.Contains(out, "This project uses:") {
		t.Errorf("should not show jig chain without project.yaml, got: %s", out)
	}
}

func TestRootCommand_JigChainComposable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "my-proj")
	os.MkdirAll(filepath.Join(projDir, "blue-bear"), 0755)
	writeMinimalSpec(t, filepath.Join(projDir, "blue-bear", "spec.yaml"), "blue-bear", "my-proj")

	// Create a composable user jig
	jigsDir := filepath.Join(benchDir, "jigs")
	os.MkdirAll(jigsDir, 0755)
	composableJig := `---
name: deploy
description: Deployment jig
version: 1
phase: implementation
composable: true
status_values:
  - build
  - test
  - ship
passes:
  - name: "Build"
    status: build
    output: ["build.log"]
  - name: "Test"
    status: test
    output: ["test.log"]
  - name: "Ship"
    status: ship
    output: ["deploy.log"]
---

# Deploy
`
	os.WriteFile(filepath.Join(jigsDir, "deploy.md"), []byte(composableJig), 0644)

	// project.yaml with composable jig and pass filtering
	projectYAML := "jigs:\n  - plan\n  - deploy\npasses:\n  deploy:\n    - Build\n    - Ship\n"
	os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte(projectYAML), 0644)

	gitRepo := testutil.SetupGitRepo(t)
	os.MkdirAll(filepath.Join(gitRepo, ".kerf"), 0755)
	os.WriteFile(filepath.Join(gitRepo, ".kerf", "project-identifier"), []byte("my-proj\n"), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	out := captureOutput(t, func() {
		rootCmd.SetArgs([]string{})
		rootCmd.Run(rootCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "This project uses:")
	testutil.AssertStringContains(t, out, "deploy (Build, Ship)")
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
