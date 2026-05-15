package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

func TestE2E_CoordinationFlow(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	projectID := "coord-proj"

	projectFlag = projectID
	t.Cleanup(func() { projectFlag = "" })

	// 1. Add areas via `kerf areas add`.
	out := captureOutput(t, func() {
		areasAddDescription = "Public API surface"
		defer func() { areasAddDescription = "" }()
		areasAddCmd.RunE(areasAddCmd, []string{"api"})
	})
	testutil.AssertStringContains(t, out, "Area 'api' added")

	out = captureOutput(t, func() {
		areasAddDescription = "Authentication"
		defer func() { areasAddDescription = "" }()
		areasAddCmd.RunE(areasAddCmd, []string{"auth"})
	})
	testutil.AssertStringContains(t, out, "Area 'auth' added")

	// 2. Create work A via `kerf new` with --area api.
	out = captureOutput(t, func() {
		newJigFlag = "plan"
		newAreaFlag = []string{"api"}
		newTitle = "First API work"
		newType = ""
		defer func() {
			newJigFlag = ""
			newAreaFlag = nil
			newTitle = ""
		}()
		newCmd.RunE(newCmd, []string{"work-a"})
	})
	testutil.AssertStringContains(t, out, "Work created: work-a")
	if strings.Contains(out, "Area overlap:") {
		t.Errorf("work-a should not emit area overlap warning (no prior work in api), got:\n%s", out)
	}

	// 3. Create work B via `kerf new` with --area api — should emit overlap warning.
	out = captureOutput(t, func() {
		newJigFlag = "plan"
		newAreaFlag = []string{"api"}
		newTitle = "Second API work"
		newType = ""
		defer func() {
			newJigFlag = ""
			newAreaFlag = nil
			newTitle = ""
		}()
		newCmd.RunE(newCmd, []string{"work-b"})
	})
	testutil.AssertStringContains(t, out, "Work created: work-b")
	testutil.AssertStringContains(t, out, "Area overlap:")
	testutil.AssertStringContains(t, out, "api")
	testutil.AssertStringContains(t, out, "work-a")
	testutil.AssertStringContains(t, out, "also touched by:")

	// 4. `kerf map` groups works under area headers.
	out = captureOutput(t, func() {
		mapCmd.RunE(mapCmd, []string{})
	})
	testutil.AssertStringContains(t, out, "Map for "+projectID)
	testutil.AssertStringContains(t, out, "api:")
	testutil.AssertStringContains(t, out, "auth:")
	testutil.AssertStringContains(t, out, "work-a")
	testutil.AssertStringContains(t, out, "work-b")

	apiIdx := strings.Index(out, "api:")
	authIdx := strings.Index(out, "auth:")
	workAIdx := strings.Index(out, "work-a")
	workBIdx := strings.Index(out, "work-b")
	if apiIdx < 0 || workAIdx < apiIdx {
		t.Errorf("work-a should appear after 'api:' header in map output")
	}
	if workBIdx < apiIdx {
		t.Errorf("work-b should appear after 'api:' header in map output")
	}
	if authIdx > 0 && workAIdx > authIdx && workBIdx > authIdx {
		t.Errorf("works should be under api:, not after auth:")
	}

	// 5. `kerf next` returns an item feed mentioning both works. With no
	// beads in the store and no warnings, the v1 cleanup detector emits one
	// "no attached beads" item per active work. Without beads we may instead
	// see the empty-feed line — accept either shape.
	resetNextFlags()
	defer resetNextFlags()
	var nbuf bytes.Buffer
	nextCmd.SetOut(&nbuf)
	defer nextCmd.SetOut(nil)
	if err := runNext(nextCmd); err != nil {
		t.Fatalf("kerf next: %v", err)
	}
	out = nbuf.String()
	if !strings.Contains(out, "work-a") && !strings.Contains(out, "work-b") &&
		!strings.Contains(out, nextEmptyText) {
		t.Errorf("kerf next output should mention work-a/work-b or empty-feed line; got:\n%s", out)
	}

	// 6. `kerf show work-b` prints Areas: line and Area overlap: section listing work-a.
	out = captureOutput(t, func() {
		showCmd.RunE(showCmd, []string{"work-b"})
	})
	testutil.AssertStringContains(t, out, "Work: work-b")
	testutil.AssertStringContains(t, out, "Areas: api")
	testutil.AssertStringContains(t, out, "Area overlap:")
	testutil.AssertStringContains(t, out, "work-a")

	// Sanity: areas file lives at the documented path.
	areasPath := filepath.Join(tmp, ".kerf", "projects", projectID, "areas.yaml")
	testutil.AssertFileExists(t, areasPath)
}
