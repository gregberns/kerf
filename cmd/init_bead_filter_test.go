package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/kerf/internal/beads"
	"github.com/gregberns/kerf/internal/config"
	"github.com/gregberns/kerf/internal/storage"
	"github.com/gregberns/kerf/internal/testutil"
)

// stubBr installs a fake `br` binary on PATH that emits the given JSON when
// invoked. Returns the dir holding the stub.
func stubBr(t *testing.T, json string) string {
	t.Helper()
	dir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\ncat <<'EOF'\n%s\nEOF\n", json)
	path := filepath.Join(dir, "br")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub br: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func makeResolverWithWorks(t *testing.T, codenames []string) *storage.Resolver {
	t.Helper()
	bench := t.TempDir()
	projectID := "test-project"
	r, err := storage.NewResolver(bench, projectID, "")
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if err := os.MkdirAll(r.WorksDir(), 0o755); err != nil {
		t.Fatalf("works dir: %v", err)
	}
	for _, cn := range codenames {
		if err := os.MkdirAll(r.WorkDir(cn), 0o755); err != nil {
			t.Fatalf("work %s: %v", cn, err)
		}
	}
	return r
}

// --- detectBeadFilter unit tests (non-interactive, tri-state). -------------

// br unavailable → detector returns priorFilter unchanged (nil here).
func TestDetectBeadFilter_BrUnavailable_ReturnsPriorFilter(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	r := makeResolverWithWorks(t, []string{"auth"})
	var out bytes.Buffer
	got := detectBeadFilter(r, beadFilterModeDefault, &out, nil)
	if got != nil {
		t.Errorf("expected nil filter when br unavailable, got %+v", got)
	}
	if out.Len() != 0 {
		t.Errorf("expected silent detector output, got %q", out.String())
	}
}

// No codenames → detector returns nil; the run codename inventory is empty
// so no prefix can match.
func TestDetectBeadFilter_NoCodenames_ReturnsNil(t *testing.T) {
	stubBr(t, `[{"id":"x-1","labels":["subsystem:auth"]}]`)
	r := makeResolverWithWorks(t, nil)
	var out bytes.Buffer
	got := detectBeadFilter(r, beadFilterModeDefault, &out, nil)
	if got != nil {
		t.Errorf("expected nil filter when no codenames exist, got %+v", got)
	}
}

// Empty bead store → ConfidenceNone → detector returns nil silently.
func TestDetectBeadFilter_EmptyStore_ReturnsNilSilently(t *testing.T) {
	stubBr(t, `[]`)
	r := makeResolverWithWorks(t, []string{"auth"})
	var out bytes.Buffer
	got := detectBeadFilter(r, beadFilterModeDefault, &out, nil)
	if got != nil {
		t.Errorf("expected nil filter for empty store, got %+v", got)
	}
	if strings.Contains(out.String(), "Detected") {
		t.Errorf("expected silent output on empty store, got %q", out.String())
	}
}

// ConfidenceConfident corpus → detector writes Detected: line and returns
// the dominant prefix as a Filter.
func TestDetectBeadFilter_ConfidentCandidate_WritesFilter(t *testing.T) {
	stubBr(t, `[
		{"id":"x-1","labels":["subsystem:auth"]},
		{"id":"x-2","labels":["subsystem:db"]},
		{"id":"x-3","labels":["subsystem:api"]},
		{"id":"x-4","labels":["subsystem:ui"]}
	]`)
	r := makeResolverWithWorks(t, []string{"auth", "db", "api", "ui"})
	var out bytes.Buffer
	got := detectBeadFilter(r, beadFilterModeDefault, &out, nil)
	if got == nil || got.Label != "subsystem:{codename}" {
		t.Fatalf("expected subsystem:{codename}, got %+v, out=%q", got, out.String())
	}
	if !strings.Contains(out.String(), "Detected") {
		t.Errorf("expected 'Detected' announcement, got %q", out.String())
	}
}

// ConfidenceLow corpus → count floor met, score floor not met → detector
// stays silent (kerf-yxl). priorFilter (nil here) is returned verbatim.
func TestDetectBeadFilter_LowConfidence_StaysSilent(t *testing.T) {
	stubBr(t, `[
		{"id":"x-1","labels":["subsystem:foo"]},
		{"id":"x-2","labels":["subsystem:bar"]},
		{"id":"x-3","labels":["subsystem:baz"]},
		{"id":"y-1","labels":["epic:alpha"]},
		{"id":"y-2","labels":["epic:beta"]},
		{"id":"y-3","labels":["epic:gamma"]}
	]`)
	r := makeResolverWithWorks(t, []string{"unrelated"})
	var out bytes.Buffer
	got := detectBeadFilter(r, beadFilterModeDefault, &out, nil)
	if got != nil {
		t.Errorf("expected nil for low-confidence corpus, got %+v", got)
	}
	if strings.Contains(out.String(), "Detected") {
		t.Errorf("expected silent detector on low confidence, got %q", out.String())
	}
}

// ConfidenceNone (tiny corpus, < count floor) → detector silent, nil result.
func TestDetectBeadFilter_TinyCorpus_StaysSilent(t *testing.T) {
	stubBr(t, `[{"id":"x-1","labels":["subsystem:auth"]}]`)
	r := makeResolverWithWorks(t, []string{"auth"})
	var out bytes.Buffer
	got := detectBeadFilter(r, beadFilterModeDefault, &out, nil)
	if got != nil {
		t.Errorf("expected nil on 1-bead corpus, got %+v", got)
	}
	if strings.Contains(out.String(), "Detected") {
		t.Errorf("expected silent detector on tiny corpus, got %q", out.String())
	}
}

// beadFilterModeNo short-circuits the detector and returns priorFilter
// verbatim — never inspects the bead store.
func TestDetectBeadFilter_ModeNo_ReturnsPriorFilterUnchanged(t *testing.T) {
	// Stub br with a corpus that would otherwise produce a confident
	// suggestion; --no must skip detection entirely.
	stubBr(t, `[
		{"id":"x-1","labels":["subsystem:auth"]},
		{"id":"x-2","labels":["subsystem:db"]},
		{"id":"x-3","labels":["subsystem:api"]}
	]`)
	r := makeResolverWithWorks(t, []string{"auth", "db", "api"})
	prior := &beads.Filter{Label: "kept:{codename}"}
	var out bytes.Buffer
	got := detectBeadFilter(r, beadFilterModeNo, &out, prior)
	if got != prior {
		t.Errorf("--no mode must return priorFilter unchanged, got %+v", got)
	}
	if out.Len() != 0 {
		t.Errorf("--no mode must be silent, got %q", out.String())
	}
}

// --- End-to-end: confident detection writes bead_filter through `kerf init`. -

func TestInit_DetectsAndWritesBeadFilterOnForceRerun(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	gitRepo := testutil.SetupGitRepo(t)
	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	t.Cleanup(func() { os.Chdir(oldWd) })

	// Reset flags after the test.
	t.Cleanup(func() {
		initForceFlag = false
		initYesFlag = false
		initNoFlag = false
		initBeadFilterFlag = ""
		initJigFlag = ""
	})

	stubBr(t, `[]`)
	captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("first init: %v", err)
		}
	})

	pidData, err := os.ReadFile(filepath.Join(gitRepo, ".kerf", "project-identifier"))
	if err != nil {
		t.Fatalf("reading project-identifier: %v", err)
	}
	projectID := strings.TrimSpace(string(pidData))
	worksDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	for _, cn := range []string{"auth", "db", "api"} {
		if err := os.MkdirAll(filepath.Join(worksDir, cn), 0o755); err != nil {
			t.Fatalf("seeding work dir %s: %v", cn, err)
		}
	}

	stubBr(t, `[
		{"id":"x-1","labels":["subsystem:auth"]},
		{"id":"x-2","labels":["subsystem:db"]},
		{"id":"x-3","labels":["subsystem:api"]}
	]`)

	initForceFlag = true
	captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("force re-init: %v", err)
		}
	})

	projCfgPath := config.ProjectConfigPath(filepath.Join(tmp, ".kerf"), projectID)
	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	if cfg.BeadFilter == nil {
		t.Fatalf("expected BeadFilter to be set after confident detection, got nil")
	}
	if cfg.BeadFilter.Label != "subsystem:{codename}" {
		t.Errorf("expected label subsystem:{codename}, got %q", cfg.BeadFilter.Label)
	}
}

// Sanity: the heuristic-to-filter glue produces a spec-valid Filter.
func TestDetectFilterPrefix_ProducedFilterValidates(t *testing.T) {
	all := []beads.Bead{
		{ID: "1", Labels: []string{"subsystem:auth"}},
		{ID: "2", Labels: []string{"subsystem:db"}},
		{ID: "3", Labels: []string{"subsystem:api"}},
	}
	prefix, _, _ := beads.DetectFilterPrefix(all, []string{"auth", "db", "api"})
	f := &beads.Filter{Label: prefix + ":{codename}"}
	if err := f.Validate(); err != nil {
		t.Fatalf("constructed filter failed validation: %v", err)
	}
}
