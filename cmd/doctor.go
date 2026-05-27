package cmd

// kerf doctor — project health check.
//
// Plan 017 / B5 — command scaffold. This file is the CLI shell only:
// flag plumbing, project resolution, detector selection, formatter
// dispatch, and exit-code semantics. The detectors themselves live in
// internal/doctor/ and are added one per downstream bead
// (kerf-7b4 / kerf-9jh / kerf-47z / kerf-kqn / kerf-7lq).
//
// Spec references:
//   - specs/commands.md §"kerf doctor" — syntax, flags, behavior, output
//     (text + JSON), exit codes, errors.
//   - plans/017_storage_reconciliation/_plan.md §B5 — scaffold scope.
//
// Exit-code semantics (specs/commands.md §"kerf doctor" §"Exit codes"):
//   - 0  when no red findings, regardless of --strict.
//   - 0  when red findings present and --strict is absent.
//   - 1  when red findings present and --strict is given. <!-- TBD:
//        Plan 017 open question 4 — pinned to "1" in v1, may differentiate
//        red severity exit code later. -->
//   - 1  on infrastructure error (project not initialised, etc.) via
//        cobra's default err != nil → os.Exit(1).

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gregberns/kerf/internal/cmdutil"
	"github.com/gregberns/kerf/internal/doctor"
)

var (
	doctorFormat    string
	doctorDetectors []string
	doctorQuiet     bool
	doctorStrict    bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Project health check (project.yaml, storage drift, symlinks, filters, archives)",
	Long: `Health check for the current project. Reports green / yellow / red
findings across project.yaml shape, storage drift, symlink integrity
(local mode), bead_filter coverage, and archive orphans.

kerf doctor is read-only: it surfaces findings and names the command
that would fix each. It does not mutate state.`,
	// SilenceUsage: errors returned by runDoctor are user-facing
	// diagnostics (scaffolding failures, unreadable project state), not
	// flag-misuse signals. Suppress cobra's default usage dump so scripts
	// see only the single-line error before exit 1 (kerf-jy2i — symmetry
	// with kerf next / kerf-1d6).
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoctor(cmd)
	},
}

func init() {
	doctorCmd.Flags().StringVar(&doctorFormat, "format", "text", "Output format: text or json")
	doctorCmd.Flags().StringArrayVar(&doctorDetectors, "detector", nil, "Run only the named detector (repeatable). When omitted, all detectors run.")
	doctorCmd.Flags().BoolVar(&doctorQuiet, "quiet", false, "Suppress green findings; emit only yellow and red")
	doctorCmd.Flags().BoolVar(&doctorStrict, "strict", false, "Exit 1 when any red finding is present")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	// --- Validate --format ---------------------------------------------
	format := strings.ToLower(strings.TrimSpace(doctorFormat))
	if format == "" {
		format = "text"
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("unknown format '%s'. Supported: text, json", doctorFormat)
	}

	// --- Resolve project + storage -------------------------------------
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}
	r, err := cmdutil.Resolver(projectID)
	if err != nil {
		return err
	}

	// --- Select detectors ----------------------------------------------
	detectors, err := doctor.DefaultRegistry.Select(doctorDetectors)
	if err != nil {
		return err
	}

	// --- Run -----------------------------------------------------------
	dctx := &doctor.Context{
		ProjectID: projectID,
		Resolver:  r,
		BenchPath: r.BenchPath,
	}
	report, err := doctor.Run(doctor.DefaultRegistry, dctx, detectors)
	if err != nil {
		return err
	}

	// --- Render --------------------------------------------------------
	switch format {
	case "json":
		if err := doctor.RenderJSON(out, report, doctorQuiet); err != nil {
			return err
		}
	default:
		if err := doctor.RenderText(out, report, doctorQuiet); err != nil {
			return err
		}
	}

	// --- Exit-code policy ----------------------------------------------
	// Default: exit 0 regardless of findings.
	// --strict: exit 1 when any red finding is present. Cobra's default
	// "err != nil → os.Exit(1)" doesn't fit cleanly here (we've already
	// rendered the report; returning an error would print "Error: ..."
	// after the report body), so we mirror triage's process-exit hook.
	if doctorStrict && report.HasRed() {
		doctorExit(1)
		return &doctorExitError{Code: 1}
	}
	return nil
}

// --- Exit-code wiring (mirrors cmd/triage.go) ----------------------------
//
// doctorExitFn defaults to os.Exit and is indirected through a variable
// so tests can swap it for a no-op recorder and assert the requested
// exit code without terminating the test runner.

type doctorExitError struct{ Code int }

func (e *doctorExitError) Error() string { return fmt.Sprintf("doctor: exit %d", e.Code) }

var doctorExitFn = func(code int) { os.Exit(code) }

var doctorLastExitCode int

func doctorExit(code int) {
	doctorLastExitCode = code
	doctorExitFn(code)
}

// DoctorExitCode reports the exit code carried by err if it was
// produced by `kerf doctor --strict` running under a non-exiting test
// hook.
func DoctorExitCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var de *doctorExitError
	if errors.As(err, &de) {
		return de.Code, true
	}
	return 0, false
}
