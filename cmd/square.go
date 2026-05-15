package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/dep"
	"github.com/gberns/kerf/internal/jig"
)

var squareCmd = &cobra.Command{
	Use:   "square <codename>",
	Short: "Verify work completeness against jig requirements",
	Args:  cobra.ExactArgs(1),
	RunE:  runSquare,
}

func init() {
	rootCmd.AddCommand(squareCmd)
}

// squareResult holds the outcome of a square check.
type squareResult struct {
	StatusPass     bool
	StatusDetail   string
	FilesPass      bool
	FilesPresent   int
	FilesTotal     int
	MissingFiles   []string
	ProcessPass    bool
	ProcessComplete int
	ProcessTotal    int
	ProcessDetails []string // "pass-name: done/active/pending"
	HasProcessPasses bool
	BeadTotal      int
	BeadClosed     int
	BeadOpen       int
	HasBeadInfo    bool
	DepsPass       bool
	DepsComplete   int
	DepsTotal      int
	IncompleteDeps []string
	UnresolveDeps  []string
}

func (r *squareResult) IsSquare() bool {
	if r.HasProcessPasses {
		return r.StatusPass && r.FilesPass && r.ProcessPass && r.DepsPass
	}
	return r.StatusPass && r.FilesPass && r.DepsPass
}

func runSquare(cmd *cobra.Command, args []string) error {
	codename := args[0]

	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	result, err := checkSquare(projectID, codename)
	if err != nil {
		return err
	}

	printSquareResult(codename, result)
	return nil
}

func checkSquare(projectID, codename string) (*squareResult, error) {
	s, workDir, err := cmdutil.LoadWorkWithChecks(projectID, codename)
	if err != nil {
		return nil, fmt.Errorf("work '%s' not found in project '%s'", codename, projectID)
	}

	bp, _ := bench.BenchPath()
	jigsDir := filepath.Join(bp, "jigs")
	jigDef, _, err := jig.Resolve(s.Jig, jigsDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve jig '%s': %w", s.Jig, err)
	}

	result := &squareResult{}

	// Status check
	terminal := jigDef.TerminalStatus()
	if jigDef.IsAtOrPastTerminal(s.Status) {
		result.StatusPass = true
		result.StatusDetail = fmt.Sprintf("%s (expected: %s or later)", s.Status, terminal)
	} else {
		result.StatusPass = false
		result.StatusDetail = fmt.Sprintf("%s (expected: %s or later)", s.Status, terminal)
	}

	// File check — detect components from existing directory structure
	components := detectComponents(workDir, jigDef.FileStructure)
	expectedFiles := jig.ExpandComponents(jigDef.FileStructure, components)
	result.FilesTotal = len(expectedFiles)
	for _, f := range expectedFiles {
		fullPath := filepath.Join(workDir, f)
		if _, err := os.Stat(fullPath); err == nil {
			result.FilesPresent++
		} else {
			result.MissingFiles = append(result.MissingFiles, f)
		}
	}
	result.FilesPass = result.FilesPresent == result.FilesTotal

	// Process pass check
	checkProcessPasses(jigDef, s.Status, projectID, result)

	// Dependency check
	blockingResults := dep.CheckBlocking(s.DependsOn, bp, projectID)
	result.DepsTotal = len(blockingResults)
	for _, dr := range blockingResults {
		if dr.Unresolvable {
			result.UnresolveDeps = append(result.UnresolveDeps, fmt.Sprintf("%s (project: %s — not found on bench)", dr.Codename, dr.Project))
			// Unresolvable deps don't fail the check
		} else if dr.Complete {
			result.DepsComplete++
		} else {
			result.IncompleteDeps = append(result.IncompleteDeps, fmt.Sprintf("%s [%s]", dr.Codename, dr.Status))
		}
	}
	// Pass if all resolvable deps are complete
	result.DepsPass = len(result.IncompleteDeps) == 0

	return result, nil
}

