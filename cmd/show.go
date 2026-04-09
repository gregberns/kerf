package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/dep"
	"github.com/gberns/kerf/internal/jig"
)

var showCmd = &cobra.Command{
	Use:   "show <codename>",
	Short: "Display full details for a work",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	codename := args[0]

	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	s, workDir, err := cmdutil.LoadWorkWithChecks(projectID, codename)
	if err != nil {
		return fmt.Errorf("work '%s' not found in project '%s'", codename, projectID)
	}

	// Load jig
	bp, _ := bench.BenchPath()
	jigsDir := filepath.Join(bp, "jigs")
	jigDef, _, _ := jig.Resolve(s.Jig, jigsDir)

	// Metadata
	fmt.Printf("Work: %s\n", s.Codename)
	if s.Title != nil {
		fmt.Printf("Title: %s\n", *s.Title)
	}
	fmt.Printf("Type: %s\n", s.Type)
	fmt.Printf("Status: %s\n", s.Status)
	fmt.Printf("Project: %s\n", s.Project.ID)
	fmt.Printf("Jig: %s (v%d)\n", s.Jig, s.JigVersion)
	fmt.Printf("Created: %s\n", s.Created.Format(time.RFC3339))
	fmt.Printf("Updated: %s\n", s.Updated.Format(time.RFC3339))
	fmt.Println()

	// Jig context — current pass instructions
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

	// File tree (excluding .history/)
	fmt.Println("Files:")
	printFileTree(workDir, workDir, "  ")
	fmt.Println()

	// Session history
	if len(s.Sessions) > 0 {
		fmt.Println("Sessions:")
		for _, sess := range s.Sessions {
			id := "anonymous"
			if sess.ID != nil {
				id = *sess.ID
			}
			active := ""
			if s.ActiveSession != nil && ((sess.ID != nil && *sess.ID == *s.ActiveSession) || (sess.ID == nil && *s.ActiveSession == "anonymous")) {
				active = " [active]"
			}
			started := sess.Started.Format(time.RFC3339)
			ended := "(active)"
			if sess.Ended != nil {
				ended = sess.Ended.Format(time.RFC3339)
			}
			fmt.Printf("  %s  started: %s  ended: %s%s\n", id, started, ended, active)
		}
		fmt.Println()
	}

	// Dependencies
	if len(s.DependsOn) > 0 {
		fmt.Println("Dependencies:")
		for _, d := range s.DependsOn {
			result := dep.Resolve(d, bp, projectID)
			status := result.Status
			if result.Unresolvable {
				status = "unresolvable"
			}
			project := projectID
			if d.Project != nil {
				project = *d.Project
			}
			fmt.Printf("  %s (project: %s, relationship: %s) — %s\n", d.Codename, project, d.Relationship, status)
		}
		fmt.Println()
	}

	// SESSION.md contents
	sessionMDPath := filepath.Join(workDir, "SESSION.md")
	if data, err := os.ReadFile(sessionMDPath); err == nil {
		fmt.Println("SESSION.md:")
		fmt.Println(string(data))
		fmt.Println()
	}

	// Commands block
	fmt.Println("Commands:")
	fmt.Printf("  kerf resume %s                 Resume working\n", codename)
	fmt.Printf("  kerf status %s <next-status>   Advance status\n", codename)
	fmt.Printf("  kerf square %s                 Verify completeness\n", codename)
	fmt.Printf("  kerf shelve %s                 Pause work\n", codename)

	return nil
}

func printFileTree(root, dir, indent string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() == ".history" {
			continue
		}
		rel, _ := filepath.Rel(root, filepath.Join(dir, e.Name()))
		if e.IsDir() {
			fmt.Printf("%s%s/\n", indent, rel)
			printFileTree(root, filepath.Join(dir, e.Name()), indent)
		} else {
			fmt.Printf("%s%s\n", indent, rel)
		}
	}
}

// formatRelativeTime produces a human-friendly relative time string.
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

// statusProgression renders the status progression with a pointer to current.
func statusProgression(statusValues []string, current string) string {
	parts := make([]string, len(statusValues))
	pointer := -1
	for i, sv := range statusValues {
		parts[i] = sv
		if sv == current {
			pointer = i
		}
	}
	line := "  " + strings.Join(parts, " -> ")
	if pointer >= 0 {
		// Build pointer line
		pos := 2 // initial indent
		for i := 0; i < pointer; i++ {
			pos += len(parts[i]) + 4 // " -> " is 4 chars
		}
		mid := pos + len(parts[pointer])/2
		pointerLine := strings.Repeat(" ", mid) + "^^ current"
		return line + "\n" + pointerLine
	}
	// current status not in list
	return line + "\n  (current status '" + current + "' is not in the jig's list)"
}
