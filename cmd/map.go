package cmd

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/areas"
	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/storage"
)

var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "Show works grouped by area",
	Long: `Show all active work items grouped by area with status and bead progress.

Examples:
  kerf map                  Show area map for current project
  kerf map --project foo    Show area map for a specific project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMap()
	},
}

func init() {
	rootCmd.AddCommand(mapCmd)
}

// mapWork holds data about a single work for the map view.
type mapWork struct {
	codename string
	workType string
	status   string
	title    string
	areas    []string
	deps     []spec.Dependency
	// bead progress — negative means unavailable
	beadsDone  int
	beadsTotal int
}

func runMap() error {
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	r, err := cmdutil.Resolver(projectID)
	if err != nil {
		return err
	}

	// Load areas taxonomy.
	areasPath := r.AreasPath()
	af, err := areas.Load(areasPath)
	if err != nil {
		return err
	}

	// Load all active works.
	codenames, err := r.ListWorks()
	if err != nil {
		return err
	}

	var works []mapWork
	for _, cn := range codenames {
		dir := r.WorkDir(cn)
		specPath := filepath.Join(dir, "spec.yaml")
		s, err := spec.Read(specPath)
		if err != nil {
			continue
		}
		title := ""
		if s.Title != nil {
			title = *s.Title
		}
		works = append(works, mapWork{
			codename:   s.Codename,
			workType:   s.Type,
			status:     s.Status,
			title:      title,
			areas:      s.Areas,
			deps:       s.DependsOn,
			beadsDone:  -1, // not yet loaded
			beadsTotal: -1,
		})
	}

	// Optionally load bead data.
	if beads.IsAvailable() {
		allBeads, _ := beads.List()
		if len(allBeads) > 0 {
			for i := range works {
				wb := beads.ForWork(allBeads, works[i].codename)
				if len(wb) > 0 {
					done := 0
					for _, b := range wb {
						if isBeadComplete(b.Status) {
							done++
						}
					}
					works[i].beadsDone = done
					works[i].beadsTotal = len(wb)
				}
			}
		}
	}

	// No works at all.
	if len(works) == 0 {
		fmt.Printf("No works found for project '%s'.\n", projectID)
		fmt.Println()
		fmt.Println("Get started:")
		fmt.Println("  kerf new    Create a new work")
		return nil
	}

	// Build area -> works mapping.
	areaNames := areas.Names(af)
	areaWorks := make(map[string][]int) // area name -> indices into works
	var unassigned []int

	for i, w := range works {
		if len(w.areas) == 0 {
			unassigned = append(unassigned, i)
		} else {
			for _, a := range w.areas {
				areaWorks[a] = append(areaWorks[a], i)
			}
		}
	}

	// Column widths for alignment.
	maxCN, maxType, maxStatus := 0, 0, 0
	for _, w := range works {
		if len(w.codename) > maxCN {
			maxCN = len(w.codename)
		}
		if len(w.workType) > maxType {
			maxType = len(w.workType)
		}
		if len(w.status) > maxStatus {
			maxStatus = len(w.status)
		}
	}

	fmt.Printf("Map for %s:\n", projectID)

	// Print areas in sorted order.
	for _, areaName := range areaNames {
		indices := areaWorks[areaName]
		fmt.Println()
		if len(indices) == 0 {
			fmt.Printf("  %s: [no active work]\n", areaName)
			continue
		}
		fmt.Printf("  %s:\n", areaName)
		printMapWorks(works, indices, maxCN, maxType, maxStatus)
	}

	// Unassigned works.
	if len(unassigned) > 0 {
		fmt.Println()
		fmt.Printf("  unassigned:\n")
		printMapWorks(works, unassigned, maxCN, maxType, maxStatus)
	}

	// Dependencies section.
	var depLines []string
	for _, w := range works {
		for _, d := range w.deps {
			depStatus := lookupDepStatusForMap(r, d)
			depLines = append(depLines, fmt.Sprintf("  %s -> %s [%s]", w.codename, d.Codename, depStatus))
		}
	}
	if len(depLines) > 0 {
		fmt.Println()
		fmt.Println("Dependencies:")
		for _, l := range depLines {
			fmt.Println(l)
		}
	}

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  kerf show <codename>      View work details")
	fmt.Println("  kerf next                 See suggested work ordering")
	fmt.Println("  kerf areas list           View all areas")

	return nil
}

func printMapWorks(works []mapWork, indices []int, maxCN, maxType, maxStatus int) {
	// Sort indices by codename for stable output.
	sort.Slice(indices, func(i, j int) bool {
		return works[indices[i]].codename < works[indices[j]].codename
	})

	for _, idx := range indices {
		w := works[idx]
		titleStr := ""
		if w.title != "" {
			titleStr = fmt.Sprintf("  %q", w.title)
		}
		beadStr := "  \u2014"
		if w.beadsTotal > 0 {
			beadStr = fmt.Sprintf("  %d/%d beads", w.beadsDone, w.beadsTotal)
		}
		fmt.Printf("    %-*s  %-*s  %-*s%s%s\n",
			maxCN, w.codename,
			maxType, w.workType,
			maxStatus, w.status,
			titleStr,
			beadStr,
		)
	}
}

func isBeadComplete(status string) bool {
	switch status {
	case "closed", "done", "complete":
		return true
	}
	return false
}

func lookupDepStatusForMap(r *storage.Resolver, d spec.Dependency) string {
	depProject := r.ProjectID
	if d.Project != nil && *d.Project != "" {
		depProject = *d.Project
	}
	var dr *storage.Resolver
	if depProject == r.ProjectID {
		dr = r
	} else {
		nr, err := cmdutil.Resolver(depProject)
		if err != nil {
			return "unknown"
		}
		dr = nr
	}
	specPath := filepath.Join(dr.WorkDir(d.Codename), "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		return "unknown"
	}
	return s.Status
}
