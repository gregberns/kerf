package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/kerf/internal/testutil"
)

func TestSetupNoProjectYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a git repo with project-identifier
	gitRepo := testutil.SetupGitRepo(t)
	os.MkdirAll(filepath.Join(gitRepo, ".kerf"), 0o755)
	os.WriteFile(filepath.Join(gitRepo, ".kerf", "project-identifier"), []byte("test-proj\n"), 0o644)

	// Create bench (so BenchPath works)
	os.MkdirAll(filepath.Join(tmp, ".kerf", "projects", "test-proj"), 0o755)

	// Run setup from inside the git repo
	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	out := captureOutput(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		if err != nil {
			t.Fatalf("setup error: %v", err)
		}
	})

	testutil.AssertStringContains(t, out, "No project.yaml found")
	testutil.AssertStringContains(t, out, "kerf new --jig <name>")
	testutil.AssertStringContains(t, out, "Available jigs:")
	testutil.AssertStringContains(t, out, "plan")
	testutil.AssertStringContains(t, out, "bug")
}

func TestSetupWithProjectYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a git repo with project-identifier
	gitRepo := testutil.SetupGitRepo(t)
	os.MkdirAll(filepath.Join(gitRepo, ".kerf"), 0o755)
	os.WriteFile(filepath.Join(gitRepo, ".kerf", "project-identifier"), []byte("my-proj\n"), 0o644)

	// Create bench with project.yaml
	projDir := filepath.Join(tmp, ".kerf", "projects", "my-proj")
	os.MkdirAll(projDir, 0o755)
	projectYAML := `jigs:
  - plan
  - bug
`
	os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte(projectYAML), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	out := captureOutput(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		if err != nil {
			t.Fatalf("setup error: %v", err)
		}
	})

	testutil.AssertStringContains(t, out, "START AGENT INSTRUCTIONS")
	testutil.AssertStringContains(t, out, "END AGENT INSTRUCTIONS")
	testutil.AssertStringContains(t, out, "my-proj")
	testutil.AssertStringContains(t, out, "plan")
	testutil.AssertStringContains(t, out, "bug")
	testutil.AssertStringContains(t, out, "kerf new --jig plan")
	testutil.AssertStringContains(t, out, "Process instructions")
}

func TestSetupWithProjectFlag(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create bench — no git repo needed when using --project flag
	os.MkdirAll(filepath.Join(tmp, ".kerf", "projects", "flag-proj"), 0o755)

	oldFlag := projectFlag
	projectFlag = "flag-proj"
	defer func() { projectFlag = oldFlag }()

	out := captureOutput(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		if err != nil {
			t.Fatalf("setup error: %v", err)
		}
	})

	// No project.yaml exists, so should get default instructions
	testutil.AssertStringContains(t, out, "No project.yaml found")
	testutil.AssertStringContains(t, out, "Available jigs:")
}

func TestSetupComposablePassFiltering(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a git repo with project-identifier
	gitRepo := testutil.SetupGitRepo(t)
	os.MkdirAll(filepath.Join(gitRepo, ".kerf"), 0o755)
	os.WriteFile(filepath.Join(gitRepo, ".kerf", "project-identifier"), []byte("comp-proj\n"), 0o644)

	// Create a composable user jig
	jigsDir := filepath.Join(tmp, ".kerf", "jigs")
	os.MkdirAll(jigsDir, 0o755)
	composableJig := `---
name: deploy
description: Deployment jig
version: 1
phase: implementation
tools:
  - docker
composable: true
status_values:
  - build
  - test
  - ship
passes:
  - name: "Build"
    status: build
    output: ["build.log"]
    tools:
      - docker
  - name: "Test"
    status: test
    output: ["test.log"]
  - name: "Ship"
    status: ship
    output: ["deploy.log"]
---

# Deploy
`
	os.WriteFile(filepath.Join(jigsDir, "deploy.md"), []byte(composableJig), 0o644)

	// Create project.yaml that filters passes
	projDir := filepath.Join(tmp, ".kerf", "projects", "comp-proj")
	os.MkdirAll(projDir, 0o755)
	projectYAML := `jigs:
  - deploy
passes:
  deploy:
    - Build
    - Ship
`
	os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte(projectYAML), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	out := captureOutput(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		if err != nil {
			t.Fatalf("setup error: %v", err)
		}
	})

	testutil.AssertStringContains(t, out, "deploy")
	testutil.AssertStringContains(t, out, "[composable]")
	testutil.AssertStringContains(t, out, "Build")
	testutil.AssertStringContains(t, out, "Ship")
	testutil.AssertStringContains(t, out, "docker")

	// "Test" pass should be filtered out
	if containsStr([]string{out}, "status: test") {
		t.Error("Test pass should be filtered out but was found in output")
	}
}

func TestSetupNotInGitRepo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// No git repo, no --project flag
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	oldFlag := projectFlag
	projectFlag = ""
	defer func() { projectFlag = oldFlag }()

	err := setupCmd.RunE(setupCmd, []string{})
	if err == nil {
		t.Fatal("expected error when not in a git repo")
	}
	testutil.AssertStringContains(t, err.Error(), "not in a git repository")
}

func TestSetupNoProjectIdentifier(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Git repo but no .kerf/project-identifier
	gitRepo := testutil.SetupGitRepo(t)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	oldFlag := projectFlag
	projectFlag = ""
	defer func() { projectFlag = oldFlag }()

	err := setupCmd.RunE(setupCmd, []string{})
	if err == nil {
		t.Fatal("expected error when no project-identifier")
	}
	testutil.AssertStringContains(t, err.Error(), "project not initialized")
}

