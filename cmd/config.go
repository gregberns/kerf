package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config [key] [value]",
	Short: "View or modify bench configuration",
	Long: `View or modify bench configuration.

Examples:
  kerf config                          Display all settings
  kerf config default_jig              Display single value
  kerf config default_jig bug          Set value
  kerf config snapshots.enabled false  Set nested value`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfig(args)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
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
	val, err := cfg.Get(key)
	if err != nil {
		return fmt.Errorf("unknown configuration key '%s'", key)
	}
	fmt.Printf("%s: %s\n", key, val)
	return nil
}

func configSet(cfg *config.Config, cfgPath, benchPath, key, value string) error {
	if err := cfg.Set(key, value); err != nil {
		return err
	}

	// Ensure bench directory exists before writing config.
	if err := os.MkdirAll(benchPath, 0755); err != nil {
		return fmt.Errorf("creating bench directory: %w", err)
	}

	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}
