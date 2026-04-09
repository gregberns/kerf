package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/jig"
	"github.com/gberns/kerf/internal/snapshot"
	"github.com/gberns/kerf/internal/spec"
)

var statusCmd = &cobra.Command{
	Use:   "status <codename> [new-status]",
	Short: "Get or set a work's status",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	codename := args[0]

	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	s, workDir, err := cmdutil.LoadWorkWithChecks(projectID, codename)
	if err != nil {
		return fmt.Errorf("work '%s' not found in project '%s'", codename, projectID)
	}

	// Load jig
	bp, _ := bench.BenchPath()
	jigsDir := filepath.Join(bp, "jigs")
	jigDef, _, _ := jig.Resolve(s.Jig, jigsDir)

	if len(args) == 1 {
		return statusRead(s, jigDef, codename)
	}

	return statusWrite(s, jigDef, workDir, codename, args[1], bp)
}

func statusRead(s *spec.SpecYAML, jigDef *jig.JigDefinition, codename string) error {
	fmt.Printf("Work: %s\n", codename)
	fmt.Printf("Status: %s\n", s.Status)
	fmt.Println()

	if jigDef != nil && len(jigDef.StatusValues) > 0 {
		fmt.Printf("Status progression (%s jig):\n", jigDef.Name)
		fmt.Println(statusProgression(jigDef.StatusValues, s.Status))
	} else if len(s.StatusValues) > 0 {
		fmt.Println("Status progression:")
		fmt.Println(statusProgression(s.StatusValues, s.Status))
	}

	return nil
}

func statusWrite(s *spec.SpecYAML, jigDef *jig.JigDefinition, workDir, codename, newStatus, benchPath string) error {
	oldStatus := s.Status

	// Warn if not in recommended list
	recommended := s.StatusValues
	if jigDef != nil {
		recommended = jigDef.StatusValues
	}
	isRecommended := false
	for _, sv := range recommended {
		if sv == newStatus {
			isRecommended = true
			break
		}
	}
	if !isRecommended && len(recommended) > 0 {
		jigName := "unknown"
		if jigDef != nil {
			jigName = jigDef.Name
		}
		fmt.Printf("Warning: '%s' is not in the %s jig's recommended statuses.\n", newStatus, jigName)
		fmt.Printf("Recommended: %s\n\n", strings.Join(recommended, ", "))
	}

	// Update status
	s.Status = newStatus
	specPath := filepath.Join(workDir, "spec.yaml")
	if err := spec.Write(specPath, s); err != nil {
		return fmt.Errorf("updating spec.yaml: %w", err)
	}

	// Take snapshot
	cfgPath := filepath.Join(benchPath, "config.yaml")
	cfg, _ := config.Load(cfgPath)
	if cfg.EffectiveSnapshotsEnabled() {
		snapshot.Take(workDir, "")
		snapshot.Prune(workDir, cfg.EffectiveMaxSnapshots())
	}

	fmt.Printf("Status updated: %s -> %s\n", oldStatus, newStatus)

	// Emit jig instructions for the new pass
	if jigDef != nil {
		pass := jigDef.PassForStatus(newStatus)
		if pass != nil {
			fmt.Println()
			instructions := jigDef.InstructionsForPass(pass.Name)
			if instructions != "" {
				fmt.Println(instructions)
			}
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Printf("  Work through the %s pass, producing:\n", pass.Name)
			for _, out := range pass.Output {
				fmt.Printf("    - %s\n", out)
			}
		}
	}

	return nil
}
