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

var (
	initJigFlag   string
	initForceFlag bool
)

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
	initCmd.Flags().BoolVar(&initForceFlag, "force", false, "Re-run init even when project.yaml already exists")
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

	// Resolve storage so we know where project.yaml should live (local vs bench).
	resolver, err := storage.NewResolver(benchPath, projectID, gitRoot)
	if err != nil {
		return fmt.Errorf("resolving storage mode: %w", err)
	}
	projCfgPath := resolver.ProjectConfigPath()

	// Step 4 (specs/commands.md §"kerf init"): detect existing project.yaml and
	// dispatch per the re-run rule.
	var existingCfg *config.ProjectConfig
	if _, statErr := os.Stat(projCfgPath); statErr == nil {
		existingCfg, err = config.LoadProjectConfig(projCfgPath)
		if err != nil {
			if initForceFlag {
				return fmt.Errorf("--force requested but existing project.yaml at %s is unreadable: %v. Move or delete the file manually before re-running.", projCfgPath, err)
			}
			// Without --force we still want to be informative but cannot
			// summarise; fall through to the skip-path with what we have.
			existingCfg = nil
		}
	}

	if existingCfg != nil && !initForceFlag {
		// Skip-with-informative-output path. Steps 8–10 are skipped; the
		// safe-to-repeat steps (project-identifier, --jig, kerf setup) already
		// ran above or run below.
		printExistingProjectSummary(projCfgPath, existingCfg)

		// Run kerf setup — the single source of the AGENT SETUP INSTRUCTIONS
		// block (specs/commands.md §kerf init step 11).
		fmt.Println()
		if err := runSetup(); err != nil {
			fmt.Printf("Note: could not generate setup instructions: %v\n", err)
		}
		fmt.Println()
		fmt.Println("Use 'kerf init --force' to overwrite project.yaml, or edit it directly.")
		return nil
	}

	// --force overwrite path or fresh init.
	if existingCfg != nil && initForceFlag {
		fmt.Printf("WARNING: overwriting existing project.yaml at %s\n", projCfgPath)
	}

	// Step 9 (specs/commands.md §"kerf init"): auto-detect bead_filter.
	// Best-effort: errors here never fail init. When --force is re-running over
	// an existing config we seed the detection with the prior filter so that
	// (a) non-interactively the existing filter is preserved verbatim, and
	// (b) interactively the previous value is offered as the default answer.
	var priorFilter *beads.Filter
	if existingCfg != nil {
		priorFilter = existingCfg.BeadFilter
	}
	detectedFilter := detectBeadFilter(resolver, os.Stdin, os.Stdout, priorFilter)

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

// detectBeadFilter implements specs/commands.md §"kerf init" step 9.
// Returns a *beads.Filter when a confident candidate is found and accepted (or
// auto-applied non-interactively), or nil to leave bead_filter unset.
// Failures degrade silently — init must always succeed.
//
// priorFilter (when non-nil) carries the user's existing bead_filter from a
// pre-existing project.yaml. In a --force re-init it is used so that:
//   - non-interactively, the prior filter is preserved verbatim (no silent
//     replacement), and
//   - interactively, the prior literal is offered as the default response in
//     the confirmation prompt.
func detectBeadFilter(resolver *storage.Resolver, stdin *os.File, stdout io.Writer, priorFilter *beads.Filter) *beads.Filter {
	interactive := isInteractiveStdin(stdin)

	// Honor project.yaml tools.tasks (default "br") so detection runs against
	// the same bead store the rest of kerf will use after init.
	detectTool := beads.DefaultToolName
	if cfg, cerr := config.LoadProjectConfig(resolver.ProjectConfigPath()); cerr == nil && cfg != nil {
		detectTool = beads.ResolveToolName(cfg.Tools)
	}
	if !beads.IsAvailableNamed(detectTool) {
		// Tool missing: cannot detect. Preserve any prior filter rather than
		// silently dropping it on a --force re-run.
		return priorFilter
	}

	all, err := beads.ListNamed(detectTool)
	if err != nil || len(all) == 0 {
		// Bead store empty or unreachable → silent skip; preserve prior.
		return priorFilter
	}

	codenames, _ := resolver.ListWorks()
	// Per spec step 9.2: with zero codenames we cannot correlate prefixes with
	// works. Non-interactively we preserve the prior filter (or leave unset);
	// interactively we still offer the user the top-by-count fallback so a
	// fresh project (no works yet) can pick a prefix at init time. This is the
	// repair for the A:F3 regression: previously, an empty works directory
	// returned silently, so detection never fired even when the store had a
	// dominant prefix.
	prefix, score, top := beads.DetectFilterPrefix(all, codenames)

	if prefix != "" {
		filter := &beads.Filter{Label: prefix + ":{codename}"}
		if !interactive {
			fmt.Fprintf(stdout, "Detected: %d%% of beads use `%s:*` labels. Setting project-wide bead_filter to `%s:{codename}`.\n",
				int(score*100+0.5), prefix, prefix)
			return filter
		}
		fmt.Fprintf(stdout, "Detected: %d%% of beads use `%s:*` labels.\n", int(score*100+0.5), prefix)
		if priorFilter != nil && priorFilter.Label != "" && priorFilter.Label != filter.Label {
			fmt.Fprintf(stdout, "Current project bead_filter is `%s`. Replace with `%s:{codename}`? [y/N] ", priorFilter.Label, prefix)
			if confirmNoDefault(stdin) {
				return filter
			}
			return priorFilter
		}
		fmt.Fprintf(stdout, "Set project-wide bead_filter to `%s:{codename}`? [Y/n] ", prefix)
		if confirmYesDefault(stdin) {
			return filter
		}
		return priorFilter
	}

	// No confident candidate. Non-interactive: preserve any prior filter.
	if !interactive {
		return priorFilter
	}
	if len(top) == 0 {
		return priorFilter
	}
	chosen := promptFallbackPrefix(top, stdin, stdout)
	if chosen == nil {
		return priorFilter
	}
	return chosen
}

// confirmNoDefault reads a line from stdin and returns true only for explicit
// "y" / "yes" answers; "", "n", "no" all return false. Used when the safer
// default is to NOT replace an existing value.
func confirmNoDefault(stdin *os.File) bool {
	if stdin == nil {
		return false
	}
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
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

// printExistingProjectSummary prints the resolved project.yaml path, the
// active jigs (with passes for composable jigs), and the current bead_filter
// (or "built-in default" if unset). Used on the skip-with-informative-output
// path when kerf init detects an existing project.yaml without --force.
func printExistingProjectSummary(path string, cfg *config.ProjectConfig) {
	fmt.Printf("project.yaml already exists at %s — skipping re-initialisation.\n", path)
	if len(cfg.Jigs) > 0 {
		fmt.Printf("  Active jigs: %s\n", strings.Join(cfg.Jigs, ", "))
	} else {
		fmt.Println("  Active jigs: (none)")
	}
	for _, j := range cfg.Jigs {
		if passes, ok := cfg.Passes[j]; ok && len(passes) > 0 {
			fmt.Printf("    %s passes: %s\n", j, strings.Join(passes, ", "))
		}
	}
	if cfg.BeadFilter != nil && cfg.BeadFilter.Label != "" {
		fmt.Printf("  bead_filter: label=%s\n", cfg.BeadFilter.Label)
	} else {
		fmt.Println("  bead_filter: (built-in default)")
	}
}

