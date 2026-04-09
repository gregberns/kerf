package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/spec"
)

var (
	listStatusFilter string
	listAll          bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all works on the bench",
	Long: `Show all works on the bench for the current project.

Examples:
  kerf list                 List active works
  kerf list --status research  Filter by status
  kerf list --all           Include archived works`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList()
	},
}

func init() {
	listCmd.Flags().StringVar(&listStatusFilter, "status", "", "Filter to works with this status")
	listCmd.Flags().BoolVar(&listAll, "all", false, "Include archived works")
	rootCmd.AddCommand(listCmd)
}

type workEntry struct {
	codename string
	workType string
	status   string
	updated  time.Time
	archived bool
	deps     []spec.Dependency
}

func runList() error {
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	bp, err := bench.BenchPath()
	if err != nil {
		return err
	}

	var entries []workEntry

	// Active works.
	codenames, err := bench.ListWorks(bp, projectID)
	if err != nil {
		return err
	}
	for _, cn := range codenames {
		if e, ok := readWorkEntry(bp, projectID, cn, false); ok {
			entries = append(entries, e)
		}
	}

	// Archived works if --all.
	if listAll {
		archived, err := bench.ListArchivedWorks(bp, projectID)
		if err != nil {
			return err
		}
		for _, cn := range archived {
			dir := bench.ArchiveDir(bp, projectID, cn)
			specPath := filepath.Join(dir, "spec.yaml")
			s, err := spec.Read(specPath)
			if err != nil {
				continue
			}
			entries = append(entries, workEntry{
				codename: s.Codename,
				workType: s.Type,
				status:   s.Status,
				updated:  s.Updated,
				archived: true,
				deps:     s.DependsOn,
			})
		}
	}

	// Filter by status.
	if listStatusFilter != "" {
		var filtered []workEntry
		for _, e := range entries {
			if e.status == listStatusFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Sort by updated, most recent first.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].updated.After(entries[j].updated)
	})

	if len(entries) == 0 {
		fmt.Printf("No works found for project '%s'.\n", projectID)
		fmt.Println()
		fmt.Println("Get started:")
		fmt.Println("  kerf new    Create a new work")
		return nil
	}

	fmt.Printf("On the bench for %s:\n", projectID)

	// Find column widths for alignment.
	maxCN, maxType, maxStatus := 0, 0, 0
	for _, e := range entries {
		if len(e.codename) > maxCN {
			maxCN = len(e.codename)
		}
		if len(e.workType) > maxType {
			maxType = len(e.workType)
		}
		sl := len(e.status)
		if e.archived {
			sl += 11 // " [archived]"
		}
		if sl > maxStatus {
			maxStatus = sl
		}
	}

	for _, e := range entries {
		statusStr := e.status
		if e.archived {
			statusStr += " [archived]"
		}
		fmt.Printf("  %-*s  %-*s  %-*s  %s\n",
			maxCN, e.codename,
			maxType, e.workType,
			maxStatus, statusStr,
			relativeTime(e.updated),
		)
	}

	// Dependencies section.
	var depLines []string
	for _, e := range entries {
		for _, d := range e.deps {
			depStatus := lookupDepStatus(bp, projectID, d)
			depLines = append(depLines, fmt.Sprintf("  %s -> %s [%s]", e.codename, d.Codename, depStatus))
		}
	}
	if len(depLines) > 0 {
		fmt.Println()
		fmt.Println("  Dependencies:")
		for _, l := range depLines {
			fmt.Println(l)
		}
	}

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  kerf show <codename>      View work details")
	fmt.Println("  kerf resume <codename>    Resume working on a work")
	fmt.Println("  kerf new                  Start a new work")

	return nil
}

func readWorkEntry(bp, projectID, codename string, archived bool) (workEntry, bool) {
	dir := bench.WorkDir(bp, projectID, codename)
	specPath := filepath.Join(dir, "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		return workEntry{}, false
	}
	return workEntry{
		codename: s.Codename,
		workType: s.Type,
		status:   s.Status,
		updated:  s.Updated,
		archived: archived,
		deps:     s.DependsOn,
	}, true
}

func lookupDepStatus(bp, projectID string, d spec.Dependency) string {
	depProject := projectID
	if d.Project != nil && *d.Project != "" {
		depProject = *d.Project
	}
	dir := bench.WorkDir(bp, depProject, d.Codename)
	specPath := filepath.Join(dir, "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		return "unknown"
	}
	return s.Status
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
