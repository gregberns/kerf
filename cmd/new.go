package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/codename"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/jig"
	"github.com/gberns/kerf/internal/project"
	"github.com/gberns/kerf/internal/session"
	"github.com/gberns/kerf/internal/snapshot"
	"github.com/gberns/kerf/internal/spec"
)

var (
	newTitle   string
	newType    string
	newJigFlag string
)

var newCmd = &cobra.Command{
	Use:   "new [codename]",
	Short: "Create a new work on the bench",
	Long: `Create a new work on the bench.

Examples:
  kerf new                          Auto-generate codename, use default jig
  kerf new auth-rewrite             Use specific codename
  kerf new --jig bug --title "Login timeout"  Use bug jig with title`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var cn string
		if len(args) > 0 {
			cn = args[0]
		}
		return runNew(cn)
	},
}

func init() {
	newCmd.Flags().StringVar(&newTitle, "title", "", "Human-friendly title for the work")
	newCmd.Flags().StringVar(&newType, "type", "", "Work type (defaults to jig name)")
	newCmd.Flags().StringVar(&newJigFlag, "jig", "", "Jig to use (default: from config)")
	rootCmd.AddCommand(newCmd)
}

func runNew(cn string) error {
	// 1. Resolve project identity.
	projectID, firstUse, err := resolveProjectForNew()
	if err != nil {
		return err
	}
	if firstUse {
		fmt.Printf("Project ID derived: %s\n", projectID)
	}

	// 2. Ensure bench exists.
	bp, err := bench.EnsureBench()
	if err != nil {
		return err
	}

	// 3. Resolve jig.
	jigName := newJigFlag
	if jigName == "" {
		cfgPath := filepath.Join(bp, "config.yaml")
		cfg, _ := config.Load(cfgPath)
		jigName = cfg.EffectiveDefaultJig()
	}

	jigsDir := filepath.Join(bp, "jigs")
	j, _, err := jig.Resolve(jigName, jigsDir)
	if err != nil {
		return fmt.Errorf("jig '%s' not found. Run 'kerf jig list' to see available jigs", jigName)
	}

	// 4. Resolve codename.
	if cn == "" {
		cn = codename.Generate()
	}
	if err := codename.Validate(cn); err != nil {
		return fmt.Errorf("codename must be lowercase alphanumeric and hyphens (matching [a-z0-9]+(-[a-z0-9]+)*)")
	}
	if bench.WorkExists(bp, projectID, cn) {
		return fmt.Errorf("work '%s' already exists in project '%s'", cn, projectID)
	}

	// 5. Create work directory.
	if err := bench.CreateWork(bp, projectID, cn); err != nil {
		return err
	}
	workDir := bench.WorkDir(bp, projectID, cn)

	// 6. Initialize spec.yaml.
	workType := newType
	if workType == "" {
		workType = jigName
	}
	var title *string
	if newTitle != "" {
		title = &newTitle
	}

	now := time.Now().UTC().Truncate(time.Second)
	s := &spec.SpecYAML{
		Codename:     cn,
		Title:        title,
		Type:         workType,
		Project:      spec.Project{ID: projectID},
		Jig:          jigName,
		JigVersion:   j.Version,
		Status:       j.StatusValues[0],
		StatusValues: j.StatusValues,
		Created:      now,
		Updated:      now,
		Sessions:     []spec.Session{},
		DependsOn:    []spec.Dependency{},
		Implementation: spec.Implementation{
			Commits: []string{},
		},
	}

	// 7. Record initial session.
	session.StartSession(s, "")

	specPath := filepath.Join(workDir, "spec.yaml")
	if err := spec.Write(specPath, s); err != nil {
		return err
	}

	// 8. Take snapshot.
	snapshot.Take(workDir, "")

	// Output.
	fmt.Println()
	fmt.Printf("Work created: %s\n", cn)
	fmt.Printf("  Project:  %s\n", projectID)
	fmt.Printf("  Jig:      %s (v%d)\n", jigName, j.Version)
	fmt.Printf("  Status:   %s\n", s.Status)
	fmt.Printf("  Path:     %s\n", workDir)
	fmt.Println()

	// Jig process overview.
	fmt.Printf("Process overview (%s jig):\n", jigName)
	for i, p := range j.Passes {
		fmt.Printf("  %d. %s (status: %s)\n", i+1, p.Name, p.Status)
		if len(p.Output) > 0 {
			fmt.Printf("     Output: %s\n", strings.Join(p.Output, ", "))
		}
	}
	fmt.Println()

	// First pass instructions.
	if len(j.Passes) > 0 {
		firstPass := j.Passes[0]
		instructions := j.InstructionsForPass(firstPass.Name)
		if instructions != "" {
			fmt.Printf("--- %s ---\n", firstPass.Name)
			fmt.Println(instructions)
			fmt.Println()
		}
	}

	// Next steps.
	fmt.Println("Next steps:")
	fmt.Printf("  1. Begin the first pass: write artifacts in %s\n", workDir)
	fmt.Printf("  2. Advance status: kerf status %s <next-status>\n", cn)
	fmt.Printf("  3. When done: kerf shelve %s\n", cn)

	return nil
}

// resolveProjectForNew resolves the project ID for `kerf new`.
// Returns (projectID, firstUse, error). firstUse is true when the project ID
// was derived and written for the first time.
func resolveProjectForNew() (string, bool, error) {
	if projectFlag != "" {
		return projectFlag, false, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", false, fmt.Errorf("getting working directory: %w", err)
	}

	gitRoot, err := project.FindGitRoot(cwd)
	if err != nil {
		return "", false, fmt.Errorf("not in a git repository. Use --project <project-id> to specify a project")
	}

	// Try existing identifier.
	if id, err := project.ReadIdentifier(gitRoot); err == nil {
		return id, false, nil
	}

	// First use — derive and write.
	bp, _ := bench.BenchPath()
	var projectID string
	if derived, err := project.DeriveFromRemote(gitRoot); err == nil {
		projectID = derived
	} else {
		projectID = project.DeriveFromDirectory(gitRoot)
	}

	// Check collision.
	if bp != "" {
		projectDir := filepath.Join(bp, "projects", projectID)
		if _, err := os.Stat(projectDir); err == nil {
			fmt.Printf("Warning: project ID '%s' already exists on the bench. If this is a different repo, edit .kerf/project-identifier manually.\n", projectID)
		}
	}

	if err := project.WriteIdentifier(gitRoot, projectID); err != nil {
		return "", false, fmt.Errorf("writing project identifier: %w", err)
	}

	return projectID, true, nil
}
