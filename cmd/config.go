package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config [key] [value]",
	Short: "View or modify kerf configuration",
	Long: `View or modify kerf configuration.

kerf has two configuration layers:

  • Bench-wide  — ~/.kerf/config.yaml (applies to every project)
  • Project    — ~/.kerf/projects/<id>/project.yaml (per-project overrides)

Routing of dot-notation keys to a layer:

  • tools.<name>      → project.yaml (e.g. tools.tasks)
  • doctor.footer     → project.yaml
  • bead_filter       → project.yaml (read-only via this command)
  • default_jig       → BOTH bench and project (project value wins)
  • everything else   → bench config.yaml

Project-scoped writes resolve the current project from cwd (.kerf/project-identifier
or git remote), or honor --project.

Examples:
  kerf config                          Display all bench settings
  kerf config default_jig              Display value (project wins, falls back to bench)
  kerf config default_jig bug          Set value in bench and project
  kerf config tools.tasks bd           Set per-project tasks-tool binary
  kerf config snapshots.enabled false  Set nested bench value`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfig(args)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

// Project-scoped keys that this command knows how to route to project.yaml.
// Note: any `tools.<name>` is also a project-scoped key; matched by prefix.
var projectScopedKeys = []string{
	"tools.tasks",
	"default_jig",
	"doctor.footer",
	"bead_filter",
}

// isProjectScopedKey reports whether the dot-notation key writes to
// project.yaml. `default_jig` is dual-scoped (handled separately).
func isProjectScopedKey(key string) bool {
	if strings.HasPrefix(key, "tools.") {
		return true
	}
	switch key {
	case "default_jig", "doctor.footer", "bead_filter":
		return true
	}
	return false
}

func runConfig(args []string) error {
	bp, err := bench.BenchPath()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(bp, "config.yaml")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	switch len(args) {
	case 0:
		return configShowAll(cfg, cfgPath)
	case 1:
		return configGet(cfg, args[0])
	case 2:
		return configSet(cfg, cfgPath, bp, args[0], args[1])
	}
	return nil
}

func configShowAll(cfg *config.Config, cfgPath string) error {
	fmt.Printf("kerf configuration (%s):\n", cfgPath)
	for _, key := range config.ValidKeys() {
		val, _ := cfg.Get(key)
		fmt.Printf("  %-32s %s\n", key+":", val)
	}
	return nil
}

func configGet(cfg *config.Config, key string) error {
	// Project-scoped or dual-scoped keys: try project.yaml first, fall back
	// to bench (for default_jig) or just project (others).
	if isProjectScopedKey(key) {
		val, found, err := getProjectScoped(key)
		if err != nil {
			return err
		}
		if found {
			fmt.Printf("%s: %s\n", key, val)
			return nil
		}
		// default_jig falls back to bench.
		if key == "default_jig" {
			val, gerr := cfg.Get(key)
			if gerr == nil {
				fmt.Printf("%s: %s\n", key, val)
				return nil
			}
		}
		fmt.Printf("%s: \n", key)
		return nil
	}

	val, err := cfg.Get(key)
	if err != nil {
		return unknownKeyError(key)
	}
	fmt.Printf("%s: %s\n", key, val)
	return nil
}

