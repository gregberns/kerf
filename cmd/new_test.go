package cmd

import (
	"os"
	"path/filepath"
	"strings"
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

func TestNewCommand_BeadFilter_Valid(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		newBeadFilter = "label=subsystem:bridge"
		defer func() { projectFlag = ""; newJigFlag = ""; newBeadFilter = "" }()
		if err := newCmd.RunE(newCmd, []string{"bf-work"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "bf-work", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if s.BeadFilter == nil {
		t.Fatal("expected BeadFilter to be set, got nil")
	}
	if s.BeadFilter.Label != "subsystem:bridge" {
		t.Errorf("BeadFilter.Label = %q, want %q", s.BeadFilter.Label, "subsystem:bridge")
	}
	if s.BeadFilter.IDPrefix != "" {
		t.Errorf("BeadFilter.IDPrefix = %q, want empty", s.BeadFilter.IDPrefix)
	}
	if s.PinnedBeads == nil {
		t.Error("expected PinnedBeads to be non-nil empty slice")
	}
	if len(s.PinnedBeads) != 0 {
		t.Errorf("PinnedBeads length = %d, want 0", len(s.PinnedBeads))
	}

	// Verify raw YAML renders bead_filter and pinned_beads: [].
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading raw spec.yaml: %v", err)
	}
	rawStr := string(raw)
	testutil.AssertStringContains(t, rawStr, "bead_filter:")
	testutil.AssertStringContains(t, rawStr, "subsystem:bridge")
	testutil.AssertStringContains(t, rawStr, "pinned_beads: []")
}

func TestNewCommand_BeadFilter_IDPrefix(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		newBeadFilter = "id_prefix=hk-cb-"
		defer func() { projectFlag = ""; newJigFlag = ""; newBeadFilter = "" }()
		if err := newCmd.RunE(newCmd, []string{"bf-id"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "bf-id", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if s.BeadFilter == nil || s.BeadFilter.IDPrefix != "hk-cb-" {
		t.Errorf("BeadFilter.IDPrefix = %v, want %q", s.BeadFilter, "hk-cb-")
	}
}

func TestNewCommand_BeadFilter_Invalid(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	err := func() error {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		newBeadFilter = "not-a-clause"
		defer func() { projectFlag = ""; newJigFlag = ""; newBeadFilter = "" }()
		return newCmd.RunE(newCmd, []string{"bf-bad"})
	}()

	if err == nil {
		t.Fatal("expected error for invalid --bead-filter value")
	}
	testutil.AssertStringContains(t, err.Error(), "--bead-filter")

	// Ensure no work directory was created.
	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "bf-bad", "spec.yaml")
	if _, statErr := os.Stat(specPath); statErr == nil {
		t.Errorf("spec.yaml should not have been created on invalid --bead-filter")
	}
}

func TestNewCommand_BeadFilter_Absent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		newBeadFilter = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		if err := newCmd.RunE(newCmd, []string{"bf-absent"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "bf-absent", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if s.BeadFilter != nil {
		t.Errorf("expected BeadFilter to be nil (parses as absent) when --bead-filter absent, got %+v", s.BeadFilter)
	}

	// Per Plan 019 (kerf-3ac): bead_filter key is always emitted by
	// `kerf new`, with an empty value when no clause is supplied. Absent
	// and present-but-empty resolve identically.
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading raw spec.yaml: %v", err)
	}
	rawStr := string(raw)
	if !strings.Contains(rawStr, "bead_filter:") {
		t.Errorf("expected bead_filter key to be present (always-emit), got:\n%s", rawStr)
	}
	// pinned_beads: [] must always render.
	testutil.AssertStringContains(t, rawStr, "pinned_beads: []")
}

// kerf-r1i: KERF_SESSION_ID env var must be recorded as sessions[0].id so that
// `kerf list --created-by self` can attribute the work to the agent that
// created it. When unset, the session must remain anonymous (sessions[0].id is
// nil, ActiveSession is "anonymous").
func TestNewCommand_RecordsKerfSessionIDFromEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("KERF_SESSION_ID", "alice")

	captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		if err := newCmd.RunE(newCmd, []string{"sid-work"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "sid-work", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if len(s.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(s.Sessions))
	}
	if s.Sessions[0].ID == nil {
		t.Fatalf("expected sessions[0].id to be 'alice', got nil")
	}
	if *s.Sessions[0].ID != "alice" {
		t.Errorf("sessions[0].id = %q, want %q", *s.Sessions[0].ID, "alice")
	}
	if s.ActiveSession == nil || *s.ActiveSession != "alice" {
		t.Errorf("active_session = %v, want 'alice'", s.ActiveSession)
	}
}

func TestNewCommand_AnonymousWhenKerfSessionIDUnset(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Ensure env var is unset for this test, regardless of host env.
	t.Setenv("KERF_SESSION_ID", "")

	captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		defer func() { projectFlag = ""; newJigFlag = "" }()
		if err := newCmd.RunE(newCmd, []string{"anon-work"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "anon-work", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if len(s.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(s.Sessions))
	}
	if s.Sessions[0].ID != nil {
		t.Errorf("expected sessions[0].id to be nil (anonymous), got %q", *s.Sessions[0].ID)
	}
	if s.ActiveSession == nil || *s.ActiveSession != "anonymous" {
		t.Errorf("active_session = %v, want 'anonymous'", s.ActiveSession)
	}
}

// kerf-259: `kerf new <codename>` auto-populates bead_filter from a dominant
// codename label match in the bead store. When >=3 beads carry one of the
// candidate label shapes (e.g. codename:auth), the new work's bead_filter is
// pre-populated. --no-auto-filter bypasses; empty store leaves null.

func TestNewCommand_AutoBeadFilter_DominantMatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	stubBr(t, `[
		{"id":"x-1","status":"open","labels":["codename:auth"]},
		{"id":"x-2","status":"open","labels":["codename:auth"]},
		{"id":"x-3","status":"open","labels":["codename:auth"]},
		{"id":"x-4","status":"open","labels":["codename:auth"]},
		{"id":"x-5","status":"open","labels":["unrelated"]}
	]`)

	captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		newBeadFilter = ""
		newNoAutoFilter = false
		defer func() {
			projectFlag = ""
			newJigFlag = ""
			newNoAutoFilter = false
		}()
		if err := newCmd.RunE(newCmd, []string{"auth"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "auth", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if s.BeadFilter == nil {
		t.Fatal("expected BeadFilter to be auto-populated, got nil")
	}
	if s.BeadFilter.Label != "codename:auth" {
		t.Errorf("BeadFilter.Label = %q, want %q", s.BeadFilter.Label, "codename:auth")
	}
}

func TestNewCommand_AutoBeadFilter_DisabledByFlag(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	stubBr(t, `[
		{"id":"x-1","status":"open","labels":["codename:auth"]},
		{"id":"x-2","status":"open","labels":["codename:auth"]},
		{"id":"x-3","status":"open","labels":["codename:auth"]},
		{"id":"x-4","status":"open","labels":["codename:auth"]}
	]`)

	captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		newBeadFilter = ""
		newNoAutoFilter = true
		defer func() {
			projectFlag = ""
			newJigFlag = ""
			newNoAutoFilter = false
		}()
		if err := newCmd.RunE(newCmd, []string{"auth"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "auth", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if s.BeadFilter != nil {
		t.Errorf("expected BeadFilter to remain nil under --no-auto-filter, got %+v", s.BeadFilter)
	}
}

func TestNewCommand_AutoBeadFilter_EmptyStore(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	stubBr(t, `[]`)

	captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		newBeadFilter = ""
		newNoAutoFilter = false
		defer func() {
			projectFlag = ""
			newJigFlag = ""
			newNoAutoFilter = false
		}()
		if err := newCmd.RunE(newCmd, []string{"auth"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "auth", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if s.BeadFilter != nil {
		t.Errorf("expected BeadFilter to be nil with empty bead store, got %+v", s.BeadFilter)
	}
}

// User-provided --bead-filter wins over the auto-detector.
func TestNewCommand_AutoBeadFilter_ExplicitFilterWins(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	stubBr(t, `[
		{"id":"x-1","status":"open","labels":["codename:auth"]},
		{"id":"x-2","status":"open","labels":["codename:auth"]},
		{"id":"x-3","status":"open","labels":["codename:auth"]},
		{"id":"x-4","status":"open","labels":["codename:auth"]}
	]`)

	captureOutput(t, func() {
		projectFlag = "proj"
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		newBeadFilter = "label=subsystem:bridge"
		newNoAutoFilter = false
		defer func() {
			projectFlag = ""
			newJigFlag = ""
			newBeadFilter = ""
			newNoAutoFilter = false
		}()
		if err := newCmd.RunE(newCmd, []string{"auth"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	bp := filepath.Join(tmp, ".kerf")
	specPath := filepath.Join(bp, "projects", "proj", "auth", "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	if s.BeadFilter == nil || s.BeadFilter.Label != "subsystem:bridge" {
		t.Errorf("expected explicit --bead-filter to win, got %+v", s.BeadFilter)
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

// kerf-vu0r: a corrupt .kerf/project-identifier must surface non-zero from
// `kerf new` rather than silently falling through to derive-and-overwrite,
// which would route the new work at the wrong project (or clobber the
// damaged file with a fresh derived ID). Mirrors kerf-dlb's test for `kerf
// init`. The fall-through "no file exists" path is exercised by
// TestNewCommand_AutoCodename and friends.
func TestNewCommand_CorruptProjectIdentifier_Errors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)

	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatal(err)
	}
	idPath := filepath.Join(repo, ".kerf", "project-identifier")
	garbage := []byte("bad/\x00id\n")
	if err := os.WriteFile(idPath, garbage, 0o644); err != nil {
		t.Fatal(err)
	}

	err := func() error {
		// Reset flags so we exercise the identifier path (not --project override).
		projectFlag = ""
		newJigFlag = "plan"
		newTitle = ""
		newType = ""
		defer func() { newJigFlag = "" }()
		return captureErr(func() error {
			return newCmd.RunE(newCmd, []string{"vu0r-test"})
		})
	}()
	if err == nil {
		t.Fatal("expected error when project-identifier is corrupt, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"corrupt project identifier", idPath, "replace with a clean slug"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}

	// On-disk bytes must be untouched.
	after, readErr := os.ReadFile(idPath)
	if readErr != nil {
		t.Fatalf("reading project-identifier after refused new: %v", readErr)
	}
	if string(after) != string(garbage) {
		t.Errorf("corrupt project-identifier was modified; want %q got %q", garbage, after)
	}
}
