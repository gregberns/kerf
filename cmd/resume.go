package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/dep"
	"github.com/gberns/kerf/internal/jig"
	"github.com/gberns/kerf/internal/session"
	"github.com/gberns/kerf/internal/snapshot"
	"github.com/gberns/kerf/internal/spec"
)

var resumeCmd = &cobra.Command{
	Use:   "resume <codename>",
	Short: "Load context for resuming work on a shelved work",
	Long: `Load context for resuming work. Outputs full state so an agent
can orient and continue working.

Examples:
  kerf resume blue-bear`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runResume(args[0])
	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}

func runResume(cn string) error {
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	s, workDir, err := cmdutil.LoadWork(projectID, cn)
	if err != nil {
		return fmt.Errorf("work '%s' not found in project '%s'", cn, projectID)
	}

	// Check for active session.
	if s.ActiveSession != nil {
		// Find the active session's start time.
		var started string
		for i := len(s.Sessions) - 1; i >= 0; i-- {
			if s.Sessions[i].Ended == nil {
				started = s.Sessions[i].Started.Format(time.RFC3339)
				break
			}
		}
		return fmt.Errorf("work '%s' has an active session (started %s). Run 'kerf shelve %s' or 'kerf shelve --force %s' to end it before resuming",
			cn, started, cn, cn)
	}

	// Record new session.
	session.StartSession(s, "")

	// Write spec.yaml (also updates timestamp).
	specPath := filepath.Join(workDir, "spec.yaml")
	if err := spec.Write(specPath, s); err != nil {
		return err
	}

	// Take snapshot.
	snapshot.Take(workDir, "")

	// Load jig.
	bp, _ := bench.BenchPath()
	jigsDir := filepath.Join(bp, "jigs")
	jigDef, _, _ := jig.Resolve(s.Jig, jigsDir)

	// === Output ===

	// Work metadata.
	fmt.Printf("Resuming work: %s\n", s.Codename)
	if s.Title != nil {
		fmt.Printf("Title: %s\n", *s.Title)
	}
	fmt.Printf("Type: %s\n", s.Type)
	fmt.Printf("Status: %s\n", s.Status)
	fmt.Printf("Project: %s\n", s.Project.ID)
	fmt.Println()

	// SESSION.md contents.
	sessionMDPath := filepath.Join(workDir, "SESSION.md")
	if data, err := os.ReadFile(sessionMDPath); err == nil {
		fmt.Println("SESSION.md:")
		fmt.Println(string(data))
		fmt.Println()
	} else {
		fmt.Println("SESSION.md not found — resuming without interpreted session state.")
		fmt.Println()
	}

	// Current pass instructions.
	if jigDef != nil {
		pass := jigDef.PassForStatus(s.Status)
		if pass != nil {
			fmt.Printf("Current pass: %s (status: %s)\n", pass.Name, pass.Status)
			fmt.Println()
			instructions := jigDef.InstructionsForPass(pass.Name)
			if instructions != "" {
				fmt.Println(instructions)
				fmt.Println()
			}
		}
	}

	// Session history (previous sessions, not the newly created one).
	if len(s.Sessions) > 1 {
		fmt.Println("Session history:")
		for _, sess := range s.Sessions[:len(s.Sessions)-1] {
			id := "anonymous"
			if sess.ID != nil {
				id = *sess.ID
			}
			started := sess.Started.Format(time.RFC3339)
			ended := "(active)"
			if sess.Ended != nil {
				ended = sess.Ended.Format(time.RFC3339)
			}
			fmt.Printf("  %s  started: %s  ended: %s\n", id, started, ended)
		}
		fmt.Println()
	}

	// Dependencies.
	if len(s.DependsOn) > 0 {
		fmt.Println("Dependencies:")
		for _, d := range s.DependsOn {
			result := dep.Resolve(d, bp, projectID)
			status := result.Status
			if result.Unresolvable {
				status = "unresolvable"
			}
			fmt.Printf("  %s — %s [%s]\n", d.Codename, d.Relationship, status)
		}
		fmt.Println()
	}

	// File listing.
	fmt.Println("Files:")
	printFileTree(workDir, workDir, "  ")
	fmt.Println()

	// Next steps.
	fmt.Println("Next steps:")
	fmt.Printf("  1. Continue working in %s\n", workDir)
	fmt.Printf("  2. Advance status: kerf status %s <next-status>\n", cn)
	fmt.Printf("  3. When done: kerf shelve %s\n", cn)

	return nil
}
