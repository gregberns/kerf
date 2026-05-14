package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/queue"
	"github.com/gberns/kerf/internal/spec"
)

var (
	nextLimit int
	nextArea  string
)

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Show suggested work ordering",
	Long: `Computed ordering of actionable work items. Filters out blocked and shelved works.
Orders by dependency depth and completion momentum.

Examples:
  kerf next                  Show next actions for current project
  kerf next --limit 5        Show top 5 actions
  kerf next --area api       Only works touching the api area
  kerf next --project foo    Show next actions for a specific project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNext()
	},
}

func init() {
	nextCmd.Flags().IntVar(&nextLimit, "limit", 0, "Show only top N results (0 = show all)")
	nextCmd.Flags().StringVar(&nextArea, "area", "", "Filter to works touching a specific area")
	rootCmd.AddCommand(nextCmd)
}

func runNext() error {
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	r, err := cmdutil.Resolver(projectID)
	if err != nil {
		return err
	}

	// Load all active works.
	codenames, err := r.ListWorks()
	if err != nil {
		return err
	}

	var works []*spec.SpecYAML
	for _, cn := range codenames {
		dir := r.WorkDir(cn)
		specPath := filepath.Join(dir, "spec.yaml")
		s, err := spec.Read(specPath)
		if err != nil {
			continue
		}
		works = append(works, s)
	}

	// Filter by area if requested.
	if nextArea != "" {
		var filtered []*spec.SpecYAML
		for _, w := range works {
			if workTouchesArea(w, nextArea) {
				filtered = append(filtered, w)
			}
		}
		works = filtered
	}

	// Load bead summaries per work.
	beadsByWork := make(map[string]beads.EpicSummary)
	if beads.IsAvailable() {
		allBeads, _ := beads.List()
		if len(allBeads) > 0 {
			for _, w := range works {
				wb := beads.ForWork(allBeads, w.Codename)
				if len(wb) > 0 {
					done := 0
					inProgress := 0
					blocked := 0
					for _, b := range wb {
						if isBeadComplete(b.Status) {
							done++
						}
						// We only need Complete and Total for queue scoring,
						// but fill in what we can.
						switch strings.ToLower(b.Status) {
						case "blocked":
							blocked++
						case "in-progress", "in_progress", "active", "wip":
							inProgress++
						}
					}
					beadsByWork[w.Codename] = beads.EpicSummary{
						Total:      len(wb),
						Complete:   done,
						InProgress: inProgress,
						Blocked:    blocked,
						Rework:     beads.ReworkCount(wb),
					}
				}
			}
		}
	}

	// Resolve queue weights: defaults overlaid with any project.yaml overrides.
	defaults := config.ResolvedQueueWeights{
		FanOut:   queue.WeightFanOut,
		Momentum: queue.WeightMomentum,
		Creation: queue.WeightCreation,
		Rework:   queue.WeightRework,
	}
	projCfg, _ := config.LoadProjectConfig(r.ProjectConfigPath())
	resolved := projCfg.QueueWeights(defaults)
	weights := queue.Weights{
		FanOut:   resolved.FanOut,
		Momentum: resolved.Momentum,
		Creation: resolved.Creation,
		Rework:   resolved.Rework,
	}

	// Compute the queue ordering.
	entries := queue.Compute(works, beadsByWork, weights)

	if len(entries) == 0 {
		fmt.Printf("No actionable works for project '%s'.\n", projectID)
		return nil
	}

	// Apply limit.
	if nextLimit > 0 && nextLimit < len(entries) {
		entries = entries[:nextLimit]
	}

	fmt.Printf("Next actions for %s:\n", projectID)

	// Column widths for alignment.
	maxCN, maxType, maxStatus := 0, 0, 0
	for _, e := range entries {
		if len(e.Codename) > maxCN {
			maxCN = len(e.Codename)
		}
		if len(e.Status) > maxStatus {
			maxStatus = len(e.Status)
		}
	}
	// Look up types from works for display.
	typeByName := make(map[string]string)
	for _, w := range works {
		typeByName[w.Codename] = w.Type
		if len(w.Type) > maxType {
			maxType = len(w.Type)
		}
	}

	for i, e := range entries {
		areasStr := ""
		if len(e.Areas) > 0 {
			areasStr = fmt.Sprintf("  Areas: %s", strings.Join(e.Areas, ", "))
		}

		titleStr := ""
		if e.Title != "" {
			titleStr = fmt.Sprintf("  %q", e.Title)
		}

		wType := typeByName[e.Codename]

		fmt.Println()
		fmt.Printf("  %d. %-*s  %-*s  %-*s%s%s\n",
			i+1,
			maxCN, e.Codename,
			maxType, wType,
			maxStatus, e.Status,
			areasStr,
			titleStr,
		)

		// Print reasons as the suggested action line.
		if len(e.Reasons) > 0 {
			for _, r := range e.Reasons {
				fmt.Printf("     %s\n", r)
			}
		}
	}

	// Suggest a specific action for the top work.
	suggestedAction := suggestAction(works, entries[0].Codename)
	if suggestedAction != "" {
		fmt.Println()
		fmt.Printf("  Suggested: %s\n", suggestedAction)
	}

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  kerf resume <codename>    Resume working on a work")
	fmt.Println("  kerf show <codename>      View work details")

	return nil
}

// workTouchesArea returns true if the work has the given area in its Areas list.
func workTouchesArea(w *spec.SpecYAML, area string) bool {
	for _, a := range w.Areas {
		if strings.EqualFold(a, area) {
			return true
		}
	}
	return false
}

// suggestAction returns a human-readable suggestion for what to do next
// with the given work, based on its current status and jig passes.
func suggestAction(works []*spec.SpecYAML, codename string) string {
	for _, w := range works {
		if w.Codename != codename {
			continue
		}

		// Find where the status sits in status_values.
		statusIdx := -1
		for i, sv := range w.StatusValues {
			if sv == w.Status {
				statusIdx = i
				break
			}
		}

		if statusIdx < 0 {
			return ""
		}

		// If at the last status, suggest finalization.
		if statusIdx == len(w.StatusValues)-1 {
			return fmt.Sprintf("kerf finalize %s --branch <name>  (ready for finalization)", codename)
		}

		// Otherwise suggest continuing or resuming.
		return fmt.Sprintf("kerf resume %s  (continue %s pass)", codename, w.Status)
	}
	return ""
}