// checkProcessPasses identifies process passes and checks their completion
// by comparing the work's current status against each pass's status in the jig's ordering.
//
// Process passes are passes with empty output in composable jigs (e.g., the Implement and
// Complete passes in the implementation jig). Terminal passes with empty output in
// non-composable jigs (e.g., Ready in plan, Square in spike/retrofit) are NOT process passes.
// Per verification.md: "Process pass checks apply only to jigs that have process passes.
// Spec-writing jigs (plan, spec, bug) have only artifact passes and are unaffected."
func checkProcessPasses(jigDef *jig.JigDefinition, workStatus, projectID string, result *squareResult) {
	// Only composable jigs have process passes
	if !jigDef.Composable {
		return
	}

	// Build status index for ordering comparison
	statusIndex := make(map[string]int)
	for i, sv := range jigDef.StatusValues {
		statusIndex[sv] = i
	}

	var processPasses []jig.Pass
	for _, p := range jigDef.Passes {
		if len(p.Output) == 0 {
			processPasses = append(processPasses, p)
		}
	}

	if len(processPasses) == 0 {
		return
	}

	result.HasProcessPasses = true
	result.ProcessTotal = len(processPasses)

	// If the work has reached terminal status, all process passes are complete
	atTerminal := jigDef.IsAtOrPastTerminal(workStatus)

	workIdx, workKnown := statusIndex[workStatus]

	for _, p := range processPasses {
		passIdx, passKnown := statusIndex[p.Status]

		var complete bool
		if atTerminal {
			complete = true
		} else if !workKnown {
			// Work status not in list — past all known statuses
			complete = true
		} else if !passKnown {
			// Pass status not in list — cannot determine, treat as incomplete
			complete = false
		} else {
			complete = workIdx > passIdx
		}

		if complete {
			result.ProcessComplete++
			result.ProcessDetails = append(result.ProcessDetails, p.Name+": done")
		} else if workKnown && passKnown && workIdx == passIdx {
			result.ProcessDetails = append(result.ProcessDetails, p.Name+": active")
		} else {
			result.ProcessDetails = append(result.ProcessDetails, p.Name+": pending")
		}
	}

	result.ProcessPass = result.ProcessComplete == result.ProcessTotal

	// Try to get bead info for implementation jigs
	if jigDef.Name == "implementation" {
		tryLoadBeadInfo(projectID, result)
	}
}

// tryLoadBeadInfo attempts to read bead status via the configured beads CLI
// (project.yaml `tools.tasks`, default `br`). Fails silently if the tool is
// unavailable. Uses the canonical `br list` text output via internal/beads.
func tryLoadBeadInfo(projectID string, result *squareResult) {
	toolName := beads.DefaultToolName
	if r, err := cmdutil.Resolver(projectID); err == nil {
		if cfg, err := config.LoadProjectConfig(r.ProjectConfigPath()); err == nil && cfg != nil {
			toolName = beads.ResolveToolName(cfg.Tools)
		}
	}

	bs, err := beads.ListNamed(toolName)
	if err != nil || len(bs) == 0 {
		return
	}

	total := len(bs)
	closed := 0
	open := 0
	for _, b := range bs {
		switch strings.ToLower(b.Status) {
		case "closed", "done", "complete":
			closed++
		default:
			open++
		}
	}
	result.HasBeadInfo = true
	result.BeadTotal = total
	result.BeadClosed = closed
	result.BeadOpen = open
}

// parseBeadOutput extracts bead counts from beads-CLI text list output.
// It looks for lines containing "total", "closed", "open" with numeric values.
// Retained for back-compat / unit tests; new code paths use tryLoadBeadInfo
// which delegates to internal/beads.
func parseBeadOutput(output string) (total, closed, open int) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		label := strings.ToLower(fields[1])
		switch {
		case strings.HasPrefix(label, "total"):
			total = n
		case strings.HasPrefix(label, "closed"):
			closed = n
		case strings.HasPrefix(label, "open"):
			open = n
		}
	}
	return
}

