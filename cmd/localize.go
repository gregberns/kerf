package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/project"
	"github.com/gberns/kerf/internal/storage"
)

var localizeCheckFlag bool

var localizeCmd = &cobra.Command{
	Use:   "localize",
	Short: "Migrate this project from bench storage to local (in-repo) storage",
	Long: `Move all in-progress works for the current project from the bench
(~/.kerf/projects/{project-id}/) into the repo at .kerf/works/. Writes
.kerf/config.yaml with storage: local, and creates a symlink on the bench
pointing at the repo's works directory so cross-project queries still work.

Use --check (alias --dry-run) to preview the migration without touching disk.

To go back to bench storage (no automated command in v1):
  1. mv .kerf/works/* ~/.kerf/projects/{project-id}/
  2. mv .kerf/project.yaml ~/.kerf/projects/{project-id}/project.yaml
  3. rm ~/.kerf/projects/{project-id}  (the symlink)
  4. mkdir ~/.kerf/projects/{project-id} and move works back, or just rename
  5. Remove the storage field from .kerf/config.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if localizeCheckFlag {
			return runLocalizeCheck()
		}
		return runLocalize()
	},
}

func init() {
	localizeCmd.Flags().BoolVar(&localizeCheckFlag, "check", false, "Preview the migration without changing anything on disk")
	localizeCmd.Flags().BoolVar(&localizeCheckFlag, "dry-run", false, "Alias for --check")
	rootCmd.AddCommand(localizeCmd)
}

// runLocalizeCheck performs steps 1–5 of the localize flow (resolution and
// pre-flight verification) and prints the planned moves without mutating
// anything on disk. See specs/commands.md `kerf localize --check`.
func runLocalizeCheck() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	repoRoot, err := project.FindGitRoot(cwd)
	if err != nil {
		if projectFlag == "" {
			return fmt.Errorf("not in a git repository. Use --project <project-id> to specify a project")
		}
		return fmt.Errorf("kerf localize must be run from inside the target repo")
	}

	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	bp, err := bench.BenchPath()
	if err != nil {
		return err
	}

	repoCfg, err := storage.LoadRepoConfig(repoRoot)
	if err != nil {
		return err
	}
	if repoCfg.Storage == string(storage.ModeLocal) {
		fmt.Printf("Already using local storage for project '%s'.\n", projectID)
		return nil
	}

	benchProjectDir := filepath.Join(bp, "projects", projectID)
	if info, err := os.Lstat(benchProjectDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is already a symlink; project may already be localized", benchProjectDir)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s exists but is not a directory", benchProjectDir)
		}
	}

	repoKerfDir := filepath.Join(repoRoot, ".kerf")
	repoWorksDir := filepath.Join(repoKerfDir, "works")
	if info, err := os.Stat(repoWorksDir); err == nil {
		entries, _ := os.ReadDir(repoWorksDir)
		if info.IsDir() && len(entries) > 0 {
			return fmt.Errorf("%s already exists and is not empty; aborting", repoWorksDir)
		}
	}

	var plannedWorks []string
	var planProjectYAML, planAreasYAML bool
	if _, err := os.Stat(benchProjectDir); err == nil {
		entries, err := os.ReadDir(benchProjectDir)
		if err != nil {
			return fmt.Errorf("reading bench project dir: %w", err)
		}
		for _, e := range entries {
			switch e.Name() {
			case "project.yaml":
				planProjectYAML = true
				continue
			case "areas.yaml":
				planAreasYAML = true
				continue
			}
			if e.IsDir() {
				plannedWorks = append(plannedWorks, e.Name())
			}
		}
	}

	fmt.Printf("Preview: would localize project '%s' (no changes made).\n", projectID)
	fmt.Println()
	if len(plannedWorks) == 0 {
		fmt.Printf("No works to move from %s.\n", benchProjectDir)
	} else {
		fmt.Printf("Would move %d work directories from %s -> %s:\n", len(plannedWorks), benchProjectDir, repoWorksDir)
		for _, w := range plannedWorks {
			fmt.Printf("  %s/%s -> %s/%s\n", benchProjectDir, w, repoWorksDir, w)
		}
	}
	if planProjectYAML {
		fmt.Printf("Would move %s/project.yaml -> %s/project.yaml\n", benchProjectDir, repoKerfDir)
	}
	if planAreasYAML {
		fmt.Printf("Would move %s/areas.yaml -> %s/areas.yaml\n", benchProjectDir, repoKerfDir)
	}
	fmt.Printf("Would replace %s with symlink -> %s\n", benchProjectDir, repoWorksDir)
	fmt.Printf("Would set storage: local in %s/config.yaml\n", repoKerfDir)
	fmt.Println()
	fmt.Println("Re-run without --check to apply.")
	return nil
}

func runLocalize() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	repoRoot, err := project.FindGitRoot(cwd)
	if err != nil {
		if projectFlag == "" {
			return fmt.Errorf("not in a git repository. Use --project <project-id> to specify a project")
		}
		return fmt.Errorf("kerf localize must be run from inside the target repo")
	}

	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	bp, err := bench.BenchPath()
	if err != nil {
		return err
	}

	repoCfg, err := storage.LoadRepoConfig(repoRoot)
	if err != nil {
		return err
	}
	if repoCfg.Storage == string(storage.ModeLocal) {
		fmt.Printf("Already using local storage for project '%s'.\n", projectID)
		return nil
	}

	benchProjectDir := filepath.Join(bp, "projects", projectID)
	if info, err := os.Lstat(benchProjectDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is already a symlink; project may already be localized", benchProjectDir)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s exists but is not a directory", benchProjectDir)
		}
	}

	repoKerfDir := filepath.Join(repoRoot, ".kerf")
	repoWorksDir := filepath.Join(repoKerfDir, "works")
	if info, err := os.Stat(repoWorksDir); err == nil {
		entries, _ := os.ReadDir(repoWorksDir)
		if info.IsDir() && len(entries) > 0 {
			return fmt.Errorf("%s already exists and is not empty; aborting", repoWorksDir)
		}
	}

	if err := os.MkdirAll(repoWorksDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", repoWorksDir, err)
	}

	var movedWorks []string
	var movedProjectYAML bool
	if _, err := os.Stat(benchProjectDir); err == nil {
		entries, err := os.ReadDir(benchProjectDir)
		if err != nil {
			return fmt.Errorf("reading bench project dir: %w", err)
		}
		for _, e := range entries {
			src := filepath.Join(benchProjectDir, e.Name())
			if e.Name() == "project.yaml" {
				dst := filepath.Join(repoKerfDir, "project.yaml")
				if err := os.Rename(src, dst); err != nil {
					rollbackLocalize(benchProjectDir, repoKerfDir, repoWorksDir, movedWorks, movedProjectYAML)
					return fmt.Errorf("moving project.yaml: %w", err)
				}
				movedProjectYAML = true
				continue
			}
			if !e.IsDir() {
				continue
			}
			dst := filepath.Join(repoWorksDir, e.Name())
			if err := os.Rename(src, dst); err != nil {
				rollbackLocalize(benchProjectDir, repoKerfDir, repoWorksDir, movedWorks, movedProjectYAML)
				return fmt.Errorf("moving %s: %w. Localization aborted — no changes made", e.Name(), err)
			}
			movedWorks = append(movedWorks, e.Name())
		}
	} else {
		fmt.Printf("Warning: no works found on bench for project '%s'.\n", projectID)
	}

	if err := os.RemoveAll(benchProjectDir); err != nil {
		rollbackLocalize(benchProjectDir, repoKerfDir, repoWorksDir, movedWorks, movedProjectYAML)
		return fmt.Errorf("removing %s: %w", benchProjectDir, err)
	}
	if err := storage.EnsureSymlink(benchProjectDir, repoWorksDir); err != nil {
		rollbackLocalize(benchProjectDir, repoKerfDir, repoWorksDir, movedWorks, movedProjectYAML)
		return fmt.Errorf("creating symlink: %w", err)
	}

	repoCfg.Storage = string(storage.ModeLocal)
	if err := storage.SaveRepoConfig(repoRoot, repoCfg); err != nil {
		rollbackLocalize(benchProjectDir, repoKerfDir, repoWorksDir, movedWorks, movedProjectYAML)
		return fmt.Errorf("writing repo config: %w", err)
	}

	fmt.Printf("Localized project '%s' to %s\n", projectID, repoWorksDir)
	if len(movedWorks) > 0 {
		fmt.Printf("Moved %d works: ", len(movedWorks))
		for i, w := range movedWorks {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(w)
		}
		fmt.Println()
	}
	fmt.Printf("Symlink: %s -> %s\n", benchProjectDir, repoWorksDir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  git add .kerf/config.yaml .kerf/works/")
	if movedProjectYAML {
		fmt.Println("  git add .kerf/project.yaml")
	}
	fmt.Println("  git commit -m \"kerf: enable local storage\"")
	fmt.Println()
	fmt.Println("Tip: To exclude snapshots from git, add to .gitignore:")
	fmt.Println("  .kerf/works/*/.history/")

	return nil
}

func rollbackLocalize(benchProjectDir, repoKerfDir, repoWorksDir string, movedWorks []string, movedProjectYAML bool) {
	if info, err := os.Lstat(benchProjectDir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		_ = os.Remove(benchProjectDir)
	}
	_ = os.MkdirAll(benchProjectDir, 0o755)
	for _, w := range movedWorks {
		_ = os.Rename(filepath.Join(repoWorksDir, w), filepath.Join(benchProjectDir, w))
	}
	if movedProjectYAML {
		_ = os.Rename(filepath.Join(repoKerfDir, "project.yaml"), filepath.Join(benchProjectDir, "project.yaml"))
	}
}

// ensureLocalSymlink creates the bench symlink for a local-storage project, if
// the resolver indicates local mode and the symlink isn't already present.
func ensureLocalSymlink(r *storage.Resolver) error {
	if r.Mode != storage.ModeLocal {
		return nil
	}
	if r.RepoRoot == "" {
		return nil
	}
	target := filepath.Join(r.RepoRoot, ".kerf", "works")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	link := filepath.Join(r.BenchPath, "projects", r.ProjectID)
	if info, err := os.Lstat(link); err == nil && info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists and is not a symlink; cannot enable local storage", link)
	}
	return storage.EnsureSymlink(link, target)
}
