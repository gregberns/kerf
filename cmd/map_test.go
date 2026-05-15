package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gberns/kerf/internal/testutil"
)

func TestMapCommand_NoWorks(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	os.MkdirAll(filepath.Join(benchDir, "projects", "test-proj"), 0755)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		mapCmd.RunE(mapCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "No works found")
	testutil.AssertStringContains(t, out, "kerf new")
}

func TestMapCommand_WorksGroupedByArea(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")

	// Create areas.yaml.
	areasContent := `areas:
  api:
    description: "HTTP interface"
  database:
    description: "Persistence layer"
  core:
    description: "Domain logic"
`
	os.MkdirAll(filepath.Join(benchDir, "projects", "test-proj"), 0755)
	os.WriteFile(filepath.Join(benchDir, "projects", "test-proj", "areas.yaml"),
		[]byte(areasContent), 0644)

	// Create works — one touches api+database, one touches api only, one has no areas.
	writeSpecWithAreas(t,
		filepath.Join(projDir, "blue-fox", "spec.yaml"),
		"blue-fox", "test-proj", "research", "Auth rewrite", []string{"api", "database"})
	writeSpecWithAreas(t,
		filepath.Join(projDir, "red-elk", "spec.yaml"),
		"red-elk", "test-proj", "implementing", "Rate limiting", []string{"api"})
	writeSpecWithAreas(t,
		filepath.Join(projDir, "green-owl", "spec.yaml"),
		"green-owl", "test-proj", "spec", "Logging cleanup", nil)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		mapCmd.RunE(mapCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Map for test-proj")

	// api area should contain both blue-fox and red-elk.
	testutil.AssertStringContains(t, out, "api:")
	testutil.AssertStringContains(t, out, "blue-fox")
	testutil.AssertStringContains(t, out, "red-elk")

	// database area should contain blue-fox.
	testutil.AssertStringContains(t, out, "database:")

	// core area should show no active work.
	testutil.AssertStringContains(t, out, "core:")
	testutil.AssertStringContains(t, out, "no active work")

	// green-owl should be unassigned.
	testutil.AssertStringContains(t, out, "unassigned:")
	testutil.AssertStringContains(t, out, "green-owl")

	// Commands footer.
	testutil.AssertStringContains(t, out, "Commands:")
	testutil.AssertStringContains(t, out, "kerf show")
}

func TestMapCommand_NoAreas(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")
	os.MkdirAll(projDir, 0755)

	// No areas.yaml — all works should be unassigned.
	writeSpecWithAreas(t,
		filepath.Join(projDir, "blue-fox", "spec.yaml"),
		"blue-fox", "test-proj", "research", "Auth rewrite", nil)

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		mapCmd.RunE(mapCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Map for test-proj")
	testutil.AssertStringContains(t, out, "unassigned:")
	testutil.AssertStringContains(t, out, "blue-fox")
}

func TestMapCommand_Dependencies(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", "test-proj")
	os.MkdirAll(projDir, 0755)

	// Create two works with a dependency.
	writeSpecWithAreas(t,
		filepath.Join(projDir, "alpha", "spec.yaml"),
		"alpha", "test-proj", "research", "First work", nil)
	writeSpecWithDep(t,
		filepath.Join(projDir, "beta", "spec.yaml"),
		"beta", "test-proj", "spec", "Second work", "alpha")

	out := captureOutput(t, func() {
		projectFlag = "test-proj"
		defer func() { projectFlag = "" }()
		mapCmd.RunE(mapCmd, []string{})
	})

	testutil.AssertStringContains(t, out, "Dependencies:")
	testutil.AssertStringContains(t, out, "beta -> alpha [research]")
}

// TestMap_CaseSensitiveLabelMatching verifies that `kerf map` attaches
// beads to works via the spec-conformant case-sensitive matcher
// (Plan 008 / B5; specs/coordination.md L232). A project bead_filter of
// `subsystem:{codename}` must NOT match a label `Subsystem:bridge` — only
// the exact-case `subsystem:bridge`. The rendered map line for the work
// should show `1/2 beads`, not the case-insensitive count.
func TestMap_CaseSensitiveLabelMatching(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	projectID := "case-map-proj"
	benchDir := filepath.Join(tmp, ".kerf")
	projDir := filepath.Join(benchDir, "projects", projectID)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	// Project-level bead_filter — case-sensitive matching is the
	// spec-conformant rule (specs/coordination.md L232).
	projectYAML := []byte("bead_filter:\n  label: \"subsystem:{codename}\"\n")
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), projectYAML, 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}

	// One work; one bead with the exact-case label (open), one with the
	// mixed-case label that must be rejected, one closed exact-case bead.
	writeSpecWithAreas(t,
		filepath.Join(projDir, "bridge", "spec.yaml"),
		"bridge", projectID, "research", "Span the chasm", nil)

	stubBr(t, `[
		{"id":"b-match-open","status":"open","labels":["subsystem:bridge"]},
		{"id":"b-match-closed","status":"closed","labels":["subsystem:bridge"]},
		{"id":"b-case","status":"open","labels":["Subsystem:bridge"]}
	]`)

	out := captureOutput(t, func() {
		projectFlag = projectID
		defer func() { projectFlag = "" }()
		mapCmd.RunE(mapCmd, []string{})
	})

	// Exact-case matching: 2 beads attach (1 done, 1 open). The
	// mixed-case label is rejected.
	testutil.AssertStringContains(t, out, "1/2 beads")
	if containsString(out, "1/3 beads") {
		t.Errorf("map rendered 1/3 beads — case-insensitive match leaked through; expected 1/2.\nOutput:\n%s", out)
	}
}

