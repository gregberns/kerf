package cmd

import (
	"errors"
	"fmt"
	"os"
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

var statusQuiet bool

var statusCmd = &cobra.Command{
	Use:   "status <codename> [new-status]",
	Short: "Get or set a work's status",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusQuiet, "quiet", false, "Suppress jig-instructions block; emit only the single-line transition confirmation")
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

	// Pre-create the next pass's output directory (and copy template if any).
	// Idempotent: existing directories and files are left alone.
	var pass *jig.Pass
	if jigDef != nil {
		pass = jigDef.PassForStatus(newStatus)
		if pass != nil {
			if err := preCreatePassOutputs(workDir, jigDef.Name, pass); err != nil {
				// Don't fail the status advance on a pre-create hiccup; surface a warning.
				fmt.Fprintf(os.Stderr, "warning: pre-create pass outputs: %v\n", err)
			}
		}
	}

	if statusQuiet {
		return nil
	}

	// Emit jig instructions for the new pass
	if jigDef != nil && pass != nil {
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

	return nil
}

// preCreatePassOutputs ensures the directory prefix of each output declared
// by the pass exists, and copies the matching template into place when the
// target file is absent. Output paths containing `{component}` defer to a
// later pre-creation step — only the static directory prefix is created.
func preCreatePassOutputs(workDir, jigName string, pass *jig.Pass) error {
	if pass == nil {
		return nil
	}
	for _, out := range pass.Output {
		dir, file, hasComponent := splitPassOutput(out)
		if dir != "" {
			absDir := filepath.Join(workDir, dir)
			if err := os.MkdirAll(absDir, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", absDir, err)
			}
		}
		// Skip template copy when the output path still has unresolved
		// {component} segments — we don't know the target filename yet.
		if hasComponent || file == "" {
			continue
		}
		target := filepath.Join(workDir, dir, file)
		if _, err := os.Stat(target); err == nil {
			continue // exists — leave it alone
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", target, err)
		}
		data, err := jig.TemplateForPass(jigName, pass)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // no template ships for this pass
			}
			continue // template lookup failed (treat as missing)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("copy template to %s: %w", target, err)
		}
	}
	return nil
}

// splitPassOutput splits a pass output path into (dirPrefix, fileName, hasComponent).
// `{component}` placeholders defer template copying: hasComponent is true when
// the path contains one. The returned dirPrefix is the longest leading run of
// path segments that does not contain `{component}`.
func splitPassOutput(out string) (string, string, bool) {
	parts := strings.Split(out, "/")
	hasComponent := false
	for _, p := range parts {
		if strings.Contains(p, "{component}") {
			hasComponent = true
			break
		}
	}
	// Identify the static prefix: segments up to (but not including) the first
	// `{component}` segment.
	var prefix []string
	for _, p := range parts[:len(parts)-1] {
		if strings.Contains(p, "{component}") {
			break
		}
		prefix = append(prefix, p)
	}
	dir := strings.Join(prefix, "/")
	file := parts[len(parts)-1]
	if hasComponent {
		// Don't return a filename if any segment along the way is templated.
		return dir, "", true
	}
	return dir, file, false
}
