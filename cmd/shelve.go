package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/session"
	"github.com/gberns/kerf/internal/snapshot"
	"github.com/gberns/kerf/internal/spec"
)

var shelveForce bool

var shelveCmd = &cobra.Command{
	Use:   "shelve [codename]",
	Short: "Pause work with state preservation",
	Long: `Pause work with state preservation.

Without codename: infers the active work in the current project.
With --force: clears a stale active_session without SESSION.md instructions.

Examples:
  kerf shelve                    Shelve the active work
  kerf shelve blue-bear          Shelve a specific work
  kerf shelve --force blue-bear  Force-clear a stale session`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var cn string
		if len(args) > 0 {
			cn = args[0]
		}
		return runShelve(cn)
	},
}

func init() {
	shelveCmd.Flags().BoolVar(&shelveForce, "force", false, "Clear stale active_session without SESSION.md instructions")
	rootCmd.AddCommand(shelveCmd)
}

func runShelve(cn string) error {
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	bp, err := bench.BenchPath()
	if err != nil {
		return err
	}

	// Resolve target work.
	if cn == "" {
		if shelveForce {
			return fmt.Errorf("codename is required when using --force")
		}
		cn, err = inferActiveWork(bp, projectID)
		if err != nil {
			return err
		}
	}

	s, workDir, err := cmdutil.LoadWork(projectID, cn)
	if err != nil {
		return fmt.Errorf("work '%s' not found in project '%s'", cn, projectID)
	}

	if s.ActiveSession == nil && !shelveForce {
		return fmt.Errorf("work '%s' has no active session to shelve", cn)
	}

	if shelveForce {
		return runForceShelve(s, workDir, cn, projectID)
	}
	return runNormalShelve(s, workDir, cn, projectID, bp)
}

func runNormalShelve(s *spec.SpecYAML, workDir, cn, projectID, bp string) error {
	// 1. Take snapshot.
	snapshot.Take(workDir, "")

	// 2. End session.
	session.EndSession(s)

	// 3. Write spec.yaml (updates timestamp).
	specPath := filepath.Join(workDir, "spec.yaml")
	if err := spec.Write(specPath, s); err != nil {
		return err
	}

	// 4. Emit SESSION.md instructions.
	fmt.Printf("Work %s shelved.\n", cn)
	fmt.Println()
	fmt.Println("Before ending this session, write SESSION.md in the work directory with:")
	fmt.Println("- Current pass and progress within it")
	fmt.Println("- Decisions made during this session")
	fmt.Println("- Open questions")
	fmt.Println("- Suggested next steps")
	fmt.Println("- Reading order for a new session picking this up")
	fmt.Println()
	fmt.Printf("Path: %s\n", filepath.Join(workDir, "SESSION.md"))

	return nil
}

func runForceShelve(s *spec.SpecYAML, workDir, cn, projectID string) error {
	// 1. End session.
	session.EndSession(s)

	// 2. Write spec.yaml (updates timestamp).
	specPath := filepath.Join(workDir, "spec.yaml")
	if err := spec.Write(specPath, s); err != nil {
		return err
	}

	// 3. Take snapshot.
	snapshot.Take(workDir, "")

	// 4. No SESSION.md instructions.
	fmt.Printf("Work %s force-shelved. Stale session cleared.\n", cn)

	return nil
}

func inferActiveWork(bp, projectID string) (string, error) {
	projectDir := filepath.Join(bp, "projects", projectID)
	cn, err := session.FindActiveWork(projectDir)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "no work") {
			return "", fmt.Errorf("no active session found in project '%s'. Specify a codename", projectID)
		}
		if strings.Contains(errMsg, "multiple") {
			return "", fmt.Errorf("multiple active sessions in project '%s': %s. Specify a codename", projectID, errMsg)
		}
		return "", err
	}
	return cn, nil
}