// writeSpecWithAreas creates a minimal spec.yaml with areas and title.
func writeSpecWithAreas(t *testing.T, path, codename, projectID, status, title string, areaList []string) {
	t.Helper()
	areasYAML := "areas: []\n"
	if len(areaList) > 0 {
		areasYAML = "areas:\n"
		for _, a := range areaList {
			areasYAML += "  - " + a + "\n"
		}
	}
	titleYAML := ""
	if title != "" {
		titleYAML = "title: " + title + "\n"
	}
	content := `codename: ` + codename + `
` + titleYAML + `type: plan
project:
  id: ` + projectID + `
jig: plan
jig_version: 1
status: ` + status + `
status_values: [problem-space, research, spec, implementing, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on: []
` + areasYAML + `implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeSpecWithAreas: %v", err)
	}
	// Ensure the enclosing project has a project.yaml (post-Plan 008 /
	// B10-code; specs/commands.md §"Warning kinds" → `no_project_yaml`).
	projectYAML := filepath.Join(filepath.Dir(filepath.Dir(path)), "project.yaml")
	if _, err := os.Stat(projectYAML); os.IsNotExist(err) {
		if werr := os.WriteFile(projectYAML, []byte("jigs: []\n"), 0644); werr != nil {
			t.Fatalf("write project.yaml: %v", werr)
		}
	}
}

// writeSpecWithDep creates a minimal spec.yaml with a dependency.
func writeSpecWithDep(t *testing.T, path, codename, projectID, status, title, depCodename string) {
	t.Helper()
	content := `codename: ` + codename + `
title: ` + title + `
type: plan
project:
  id: ` + projectID + `
jig: plan
jig_version: 1
status: ` + status + `
status_values: [problem-space, research, spec, implementing, ready]
created: 2026-04-09T00:00:00Z
updated: 2026-04-09T00:00:00Z
sessions: []
depends_on:
  - codename: ` + depCodename + `
    relationship: must-complete-first
areas: []
implementation:
  branch: null
  pr: null
  commits: []
`
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeSpecWithDep: %v", err)
	}
}