func configSet(cfg *config.Config, cfgPath, benchPath, key, value string) error {
	// Route to project.yaml for project-scoped keys. default_jig also
	// writes to bench (per specs/architecture.md §"Project Configuration"
	// and specs/commands.md §"kerf init" — project value wins).
	if isProjectScopedKey(key) {
		if key == "bead_filter" {
			return fmt.Errorf("'bead_filter' is read-only via 'kerf config'. Use 'kerf init', 'kerf bootstrap-filters', or 'kerf work edit --bead-filter-add' to set it")
		}
		if err := setProjectScoped(key, value); err != nil {
			return err
		}
		if key == "default_jig" {
			// Also write to bench for non-project contexts.
			if err := cfg.Set(key, value); err != nil {
				return err
			}
			if err := os.MkdirAll(benchPath, 0o755); err != nil {
				return fmt.Errorf("creating bench directory: %w", err)
			}
			if err := config.Save(cfgPath, cfg); err != nil {
				return err
			}
		}
		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	}

	// Bench-scoped key.
	if err := cfg.Set(key, value); err != nil {
		// Wrap unknown-key error so the message lists all valid surfaces.
		if strings.Contains(err.Error(), "unknown configuration key") {
			return unknownKeyError(key)
		}
		return err
	}

	if err := os.MkdirAll(benchPath, 0o755); err != nil {
		return fmt.Errorf("creating bench directory: %w", err)
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

// unknownKeyError builds the canonical error for unknown keys, listing
// project-scoped surfaces alongside bench keys so agents can self-correct.
// Project-scoped and bench key lists overlap (e.g. default_jig is dual-scoped),
// so dedupe while preserving project-first canonical order.
func unknownKeyError(key string) error {
	seen := make(map[string]struct{})
	ordered := make([]string, 0, len(projectScopedKeys)+len(config.ValidKeys()))
	for _, k := range projectScopedKeys {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		ordered = append(ordered, k)
	}
	for _, k := range config.ValidKeys() {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		ordered = append(ordered, k)
	}
	return fmt.Errorf("unknown configuration key '%s'. Valid keys: %s", key, strings.Join(ordered, ", "))
}

// getProjectScoped returns the value of a project-scoped key from the
// active project's project.yaml. Second return is false when the key is
// unset (caller decides fallback behavior).
func getProjectScoped(key string) (string, bool, error) {
	pcfg, _, err := loadActiveProjectConfig()
	if err != nil {
		// Not in a project context: treat as unset rather than fatal so
		// `kerf config get tools.tasks` outside a project still prints
		// an empty value.
		return "", false, nil
	}
	switch {
	case strings.HasPrefix(key, "tools."):
		name := strings.TrimPrefix(key, "tools.")
		if pcfg.Tools == nil {
			return "", false, nil
		}
		v, ok := pcfg.Tools[name]
		return v, ok && v != "", nil
	case key == "default_jig":
		return pcfg.DefaultJig, pcfg.DefaultJig != "", nil
	case key == "doctor.footer":
		if pcfg.Doctor == nil || pcfg.Doctor.Footer == nil {
			return "", false, nil
		}
		return strconv.FormatBool(*pcfg.Doctor.Footer), true, nil
	case key == "bead_filter":
		if pcfg.BeadFilter == nil {
			return "", false, nil
		}
		// Render the filter as a one-line YAML mapping for display.
		return fmt.Sprintf("%+v", pcfg.BeadFilter), true, nil
	}
	return "", false, nil
}

// setProjectScoped writes a single key into the active project's
// project.yaml, preserving other fields.
func setProjectScoped(key, value string) error {
	pcfg, path, err := loadActiveProjectConfig()
	if err != nil {
		return err
	}
	switch {
	case strings.HasPrefix(key, "tools."):
		name := strings.TrimPrefix(key, "tools.")
		if name == "" {
			return fmt.Errorf("invalid tools key: %q", key)
		}
		if pcfg.Tools == nil {
			pcfg.Tools = map[string]string{}
		}
		pcfg.Tools[name] = value
	case key == "default_jig":
		pcfg.DefaultJig = value
	case key == "doctor.footer":
		b, perr := strconv.ParseBool(value)
		if perr != nil {
			return fmt.Errorf("invalid value for 'doctor.footer': must be true or false")
		}
		if pcfg.Doctor == nil {
			pcfg.Doctor = &config.DoctorConfig{}
		}
		pcfg.Doctor.Footer = &b
	default:
		return unknownKeyError(key)
	}
	return config.SaveProjectConfig(path, pcfg)
}

// loadActiveProjectConfig resolves the current project and loads its
// project.yaml. The path is returned so callers can write back.
func loadActiveProjectConfig() (*config.ProjectConfig, string, error) {
	projectID, err := cmdutil.ResolveProject("")
	if err != nil {
		return nil, "", fmt.Errorf("cannot determine project for project-scoped config: %w", err)
	}
	r, err := cmdutil.Resolver(projectID)
	if err != nil {
		return nil, "", err
	}
	path := r.ProjectConfigPath()
	pcfg, err := config.LoadProjectConfig(path)
	if err != nil {
		return nil, "", err
	}
	if pcfg == nil {
		pcfg = &config.ProjectConfig{}
	}
	return pcfg, path, nil
}
