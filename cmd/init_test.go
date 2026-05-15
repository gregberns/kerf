package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/beads"
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

// Plan 008 / B9-code: re-running kerf init without --force on an existing
// project.yaml prints the skip-with-informative-output summary and does NOT
// overwrite the file (spec §kerf init "Re-running on an existing project").
func TestInit_Rerun_PreservesBeadFilter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	gitRepo := testutil.SetupGitRepo(t)
	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	// First init creates project.yaml with default jigs and no bead_filter.
	captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("first init: %v", err)
		}
	})

	pidData, _ := os.ReadFile(filepath.Join(gitRepo, ".kerf", "project-identifier"))
	projectID := strings.TrimSpace(string(pidData))
	projCfgPath := config.ProjectConfigPath(filepath.Join(tmp, ".kerf"), projectID)

	// Hand-set a bead_filter to simulate user edits we must preserve.
	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	cfg.BeadFilter = &beads.Filter{Label: "team:{codename}"}
	if err := config.SaveProjectConfig(projCfgPath, cfg); err != nil {
		t.Fatalf("saving project config: %v", err)
	}
	originalContent, _ := os.ReadFile(projCfgPath)

	// Second init without --force: must skip the project.yaml write entirely.
	out := captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("second init: %v", err)
		}
	})

	// Spec: prints skip message, exits 0, includes path + jigs + bead_filter.
	testutil.AssertStringContains(t, out, "already exists at")
	testutil.AssertStringContains(t, out, "skipping re-initialisation")
	testutil.AssertStringContains(t, out, "Active jigs")
	testutil.AssertStringContains(t, out, "team:{codename}")
	testutil.AssertStringContains(t, out, "kerf init --force")

	// File contents must be byte-identical — no overwrite happened.
	afterContent, _ := os.ReadFile(projCfgPath)
	if string(afterContent) != string(originalContent) {
		t.Errorf("project.yaml was overwritten on re-init without --force")
	}

	// Reload and confirm hand-set bead_filter is intact.
	cfg2, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("reloading project config: %v", err)
	}
	if cfg2.BeadFilter == nil || cfg2.BeadFilter.Label != "team:{codename}" {
		t.Errorf("bead_filter not preserved on skip-path re-init: got %+v", cfg2.BeadFilter)
	}
}

// --force re-runs the full init flow and rewrites project.yaml.
func TestInit_Force_Rewrites(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	gitRepo := testutil.SetupGitRepo(t)
	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("first init: %v", err)
		}
	})

	pidData, _ := os.ReadFile(filepath.Join(gitRepo, ".kerf", "project-identifier"))
	projectID := strings.TrimSpace(string(pidData))
	projCfgPath := config.ProjectConfigPath(filepath.Join(tmp, ".kerf"), projectID)

	// Mutate file to a sentinel so we can verify it gets rewritten.
	if err := os.WriteFile(projCfgPath, []byte("jigs: []\n# sentinel marker\n"), 0o644); err != nil {
		t.Fatalf("seeding sentinel: %v", err)
	}

	// Non-TTY stdin so detection auto-applies without prompting.
	origStdin := os.Stdin
	os.Stdin = nonTTYStdin(t)
	defer func() { os.Stdin = origStdin }()

	initForceFlag = true
	defer func() { initForceFlag = false }()
	out := captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("force re-init: %v", err)
		}
	})

	testutil.AssertStringContains(t, out, "overwriting existing project.yaml")

	// File must no longer contain the sentinel; jigs list must be repopulated.
	after, _ := os.ReadFile(projCfgPath)
	if strings.Contains(string(after), "sentinel marker") {
		t.Errorf("--force did not rewrite project.yaml; sentinel remains")
	}
	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading after force: %v", err)
	}
	if len(cfg.Jigs) == 0 {
		t.Errorf("expected jigs to be repopulated by --force")
	}
}

