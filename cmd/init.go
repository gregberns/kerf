package cmd

import (
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

// beadFilterMode encodes the user's bead-filter resolution choice (kerf-pjs).
// Spec §"kerf init" step 9 precedence: --bead-filter > --no > --yes > default.
// The default (no flag) is identical to --yes: run the detector and accept a
// confident suggestion. kerf init is non-interactive — there is no prompt mode.
type beadFilterMode int

const (
	beadFilterModeDefault  beadFilterMode = iota // identical to --yes
	beadFilterModeYes                            // accept confident suggestion
	beadFilterModeNo                             // skip detection
	beadFilterModeExplicit                       // use the literal from --bead-filter
)

var (
	initJigFlag        string
	initForceFlag      bool
	initYesFlag        bool
	initNoFlag         bool
	initBeadFilterFlag string
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
	initCmd.Flags().BoolVar(&initYesFlag, "yes", false, "Accept the bead-filter detector's confident suggestion (default behaviour)")
	initCmd.Flags().BoolVar(&initNoFlag, "no", false, "Skip bead-filter detection; leave bead_filter unset")
	initCmd.Flags().StringVar(&initBeadFilterFlag, "bead-filter", "", "Explicit bead-filter literal (e.g. 'label=subsystem:auth'); bypasses the detector")
	rootCmd.AddCommand(initCmd)
}

// initStateChange is one row in the state-change summary block emitted at the
// end of `kerf init` output. It implements specs/commands.md §"kerf init"
// Output bullet 7 and specs/cli.md §"State-Change Summary (init)".
//
// Status is one of "created", "updated", "unchanged". Detail is optional
// trailing parenthetical text (e.g. the bead_filter literal or a follow-up
// command). Only artifacts init actually touched are recorded — init does not
// advertise state for artifacts it does not write.
type initStateChange struct {
	Name   string
	Status string
	Detail string
}

// initStateTracker collects state-change rows in registration order so the
// final summary block is stable and diffable across runs. Adding a new
// artifact is a single Record call at the site that owns it; downstream
// beads (e.g. B4's default_jig) layer on by registering their own row.
type initStateTracker struct {
	rows []initStateChange
}

func (t *initStateTracker) Record(name, status, detail string) {
	t.rows = append(t.rows, initStateChange{Name: name, Status: status, Detail: detail})
}

// Emit writes the fenced state-change summary block. The block is the last
// thing init prints on its successful paths. Columns are space-aligned so
// human-readable diffs stay clean; the trailing detail (when present) is
// rendered in parentheses.
func (t *initStateTracker) Emit(w io.Writer) {
	if len(t.rows) == 0 {
		return
	}
	width := 0
	for _, r := range t.rows {
		if len(r.Name) > width {
			width = len(r.Name)
		}
	}
	fmt.Fprintln(w, "State changes:")
	fmt.Fprintln(w, "```")
	for _, r := range t.rows {
		pad := strings.Repeat(" ", width-len(r.Name))
		if r.Detail != "" {
			fmt.Fprintf(w, "  %s%s   %s (%s)\n", r.Name, pad, r.Status, r.Detail)
		} else {
			fmt.Fprintf(w, "  %s%s   %s\n", r.Name, pad, r.Status)
		}
	}
	fmt.Fprintln(w, "```")
}

func runInit() error {
	// Resolve bead-filter flags up-front (kerf-pjs). Precedence per spec
	// §"kerf init" step 9: --bead-filter > --no > --yes > default (=--yes).
	if initYesFlag && initNoFlag {
		return fmt.Errorf("--yes and --no are mutually exclusive")
	}
	var explicitFilter *beads.Filter
	if initBeadFilterFlag != "" {
		f, perr := beads.ParseFilterClause(initBeadFilterFlag)
		if perr != nil {
			return fmt.Errorf("--bead-filter expects 'label=<value>' or 'id_prefix=<value>', got %q", initBeadFilterFlag)
		}
		explicitFilter = f
	}
	mode := beadFilterModeDefault
	switch {
	case explicitFilter != nil:
		mode = beadFilterModeExplicit
	case initNoFlag:
		mode = beadFilterModeNo
	case initYesFlag:
		mode = beadFilterModeYes
	}

	tracker := &initStateTracker{}

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
		tracker.Record(".kerf/project-identifier", "created", projectID)
	} else {
		fmt.Printf("Project already initialized: %s\n", projectID)
		tracker.Record(".kerf/project-identifier", "unchanged", projectID)
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
		tracker.Record("project.yaml", "unchanged", projCfgPath)
		if existingCfg.BeadFilter != nil && existingCfg.BeadFilter.Label != "" {
			tracker.Record("bead_filter", "unchanged", "label="+existingCfg.BeadFilter.Label)
		} else {
			tracker.Record("bead_filter", "unchanged", "unset; run 'kerf config bead_filter <expr>' to set")
		}

		// Run kerf setup — the single source of the AGENT SETUP INSTRUCTIONS
		// block (specs/commands.md §kerf init step 11).
		fmt.Println()
		if err := runSetup(); err != nil {
			fmt.Printf("Note: could not generate setup instructions: %v\n", err)
		}
		fmt.Println()
		fmt.Println("Use 'kerf init --force' to overwrite project.yaml, or edit it directly.")
		fmt.Println()
		tracker.Emit(os.Stdout)
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
	// If --bead-filter was supplied, use it verbatim; otherwise the detector
	// decides per mode (kerf-pjs).
	var detectedFilter *beads.Filter
	if mode == beadFilterModeExplicit {
		detectedFilter = explicitFilter
	} else {
		detectedFilter = detectBeadFilter(resolver, mode, os.Stdout, priorFilter)
	}

	if err := createDefaultProjectConfig(projCfgPath, detectedFilter); err != nil {
		return fmt.Errorf("creating project.yaml: %w", err)
	}
	if existingCfg != nil {
		tracker.Record("project.yaml", "updated", projCfgPath)
	} else {
		tracker.Record("project.yaml", "created", projCfgPath)
	}
	switch {
	case detectedFilter != nil && detectedFilter.Label != "" && (existingCfg == nil || existingCfg.BeadFilter == nil || existingCfg.BeadFilter.Label != detectedFilter.Label):
		verb := "created"
		if existingCfg != nil && existingCfg.BeadFilter != nil {
			verb = "updated"
		}
		tracker.Record("bead_filter", verb, "label="+detectedFilter.Label)
	case detectedFilter != nil && detectedFilter.Label != "":
		// --force re-run preserved the same prior filter.
		tracker.Record("bead_filter", "unchanged", "label="+detectedFilter.Label)
	default:
		tracker.Record("bead_filter", "unchanged", "detector returned no confident suggestion; run 'kerf config bead_filter <expr>' to set")
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

	// Run kerf setup — the single source of the AGENT SETUP INSTRUCTIONS
	// block (specs/commands.md §kerf init step 11). The earlier
	// bootstrapInstructions helper was removed in b40df97 but this call
	// site was missed, leaving the package broken at HEAD — fixed in
	// passing here so plan 021 tests can compile.
	fmt.Println()
	if err := runSetup(); err != nil {
		// Non-fatal: setup may fail if project resolution differs, but init already succeeded
		fmt.Printf("Note: could not generate setup instructions: %v\n", err)
	}

	fmt.Println()
	tracker.Emit(os.Stdout)
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

// detectBeadFilter implements specs/commands.md §"kerf init" step 9.
//
// The function is non-interactive: it never reads from stdin (kerf-pjs).
// Resolution is driven by the supplied beadFilterMode:
//
//   - beadFilterModeNo: skip detection entirely, return priorFilter unchanged.
//   - beadFilterModeYes / beadFilterModeDefault: run the detector; on a
//     confident suggestion, return it; otherwise return priorFilter.
//
// beadFilterModeExplicit is handled by the caller before this function runs.
// priorFilter (when non-nil) carries the user's existing bead_filter from a
// pre-existing project.yaml and is preserved verbatim when the detector has
// no confident suggestion, so --force never silently discards a user value.
func detectBeadFilter(resolver *storage.Resolver, mode beadFilterMode, stdout io.Writer, priorFilter *beads.Filter) *beads.Filter {
	if mode == beadFilterModeNo {
		return priorFilter
	}

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
	prefix, score, _, confidence := beads.DetectFilterPrefixConfidence(all, codenames)

	// Only ConfidenceConfident produces a suggestion. ConfidenceLow and
	// ConfidenceNone both stay silent per Plan 016 Open Q 2 — kerf init
	// leaves bead_filter unset and the agent can set it later.
	if confidence == beads.ConfidenceConfident && prefix != "" {
		filter := &beads.Filter{Label: prefix + ":{codename}"}
		fmt.Fprintf(stdout, "Detected: %d%% of beads use `%s:*` labels. Setting project-wide bead_filter to `%s:{codename}`.\n",
			int(score*100+0.5), prefix, prefix)
		return filter
	}

	// No confident candidate — preserve any prior filter; otherwise leave unset.
	return priorFilter
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

