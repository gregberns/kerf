package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/kerf/internal/testutil"
)

func TestPreview_RendersFuturePassWithoutAdvancing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	bp := filepath.Join(tmp, ".kerf")
	proj := "preview-test-proj"

	// Create a spec-jig work; default status is the first value (problem-space).
	out := captureOutput(t, func() {
		projectFlag = proj
		newJigFlag = "spec"
		newTitle = "Preview demo"
		defer func() { projectFlag = ""; newJigFlag = ""; newTitle = "" }()
		_ = newCmd.RunE(newCmd, []string{"preview-demo"})
	})
	testutil.AssertStringContains(t, out, "Work created: preview-demo")

	// Capture pre-preview spec.yaml mtime/content marker.
	specPath := filepath.Join(bp, "projects", proj, "preview-demo", "spec.yaml")
	preBytes := readFileForTest(t, specPath)

	// Preview a non-current pass.
	out = captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		if err := previewCmd.RunE(previewCmd, []string{"preview-demo", "research"}); err != nil {
			t.Fatalf("preview: %v", err)
		}
	})

	if !strings.Contains(out, "PREVIEW (read-only)") {
		t.Errorf("preview output missing read-only header. got:\n%s", out)
	}
	if !strings.Contains(out, "Preview for preview-demo") {
		t.Errorf("preview output missing codename header. got:\n%s", out)
	}
	if !strings.Contains(out, "Output:") {
		t.Errorf("preview output missing Output: line. got:\n%s", out)
	}

	// spec.yaml must be unchanged — preview is read-only.
	postBytes := readFileForTest(t, specPath)
	if string(preBytes) != string(postBytes) {
		t.Errorf("spec.yaml changed during preview; preview must be read-only")
	}
}

func TestPreview_UnknownStatusErrors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	proj := "preview-err-proj"

	captureOutput(t, func() {
		projectFlag = proj
		newJigFlag = "spec"
		newTitle = "Preview error"
		defer func() { projectFlag = ""; newJigFlag = ""; newTitle = "" }()
		_ = newCmd.RunE(newCmd, []string{"preview-err"})
	})

	var err error
	captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		err = previewCmd.RunE(previewCmd, []string{"preview-err", "not-a-status"})
	})
	if err == nil {
		t.Fatal("expected error for unknown status, got nil")
	}
	if !strings.Contains(err.Error(), "not declared in jig") {
		t.Errorf("expected error to mention 'not declared in jig', got: %v", err)
	}
}

func TestPreview_UnknownWorkErrors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	proj := "preview-nowork-proj"

	var err error
	captureOutput(t, func() {
		projectFlag = proj
		defer func() { projectFlag = "" }()
		err = previewCmd.RunE(previewCmd, []string{"ghost", "research"})
	})
	if err == nil {
		t.Fatal("expected error for unknown work, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to mention 'not found', got: %v", err)
	}
}

func readFileForTest(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
