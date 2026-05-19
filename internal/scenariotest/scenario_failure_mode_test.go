package scenariotest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestScenarioD_FailureMode_BdFails — kerf-cz2t.
//
// Pins the kerf-1d6 / kerf-jy2i invariant end-to-end across every kerf command
// that shells out to the configured `tools.tasks` subprocess. Tools.tasks is
// pointed at a shim that always exits non-zero with a recognisable diagnostic;
// kerf must:
//
//   - Exit non-zero (the BLOCKER #3 regression: kerf next previously swallowed
//     the failure and exited 0).
//   - Mention the configured tool name on stderr so scripts/users can identify
//     the misconfiguration source.
//   - NOT dump cobra's usage block — that "Error: ... \nUsage:\n  ..." shape is
//     the symptom of a missing SilenceUsage and is what made the dogfood log
//     describe the regression as "kerf next dumps help on br failure".
//
// `kerf doctor` without --strict is the documented exception (specs/commands.md
// §"kerf doctor" §"Exit codes"): the bd-store failure renders as a [red]
// finding but the command still exits 0. Both branches are asserted.
func TestScenarioD_FailureMode_BdFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell shim is /bin/sh; not portable to Windows test runners")
	}

	r := New(t)

	// Stage a shim named `failbd` that always exits non-zero with a message
	// containing the literal "failbd" so we can grep for the tool name in
	// stderr. Place it in a fresh dir and prepend that to PATH for every
	// subsequent kerf invocation. Naming it `failbd` (not `bd`/`br`) keeps
	// the canonical tool resolution honest: kerf must respect project.yaml's
	// tools.tasks rather than hard-coding `bd`.
	shimDir := t.TempDir()
	shimPath := filepath.Join(shimDir, "failbd")
	shimBody := "#!/bin/sh\n" +
		"echo 'failbd: simulated tools.tasks failure for kerf-cz2t' 1>&2\n" +
		"exit 2\n"
	if err := os.WriteFile(shimPath, []byte(shimBody), 0o755); err != nil {
		t.Fatalf("write failbd shim: %v", err)
	}
	// Prepend the shim dir to the existing PATH so `failbd` resolves first
	// while leaving the rest of the environment intact (notably `sh`).
	currentPath := ""
	for _, kv := range r.env {
		if strings.HasPrefix(kv, "PATH=") {
			currentPath = strings.TrimPrefix(kv, "PATH=")
			break
		}
	}
	if currentPath == "" {
		currentPath = os.Getenv("PATH")
	}
	r.SetEnv("PATH", shimDir+string(os.PathListSeparator)+currentPath)

	// Project ID + bench-mode project dir. The harness's `bd init` already
	// laid down the bd store under projectRoot; the kerf-side project lives
	// under the scenario's HOME (bench mode, since there's no .kerf/project
	// inside the projectRoot git repo here).
	const projectID = "scenario-d-proj"
	projDir := filepath.Join(r.HomeDir(), ".kerf", "projects", projectID)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir projDir: %v", err)
	}

	// project.yaml: point tools.tasks at the failing shim. The minimal
	// `jigs:` list is there only so `kerf doctor` doesn't flag a separate
	// red finding for "declares no jigs" (we still want the assertion that
	// the bd-failure finding is RED; the no-jigs red is unrelated noise).
	projectYAML := "" +
		"jigs: [spec]\n" +
		"tools:\n" +
		"  tasks: failbd\n"
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte(projectYAML), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}

	// One work, so `kerf show` and `kerf work edit` have a target.
	const codename = "myw"
	workDir := filepath.Join(projDir, codename)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir workDir: %v", err)
	}
	workSpec := "" +
		"codename: " + codename + "\n" +
		"type: spec\n" +
		"project:\n" +
		"  id: " + projectID + "\n" +
		"jig: spec\n" +
		"jig_version: 1\n" +
		"status: problem-space\n" +
		"status_values: [problem-space, ready]\n" +
		"created: 2025-01-01T00:00:00Z\n" +
		"updated: 2025-01-01T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(workDir, "spec.yaml"), []byte(workSpec), 0o644); err != nil {
		t.Fatalf("write spec.yaml: %v", err)
	}

	// Table-driven sweep. Every row asserts the kerf-1d6 invariant unless
	// it is the documented doctor-without-strict exception.
	type row struct {
		name           string
		args           []string
		wantExitNonZero bool   // true ⇒ assert exit != 0
		wantRedFinding  bool   // true ⇒ assert stdout contains a RED bd-store finding
	}
	rows := []row{
		{
			name:            "kerf next",
			args:            []string{"--project", projectID, "next"},
			wantExitNonZero: true,
		},
		{
			name:            "kerf triage",
			args:            []string{"--project", projectID, "triage"},
			wantExitNonZero: true,
		},
		{
			name:            "kerf doctor --strict (with bd failure)",
			args:            []string{"--project", projectID, "doctor", "--strict"},
			wantExitNonZero: true,
			wantRedFinding:  true,
		},
		{
			name:            "kerf doctor (without --strict)",
			args:            []string{"--project", projectID, "doctor"},
			wantExitNonZero: false,
			wantRedFinding:  true,
		},
		{
			name:            "kerf show <codename>",
			args:            []string{"--project", projectID, "show", codename},
			wantExitNonZero: true,
		},
		{
			name:            "kerf bootstrap-filters",
			args:            []string{"--project", projectID, "bootstrap-filters"},
			wantExitNonZero: true,
		},
		{
			name:            "kerf work edit <codename> --bead-filter-add label=x:y",
			args:            []string{"--project", projectID, "work", "edit", codename, "--bead-filter-add", "label=x:y"},
			wantExitNonZero: true,
		},
	}

	for _, c := range rows {
		c := c
		t.Run(c.name, func(t *testing.T) {
			stdout, stderr, code, err := r.Run(c.args...)
			if err != nil {
				t.Fatalf("run %v: %v\nstdout: %s\nstderr: %s", c.args, err, stdout, stderr)
			}

			// (1) Exit code contract.
			if c.wantExitNonZero {
				if code == 0 {
					t.Fatalf("expected non-zero exit when tools.tasks fails; got exit 0\nargs: %v\nstdout: %s\nstderr: %s", c.args, stdout, stderr)
				}
			} else {
				if code != 0 {
					t.Fatalf("expected exit 0 (documented degrade path); got %d\nargs: %v\nstdout: %s\nstderr: %s", code, c.args, stdout, stderr)
				}
			}

			// (2) Tool-name reference. The configured tool ("failbd") must
			// appear in stderr or stdout so users/scripts know which
			// subprocess failed. We accept either stream because some
			// commands route diagnostics through stdout (doctor's
			// rendered report) rather than stderr.
			combined := stdout + stderr
			if c.wantExitNonZero {
				if !strings.Contains(combined, "failbd") {
					t.Errorf("expected diagnostic to name the configured tool 'failbd'; got\nstdout: %s\nstderr: %s", stdout, stderr)
				}
			}

			// (3) No cobra usage dump. Post kerf-1d6 / kerf-jy2i, the
			// failure mode must be a single-line user-facing error, not a
			// help block. The usage block always starts with "Usage:" and
			// lists the command shape on the next line; either signature
			// is a regression.
			usageMarkers := []string{
				"Usage:\n  kerf ",
				"\nFlags:\n",
				"Run 'kerf --help'",
			}
			for _, marker := range usageMarkers {
				if strings.Contains(combined, marker) {
					t.Errorf("kerf %v dumped cobra usage on subprocess failure (kerf-1d6 / kerf-jy2i regression).\nmarker: %q\nstdout: %s\nstderr: %s", c.args, marker, stdout, stderr)
					break
				}
			}

			// (4) RED finding on the doctor paths.
			if c.wantRedFinding {
				// Render-text emits "[red]" prefixed findings; the
				// bead-store unavailability detector is the surface we
				// want here.
				if !strings.Contains(stdout, "[red]") {
					t.Errorf("expected at least one [red] finding in doctor output; got stdout:\n%s", stdout)
				}
				if !strings.Contains(stdout, "failbd") {
					t.Errorf("expected the doctor RED finding to name the configured tool 'failbd'; got stdout:\n%s", stdout)
				}
			}
		})
	}
}
