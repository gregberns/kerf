package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gregberns/kerf/internal/bench"
	"github.com/gregberns/kerf/internal/cmdutil"
	"github.com/gregberns/kerf/internal/config"
	"github.com/gregberns/kerf/internal/jig"
)

var jigCmd = &cobra.Command{
	Use:   "jig",
	Short: "Manage jig definitions",
	Long: `Manage jig definitions — workflow templates for spec work.

Subcommands:
  kerf jig list              Show available jigs
  kerf jig show <name>       View full jig definition
  kerf jig save <name>       Save a jig for customization
  kerf jig load <name> <path> Load a jig from file
  kerf jig sync              (not yet available)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// jig list

var jigPhaseFilter string

var jigListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show available jigs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJigList()
	},
}

func runJigList() error {
	summaries, err := jig.ListAll(userJigsDir())
	if err != nil {
		return err
	}

	if len(summaries) == 0 {
		fmt.Println("No jigs available.")
		return nil
	}

	// Filter by phase if requested.
	if jigPhaseFilter != "" {
		var filtered []jig.JigSummary
		for _, s := range summaries {
			if s.Phase == jigPhaseFilter {
				filtered = append(filtered, s)
			}
		}
		summaries = filtered
		if len(summaries) == 0 {
			fmt.Println("No jigs available.")
			return nil
		}
	}

	// Try to load project config for active/available grouping.
	var projCfg *config.ProjectConfig
	var projectID string
	if pid, err := cmdutil.ResolveProject(projectFlag); err == nil {
		projectID = pid
		if r, err := cmdutil.Resolver(pid); err == nil {
			if cfg, err := config.LoadProjectConfig(r.ProjectConfigPath()); err == nil && len(cfg.Jigs) > 0 {
				projCfg = cfg
			}
		}
	}

	if projCfg != nil {
		printGroupedJigList(summaries, projCfg, projectID)
	} else {
		printFlatJigList(summaries)
	}

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  kerf jig show <name>    View full jig definition")

	return nil
}

func printFlatJigList(summaries []jig.JigSummary) {
	fmt.Println("Available jigs:")
	printJigEntries(summaries, nil)
}

func printGroupedJigList(summaries []jig.JigSummary, projCfg *config.ProjectConfig, projectID string) {
	var active, available []jig.JigSummary
	for _, s := range summaries {
		if projCfg.IsJigActive(s.Name) {
			active = append(active, s)
		} else {
			available = append(available, s)
		}
	}

	fmt.Printf("Jigs for %s:\n", projectID)
	if len(active) > 0 {
		fmt.Println()
		fmt.Println("  Active:")
		printJigEntries(active, projCfg)
	}
	if len(available) > 0 {
		fmt.Println()
		fmt.Println("  Available (not active):")
		printJigEntries(available, nil)
	}
}

func printJigEntries(summaries []jig.JigSummary, projCfg *config.ProjectConfig) {
	// Build display names and compute column widths.
	displayNames := make([]string, len(summaries))
	maxName, maxDesc, maxPhase := 0, 0, 0
	for i, s := range summaries {
		dn := s.Name
		if len(s.Aliases) > 0 {
			dn += " (also: " + strings.Join(s.Aliases, ", ") + ")"
		}
		displayNames[i] = dn
		if len(dn) > maxName {
			maxName = len(dn)
		}
		if len(s.Description) > maxDesc {
			maxDesc = len(s.Description)
		}
		if len(s.Phase) > maxPhase {
			maxPhase = len(s.Phase)
		}
	}

	for i, s := range summaries {
		phase := s.Phase
		if phase == "" {
			phase = "—"
		}
		fmt.Printf("    %-*s  %-*s  v%d  %-*s  %s\n",
			maxName, displayNames[i],
			maxDesc, s.Description,
			s.Version,
			maxPhase, phase,
			s.Source,
		)

		// Show passes for composable jigs.
		if s.Composable {
			var passNames []string
			if projCfg != nil {
				if activePasses := projCfg.GetActivePasses(s.Name); activePasses != nil {
					passNames = activePasses
				}
			}
			if passNames == nil {
				// Show all passes — need to resolve the full jig to get pass names.
				if j, _, err := jig.Resolve(s.Name, userJigsDir()); err == nil {
					for _, p := range j.Passes {
						passNames = append(passNames, strings.ToLower(p.Name))
					}
				}
			}
			if len(passNames) > 0 {
				fmt.Printf("      Passes: %s\n", strings.Join(passNames, ", "))
			}
		}

		// Show tools.
		if len(s.Tools) > 0 {
			fmt.Printf("      Tools: %s\n", strings.Join(s.Tools, ", "))
		}
	}
}

// jig show

var jigShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "View full jig definition",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJigShow(args[0])
	},
}

func runJigShow(name string) error {
	j, source, err := jig.Resolve(name, userJigsDir())
	if err != nil {
		return fmt.Errorf("jig '%s' not found. Run 'kerf jig list' to see available jigs", name)
	}

	fmt.Printf("Jig: %s (v%d, %s)\n", j.Name, j.Version, source)
	if j.Description != "" {
		fmt.Printf("Description: %s\n", j.Description)
	}
	fmt.Println()

	// Status values.
	fmt.Println("Status values:")
	fmt.Printf("  %s\n", strings.Join(j.StatusValues, " -> "))
	fmt.Println()

	// Passes.
	fmt.Println("Passes:")
	for i, p := range j.Passes {
		fmt.Printf("  %d. %s (status: %s)\n", i+1, p.Name, p.Status)
		if len(p.Output) > 0 {
			fmt.Printf("     Output: %s\n", strings.Join(p.Output, ", "))
		}
	}
	fmt.Println()

	// File structure.
	if len(j.FileStructure) > 0 {
		fmt.Println("File structure:")
		for _, f := range j.FileStructure {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
	}

	// Agent instructions (markdown body).
	if j.Body != "" {
		fmt.Println("Agent instructions:")
		fmt.Println(j.Body)
	}

	return nil
}

// jig save

var jigSaveFrom string

var jigSaveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save a jig for customization",
	Long: `Save a jig to the user's jigs directory for customization.

Without --from: copies the currently resolved jig (e.g., a built-in) to the user directory.
With --from: validates the file as a jig definition and copies it to the user directory.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJigSave(args[0])
	},
}

