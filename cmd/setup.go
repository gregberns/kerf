package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gregberns/kerf/internal/cmdutil"
	"github.com/gregberns/kerf/internal/config"
	"github.com/gregberns/kerf/internal/jig"
	"github.com/gregberns/kerf/internal/project"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Generate agent-facing instructions from active jigs",
	Long: `Generate agent-facing instructions from the project's active jigs.

The output is a block of instructions that the agent applies to its
configuration file (CLAUDE.md, AGENTS.md, etc.). kerf does not write
to these files directly.

Re-runnable: generates fresh instructions whenever jigs are updated.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetup()
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

// phaseOrder defines the SDLC ordering for jig sequencing.
var phaseOrder = map[string]int{
	"planning":       0,
	"implementation": 1,
	"bug-fix":        2,
	"exploration":    3,
}

func runSetup() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Resolve project ID
	var projectID string
	if projectFlag != "" {
		projectID = projectFlag
	} else {
		gitRoot, err := project.FindGitRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier")
		}
		pid, err := project.ReadIdentifier(gitRoot)
		if err != nil {
			// Distinguish a corrupt identifier (surface verbatim per kerf-dlb /
			// kerf-vu0r) from a missing file (legitimate "run kerf init" path).
			if project.IsCorruptIdentifier(err) {
				return err
			}
			return fmt.Errorf("project not initialized. Run 'kerf init' first")
		}
		projectID = pid
	}

	r, err := cmdutil.Resolver(projectID)
	if err != nil {
		return err
	}

	// Load project config
	projCfgPath := r.ProjectConfigPath()
	projCfg, err := config.LoadProjectConfig(projCfgPath)
	if err != nil {
		return fmt.Errorf("loading project config: %w", err)
	}

	// List all available jigs
	summaries, err := jig.ListAll(userJigsDir())
	if err != nil {
		return fmt.Errorf("listing jigs: %w", err)
	}

	hasProjectYAML := fileExists(projCfgPath)

	if !hasProjectYAML {
		printDefaultInstructions(summaries)
		return nil
	}

	printProjectInstructions(projectID, projCfg, summaries)
	return nil
}

func printDefaultInstructions(summaries []jig.JigSummary) {
	fmt.Println("No project.yaml found — showing default instructions.")
	fmt.Println("All available jigs can be used with `kerf new --jig <name>`.")
	fmt.Println()
	printKerfUsage()
	fmt.Println()
	fmt.Println("Available jigs:")
	fmt.Println()
	for _, s := range summaries {
		aliases := ""
		if len(s.Aliases) > 0 {
			aliases = fmt.Sprintf(" (aliases: %s)", strings.Join(s.Aliases, ", "))
		}
		fmt.Printf("  %-20s %s%s\n", s.Name, s.Description, aliases)
	}
	fmt.Println()
}

func printProjectInstructions(projectID string, projCfg *config.ProjectConfig, summaries []jig.JigSummary) {
	// Filter to active jigs and load full definitions
	type activeJig struct {
		summary jig.JigSummary
		def     *jig.JigDefinition
	}
	var active []activeJig

	for _, s := range summaries {
		if !projCfg.IsJigActive(s.Name) {
			continue
		}
		def, _, err := jig.Resolve(s.Name, userJigsDir())
		if err != nil {
			continue
		}
		active = append(active, activeJig{summary: s, def: def})
	}

	// Sort by SDLC phase order
	sort.Slice(active, func(i, j int) bool {
		oi := phaseOrder[active[i].def.Phase]
		oj := phaseOrder[active[j].def.Phase]
		if oi != oj {
			return oi < oj
		}
		return active[i].summary.Name < active[j].summary.Name
	})

	// Collect tool requirements
	toolSet := make(map[string]bool)
	for _, aj := range active {
		for _, tool := range aj.def.Tools {
			toolSet[tool] = true
		}
	}

	fmt.Println("--- START AGENT INSTRUCTIONS ---")
	fmt.Println()
	fmt.Printf("## kerf — project: %s\n", projectID)
	fmt.Println()
	fmt.Println("This project uses kerf for structured specification work.")
	fmt.Println("Before implementing non-trivial changes, create a kerf work.")
	fmt.Println()

	// Tool requirements
	if len(toolSet) > 0 {
		fmt.Println("### Tool requirements")
		fmt.Println()
		var tools []string
		for t := range toolSet {
			tools = append(tools, t)
		}
		sort.Strings(tools)
		for _, t := range tools {
			fmt.Printf("- %s\n", t)
		}
		fmt.Println()
	}

	// Jig sequencing
	fmt.Println("### Available jigs (SDLC order)")
	fmt.Println()
	for _, aj := range active {
		phase := aj.def.Phase
		if phase == "" {
			phase = "general"
		}
		composable := ""
		if aj.def.Composable {
			composable = " [composable]"
		}
		fmt.Printf("- **%s** (%s)%s — %s\n", aj.summary.Name, phase, composable, aj.summary.Description)
		fmt.Printf("  `kerf new --jig %s`\n", aj.summary.Name)
	}
	fmt.Println()

	// Process instructions per jig
	fmt.Println("### Process instructions")
	fmt.Println()
	for _, aj := range active {
		fmt.Printf("#### %s\n", aj.summary.Name)
		fmt.Println()

		activePasses := projCfg.GetActivePasses(aj.summary.Name)

		for _, pass := range aj.def.Passes {
			// For composable jigs, filter to active passes
			if aj.def.Composable && activePasses != nil {
				if !containsStr(activePasses, pass.Name) {
					continue
				}
			}

			passTools := ""
			if len(pass.Tools) > 0 {
				passTools = fmt.Sprintf(" [tools: %s]", strings.Join(pass.Tools, ", "))
			}
			outputs := ""
			if len(pass.Output) > 0 {
				outputs = fmt.Sprintf(" → %s", strings.Join(pass.Output, ", "))
			}
			fmt.Printf("- **%s** (status: %s)%s%s\n", pass.Name, pass.Status, passTools, outputs)
		}
		fmt.Println()
	}

	// kerf command reference
	fmt.Println("### kerf commands")
	fmt.Println()
	printKerfUsage()
	fmt.Println()

	// .gitignore pattern
	printGitignoreBlock()
	fmt.Println()

	// Bench location (placeholder — Plan 017 fills in cheat-sheet body)
	printBenchLocationBlock(projectID)

	fmt.Println("--- END AGENT INSTRUCTIONS ---")
}

func printKerfUsage() {
	fmt.Println("  kerf new <codename>              Create a new work")
	fmt.Println("  kerf show <codename>             See current state + next steps")
	fmt.Println("  kerf status <codename>           Check current status")
	fmt.Println("  kerf status <codename> <status>  Advance to next pass")
	fmt.Println("  kerf shelve <codename>           Save progress when ending a session")
	fmt.Println("  kerf resume <codename>           Pick up where you left off")
	fmt.Println("  kerf square <codename>           Verify the work is complete")
	fmt.Println("  kerf finalize <codename> --branch <name>  Package for implementation")
	fmt.Println("  kerf list                        List active works in the project")
	fmt.Println("  kerf next                        Ranked feed of things to do")
	fmt.Println("  kerf triage                      Drift report on the bead store")
	fmt.Println("  kerf pin <codename> <bead>       Attach a specific bead to a work")
	fmt.Println("  kerf map                         Portfolio view across areas")
	fmt.Println("  kerf areas                       Define and list areas")
	fmt.Println("  kerf work edit <codename>        Mutate a work's bead-filter")
}

func printGitignoreBlock() {
	fmt.Println("### .gitignore")
	fmt.Println()
	fmt.Println("Add these two lines to the repo's `.gitignore` so bench-side state stays out of git while project identity stays committed:")
	fmt.Println()
	fmt.Println("```")
	fmt.Println(".kerf/*")
	fmt.Println("!.kerf/project-identifier")
	fmt.Println("```")
}

func printBenchLocationBlock(projectID string) {
	fmt.Println("### Bench location")
	fmt.Println()
	fmt.Printf("Bench path for this project: `~/.kerf/projects/%s/`. See the \"Where state lives\" cheat-sheet in `specs/architecture.md` for which files live on the bench vs. in the repo. <!-- placeholder: plan 017 expands this section -->\n", projectID)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