// kerf-tatj: a corrupt .kerf/project-identifier must surface the
// kerf-dlb-style validation error from `kerf setup` rather than being
// swallowed into "project not initialized". Mirrors kerf-vu0r for `kerf new`.
// The missing-file fall-through is exercised by TestSetupNoProjectIdentifier.
func TestSetupCorruptProjectIdentifier_Errors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	gitRepo := testutil.SetupGitRepo(t)
	if err := os.MkdirAll(filepath.Join(gitRepo, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	idPath := filepath.Join(gitRepo, ".kerf", "project-identifier")
	garbage := []byte("bad/\x00id\n")
	if err := os.WriteFile(idPath, garbage, 0o644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	oldFlag := projectFlag
	projectFlag = ""
	defer func() { projectFlag = oldFlag }()

	err := setupCmd.RunE(setupCmd, []string{})
	if err == nil {
		t.Fatal("expected error when project-identifier is corrupt, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"corrupt project identifier", idPath, "replace with a clean slug"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
	if strings.Contains(msg, "project not initialized") {
		t.Errorf("corrupt-identifier error must not be reported as missing-init: %q", msg)
	}
}

func TestSetupToolRequirements(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a git repo with project-identifier
	gitRepo := testutil.SetupGitRepo(t)
	os.MkdirAll(filepath.Join(gitRepo, ".kerf"), 0o755)
	os.WriteFile(filepath.Join(gitRepo, ".kerf", "project-identifier"), []byte("tools-proj\n"), 0o644)

	// Create a user jig with tools
	jigsDir := filepath.Join(tmp, ".kerf", "jigs")
	os.MkdirAll(jigsDir, 0o755)
	toolJig := `---
name: orchestrated
description: A jig with tools
version: 1
phase: implementation
tools:
  - ntm
  - agent-mail
composable: false
status_values:
  - start
  - done
passes:
  - name: "Start"
    status: start
    output: ["out.md"]
---

# Orchestrated
`
	os.WriteFile(filepath.Join(jigsDir, "orchestrated.md"), []byte(toolJig), 0o644)

	// Project.yaml includes only this jig
	projDir := filepath.Join(tmp, ".kerf", "projects", "tools-proj")
	os.MkdirAll(projDir, 0o755)
	projectYAML := `jigs:
  - orchestrated
`
	os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte(projectYAML), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	out := captureOutput(t, func() {
		err := setupCmd.RunE(setupCmd, []string{})
		if err != nil {
			t.Fatalf("setup error: %v", err)
		}
	})

	testutil.AssertStringContains(t, out, "Tool requirements")
	testutil.AssertStringContains(t, out, "ntm")
	testutil.AssertStringContains(t, out, "agent-mail")
}

// TestSetupGitignorePattern verifies kerf setup advertises the corrected
// .gitignore negation pattern. Git negation requires the parent to use
// '.kerf/*' (with trailing /*); '.kerf/' alone shadows the negation and
// re-ignores project-identifier. See bead kerf-73h.
func TestSetupGitignorePattern(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	gitRepo := testutil.SetupGitRepo(t)
	os.MkdirAll(filepath.Join(gitRepo, ".kerf"), 0o755)
	os.WriteFile(filepath.Join(gitRepo, ".kerf", "project-identifier"), []byte("gi-proj\n"), 0o644)

	projDir := filepath.Join(tmp, ".kerf", "projects", "gi-proj")
	os.MkdirAll(projDir, 0o755)
	os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte("jigs:\n  - plan\n"), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	out := captureOutput(t, func() {
		if err := setupCmd.RunE(setupCmd, []string{}); err != nil {
			t.Fatalf("setup error: %v", err)
		}
	})

	// Must contain the working pattern.
	testutil.AssertStringContains(t, out, ".kerf/*")
	testutil.AssertStringContains(t, out, "!.kerf/project-identifier")

	// Must NOT contain the broken pattern as a standalone line.
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == ".kerf/" {
			t.Errorf("output contains broken pattern '.kerf/' as a standalone line (negation will not work); got line: %q", line)
		}
	}
}

// TestSetupGitignorePatternWorksWithGit is an integration test that writes
// the advertised .gitignore pattern into a real git repo and verifies that
// git check-ignore behaves as documented.
func TestSetupGitignorePatternWorksWithGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")

	// Write the exact pattern kerf setup advertises.
	gitignore := ".kerf/*\n!.kerf/project-identifier\n"
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "foo"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte("p"), 0o644); err != nil {
		t.Fatal(err)
	}

	// .kerf/foo should be ignored (exit 0).
	cmd := exec.Command("git", "check-ignore", "--quiet", ".kerf/foo")
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		t.Errorf(".kerf/foo should be ignored by '.kerf/*', but check-ignore did not match: %v", err)
	}

	// .kerf/project-identifier should NOT be ignored (exit 1).
	cmd = exec.Command("git", "check-ignore", "--quiet", ".kerf/project-identifier")
	cmd.Dir = repo
	err := cmd.Run()
	if err == nil {
		t.Error(".kerf/project-identifier should NOT be ignored (negation must apply), but check-ignore matched it")
	} else if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
		t.Errorf("expected exit code 1 from check-ignore on project-identifier, got: %v", err)
	}
}
