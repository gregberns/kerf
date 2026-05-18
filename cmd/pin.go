package cmd

// kerf pin — single-owner bead pin per Plan 009 / B9.
//
// Spec references:
//   - specs/commands.md §"kerf pin" — syntax, behavior steps 1-7, output, errors.
//   - specs/coordination.md §"Pin layer" — single-owner invariant.
//
// Self-registers via init() (existing cmd/ pattern; see cmd/new.go:66-71).
// Does NOT advance the drift baseline (step 7).

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/snapshot"
	"github.com/gberns/kerf/internal/spec"
)

// beadIDPattern is a loose validator — accepts the common br ID shape
// (alphanumeric segments separated by hyphens). Used to reject obvious
// garbage before touching disk.
var beadIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+(?:-[A-Za-z0-9]+)+$`)

var pinCmd = &cobra.Command{
	Use:   "pin <codename> <bead-id>",
	Short: "Pin a bead to a specific work (single-owner)",
	Long: `Attach a specific bead to a specific work by ID, regardless of filter outcome.

Pins are a single-owner layer: pinning a bead to one work removes it from
every other work's pin list as part of the same operation. The drift baseline
is not advanced.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPin(cmd, args[0], args[1])
	},
}

func init() {
	rootCmd.AddCommand(pinCmd)
}

func runPin(cmd *cobra.Command, codename, beadID string) error {
	out := cmd.OutOrStdout()

	// Step 1: resolve project ID.
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	// Validate bead ID format up-front (spec error: "bead ID '{value}' is not
	// a valid identifier.").
	if !beadIDPattern.MatchString(beadID) {
		return fmt.Errorf("bead ID '%s' is not a valid identifier", beadID)
	}

	r, err := cmdutil.Resolver(projectID)
	if err != nil {
		return err
	}

	// Step 2: load the target work's spec.yaml. Error if it does not exist.
	targetSpec, targetWorkDir, err := cmdutil.LoadWork(projectID, codename)
	if err != nil {
		return fmt.Errorf("work '%s' not found in project '%s'", codename, projectID)
	}
	targetSpecPath := filepath.Join(targetWorkDir, "spec.yaml")

	// Defense-in-depth bead-existence check: when the bead store is available,
	// verify the bead ID exists in the current snapshot. When unavailable,
	// proceed (per specs/commands.md §"kerf pin" — "kerf does not validate
	// that the bead ID exists in the bead store"; the validation here only
	// fires when we have a store to check against, surfacing typos early
	// without breaking the offline-pin case).
	// Honor project.yaml tools.tasks (default "br") so this existence check
	// hits the same store as `kerf next` / `kerf triage` (plan 021).
	pinTool := beads.DefaultToolName
	if cfg, cerr := config.LoadProjectConfig(r.ProjectConfigPath()); cerr == nil && cfg != nil {
		pinTool = beads.ResolveToolName(cfg.Tools)
	}
	if beads.IsAvailableNamed(pinTool) {
		all, lerr := beads.ListNamed(pinTool)
		if lerr == nil && len(all) > 0 {
			found := false
			for _, b := range all {
				if b.ID == beadID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("bead ID '%s' not found in bead store", beadID)
			}
		}
	}

	// Step 3: idempotent — already pinned to target.
	for _, id := range targetSpec.PinnedBeads {
		if id == beadID {
			fmt.Fprintf(out, "%s is already pinned to %s. No change.\n", beadID, codename)
			return nil
		}
	}

	// Step 4: scan every OTHER active work for the bead ID in pinned_beads.
	// Remove on match and record which work it came from. Single-owner
	// invariant — at most one prior owner should ever exist.
	codenames, err := r.ListWorks()
	if err != nil {
		return fmt.Errorf("listing works: %w", err)
	}
	var removedFrom []string
	for _, otherCN := range codenames {
		if otherCN == codename {
			continue
		}
		otherSpecPath := filepath.Join(r.WorkDir(otherCN), "spec.yaml")
		otherSpec, rerr := spec.Read(otherSpecPath)
		if rerr != nil {
			// Skip unreadable specs rather than failing the whole pin
			// operation — `kerf next` surfaces these as corrupt_spec
			// warnings. The pin itself should still succeed.
			continue
		}
		hit := false
		for _, id := range otherSpec.PinnedBeads {
			if id == beadID {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		// Use the comment-preserving mutator from B1.
		if rerr := spec.RemovePinnedBead(otherSpecPath, beadID); rerr != nil {
			return fmt.Errorf("removing pin from %s: %w", otherCN, rerr)
		}
		// Bump the other work's updated timestamp (step 4 of spec: "and
		// update its `updated` timestamp"). Comment-preserving.
		if rerr := pinTouchUpdated(otherSpecPath); rerr != nil {
			return fmt.Errorf("updating timestamp on %s: %w", otherCN, rerr)
		}
		// Take a snapshot of the displaced work (spec step 7).
		_, _ = snapshot.Take(r.WorkDir(otherCN), "")
		removedFrom = append(removedFrom, otherCN)
	}

	// Step 5: append the bead ID to the target work's pinned_beads list.
	if err := spec.AddPinnedBead(targetSpecPath, beadID); err != nil {
		return fmt.Errorf("pinning %s to %s: %w", beadID, codename, err)
	}

	// Step 6: update the target work's `updated` timestamp.
	if err := pinTouchUpdated(targetSpecPath); err != nil {
		return fmt.Errorf("updating timestamp on %s: %w", codename, err)
	}

	// Step 7: snapshot the target work. The drift baseline is NOT advanced
	// — no call to drift.Advance here; that is reserved for `kerf triage
	// --ack` per specs/coordination.md §"Baseline advancement".
	_, _ = snapshot.Take(targetWorkDir, "")

	// Output (matches spec wording).
	if len(removedFrom) == 1 {
		fmt.Fprintf(out, "Pinned %s to %s (removed from %s).\n", beadID, codename, removedFrom[0])
	} else if len(removedFrom) > 1 {
		// Defense-in-depth: spec assumes at most one prior owner, but if a
		// manual edit produced multiple, surface them all.
		fmt.Fprintf(out, "Pinned %s to %s (removed from %v).\n", beadID, codename, removedFrom)
	} else {
		fmt.Fprintf(out, "Pinned %s to %s.\n", beadID, codename)
	}
	return nil
}

// pinTouchUpdated rewrites the `updated:` scalar in path's spec.yaml
// to the current UTC time (truncated to seconds), preserving comments and
// surrounding YAML formatting via go-yaml v3's node API. This mirrors the
// comment-preserving strategy used by internal/spec/mutate.go but is scoped
// to the single `updated` key so cmd/pin.go can do the timestamp bump
// without round-tripping the whole spec through spec.Write (which would
// lose comments).
func pinTouchUpdated(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("%s: empty or invalid YAML document", path)
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: root is not a mapping", path)
	}
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	for i := 0; i < len(root.Content); i += 2 {
		k := root.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == "updated" {
			v := root.Content[i+1]
			v.Value = now
			v.Tag = "!!timestamp"
			v.Style = 0
			break
		}
	}
	var buf yamlWriteBuf
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		_ = enc.Close()
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// yamlWriteBuf is a minimal io.Writer for capturing yaml.Encoder output
// without pulling in bytes.Buffer at the call site.
type yamlWriteBuf struct{ b []byte }

func (y *yamlWriteBuf) Write(p []byte) (int, error) {
	y.b = append(y.b, p...)
	return len(p), nil
}
