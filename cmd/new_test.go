package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/testutil"
)

func TestNewCommand_AutoCodename(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a git repo to resolve project from.
	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		newCmd.RunE(newCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Work created:")
	testutil.AssertStringContains(t, out, "Project:  test-proj")
	testutil.AssertStringContains(t, out, "Jig:      plan")
	testutil.AssertStringContains(t, out, "Process overview")
	testutil.AssertStringContains(t, out, "Next steps:")
}

func TestNewCommand_UserCodename(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		newJigFlag = "plan"
		newTitle = "My Feature"
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = ""; newTitle = "" }()
		newCmd.RunE(newCmd, []string{"my-feature"})
	})

	testutil.AssertStringContains(t, out, "Work created: my-feature")
	testutil.AssertStringContains(t, out, "Project:  test-proj")

	// Verify spec.yaml was created.
	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "test-proj", "my-feature", "spec.yaml")
	testutil.AssertFileExists(t, specPath)

	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if s.Codename != "my-feature" {
		t.Errorf("codename = %q, want %q", s.Codename, "my-feature")
	}
	if s.Title == nil || *s.Title != "My Feature" {
		t.Errorf("title = %v, want %q", s.Title, "My Feature")
	}
	if s.Status != "problem-space" {
		t.Errorf("status = %q, want %q", s.Status, "problem-space")
	}
	if len(s.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(s.Sessions))
	}
	if s.ActiveSession == nil {
		t.Error("expected active_session to be set")
	}
}

func TestNewCommand_DuplicateCodename(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create existing work.
	bp := filepath.Join(tmp, ".kerf")
	os.MkdirAll(filepath.Join(bp, "projects", "proj", "existing"), 0755)
	writeMinimalSpec(t,
		filepath.Join(bp, "projects", "proj", "existing", "spec.yaml"),
		"existing", "proj")

	err := func() error {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		return newCmd.RunE(newCmd, []string{"existing"})
	}()

	if err == nil {
		t.Error("expected error for duplicate codename")
	} else {
		testutil.AssertStringContains(t, err.Error(), "already exists")
	}
}

func TestNewCommand_InvalidCodename(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	err := func() error {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		return newCmd.RunE(newCmd, []string{"INVALID_NAME"})
	}()

	if err == nil {
		t.Error("expected error for invalid codename")
	} else {
		testutil.AssertStringContains(t, err.Error(), "codename must be lowercase")
	}
}

func TestNewCommand_JigNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	err := func() error {
		projectFlag = "proj"
		newJigFlag = "nonexistent"
		newTitle = ""
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		return newCmd.RunE(newCmd, []string{"test-work"})
	}()

	if err == nil {
		t.Error("expected error for nonexistent jig")
	} else {
		testutil.AssertStringContains(t, err.Error(), "not found")
	}
}

func TestNewCommand_NoRepoNoProject(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Chdir(tmp) // Not a git repo

	err := func() error {
		projectFlag = ""
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		defer func() { newJigFlag = "" }()
		return newCmd.RunE(newCmd, []string{"test-work"})
	}()

	if err == nil {
		t.Error("expected error when not in git repo and no --project")
	} else {
		testutil.AssertStringContains(t, err.Error(), "not in a git repository")
	}
}

func TestNewCommand_BugJig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "bug"
		newTitle = ""
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		newCmd.RunE(newCmd, []string{"fix-login"})
	})

	testutil.AssertStringContains(t, out, "Jig:      bug")

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "fix-login", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if s.Jig != "bug" {
		t.Errorf("jig = %q, want %q", s.Jig, "bug")
	}
	if s.Type != "bug" {
		t.Errorf("type = %q, want %q (defaults to jig name)", s.Type, "bug")
	}
}

func TestNewCommand_SnapshotCreated(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		newCmd.RunE(newCmd, []string{"snap-test"})
	})

	bp := filepath.Join(tmp, ".kerf")
	histDir := filepath.Join(bp, "projects", "proj", "snap-test", ".history")
	entries, err := os.ReadDir(histDir)
	if err != nil {
		t.Fatalf("reading .history: %v", err)
	}
	if len(entries) < 1 {
		t.Error("expected at least one snapshot after kerf new")
	}
}

func TestNewCommand_FirstUseProjectDerivation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	out := captureOutput(t, func() {
		projectFlag = ""
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		defer func() { newJigFlag = "" }()
		newCmd.RunE(newCmd, []string{"derive-test"})
	})

	testutil.AssertStringContains(t, out, "Project ID derived:")

	// Verify .kerf/project-identifier was written.
	testutil.AssertFileExists(t, filepath.Join(repo, ".kerf", "project-identifier"))
}

// --- Onboarding and canonical name tests ---

func TestNewCommand_OnboardingError_NoConfigNoJig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	err := func() error {
		projectFlag = "proj"
		newJigFlag = ""
		newTitle = ""
		newType = ""
		defer func() { projectFlag = "" }()
		return newCmd.RunE(newCmd, []string{"test-work"})
	}()

	if err == nil {
		t.Fatal("expected onboarding error when no config and no --jig flag")
	}
	testutil.AssertStringContains(t, err.Error(), "No default workflow configured")
	testutil.AssertStringContains(t, err.Error(), "kerf config default_jig plan")
	testutil.AssertStringContains(t, err.Error(), "kerf config default_jig spec")
}

func TestNewCommand_OnboardingError_NoConfigWithJig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		newCmd.RunE(newCmd, []string{"with-jig"})
	})

	testutil.AssertStringContains(t, out, "Work created: with-jig")

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "with-jig", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if s.Jig != "plan" {
		t.Errorf("jig = %q, want %q", s.Jig, "plan")
	}
}

func TestNewCommand_CanonicalName_FeatureAlias(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	out := captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "feature"
		newTitle = ""
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		newCmd.RunE(newCmd, []string{"alias-test"})
	})

	// Output should show canonical name "plan", not alias "feature".
	testutil.AssertStringContains(t, out, "Jig:      plan")

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "alias-test", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if s.Jig != "plan" {
		t.Errorf("jig = %q, want %q (canonical name, not alias)", s.Jig, "plan")
	}
	if s.Type != "plan" {
		t.Errorf("type = %q, want %q (canonical name, not alias)", s.Type, "plan")
	}
}

func TestNewCommand_ConfigDefaultJig_NoFlag(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create config with default_jig set.
	bp := filepath.Join(tmp, ".kerf")
	os.MkdirAll(bp, 0755)
	cfg := &config.Config{DefaultJig: "plan"}
	if err := config.Save(filepath.Join(bp, "config.yaml"), cfg); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	out := captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = ""
		newTitle = ""
		newType = ""
		defer func() { projectFlag = "" }()
		newCmd.RunE(newCmd, []string{"config-test"})
	})

	testutil.AssertStringContains(t, out, "Work created: config-test")
	testutil.AssertStringContains(t, out, "Jig:      plan")
}
