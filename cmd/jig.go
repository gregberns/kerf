package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/jig"
)

var jigCmd = &cobra.Command{
	Use:   "jig",
	Short: "Manage jig definitions",
	Long: `Manage jig definitions — workflow templates for spec work.

Subcommands:
  kerf jig list              Show available jigs
  kerf jig show <name>       View full jig definition
  kerf jig save <name>       Save a jig for customization
  kerf jig load <name> <path> Load a jig from file
  kerf jig sync              (not yet available)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// jig list

var jigListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show available jigs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJigList()
	},
}

func runJigList() error {
	summaries, err := jig.ListAll(userJigsDir())
	if err != nil {
		return err
	}

	if len(summaries) == 0 {
		fmt.Println("No jigs available.")
		return nil
	}

	// Build display name with aliases.
	displayNames := make([]string, len(summaries))
	for i, s := range summaries {
		dn := s.Name
		if len(s.Aliases) > 0 {
			dn += " (also: " + strings.Join(s.Aliases, ", ") + ")"
		}
		displayNames[i] = dn
	}

	// Column widths.
	maxName, maxDesc := 0, 0
	for i, s := range summaries {
		if len(displayNames[i]) > maxName {
			maxName = len(displayNames[i])
		}
		if len(s.Description) > maxDesc {
			maxDesc = len(s.Description)
		}
	}

	fmt.Println("Available jigs:")
	for i, s := range summaries {
		fmt.Printf("  %-*s  %-*s  v%d  %s\n",
			maxName, displayNames[i],
			maxDesc, s.Description,
			s.Version,
			s.Source,
		)
	}
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  kerf jig show <name>    View full jig definition")

	return nil
}

// jig show

var jigShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "View full jig definition",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJigShow(args[0])
	},
}

func runJigShow(name string) error {
	j, source, err := jig.Resolve(name, userJigsDir())
	if err != nil {
		return fmt.Errorf("jig '%s' not found. Run 'kerf jig list' to see available jigs", name)
	}

	fmt.Printf("Jig: %s (v%d, %s)\n", j.Name, j.Version, source)
	if j.Description != "" {
		fmt.Printf("Description: %s\n", j.Description)
	}
	fmt.Println()

	// Status values.
	fmt.Println("Status values:")
	fmt.Printf("  %s\n", strings.Join(j.StatusValues, " -> "))
	fmt.Println()

	// Passes.
	fmt.Println("Passes:")
	for i, p := range j.Passes {
		fmt.Printf("  %d. %s (status: %s)\n", i+1, p.Name, p.Status)
		if len(p.Output) > 0 {
			fmt.Printf("     Output: %s\n", strings.Join(p.Output, ", "))
		}
	}
	fmt.Println()

	// File structure.
	if len(j.FileStructure) > 0 {
		fmt.Println("File structure:")
		for _, f := range j.FileStructure {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
	}

	// Agent instructions (markdown body).
	if j.Body != "" {
		fmt.Println("Agent instructions:")
		fmt.Println(j.Body)
	}

	return nil
}

// jig save

var jigSaveFrom string

var jigSaveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save a jig for customization",
	Long: `Save a jig to the user's jigs directory for customization.

Without --from: copies the currently resolved jig (e.g., a built-in) to the user directory.
With --from: validates the file as a jig definition and copies it to the user directory.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJigSave(args[0])
	},
}

func runJigSave(name string) error {
	jigsDir := userJigsDir()

	var content []byte

	if jigSaveFrom != "" {
		// Load from --from path.
		data, err := os.ReadFile(jigSaveFrom)
		if err != nil {
			return fmt.Errorf("file not found: %s", jigSaveFrom)
		}
		if _, err := jig.Parse(data); err != nil {
			return fmt.Errorf("%s is not a valid jig definition. %v", jigSaveFrom, err)
		}
		content = data
	} else {
		// Resolve existing jig and copy it.
		j, _, err := jig.Resolve(name, jigsDir)
		if err != nil {
			return fmt.Errorf("jig '%s' not found. Use --from <path> to create a new jig", name)
		}
		// Re-read the raw content to preserve formatting.
		content, err = readRawJig(name, jigsDir)
		if err != nil {
			// Fallback: just use description — shouldn't happen.
			_ = j
			return fmt.Errorf("failed to read jig content: %w", err)
		}
	}

	if err := jig.SaveToUser(name, content, jigsDir); err != nil {
		return err
	}

	fmt.Printf("Jig '%s' saved to %s\n", name, filepath.Join(jigsDir, name+".md"))
	return nil
}

// readRawJig reads the raw jig file content by trying user-level then built-in.
func readRawJig(name, userJigsDir string) ([]byte, error) {
	if userJigsDir != "" {
		path := filepath.Join(userJigsDir, name+".md")
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		}
	}
	// Try built-in via a known path pattern.
	// We need to access the embedded FS — use Resolve and reconstruct.
	// Actually, the jig package doesn't expose the raw content.
	// Let's read the built-in file through the jig package's embedded FS.
	return jig.ReadBuiltinRaw(name)
}

// jig load

var jigLoadCmd = &cobra.Command{
	Use:   "load <name> <path>",
	Short: "Load a jig from a file",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJigLoad(args[0], args[1])
	},
}

func runJigLoad(name, pathOrURL string) error {
	data, err := os.ReadFile(pathOrURL)
	if err != nil {
		return fmt.Errorf("cannot read from %s: %v", pathOrURL, err)
	}

	if _, err := jig.Parse(data); err != nil {
		return fmt.Errorf("content from %s is not a valid jig definition. %v", pathOrURL, err)
	}

	jigsDir := userJigsDir()
	if err := jig.SaveToUser(name, data, jigsDir); err != nil {
		return err
	}

	fmt.Printf("Jig '%s' loaded from %s to %s\n", name, pathOrURL, filepath.Join(jigsDir, name+".md"))
	return nil
}

// jig sync

var jigSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync jigs from a remote source",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Jig sync is not yet available.")
		return nil
	},
}

func init() {
	jigSaveCmd.Flags().StringVar(&jigSaveFrom, "from", "", "Path to a jig file to copy")

	jigCmd.AddCommand(jigListCmd)
	jigCmd.AddCommand(jigShowCmd)
	jigCmd.AddCommand(jigSaveCmd)
	jigCmd.AddCommand(jigLoadCmd)
	jigCmd.AddCommand(jigSyncCmd)
	rootCmd.AddCommand(jigCmd)
}

func userJigsDir() string {
	bp, err := bench.BenchPath()
	if err != nil {
		return ""
	}
	return filepath.Join(bp, "jigs")
}
