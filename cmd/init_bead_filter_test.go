package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/storage"
	"github.com/gberns/kerf/internal/testutil"
)

// stubBr installs a fake `br` binary on PATH that emits the given JSON when
// invoked as `br list ...`. Returns the dir holding the stub so callers can
// also unset PATH to simulate "br unavailable" by pointing PATH elsewhere.
func stubBr(t *testing.T, json string) string {
	t.Helper()
	dir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\ncat <<'EOF'\n%s\nEOF\n", json)
	path := filepath.Join(dir, "br")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub br: %v", err)
	}
	// Prepend stub dir to PATH so exec.LookPath("br") finds this one.
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

// nonTTYStdin returns the read end of a pipe — a regular file from the
// kernel's perspective, so isInteractiveStdin reports false.
func nonTTYStdin(t *testing.T) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	// Close the write end immediately so any read returns EOF.
	w.Close()
	t.Cleanup(func() { r.Close() })
	return r
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

func TestDetectBeadFilter_BrUnavailable(t *testing.T) {
	// Point PATH at an empty dir so `br` is not found.
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	r := makeResolverWithWorks(t, []string{"auth"})
	var out bytes.Buffer
	got := detectBeadFilter(r, nonTTYStdin(t), &out, nil)
	if got != nil {
		t.Errorf("expected nil filter when br unavailable, got %+v", got)
	}
}

func TestDetectBeadFilter_NoCodenames(t *testing.T) {
	stubBr(t, `[{"id":"x-1","labels":["subsystem:auth"]}]`)
	r := makeResolverWithWorks(t, nil)
	var out bytes.Buffer
	got := detectBeadFilter(r, nonTTYStdin(t), &out, nil)
	if got != nil {
		t.Errorf("expected nil filter when no codenames exist, got %+v", got)
	}
}

func TestDetectBeadFilter_EmptyStore(t *testing.T) {
	stubBr(t, `[]`)
	r := makeResolverWithWorks(t, []string{"auth"})
	var out bytes.Buffer
	got := detectBeadFilter(r, nonTTYStdin(t), &out, nil)
	if got != nil {
		t.Errorf("expected nil filter for empty store, got %+v", got)
	}
}

func TestDetectBeadFilter_NonInteractive_ConfidentCandidate(t *testing.T) {
	// 4 of 4 beads carry "subsystem:<codename>" where codename ∈ {auth, db, api, ui}.
	stubBr(t, `[
		{"id":"x-1","labels":["subsystem:auth"]},
		{"id":"x-2","labels":["subsystem:db"]},
		{"id":"x-3","labels":["subsystem:api"]},
		{"id":"x-4","labels":["subsystem:ui"]}
	]`)
	r := makeResolverWithWorks(t, []string{"auth", "db", "api", "ui"})
	var out bytes.Buffer
	got := detectBeadFilter(r, nonTTYStdin(t), &out, nil)
	if got == nil || got.Label != "subsystem:{codename}" {
		t.Fatalf("expected subsystem:{codename}, got %+v, out=%q", got, out.String())
	}
	if !strings.Contains(out.String(), "Detected") {
		t.Errorf("expected 'Detected' in output, got %q", out.String())
	}
}

func TestDetectBeadFilter_NonInteractive_NoConfidentCandidate(t *testing.T) {
	// Three prefixes, none correlate with codenames.
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
	got := detectBeadFilter(r, nonTTYStdin(t), &out, nil)
	if got != nil {
		t.Errorf("expected nil (no confident candidate, non-interactive), got %+v", got)
	}
}

// End-to-end: kerf init writes bead_filter to project.yaml when a confident
// candidate is detected non-interactively.
func TestInit_WritesDetectedBeadFilter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	gitRepo := testutil.SetupGitRepo(t)

	oldWd, _ := os.Getwd()
	os.Chdir(gitRepo)
	defer os.Chdir(oldWd)

	// First init creates the project so the works directory exists. Stub br
	// returning empty so this first init writes no filter.
	stubBr(t, `[]`)
	captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("first init: %v", err)
		}
	})

	// Read project ID and seed a work directory so codenames are discoverable.
	pidData, err := os.ReadFile(filepath.Join(gitRepo, ".kerf", "project-identifier"))
	if err != nil {
		t.Fatalf("reading project-identifier: %v", err)
	}
	projectID := strings.TrimSpace(string(pidData))
	worksDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	if err := os.MkdirAll(filepath.Join(worksDir, "auth"), 0o755); err != nil {
		t.Fatalf("seeding work dir: %v", err)
	}

	// Re-stub br with a clean dominant prefix.
	stubBr(t, `[
		{"id":"x-1","labels":["subsystem:auth"]},
		{"id":"x-2","labels":["subsystem:auth"]},
		{"id":"x-3","labels":["subsystem:auth"]}
	]`)

	// Force non-interactive stdin so detectBeadFilter auto-applies without prompting.
	origStdin := os.Stdin
	os.Stdin = nonTTYStdin(t)
	defer func() { os.Stdin = origStdin }()

	// Re-init must use --force now that init skips when project.yaml exists.
	initForceFlag = true
	defer func() { initForceFlag = false }()
	captureOutput(t, func() {
		if err := initCmd.RunE(initCmd, []string{}); err != nil {
			t.Fatalf("second init: %v", err)
		}
	})

	projCfgPath := config.ProjectConfigPath(filepath.Join(tmp, ".kerf"), projectID)
	cfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		t.Fatalf("loading project config: %v", err)
	}
	if cfg.BeadFilter == nil {
		t.Fatalf("expected BeadFilter to be set, got nil")
	}
	if cfg.BeadFilter.Label != "subsystem:{codename}" {
		t.Errorf("expected label subsystem:{codename}, got %q", cfg.BeadFilter.Label)
	}
}

// Sanity: ensure DetectFilterPrefix is wired through to a *beads.Filter (i.e.
// the heuristic-to-filter glue produces a valid filter per the spec).
func TestDetectFilterPrefix_FilterShape(t *testing.T) {
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