func runJigSave(name string) error {
	jigsDir := userJigsDir()

	var content []byte

	if jigSaveFrom != "" {
		// Load from --from path.
		data, err := os.ReadFile(jigSaveFrom)
		if err != nil {
			return fmt.Errorf("file not found: %s", jigSaveFrom)
		}
		if _, err := jig.Parse(data); err != nil {
			return fmt.Errorf("%s is not a valid jig definition. %v", jigSaveFrom, err)
		}
		content = data
	} else {
		// Resolve existing jig and copy it.
		j, _, err := jig.Resolve(name, jigsDir)
		if err != nil {
			return fmt.Errorf("jig '%s' not found. Use --from <path> to create a new jig", name)
		}
		// Re-read the raw content to preserve formatting.
		content, err = readRawJig(name, jigsDir)
		if err != nil {
			// Fallback: just use description — shouldn't happen.
			_ = j
			return fmt.Errorf("failed to read jig content: %w", err)
		}
	}

	if err := jig.SaveToUser(name, content, jigsDir); err != nil {
		return err
	}

	fmt.Printf("Jig '%s' saved to %s\n", name, filepath.Join(jigsDir, name+".md"))
	return nil
}

// readRawJig reads the raw jig file content by trying user-level then built-in.
func readRawJig(name, userJigsDir string) ([]byte, error) {
	if userJigsDir != "" {
		path := filepath.Join(userJigsDir, name+".md")
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		}
	}
	// Try built-in via a known path pattern.
	// We need to access the embedded FS — use Resolve and reconstruct.
	// Actually, the jig package doesn't expose the raw content.
	// Let's read the built-in file through the jig package's embedded FS.
	return jig.ReadBuiltinRaw(name)
}

// jig load

var jigLoadCmd = &cobra.Command{
	Use:   "load <name> <path>",
	Short: "Load a jig from a file",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJigLoad(args[0], args[1])
	},
}

func runJigLoad(name, pathOrURL string) error {
	data, err := os.ReadFile(pathOrURL)
	if err != nil {
		return fmt.Errorf("cannot read from %s: %v", pathOrURL, err)
	}

	if _, err := jig.Parse(data); err != nil {
		return fmt.Errorf("content from %s is not a valid jig definition. %v", pathOrURL, err)
	}

	jigsDir := userJigsDir()
	if err := jig.SaveToUser(name, data, jigsDir); err != nil {
		return err
	}

	fmt.Printf("Jig '%s' loaded from %s to %s\n", name, pathOrURL, filepath.Join(jigsDir, name+".md"))
	return nil
}

// jig sync

var jigSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync jigs from a remote source",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Jig sync is not yet available.")
		return nil
	},
}

func init() {
	jigSaveCmd.Flags().StringVar(&jigSaveFrom, "from", "", "Path to a jig file to copy")
	jigListCmd.Flags().StringVar(&jigPhaseFilter, "phase", "", "Filter to jigs matching this phase")

	jigCmd.AddCommand(jigListCmd)
	jigCmd.AddCommand(jigShowCmd)
	jigCmd.AddCommand(jigSaveCmd)
	jigCmd.AddCommand(jigLoadCmd)
	jigCmd.AddCommand(jigSyncCmd)
	rootCmd.AddCommand(jigCmd)
}

func userJigsDir() string {
	bp, err := bench.BenchPath()
	if err != nil {
		return ""
	}
	return filepath.Join(bp, "jigs")
}
