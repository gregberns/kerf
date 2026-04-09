package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/project"
	"github.com/gberns/kerf/internal/snapshot"
	"github.com/gberns/kerf/internal/spec"
)

var branchFlag string

var finalizeCmd = &cobra.Command{
	Use:   "finalize <codename> --branch <name>",
	Short: "Complete a work and hand off to implementation",
	Args:  cobra.ExactArgs(1),
	RunE:  runFinalize,
}

func init() {
	finalizeCmd.Flags().StringVar(&branchFlag, "branch", "", "Git branch name to create (required)")
	finalizeCmd.MarkFlagRequired("branch")
	rootCmd.AddCommand(finalizeCmd)
}

func runFinalize(cmd *cobra.Command, args []string) error {
	codename := args[0]

	if branchFlag == "" {
		return fmt.Errorf("--branch is required. Specify the branch name for the finalized work")
	}

	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	s, workDir, err := cmdutil.LoadWorkWithChecks(projectID, codename)
	if err != nil {
		return fmt.Errorf("work '%s' not found in project '%s'", codename, projectID)
	}

	// Find the target repo
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	repoRoot, err := project.FindGitRoot(cwd)
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	fmt.Printf("Finalizing %s...\n", codename)

	// Pre-flight 1: Square check
	sqResult, err := checkSquare(projectID, codename)
	if err != nil {
		return err
	}
	if !sqResult.IsSquare() {
		printSquareResult(codename, sqResult)
		return fmt.Errorf("work '%s' is not square. Fix the issues and try again", codename)
	}
	fmt.Println("  Square check: passed")

	// Pre-flight 2: Uncommitted changes
	if hasUncommittedChanges(repoRoot) {
		return fmt.Errorf("target repository has uncommitted changes. Commit or stash them before finalizing")
	}

	// Pre-flight 3: Branch doesn't exist
	if branchExists(repoRoot, branchFlag) {
		return fmt.Errorf("branch '%s' already exists in the target repository", branchFlag)
	}

	// Snapshot
	snapshot.Take(workDir, "")

	// Create branch
	if err := gitCmd(repoRoot, "checkout", "-b", branchFlag); err != nil {
		return fmt.Errorf("creating branch: %w", err)
	}
	fmt.Printf("  Branch created: %s\n", branchFlag)

	// Copy artifacts
	bp, _ := bench.BenchPath()
	cfgPath := filepath.Join(bp, "config.yaml")
	cfg, _ := config.Load(cfgPath)
	repoSpecPath := strings.ReplaceAll(cfg.EffectiveRepoSpecPath(), "{codename}", codename)
	destDir := filepath.Join(repoRoot, repoSpecPath)

	isSpecFirst := s.Jig == "spec"

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	if err := copyArtifacts(workDir, destDir, isSpecFirst); err != nil {
		return fmt.Errorf("copying artifacts: %w", err)
	}
	fmt.Printf("  Artifacts copied to: %s\n", repoSpecPath)

	// Spec-first: copy spec drafts to spec_path
	specDraftsCopied := false
	if isSpecFirst {
		specPath := cfg.EffectiveSpecPath()
		specDraftsDir := filepath.Join(workDir, "05-spec-drafts")
		copied, err := copySpecDrafts(specDraftsDir, filepath.Join(repoRoot, specPath))
		if err != nil {
			return fmt.Errorf("copying spec drafts: %w", err)
		}
		if copied {
			specDraftsCopied = true
			fmt.Printf("  Spec drafts applied to: %s\n", specPath)
		}
	}

	// Git add + commit
	if err := gitCmd(repoRoot, "add", repoSpecPath); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Stage spec_path for spec-first works
	if specDraftsCopied {
		specPath := cfg.EffectiveSpecPath()
		if err := gitCmd(repoRoot, "add", specPath); err != nil {
			return fmt.Errorf("git add spec_path: %w", err)
		}
	}

	commitMsg := fmt.Sprintf("kerf: finalize %s", codename)
	if err := gitCmd(repoRoot, "commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	// Get commit hash
	commitHash := getCommitHash(repoRoot)
	shortHash := commitHash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}
	fmt.Printf("  Commit: %s — %s\n", shortHash, commitMsg)

	// Update spec.yaml
	s.Implementation.Branch = &branchFlag
	s.Implementation.Commits = append(s.Implementation.Commits, commitHash)
	s.Status = "finalized"
	specPath := filepath.Join(workDir, "spec.yaml")
	if err := spec.Write(specPath, s); err != nil {
		return fmt.Errorf("updating spec.yaml: %w", err)
	}
	fmt.Println("  Status: finalized")

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  - Create a pull request for branch '%s'\n", branchFlag)
	fmt.Println("  - Notify the team / link external systems")
	fmt.Printf("  - Run 'kerf archive %s' when implementation is complete\n", codename)

	return nil
}

// copyArtifacts copies work files to the destination, excluding spec.yaml, SESSION.md, and .history/.
// If excludeSpecDrafts is true, 05-spec-drafts/ is also excluded (for spec-first works).
func copyArtifacts(workDir, destDir string, excludeSpecDrafts bool) error {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		name := e.Name()
		// Exclude spec.yaml, SESSION.md, and .history/
		if name == "spec.yaml" || name == "SESSION.md" || name == ".history" {
			continue
		}
		if excludeSpecDrafts && name == "05-spec-drafts" {
			continue
		}

		src := filepath.Join(workDir, name)
		dst := filepath.Join(destDir, name)

		if e.IsDir() {
			if err := copyDirRecursive(src, dst); err != nil {
				return err
			}
		} else {
			if err := copyFileSimple(src, dst); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDirRecursive(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFileSimple(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFileSimple(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

// copySpecDrafts copies files from 05-spec-drafts/ to the spec_path directory.
// Returns true if files were copied. Warns if the source directory is missing or empty.
func copySpecDrafts(specDraftsDir, destDir string) (bool, error) {
	entries, err := os.ReadDir(specDraftsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("  Warning: 05-spec-drafts/ not found, skipping spec draft copy")
			return false, nil
		}
		return false, err
	}

	if len(entries) == 0 {
		fmt.Println("  Warning: 05-spec-drafts/ is empty, skipping spec draft copy")
		return false, nil
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return false, fmt.Errorf("creating spec_path directory: %w", err)
	}

	for _, e := range entries {
		src := filepath.Join(specDraftsDir, e.Name())
		dst := filepath.Join(destDir, e.Name())
		if e.IsDir() {
			if err := copyDirRecursive(src, dst); err != nil {
				return false, err
			}
		} else {
			if err := copyFileSimple(src, dst); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

func hasUncommittedChanges(repoRoot string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return true // assume dirty on error
	}
	return strings.TrimSpace(string(out)) != ""
}

func branchExists(repoRoot, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}

func gitCmd(repoRoot string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getCommitHash(repoRoot string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
