// Package cmd — `kerf work show` subcommand.
//
// Per specs/commands.md §`kerf work show`: prints a single work's
// spec.yaml field-by-field as plain text. The output is scoped to the
// work's metadata, filter, sessions, dependencies, and areas — without
// the jig-pass, file-tree, SESSION.md, attached-beads, or commands
// sections that `kerf show` adds.
//
// The `bead_filter` slot is always rendered (literal value when present,
// `(none)` when absent or present-but-empty), per Plan 019.
package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/cmdutil"
)

var workShowCmd = &cobra.Command{
	Use:   "show <codename>",
	Short: "Dump a work's spec.yaml field-by-field as plain text",
	Long: `Dump a work's spec.yaml field-by-field as plain text.

Renders the work's metadata, bead_filter, areas, dependencies, pinned
beads, and session history without the jig-pass, file-tree, SESSION.md,
or attached-beads sections that 'kerf show' adds.

The bead_filter slot is always rendered: a literal clause when set,
'(none)' when absent or empty.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorkShow(args[0])
	},
}

func init() {
	workCmd.AddCommand(workShowCmd)
}

func runWorkShow(cn string) error {
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	s, _, err := cmdutil.LoadWorkWithChecks(projectID, cn)
	if err != nil {
		return fmt.Errorf("work '%s' not found in project '%s'", cn, projectID)
	}

	// Emit fields in the canonical order of spec.yaml (mirrors the
	// works.md schema layout).
	fmt.Printf("codename:       %s\n", s.Codename)
	if s.Title != nil {
		fmt.Printf("title:          %s\n", *s.Title)
	} else {
		fmt.Println("title:          (none)")
	}
	fmt.Printf("type:           %s\n", s.Type)
	fmt.Printf("status:         %s\n", s.Status)
	fmt.Printf("project_id:     %s\n", s.Project.ID)
	fmt.Printf("jig:            %s (v%d)\n", s.Jig, s.JigVersion)
	fmt.Printf("created:        %s\n", s.Created.Format(time.RFC3339))
	fmt.Printf("updated:        %s\n", s.Updated.Format(time.RFC3339))

	// bead_filter — always rendered (Plan 019).
	fmt.Printf("bead_filter:    %s\n", renderBeadFilterSlot(s.BeadFilter))

	if len(s.Areas) > 0 {
		fmt.Printf("areas:          %s\n", strings.Join(s.Areas, ", "))
	} else {
		fmt.Println("areas:          (none)")
	}

	if len(s.DependsOn) > 0 {
		parts := make([]string, 0, len(s.DependsOn))
		for _, d := range s.DependsOn {
			proj := ""
			if d.Project != nil {
				proj = fmt.Sprintf(" (%s)", *d.Project)
			}
			parts = append(parts, fmt.Sprintf("%s%s [%s]", d.Codename, proj, d.Relationship))
		}
		fmt.Printf("depends_on:     %s\n", strings.Join(parts, ", "))
	} else {
		fmt.Println("depends_on:     (none)")
	}

	if len(s.RelatedTo) > 0 {
		parts := make([]string, 0, len(s.RelatedTo))
		for _, r := range s.RelatedTo {
			parts = append(parts, fmt.Sprintf("%s [%s]", r.Codename, r.Relationship))
		}
		fmt.Printf("related_to:     %s\n", strings.Join(parts, ", "))
	} else {
		fmt.Println("related_to:     (none)")
	}

	if len(s.PinnedBeads) > 0 {
		fmt.Printf("pinned_beads:   %s\n", strings.Join(s.PinnedBeads, ", "))
	} else {
		fmt.Println("pinned_beads:   (none)")
	}

	if s.ActiveSession != nil {
		fmt.Printf("active_session: %s\n", *s.ActiveSession)
	} else {
		fmt.Println("active_session: (none)")
	}

	if len(s.Sessions) > 0 {
		fmt.Println("sessions:")
		for _, sess := range s.Sessions {
			ended := "active"
			if sess.Ended != nil {
				ended = sess.Ended.Format(time.RFC3339)
			}
			fmt.Printf("  - started: %s   ended: %s\n", sess.Started.Format(time.RFC3339), ended)
		}
	} else {
		fmt.Println("sessions:       (none)")
	}

	return nil
}
