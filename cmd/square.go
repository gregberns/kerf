package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
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
	StatusPass    bool
	StatusDetail  string
	FilesPass     bool
	FilesPresent  int
	FilesTotal    int
	MissingFiles  []string
	DepsPass      bool
	DepsComplete  int
	DepsTotal     int
	IncompleteDeps []string
	UnresolveDeps  []string
}

func (r *squareResult) IsSquare() bool {
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
