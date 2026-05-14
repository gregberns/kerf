package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/areas"
	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/spec"
)

var areasCmd = &cobra.Command{
	Use:   "areas",
	Short: "Manage project areas",
	Long: `Manage project areas — named regions of the system forming the project's topology.

Subcommands:
  kerf areas init                          Create areas.yaml with an empty set of areas
  kerf areas list                          Show defined areas
  kerf areas add <name> --description "…"  Define a new area
  kerf areas remove <name>                 Remove an area`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// areas init

var areasInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create areas.yaml with an initial empty set of areas",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAreasInit()
	},
}

const areasInitHeader = `# areas.yaml — project topology
#
# Areas are named regions of the system (e.g. auth, api, storage).
# Works reference areas to indicate which parts of the system they touch.
#
# Add an area with:
#   kerf areas add <name> --description "..."
#
`

func runAreasInit() error {
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	bp, err := bench.BenchPath()
	if err != nil {
		return err
	}

	path := areas.AreasPath(bp, projectID)
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("areas.yaml already exists at %s\n", path)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking areas.yaml: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating areas directory: %w", err)
	}

	content := areasInitHeader + "areas: {}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing areas.yaml: %w", err)
	}

	fmt.Printf("Created areas.yaml at %s\n", path)
	fmt.Println()
	fmt.Println("Define areas with:")
	fmt.Println("  kerf areas add <name> --description \"...\"")
	return nil
}

// areas list

var areasListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show defined areas for the current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAreasList()
	},
}

func runAreasList() error {
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	bp, err := bench.BenchPath()
	if err != nil {
		return err
	}

	path := areas.AreasPath(bp, projectID)
	af, err := areas.Load(path)
	if err != nil {
		return err
	}

	names := areas.Names(af)
	if len(names) == 0 {
		fmt.Printf("No areas defined for project '%s'.\n", projectID)
		fmt.Println()
		fmt.Println("Define areas with:")
		fmt.Println("  kerf areas add <name> --description \"...\"")
		return nil
	}

	// Count active works per area.
	workCounts := countWorksPerArea(bp, projectID)

	fmt.Printf("Areas for %s:\n", projectID)
	fmt.Println()

	// Compute column widths.
	maxName := 0
	maxDesc := 0
	for _, name := range names {
		if len(name) > maxName {
			maxName = len(name)
		}
		desc := af.Areas[name].Description
		quoted := fmt.Sprintf("%q", desc)
		if len(quoted) > maxDesc {
			maxDesc = len(quoted)
		}
	}

	for _, name := range names {
		desc := af.Areas[name].Description
		quoted := fmt.Sprintf("%q", desc)
		count := workCounts[name]
		workWord := "works"
		if count == 1 {
			workWord = "work"
		}
		fmt.Printf("  %-*s  %-*s  %d %s\n", maxName, name, maxDesc, quoted, count, workWord)
	}

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  kerf areas add <name> --description \"...\"    Define a new area")
	fmt.Println("  kerf areas remove <name>                     Remove an area")
	fmt.Println("  kerf map                                     View works by area")

	return nil
}

// countWorksPerArea loads all active works for the project and counts how many
// reference each area.
func countWorksPerArea(benchPath, projectID string) map[string]int {
	counts := make(map[string]int)

	codenames, err := bench.ListWorks(benchPath, projectID)
	if err != nil {
		return counts
	}

	for _, codename := range codenames {
		specPath := filepath.Join(bench.WorkDir(benchPath, projectID, codename), "spec.yaml")
		s, err := spec.Read(specPath)
		if err != nil {
			continue
		}
		for _, a := range s.Areas {
			counts[a]++
		}
	}

	return counts
}

// areas add

var areasAddDescription string

var areasAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Define a new area",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAreasAdd(args[0])
	},
}

func runAreasAdd(name string) error {
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	bp, err := bench.BenchPath()
	if err != nil {
		return err
	}

	path := areas.AreasPath(bp, projectID)
	af, err := areas.Load(path)
	if err != nil {
		return err
	}

	if err := areas.Add(af, name, areasAddDescription); err != nil {
		// Wrap errors with project context per spec.
		if strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("area '%s' already exists in project '%s'", name, projectID)
		}
		if strings.Contains(err.Error(), "invalid area name") {
			return fmt.Errorf("area name must be lowercase alphanumeric and hyphens (matching [a-z0-9]+(-[a-z0-9]+)*)")
		}
		return err
	}

	if err := areas.Save(path, af); err != nil {
		return err
	}

	fmt.Printf("Area '%s' added to project '%s'.\n", name, projectID)
	return nil
}

// areas remove

var areasRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an area",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAreasRemove(args[0])
	},
}

func runAreasRemove(name string) error {
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	bp, err := bench.BenchPath()
	if err != nil {
		return err
	}

	path := areas.AreasPath(bp, projectID)
	af, err := areas.Load(path)
	if err != nil {
		return err
	}

	// Check if area exists before attempting removal (spec error message).
	if _, exists := af.Areas[name]; !exists {
		return fmt.Errorf("area '%s' not found in project '%s'. Run 'kerf areas list' to see defined areas", name, projectID)
	}

	// Warn if active works reference this area.
	workCounts := countWorksPerArea(bp, projectID)
	if workCounts[name] > 0 {
		// Find the codenames that reference this area.
		codenames := worksReferencingArea(bp, projectID, name)
		fmt.Printf("Warning: the following works still reference area '%s':\n", name)
		fmt.Printf("  %s\n", strings.Join(codenames, ", "))
	}

	if err := areas.Remove(af, name); err != nil {
		return err
	}

	if err := areas.Save(path, af); err != nil {
		return err
	}

	fmt.Printf("Area '%s' removed from project '%s'.\n", name, projectID)
	return nil
}

// worksReferencingArea returns codenames of active works that reference the given area.
func worksReferencingArea(benchPath, projectID, areaName string) []string {
	var result []string

	codenames, err := bench.ListWorks(benchPath, projectID)
	if err != nil {
		return result
	}

	for _, codename := range codenames {
		specPath := filepath.Join(bench.WorkDir(benchPath, projectID, codename), "spec.yaml")
		s, err := spec.Read(specPath)
		if err != nil {
			continue
		}
		for _, a := range s.Areas {
			if a == areaName {
				result = append(result, codename)
				break
			}
		}
	}

	return result
}

func init() {
	areasAddCmd.Flags().StringVar(&areasAddDescription, "description", "", "Description of the area")

	areasCmd.AddCommand(areasInitCmd)
	areasCmd.AddCommand(areasListCmd)
	areasCmd.AddCommand(areasAddCmd)
	areasCmd.AddCommand(areasRemoveCmd)
	rootCmd.AddCommand(areasCmd)
}
