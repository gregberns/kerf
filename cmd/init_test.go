package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/testutil"
)

func TestInit_CreatesProjectYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	gitRepo := testutil.SetupGitRepo(t)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	out := captureOutput(t, func() {
		err := initCmd.RunE(initCmd, []string{})
		if err != nil {
			t.Fatalf("init error: %v", err)
		}
	})

	// Verify project-identifier was created
	testutil.AssertFileExists(t, filepath.Join(gitRepo, ".kerf", "project-identifier"))

	// Read project ID to find project.yaml
	pidData, err := os.ReadFile(filepath.Join(gitRepo, ".kerf", "project-identifier"))
	if err != nil {
		t.Fatalf("reading project-identifier: %v", err)
	}
	projectID := trimSpace(string(pidData))

	// Verify project.yaml was created
	bp := filepath.Join(tmp, ".kerf")
	projCfgPath := config.ProjectConfigPath(bp, projectID)
	testutil.AssertFileExists(t, projCfgPath)

	// Load and verify project.yaml content
	projCfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	if len(projCfg.Jigs) == 0 {
		t.Error("project.yaml Jigs should not be empty")
	}

	// Should include built-in jigs
	hasJig := func(name string) bool {
		for _, j := range projCfg.Jigs {
			if j == name {
				return true
			}
		}
		return false
	}
	if !hasJig("plan") {
		t.Error("project.yaml should include plan jig")
	}
	if !hasJig("bug") {
		t.Error("project.yaml should include bug jig")
	}

	// Verify output mentions project.yaml creation
	testutil.AssertStringContains(t, out, "project.yaml")
	testutil.AssertStringContains(t, out, "active jigs")
}

func TestInit_OutputIncludesSetup(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	gitRepo := testutil.SetupGitRepo(t)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	out := captureOutput(t, func() {
		err := initCmd.RunE(initCmd, []string{})
		if err != nil {
			t.Fatalf("init error: %v", err)
		}
	})

	// Should include bootstrap instructions
	testutil.AssertStringContains(t, out, "AGENT SETUP INSTRUCTIONS")

	// Should include setup output (agent-facing instructions from kerf setup)
	testutil.AssertStringContains(t, out, "START AGENT INSTRUCTIONS")
	testutil.AssertStringContains(t, out, "END AGENT INSTRUCTIONS")
}

func TestInit_BootstrapInstructionsMentionsAllJigTypes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	gitRepo := testutil.SetupGitRepo(t)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	out := captureOutput(t, func() {
		err := initCmd.RunE(initCmd, []string{})
		if err != nil {
			t.Fatalf("init error: %v", err)
		}
	})

	testutil.AssertStringContains(t, out, "--jig plan")
	testutil.AssertStringContains(t, out, "--jig bug")
	testutil.AssertStringContains(t, out, "--jig implementation")
	testutil.AssertStringContains(t, out, "--jig spike")
	testutil.AssertStringContains(t, out, "--jig retrofit")
}

func TestInit_AlreadyInitialized(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	gitRepo := testutil.SetupGitRepo(t)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	// Run init twice
	captureOutput(t, func() {
		err := initCmd.RunE(initCmd, []string{})
		if err != nil {
			t.Fatalf("first init error: %v", err)
		}
	})

	out := captureOutput(t, func() {
		err := initCmd.RunE(initCmd, []string{})
		if err != nil {
			t.Fatalf("second init error: %v", err)
		}
	})

	testutil.AssertStringContains(t, out, "already initialized")
}