// detectComponents scans the work directory to find component names
// by looking at directories/files that match {component} placeholder patterns.
func detectComponents(workDir string, fileStructure []string) []string {
	seen := make(map[string]bool)

	for _, pattern := range fileStructure {
		if !strings.Contains(pattern, "{component}") {
			continue
		}

		// Find the directory prefix before {component}
		idx := strings.Index(pattern, "{component}")
		prefix := pattern[:idx]

		// Check if {component} is a directory name (followed by /)
		if idx+len("{component}") < len(pattern) && pattern[idx+len("{component}")] == '/' {
			// Pattern like "03-research/{component}/findings.md"
			// List subdirs of the prefix directory
			dirPath := filepath.Join(workDir, prefix)
			entries, err := os.ReadDir(dirPath)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() {
					seen[e.Name()] = true
				}
			}
		} else {
			// Pattern like "04-plans/{component}-spec.md"
			// Extract component name from matching files
			dirPath := filepath.Join(workDir, filepath.Dir(pattern))
			suffix := pattern[idx+len("{component}"):]
			entries, err := os.ReadDir(dirPath)
			if err != nil {
				continue
			}
			prefixBase := filepath.Base(prefix)
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if strings.HasPrefix(name, prefixBase) && strings.HasSuffix(name, suffix) {
					comp := name[len(prefixBase) : len(name)-len(suffix)]
					if comp != "" {
						seen[comp] = true
					}
				}
			}
		}
	}

	var components []string
	for c := range seen {
		components = append(components, c)
	}
	return components
}

func printSquareResult(codename string, r *squareResult) {
	fmt.Printf("Square check for %s:\n\n", codename)

	// Status
	if r.StatusPass {
		fmt.Printf("  Status:        pass — %s\n", r.StatusDetail)
	} else {
		fmt.Printf("  Status:        fail — %s\n", r.StatusDetail)
	}

	// Files
	if r.FilesPass {
		fmt.Printf("  Files:         pass — %d/%d expected files present\n", r.FilesPresent, r.FilesTotal)
	} else {
		fmt.Printf("  Files:         fail — %d/%d expected files present\n", r.FilesPresent, r.FilesTotal)
		for _, f := range r.MissingFiles {
			fmt.Printf("    Missing:     %s\n", f)
		}
	}

	// Process passes
	if r.HasProcessPasses {
		if r.ProcessPass {
			fmt.Printf("  Process:       pass — %d/%d process passes complete\n", r.ProcessComplete, r.ProcessTotal)
		} else {
			fmt.Printf("  Process:       fail — %d/%d process passes complete\n", r.ProcessComplete, r.ProcessTotal)
		}
		if r.HasBeadInfo {
			fmt.Printf("    Beads:       %d total, %d closed, %d open\n", r.BeadTotal, r.BeadClosed, r.BeadOpen)
		}
	}

	// Dependencies
	if r.DepsTotal == 0 && len(r.UnresolveDeps) == 0 {
		fmt.Printf("  Dependencies:  pass — no blocking dependencies\n")
	} else if r.DepsPass {
		fmt.Printf("  Dependencies:  pass — %d/%d blocking dependencies complete\n", r.DepsComplete, r.DepsTotal)
	} else {
		fmt.Printf("  Dependencies:  fail — %d/%d blocking dependencies complete\n", r.DepsComplete, r.DepsTotal)
		for _, d := range r.IncompleteDeps {
			fmt.Printf("    Incomplete:  %s\n", d)
		}
	}
	if len(r.UnresolveDeps) > 0 {
		for _, d := range r.UnresolveDeps {
			fmt.Printf("    Unresolvable: %s\n", d)
		}
	}

	fmt.Println()
	if r.IsSquare() {
		fmt.Println("Result: SQUARE")
	} else {
		fmt.Println("Result: NOT SQUARE")
	}
}
