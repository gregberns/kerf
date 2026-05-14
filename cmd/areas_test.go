package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/areas"
	"github.com/gberns/kerf/internal/testutil"
)

// setupAreasTest creates a temp bench with a project and sets HOME so that
// bench.BenchPath() resolves to the temp directory. Returns the bench path
// and project ID.
func setupAreasTest(t *testing.T, projectID string) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", projectID)
	os.MkdirAll(projDir, 0755)

	// Set projectFlag so cmdutil.ResolveProject works without a git repo.
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	return benchDir
}

func TestAreasInit_CreatesFile(t *testing.T) {
	benchDir := setupAreasTest(t, "init-proj")

	out := captureOutput(t, func() {
		areasInitCmd.RunE(areasInitCmd, []string{})
	})

	areasPath := areas.AreasPath(benchDir, "init-proj")
	testutil.AssertFileExists(t, areasPath)
	testutil.AssertStringContains(t, out, "Created areas.yaml")
	testutil.AssertStringContains(t, out, areasPath)

	data, err := os.ReadFile(areasPath)
	if err != nil {
		t.Fatalf("reading areas.yaml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# areas.yaml") {
		t.Errorf("expected comment header in file, got:\n%s", content)
	}
	if !strings.Contains(content, "kerf areas add") {
		t.Errorf("expected usage hint in comment header, got:\n%s", content)
	}

	af, err := areas.Load(areasPath)
	if err != nil {
		t.Fatalf("loading areas.yaml: %v", err)
	}
	if len(af.Areas) != 0 {
		t.Errorf("expected empty areas map, got %d entries", len(af.Areas))
	}
}

func TestAreasInit_WarnsIfExists(t *testing.T) {
	benchDir := setupAreasTest(t, "init-existing")

	areasPath := areas.AreasPath(benchDir, "init-existing")
	original := "areas:\n  preexisting:\n    description: do not clobber\n"
	if err := os.MkdirAll(filepath.Dir(areasPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(areasPath, []byte(original), 0o644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}

	out := captureOutput(t, func() {
		if err := areasInitCmd.RunE(areasInitCmd, []string{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	testutil.AssertStringContains(t, out, "already exists")
	testutil.AssertStringContains(t, out, areasPath)

	data, err := os.ReadFile(areasPath)
	if err != nil {
		t.Fatalf("reading areas.yaml: %v", err)
	}
	if string(data) != original {
		t.Errorf("file was modified; expected %q, got %q", original, string(data))
	}
}

func TestAreasAddAndList(t *testing.T) {
	benchDir := setupAreasTest(t, "test-proj")

	// Add an area.
	out := captureOutput(t, func() {
		areasAddDescription = "Authentication and session management"
		areasAddCmd.RunE(areasAddCmd, []string{"auth"})
	})
	testutil.AssertStringContains(t, out, "Area 'auth' added to project 'test-proj'.")

	// Add another area.
	captureOutput(t, func() {
		areasAddDescription = "Public API surface"
		areasAddCmd.RunE(areasAddCmd, []string{"api"})
	})

	// Verify areas.yaml was written.
	areasPath := areas.AreasPath(benchDir, "test-proj")
	testutil.AssertFileExists(t, areasPath)

	// List should show both areas.
	out = captureOutput(t, func() {
		areasListCmd.RunE(areasListCmd, []string{})
	})
	testutil.AssertStringContains(t, out, "Areas for test-proj:")
	testutil.AssertStringContains(t, out, "api")
	testutil.AssertStringContains(t, out, "auth")
	testutil.AssertStringContains(t, out, "0 works")
	testutil.AssertStringContains(t, out, "kerf areas add")
	testutil.AssertStringContains(t, out, "kerf areas remove")
}

func TestAreasListEmpty(t *testing.T) {
	_ = setupAreasTest(t, "empty-proj")

	out := captureOutput(t, func() {
		areasListCmd.RunE(areasListCmd, []string{})
	})
	testutil.AssertStringContains(t, out, "No areas defined")
	testutil.AssertStringContains(t, out, "kerf areas add")
}

func TestAreasAddDuplicate(t *testing.T) {
	_ = setupAreasTest(t, "dup-proj")

	// Add an area.
	captureOutput(t, func() {
		areasAddDescription = "First"
		areasAddCmd.RunE(areasAddCmd, []string{"auth"})
	})

	// Add the same area again — should error.
	err := areasAddCmd.RunE(areasAddCmd, []string{"auth"})
	if err == nil {
		t.Fatal("expected error when adding duplicate area")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %s", err.Error())
	}
}

func TestAreasAddInvalidName(t *testing.T) {
	_ = setupAreasTest(t, "inv-proj")

	tests := []struct {
		name string
	}{
		{"UPPER"},
		{"has spaces"},
		{"-leading-hyphen"},
		{"trailing-hyphen-"},
		{"special!chars"},
	}

	for _, tc := range tests {
		areasAddDescription = "desc"
		err := areasAddCmd.RunE(areasAddCmd, []string{tc.name})
		if err == nil {
			t.Errorf("expected error for invalid name %q", tc.name)
		}
	}
}

func TestAreasRemove(t *testing.T) {
	_ = setupAreasTest(t, "rm-proj")

	// Add an area.
	captureOutput(t, func() {
		areasAddDescription = "To be removed"
		areasAddCmd.RunE(areasAddCmd, []string{"temp"})
	})

	// Remove it.
	out := captureOutput(t, func() {
		areasRemoveCmd.RunE(areasRemoveCmd, []string{"temp"})
	})
	testutil.AssertStringContains(t, out, "Area 'temp' removed from project 'rm-proj'.")

	// List should be empty now.
	out = captureOutput(t, func() {
		areasListCmd.RunE(areasListCmd, []string{})
	})
	testutil.AssertStringContains(t, out, "No areas defined")
}

func TestAreasRemoveNotFound(t *testing.T) {
	_ = setupAreasTest(t, "nf-proj")

	err := areasRemoveCmd.RunE(areasRemoveCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error when removing nonexistent area")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %s", err.Error())
	}
}

func TestAreasListWithWorkCounts(t *testing.T) {
	benchDir := setupAreasTest(t, "count-proj")

	// Add areas.
	captureOutput(t, func() {
		areasAddDescription = "Auth layer"
		areasAddCmd.RunE(areasAddCmd, []string{"auth"})
	})
	captureOutput(t, func() {
		areasAddDescription = "API layer"
		areasAddCmd.RunE(areasAddCmd, []string{"api"})
	})

	// Create a work that references the "auth" area.
	workDir := filepath.Join(benchDir, "projects", "count-proj", "blue-bear")
	os.MkdirAll(workDir, 0755)
	specContent := `codename: blue-bear
type: plan
project:
  id: count-proj
jig: plan
jig_version: 1
status: problem-space
areas:
  - auth
`
	os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(specContent), 0644)

	// List should show work count for auth.
	out := captureOutput(t, func() {
		areasListCmd.RunE(areasListCmd, []string{})
	})
	testutil.AssertStringContains(t, out, "auth")
	testutil.AssertStringContains(t, out, "1 work")
	testutil.AssertStringContains(t, out, "api")
	testutil.AssertStringContains(t, out, "0 works")
}

func TestAreasRemoveWithWorkWarning(t *testing.T) {
	benchDir := setupAreasTest(t, "warn-proj")

	// Add an area.
	captureOutput(t, func() {
		areasAddDescription = "Auth layer"
		areasAddCmd.RunE(areasAddCmd, []string{"auth"})
	})

	// Create a work that references the area.
	workDir := filepath.Join(benchDir, "projects", "warn-proj", "red-wave")
	os.MkdirAll(workDir, 0755)
	specContent := `codename: red-wave
type: plan
project:
  id: warn-proj
jig: plan
jig_version: 1
status: problem-space
areas:
  - auth
`
	os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(specContent), 0644)

	// Remove should warn but succeed.
	out := captureOutput(t, func() {
		areasRemoveCmd.RunE(areasRemoveCmd, []string{"auth"})
	})
	testutil.AssertStringContains(t, out, "Warning:")
	testutil.AssertStringContains(t, out, "red-wave")
	testutil.AssertStringContains(t, out, "Area 'auth' removed from project 'warn-proj'.")
}
