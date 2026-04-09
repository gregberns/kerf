package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/spec"
)

var projectFlag string

var rootCmd = &cobra.Command{
	Use:   "kerf",
	Short: "Measure twice, cut once.",
	Long: `kerf — spec-writing CLI for AI agents.
Measure twice, cut once.`,
	Run: func(cmd *cobra.Command, args []string) {
		runRoot()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&projectFlag, "project", "", "Override project inference with this project ID")
}

func runRoot() {
	fmt.Println("kerf — spec-writing CLI for AI agents.")
	fmt.Println("Measure twice, cut once.")
	fmt.Println()

	if !bench.Exists() {
		printGettingStarted()
		return
	}

	printBenchSummary()
	fmt.Println()
	printWorkflow()
	fmt.Println()
	printCommands()
}

func printGettingStarted() {
	fmt.Println("No bench found at ~/.kerf/. Get started by creating your first work:")
	fmt.Println()
	fmt.Println("  kerf new                          Create a new work (auto-generates codename)")
	fmt.Println("  kerf new my-feature --jig feature  Create with a specific codename and jig")
	fmt.Println()
	fmt.Println("kerf manages specification works on a bench at ~/.kerf/. Each work follows")
	fmt.Println("a jig (workflow template) through a series of passes from creation to finalization.")
	fmt.Println()
	printCommands()
}

func printBenchSummary() {
	bp, err := bench.BenchPath()
	if err != nil {
		return
	}

	// Count works across all projects and for current project.
	totalActive := 0
	currentProjectActive := 0
	currentProject := ""

	if pid, err := resolveProjectSilent(); err == nil {
		currentProject = pid
	}

	projects, _ := bench.ListAllProjects(bp)
	for _, pid := range projects {
		works, err := bench.ListWorks(bp, pid)
		if err != nil {
			continue
		}
		for _, codename := range works {
			specPath := filepath.Join(bench.WorkDir(bp, pid, codename), "spec.yaml")
			if _, err := spec.Read(specPath); err == nil {
				totalActive++
				if pid == currentProject {
					currentProjectActive++
				}
			}
		}
	}

	fmt.Println("Bench summary:")
	if currentProject != "" {
		fmt.Printf("  Project: %s (%d active works)\n", currentProject, currentProjectActive)
	}
	fmt.Printf("  Total active works: %d\n", totalActive)
}

func printWorkflow() {
	fmt.Println("Standard workflow:")
	fmt.Println("  1. kerf new                   Create a new work")
	fmt.Println("  2. Work through jig passes    Write artifacts, advance status")
	fmt.Println("  3. kerf shelve                Pause with state preservation")
	fmt.Println("  4. kerf resume <codename>     Load context and continue")
	fmt.Println("  5. kerf finalize <codename> --branch <name>  Complete and hand off to git")
}

func printCommands() {
	fmt.Println("Available commands:")
	fmt.Println("  kerf new               Create a new work")
	fmt.Println("  kerf list              Show all works on the bench")
	fmt.Println("  kerf show <codename>   View full work details")
	fmt.Println("  kerf status <codename> Check or update work status")
	fmt.Println("  kerf resume <codename> Resume a shelved work")
	fmt.Println("  kerf shelve [codename] Pause work with state preservation")
	fmt.Println("  kerf finalize <codename> --branch <name>  Complete and hand off")
	fmt.Println("  kerf square <codename> Verify work completeness")
	fmt.Println("  kerf snapshot <codename>  Manual snapshot")
	fmt.Println("  kerf history <codename>   View snapshot history")
	fmt.Println("  kerf restore <codename> <snapshot>  Restore to a snapshot")
	fmt.Println("  kerf archive <codename>   Move to archive")
	fmt.Println("  kerf delete <codename>    Permanently delete")
	fmt.Println("  kerf config [key] [value] View or modify configuration")
	fmt.Println("  kerf jig <subcommand>     Manage jig definitions")
	fmt.Println()
	fmt.Println("Run 'kerf <command> --help' for details on any command.")
}

// resolveProjectSilent tries to resolve the project without erroring.
func resolveProjectSilent() (string, error) {
	if projectFlag != "" {
		return projectFlag, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	bp, _ := bench.BenchPath()
	// Try to find a project identifier from the repo.
	// We import project here via a direct call to avoid a cycle.
	return resolveFromCwd(cwd, bp)
}

func resolveFromCwd(cwd, benchPath string) (string, error) {
	// Walk up to find .git, then read .kerf/project-identifier.
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			data, err := os.ReadFile(filepath.Join(dir, ".kerf", "project-identifier"))
			if err == nil {
				id := trimSpace(string(data))
				if id != "" {
					return id, nil
				}
			}
			return "", fmt.Errorf("no project identifier")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not in a git repo")
		}
		dir = parent
	}
}

func trimSpace(s string) string {
	// Simple trim of whitespace.
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	for len(s) > 0 && (s[0] == '\n' || s[0] == '\r' || s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}
