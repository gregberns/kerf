package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/jig"
)

var (
	reviewPassFlag   string
	reviewFormatFlag string
)

var reviewCmd = &cobra.Command{
	Use:   "review <codename>",
	Short: "Emit the canonical reviewer prompt for a work's current pass",
	Long: `Emit the canonical reviewer prompt for a work's current pass.

kerf review is the harness-agnostic surface for the jig's review gate. It
prints the review criteria, the artifact paths the reviewer is asked to read,
and references to prior-pass output. The calling harness pipes the output
into whichever reviewer primitive it has (sub-agent, parent orchestrator,
fresh-context re-read). kerf review does not dispatch the reviewer itself.`,
	Args: cobra.ExactArgs(1),
	RunE: runReview,
}

func init() {
	reviewCmd.Flags().StringVar(&reviewPassFlag, "pass", "", "Render the reviewer prompt for a specific pass (defaults to the work's current pass).")
	reviewCmd.Flags().StringVar(&reviewFormatFlag, "format", "text", "Output format: text or json.")
	rootCmd.AddCommand(reviewCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	codename := args[0]

	switch reviewFormatFlag {
	case "text", "json":
	default:
		return fmt.Errorf("unknown --format %q. Supported: text, json", reviewFormatFlag)
	}

	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	s, _, err := cmdutil.LoadWorkWithChecks(projectID, codename)
	if err != nil {
		return fmt.Errorf("work '%s' not found in project '%s'", codename, projectID)
	}

	bp, _ := bench.BenchPath()
	jigsDir := filepath.Join(bp, "jigs")
	jigDef, _, err := jig.Resolve(s.Jig, jigsDir)
	if err != nil || jigDef == nil {
		return fmt.Errorf("jig '%s' not found for work '%s'", s.Jig, codename)
	}

	// Resolve the pass: --pass overrides the current-status default.
	var pass *jig.Pass
	if reviewPassFlag != "" {
		for i := range jigDef.Passes {
			if jigDef.Passes[i].Name == reviewPassFlag || jigDef.Passes[i].Status == reviewPassFlag {
				pass = &jigDef.Passes[i]
				break
			}
		}
		if pass == nil {
			names := make([]string, 0, len(jigDef.Passes))
			for _, p := range jigDef.Passes {
				names = append(names, p.Name)
			}
			return fmt.Errorf("pass '%s' is not declared in jig '%s'. Known passes: %s", reviewPassFlag, jigDef.Name, strings.Join(names, ", "))
		}
	} else {
		pass = jigDef.PassForStatus(s.Status)
		if pass == nil {
			return fmt.Errorf("no pass corresponds to current status '%s' in jig '%s'", s.Status, jigDef.Name)
		}
	}

	body, ok := jigDef.ReviewForPass(pass.Name)
	if !ok {
		return fmt.Errorf("jig '%s' declares no review criteria for pass '%s'", jigDef.Name, pass.Name)
	}

	artifacts := append([]string(nil), pass.Output...)
	criteria := parseReviewCriteria(body)

	if reviewFormatFlag == "json" {
		record := struct {
			Codename  string   `json:"codename"`
			Pass      string   `json:"pass"`
			Artifacts []string `json:"artifacts"`
			Criteria  []string `json:"criteria"`
		}{
			Codename:  codename,
			Pass:      pass.Name,
			Artifacts: artifacts,
			Criteria:  criteria,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(record)
	}

	// Text output — shape per specs/commands.md §kerf review.
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Reviewer prompt for %s — pass: %s\n\n", codename, pass.Name)

	fmt.Fprintln(out, "Artifacts to read:")
	if len(artifacts) == 0 {
		fmt.Fprintln(out, "  (none declared)")
	} else {
		for _, a := range artifacts {
			fmt.Fprintf(out, "  %s\n", a)
		}
	}
	fmt.Fprintln(out)

	// Emit the jig's review block verbatim — it carries the "Inputs the reviewer
	// reads:" line (with prior-pass references) and the criteria bullets.
	fmt.Fprintln(out, "Done when the reviewer approves on:")
	for _, line := range strings.Split(body, "\n") {
		fmt.Fprintf(out, "  %s\n", line)
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "The reviewer returns either:")
	fmt.Fprintln(out, "  - \"Approved\" — the pass is ready to advance via 'kerf status "+codename+" <next>'")
	fmt.Fprintln(out, "  - \"Changes requested: <list>\" — the agent addresses each item and re-requests review")

	return nil
}

// parseReviewCriteria pulls the bullet items from a review block so JSON
// callers get a structured list. Bullets are lines beginning with "- " after
// trimming leading whitespace; non-bullet lines (e.g., the "Inputs the reviewer
// reads:" header or section labels) are ignored.
func parseReviewCriteria(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "- ") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(t, "- ")))
		}
	}
	return out
}
