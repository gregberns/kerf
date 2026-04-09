package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/project"
)

var initJigFlag string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap kerf in a project",
	Long: `Set up kerf in the current project and print agent setup instructions.

Run this once per project. It creates the project identifier, sets the default
workflow, and prints instructions that tell your AI agent how to use kerf.

The agent reads the output and does the rest — creating config files, updating
gitignore, etc. kerf doesn't know or care what agent you're using.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func init() {
	initCmd.Flags().StringVar(&initJigFlag, "jig", "", "Set default workflow: plan or spec")
	rootCmd.AddCommand(initCmd)
}

func runInit() error {
	// Find git root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	gitRoot, err := project.FindGitRoot(cwd)
	if err != nil {
		return fmt.Errorf("not in a git repository. kerf requires a git repo: %w", err)
	}

	// Ensure bench exists
	if _, err := bench.EnsureBench(); err != nil {
		return fmt.Errorf("creating bench: %w", err)
	}

	// Resolve or create project identity
	benchPath, err := bench.BenchPath()
	if err != nil {
		return err
	}

	projectID, err := project.Resolve(cwd, benchPath)
	if err != nil {
		return fmt.Errorf("resolving project identity: %w", err)
	}

	// Check if project-identifier already exists
	idPath := filepath.Join(gitRoot, ".kerf", "project-identifier")
	if _, err := os.Stat(idPath); os.IsNotExist(err) {
		if err := project.WriteIdentifier(gitRoot, projectID); err != nil {
			return fmt.Errorf("writing project identifier: %w", err)
		}
		fmt.Printf("Created .kerf/project-identifier: %s\n", projectID)
	} else {
		fmt.Printf("Project already initialized: %s\n", projectID)
	}

	// Handle --jig flag or check existing config
	cfg, _ := config.Load(filepath.Join(benchPath, "config.yaml"))
	if initJigFlag != "" {
		if initJigFlag != "plan" && initJigFlag != "spec" {
			return fmt.Errorf("--jig must be 'plan' or 'spec', got '%s'", initJigFlag)
		}
		cfg.DefaultJig = initJigFlag
		configPath := filepath.Join(benchPath, "config.yaml")
		if err := config.Save(configPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("Set default_jig: %s\n", initJigFlag)
	} else if cfg.EffectiveDefaultJig() == "" {
		fmt.Println("\nNote: No default workflow set. Choose one:")
		fmt.Println("  kerf config default_jig plan   # Best for existing projects")
		fmt.Println("  kerf config default_jig spec   # Best for new/spec-driven projects")
	}

	// Print the bootstrap instructions
	fmt.Print(bootstrapInstructions(projectID, cfg.EffectiveDefaultJig()))

	return nil
}

func bootstrapInstructions(projectID string, defaultJig string) string {
	jigNote := ""
	if defaultJig == "" {
		jigNote = `
NOTE: You must set the default workflow before proceeding:
  Run: kerf config default_jig plan    (for existing projects)
  Or:  kerf config default_jig spec    (for spec-driven projects)
`
	}

	return fmt.Sprintf(`
================================================================================
AGENT SETUP INSTRUCTIONS
================================================================================

kerf has been initialized for this project (ID: %s).
%s
Complete the following setup steps:

1. ADD TO .gitignore (if not already present):
   .kerf/

   But DO commit .kerf/project-identifier — it links this repo to its works.

2. ADD THESE INSTRUCTIONS to your agent configuration file.
   For Claude Code: CLAUDE.md
   For Cursor: .cursorrules
   For other agents: wherever the agent reads project instructions.

--- START AGENT INSTRUCTIONS ---

## Planning with kerf

This project uses kerf for structured planning. Before implementing non-trivial
changes (new features, refactors, bug investigations), create a kerf work:

  kerf new <codename>

This creates a work on the bench and shows the process to follow. The jig
(process template) guides you through structured passes — problem space,
decomposition, research, detailed spec, integration, and tasks.

### Key commands

  kerf new <codename>              Create a new work
  kerf show <codename>             See current state + jig instructions for next steps
  kerf status <codename>           Check current status
  kerf status <codename> <status>  Advance to next pass
  kerf shelve <codename>           Save progress when ending a session
  kerf resume <codename>           Pick up where you left off
  kerf square <codename>           Verify the work is complete
  kerf finalize <codename> --branch <name>  Package for implementation

### When to use kerf

- New features or subsystems → kerf new --jig plan (or spec)
- Bug investigations → kerf new --jig bug
- Trivial changes (typos, one-line fixes) → skip kerf, just make the change

### Workflow

1. kerf new <codename> — read the output, it tells you exactly what to do
2. Follow each pass: write the artifacts, advance status
3. kerf show <codename> — if you lose context, this shows where you are
4. kerf shelve / kerf resume — for multi-session work
5. kerf square — verify everything is complete
6. kerf finalize — package into a git branch for implementation

Don't skip the planning process. Measure twice, cut once.

--- END AGENT INSTRUCTIONS ---

3. VERIFY the setup by running:
   kerf new test-setup --title "Verify kerf setup"
   kerf show test-setup
   kerf delete test-setup --yes

That's it. kerf is ready to use.
================================================================================
`, projectID, jigNote)
}
