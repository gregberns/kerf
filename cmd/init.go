package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/jig"
	"github.com/gberns/kerf/internal/project"
	"github.com/gberns/kerf/internal/storage"
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

	// Create project.yaml with all available jigs. If the repo already declares
	// storage: local, write project.yaml inside the repo and create the bench
	// symlink so the project is fully wired up.
	resolver, err := storage.NewResolver(benchPath, projectID, gitRoot)
	if err != nil {
		return fmt.Errorf("resolving storage mode: %w", err)
	}

	// Step 8 (specs/commands.md §"kerf init"): auto-detect bead_filter.
	// Best-effort: errors here never fail init.
	detectedFilter := detectBeadFilter(resolver, os.Stdin, os.Stdout)

	projCfgPath := resolver.ProjectConfigPath()
	if err := createDefaultProjectConfig(projCfgPath, detectedFilter); err != nil {
		return fmt.Errorf("creating project.yaml: %w", err)
	}
	if resolver.Mode == storage.ModeLocal {
		worksDir := resolver.WorksDir()
		if err := os.MkdirAll(worksDir, 0o755); err != nil {
			return fmt.Errorf("creating works directory: %w", err)
		}
		link := filepath.Join(benchPath, "projects", projectID)
		if err := storage.EnsureSymlink(link, worksDir); err != nil {
			return fmt.Errorf("creating bench symlink: %w", err)
		}
	}

	// Print the bootstrap instructions
	fmt.Print(bootstrapInstructions(projectID, cfg.EffectiveDefaultJig()))

	// Run kerf setup to generate agent-facing instructions
	fmt.Println()
	if err := runSetup(); err != nil {
		// Non-fatal: setup may fail if project resolution differs, but init already succeeded
		fmt.Printf("Note: could not generate setup instructions: %v\n", err)
	}

	return nil
}

// createDefaultProjectConfig creates project.yaml with all available jigs.
// For composable jigs, all passes are included by default. If beadFilter is
// non-nil, it is written into the project config as the project-wide filter.
func createDefaultProjectConfig(path string, beadFilter *beads.Filter) error {
	jigsDir := userJigsDir()
	summaries, err := jig.ListAll(jigsDir)
	if err != nil {
		return fmt.Errorf("listing jigs: %w", err)
	}

	var jigNames []string
	passes := make(map[string][]string)

	for _, s := range summaries {
		jigNames = append(jigNames, s.Name)

		// For composable jigs, include all passes by default
		if s.Composable {
			def, _, err := jig.Resolve(s.Name, jigsDir)
			if err != nil {
				continue
			}
			var passNames []string
			for _, p := range def.Passes {
				passNames = append(passNames, p.Name)
			}
			if len(passNames) > 0 {
				passes[s.Name] = passNames
			}
		}
	}

	projCfg := &config.ProjectConfig{
		Jigs:       jigNames,
		Passes:     passes,
		BeadFilter: beadFilter,
	}
	if len(passes) == 0 {
		projCfg.Passes = nil
	}

	if err := config.SaveProjectConfig(path, projCfg); err != nil {
		return err
	}

	fmt.Printf("Created project.yaml with %d active jigs: %s\n", len(jigNames), strings.Join(jigNames, ", "))

	return nil
}

// isInteractiveStdin returns true when stdin is attached to a terminal (i.e.
// character device). When stdin is piped/redirected (the typical CI or
// scripted-init case), the auto-detect step does not prompt.
func isInteractiveStdin(in *os.File) bool {
	if in == nil {
		return false
	}
	fi, err := in.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// detectBeadFilter implements specs/commands.md §"kerf init" step 8.
// Returns a *beads.Filter when a confident candidate is found and accepted (or
// auto-applied non-interactively), or nil to leave bead_filter unset.
// Failures degrade silently — init must always succeed.
func detectBeadFilter(resolver *storage.Resolver, stdin *os.File, stdout io.Writer) *beads.Filter {
	if !beads.IsAvailable() {
		return nil
	}

	codenames, err := resolver.ListWorks()
	if err != nil || len(codenames) == 0 {
		// No existing works → cannot correlate label prefixes with codenames.
		// Per spec: skip auto-detect entirely, do not prompt.
		return nil
	}

	all, err := beads.List()
	if err != nil || len(all) == 0 {
		// Bead store empty or unreachable → silent skip.
		return nil
	}

	prefix, score, top := beads.DetectFilterPrefix(all, codenames)
	interactive := isInteractiveStdin(stdin)

	if prefix != "" {
		filter := &beads.Filter{Label: prefix + ":{codename}"}
		if !interactive {
			fmt.Fprintf(stdout, "Detected: %d%% of beads use `%s:*` labels. Setting project-wide bead_filter to `%s:{codename}`.\n",
				int(score*100+0.5), prefix, prefix)
			return filter
		}
		fmt.Fprintf(stdout, "Detected: %d%% of beads use `%s:*` labels.\n", int(score*100+0.5), prefix)
		fmt.Fprintf(stdout, "Set project-wide bead_filter to `%s:{codename}`? [Y/n] ", prefix)
		if confirmYesDefault(stdin) {
			return filter
		}
		return nil
	}

	// No confident candidate. Non-interactive: write nothing.
	if !interactive {
		return nil
	}
	if len(top) == 0 {
		return nil
	}
	return promptFallbackPrefix(top, stdin, stdout)
}

// confirmYesDefault reads a line from stdin and returns true for "", "y", "Y",
// "yes", "YES". Anything else is treated as "no". Read errors return false.
func confirmYesDefault(stdin *os.File) bool {
	if stdin == nil {
		return false
	}
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "" || answer == "y" || answer == "yes"
}

// promptFallbackPrefix presents the top-by-count prefixes plus an option to
// type a custom prefix or skip, and returns the user's choice as a Filter (or
// nil to skip).
func promptFallbackPrefix(top []beads.PrefixCount, stdin *os.File, stdout io.Writer) *beads.Filter {
	fmt.Fprintln(stdout, "No dominant label prefix detected. Top prefixes by raw count:")
	for i, pc := range top {
		fmt.Fprintf(stdout, "  %d. %s:* (%d beads)\n", i+1, pc.Prefix, pc.Count)
	}
	custom := len(top) + 1
	skip := len(top) + 2
	fmt.Fprintf(stdout, "  %d. type your own\n", custom)
	fmt.Fprintf(stdout, "  %d. skip\n", skip)
	fmt.Fprintf(stdout, "Choose [1-%d]: ", skip)

	reader := bufio.NewReader(stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.TrimSpace(line)
	if answer == "" {
		return nil
	}

	// Numeric choice.
	for i := range top {
		if answer == fmt.Sprintf("%d", i+1) {
			return &beads.Filter{Label: top[i].Prefix + ":{codename}"}
		}
	}
	if answer == fmt.Sprintf("%d", custom) {
		fmt.Fprint(stdout, "Enter prefix (without trailing ':'): ")
		line, _ := reader.ReadString('\n')
		p := strings.TrimSpace(line)
		p = strings.TrimSuffix(p, ":")
		if p == "" {
			return nil
		}
		return &beads.Filter{Label: p + ":{codename}"}
	}
	// "skip" or anything unrecognized → no filter.
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
- Implementation from existing spec → kerf new --jig implementation
- Quick explorations → kerf new --jig spike
- Retrofitting specs to existing code → kerf new --jig retrofit
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