// Non-interactive --force preserves a hand-set bead_filter rather than
// silently dropping it (spec §kerf init re-run rule, point 2).
func TestInit_Force_NonInteractive_PreservesBeadFilter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	gitRepo := testutil.SetupGitRepo(t)
	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	// br stub returns empty store so detection finds no candidate; the
	// non-interactive preservation rule must still apply.
	stubBr(t, `[]`)

	captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("first init: %v", err)
		}
	})

	pidData, _ := os.ReadFile(filepath.Join(gitRepo, ".kerf", "project-identifier"))
	projectID := strings.TrimSpace(string(pidData))
	projCfgPath := config.ProjectConfigPath(filepath.Join(tmp, ".kerf"), projectID)

	// Seed bead_filter before the --force re-init.
	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	cfg.BeadFilter = &beads.Filter{Label: "team:{codename}"}
	if err := config.SaveProjectConfig(projCfgPath, cfg); err != nil {
		t.Fatalf("saving project config: %v", err)
	}

	origStdin := os.Stdin
	os.Stdin = nonTTYStdin(t)
	defer func() { os.Stdin = origStdin }()

	initForceFlag = true
	defer func() { initForceFlag = false }()
	captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("force re-init: %v", err)
		}
	})

	cfg2, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading after force: %v", err)
	}
	if cfg2.BeadFilter == nil || cfg2.BeadFilter.Label != "team:{codename}" {
		t.Errorf("--force non-interactive must preserve bead_filter; got %+v", cfg2.BeadFilter)
	}
}

// Regression for A:F3: a 12-bead store with 3 prefixes and matching codenames
// fires detection and writes a bead_filter on a fresh non-interactive init.
func TestInit_DetectBeadFilter_FiresAboveThreshold(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	gitRepo := testutil.SetupGitRepo(t)
	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	// Bootstrap an empty project so the works directory exists; stub br
	// returns no beads on this first pass.
	stubBr(t, `[]`)
	captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("first init: %v", err)
		}
	})

	pidData, _ := os.ReadFile(filepath.Join(gitRepo, ".kerf", "project-identifier"))
	projectID := strings.TrimSpace(string(pidData))
	worksDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	// Seed codenames matching the dominant prefix's tail values.
	for _, cn := range []string{"foo", "bar", "baz", "qux"} {
		if err := os.MkdirAll(filepath.Join(worksDir, cn), 0o755); err != nil {
			t.Fatalf("seed work %s: %v", cn, err)
		}
	}

	// 12 beads across 3 prefixes:
	//   5 work:* (all match codenames) — score 1.0
	//   4 epic:* (none match)           — score 0.0
	//   3 subsystem:* (none match)      — score 0.0
	// Dominant should be "work" via the 0.5 match-score threshold.
	stubBr(t, `[
		{"id":"b-1","labels":["work:foo"]},
		{"id":"b-2","labels":["work:foo"]},
		{"id":"b-3","labels":["work:bar"]},
		{"id":"b-4","labels":["work:baz"]},
		{"id":"b-5","labels":["work:qux"]},
		{"id":"b-6","labels":["epic:alpha"]},
		{"id":"b-7","labels":["epic:beta"]},
		{"id":"b-8","labels":["epic:gamma"]},
		{"id":"b-9","labels":["epic:delta"]},
		{"id":"b-10","labels":["subsystem:auth"]},
		{"id":"b-11","labels":["subsystem:db"]},
		{"id":"b-12","labels":["subsystem:api"]}
	]`)

	origStdin := os.Stdin
	os.Stdin = nonTTYStdin(t)
	defer func() { os.Stdin = origStdin }()

	initForceFlag = true
	defer func() { initForceFlag = false }()
	out := captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("force re-init: %v", err)
		}
	})

	if !strings.Contains(out, "Detected") {
		t.Errorf("expected 'Detected' detection line in output; got %q", out)
	}

	projCfgPath := config.ProjectConfigPath(filepath.Join(tmp, ".kerf"), projectID)
	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	if cfg.BeadFilter == nil {
		t.Fatalf("expected bead_filter to be set, got nil; output was:\n%s", out)
	}
	if cfg.BeadFilter.Label != "work:{codename}" {
		t.Errorf("expected work:{codename}, got %q", cfg.BeadFilter.Label)
	}
}
