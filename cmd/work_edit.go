// Package cmd — `kerf work edit` subcommand.
//
// Plan 009 / Bead 10. Implements `kerf work edit <codename>
// [--bead-filter-add <clause>...] [--bead-filter-remove <clause>...]
// [--project <id>]` per specs/commands.md §`kerf work edit`.
//
// Mutators live in internal/spec (AddBeadFilterClause / RemoveBeadFilterClause,
// landed in B1). This file is the cobra wiring and the per-step behavior:
//
//  1. Resolve the project ID.
//  2. Locate the work's spec.yaml; error if missing.
//  3. Apply removals first (warning-only on no-match), then additions, via the
//     comment-preserving mutators.
//  4. Touch the work's `updated` timestamp in-place (still comment-preserving).
//  5. Take a snapshot.
//  6. Report a before/after attached-bead count when `br` is available.
//
// Per spec step 7, this command does NOT advance the drift baseline.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/snapshot"
	"github.com/gberns/kerf/internal/spec"
)

var (
	workEditBeadFilterAdd    []string
	workEditBeadFilterRemove []string
)

// workCmd is the `kerf work ...` parent. It owns the namespace; subcommands
// (currently only `edit`) self-register against it.
var workCmd = &cobra.Command{
	Use:   "work",
	Short: "Operate on an existing work",
	Long:  `Operate on an existing work. See subcommands.`,
}

var workEditCmd = &cobra.Command{
	Use:   "edit <codename>",
	Short: "Edit a work's bead-attachment configuration in place",
	Long: `Edit a work's bead-attachment configuration in place.

Adds or removes clauses from the work's bead_filter, preserving comments
and surrounding YAML formatting. Removals are applied before additions.
At least one of --bead-filter-add or --bead-filter-remove is required.

This command does not advance the drift baseline.

Examples:
  kerf work edit auth --bead-filter-add 'label=subsystem:auth'
  kerf work edit api --bead-filter-remove 'label=subsystem:gateway' \
                    --bead-filter-add 'label=subsystem:api'
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorkEdit(args[0])
	},
}

func init() {
	workEditCmd.Flags().StringArrayVar(&workEditBeadFilterAdd, "bead-filter-add", nil,
		"Add a clause to the work's bead_filter (repeatable). Accepts 'label=<value>' or 'id_prefix=<value>'.")
	workEditCmd.Flags().StringArrayVar(&workEditBeadFilterRemove, "bead-filter-remove", nil,
		"Remove a clause from the work's bead_filter (repeatable). Accepts 'label=<value>' or 'id_prefix=<value>'.")
	workCmd.AddCommand(workEditCmd)
	rootCmd.AddCommand(workCmd)
}

func runWorkEdit(cn string) error {
	// At least one of --bead-filter-add or --bead-filter-remove.
	if len(workEditBeadFilterAdd) == 0 && len(workEditBeadFilterRemove) == 0 {
		return fmt.Errorf("at least one of --bead-filter-add or --bead-filter-remove is required")
	}

	// 1. Resolve the project ID.
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	// 2. Resolve the work's spec.yaml; error if missing.
	r, err := cmdutil.Resolver(projectID)
	if err != nil {
		return err
	}
	workDir := r.WorkDir(cn)
	specPath := filepath.Join(workDir, "spec.yaml")
	if _, err := os.Stat(specPath); err != nil {
		return fmt.Errorf("work '%s' not found in project '%s'", cn, projectID)
	}

	// Pre-edit validation: fail fast on bad clause syntax before mutating.
	for _, c := range workEditBeadFilterAdd {
		if _, err := beads.ParseFilterClause(c); err != nil {
			return fmt.Errorf("clause %q does not parse. Expected 'label=<value>' or 'id_prefix=<value>'", c)
		}
	}
	for _, c := range workEditBeadFilterRemove {
		if _, err := beads.ParseFilterClause(c); err != nil {
			return fmt.Errorf("clause %q does not parse. Expected 'label=<value>' or 'id_prefix=<value>'", c)
		}
	}

	// Optional: pre-edit attached-bead count, for the "was: N beads" delta.
	// If `br` is unavailable we silently skip the count.
	beforeCount, beforeOK := attachedBeadCount(cn, specPath, projectID, r)

	// 3. Apply removals first.
	var noMatchRemovals []string
	for _, c := range workEditBeadFilterRemove {
		matched, err := clauseExistsBefore(specPath, c)
		if err != nil {
			return err
		}
		if err := spec.RemoveBeadFilterClause(specPath, c); err != nil {
			return err
		}
		if !matched {
			noMatchRemovals = append(noMatchRemovals, c)
		}
	}

	// 4. Then apply additions.
	for _, c := range workEditBeadFilterAdd {
		if err := spec.AddBeadFilterClause(specPath, c); err != nil {
			return err
		}
	}

	// 5. Touch `updated` timestamp in a comment-preserving way.
	if err := touchUpdatedTimestamp(specPath); err != nil {
		return err
	}

	// 6. Snapshot.
	snapshot.Take(workDir, "")

	// 7. (Per spec) Do NOT advance the drift baseline. Nothing to do here.

	// Output.
	fmt.Printf("Updated bead_filter for %s:\n", cn)
	for _, c := range workEditBeadFilterAdd {
		fmt.Printf("  + %s\n", c)
	}
	for _, c := range workEditBeadFilterRemove {
		fmt.Printf("  - %s\n", c)
	}
	for _, c := range noMatchRemovals {
		fmt.Printf("Warning: --bead-filter-remove '%s' did not match any existing clause. No change for that clause.\n", c)
	}

	// Post-edit count.
	afterCount, afterOK := attachedBeadCount(cn, specPath, projectID, r)
	if beforeOK && afterOK {
		fmt.Println()
		fmt.Printf("Now matches: %d beads (was: %d beads).\n", afterCount, beforeCount)
	}

	// Fall-through note when no clauses remain.
	if !specHasBeadFilter(specPath) {
		fmt.Println()
		fmt.Println("bead_filter removed; this work now falls back to the project filter or built-in default.")
	}

	return nil
}

// clauseExistsBefore reports whether `clause` currently matches some clause
// in the spec.yaml's bead_filter. Used to emit the warning when a removal is
// a no-op. Errors only on unreadable YAML or unparseable clauses; the latter
// is already pre-checked, so this is best-effort.
func clauseExistsBefore(specPath, clause string) (bool, error) {
	parsed, err := beads.ParseFilterClause(clause)
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(specPath)
	if err != nil {
		return false, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return false, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return false, nil
	}
	var bf *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Kind == yaml.ScalarNode && root.Content[i].Value == "bead_filter" {
			bf = root.Content[i+1]
			break
		}
	}
	if bf == nil || bf.Kind != yaml.MappingNode {
		return false, nil
	}
	// Either direct leaf or any:.
	var anyNode *yaml.Node
	for i := 0; i < len(bf.Content); i += 2 {
		if bf.Content[i].Kind == yaml.ScalarNode && bf.Content[i].Value == "any" {
			anyNode = bf.Content[i+1]
			break
		}
	}
	if anyNode != nil && anyNode.Kind == yaml.SequenceNode {
		for _, item := range anyNode.Content {
			if leaf := readLeafFromMapping(item); leaf != nil && beads.FilterClauseEquals(leaf, parsed) {
				return true, nil
			}
		}
		return false, nil
	}
	if leaf := readLeafFromMapping(bf); leaf != nil && beads.FilterClauseEquals(leaf, parsed) {
		return true, nil
	}
	return false, nil
}

func readLeafFromMapping(m *yaml.Node) *beads.Filter {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	f := &beads.Filter{}
	for i := 0; i < len(m.Content); i += 2 {
		k := m.Content[i]
		v := m.Content[i+1]
		if k.Kind != yaml.ScalarNode || v.Kind != yaml.ScalarNode {
			continue
		}
		switch k.Value {
		case "label":
			f.Label = v.Value
		case "id_prefix":
			f.IDPrefix = v.Value
		}
	}
	if f.Label == "" && f.IDPrefix == "" {
		return nil
	}
	return f
}

// touchUpdatedTimestamp rewrites the `updated:` scalar in spec.yaml to
// time.Now (UTC, truncated to seconds) while preserving the rest of the
// document — comments and field ordering — verbatim. If the key is absent
// it is appended at the end of the root mapping.
func touchUpdatedTimestamp(specPath string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("spec.yaml: empty document")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("spec.yaml: root is not a mapping")
	}
	stamp := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Kind == yaml.ScalarNode && root.Content[i].Value == "updated" {
			root.Content[i+1].Value = stamp
			root.Content[i+1].Tag = "!!timestamp"
			root.Content[i+1].Style = 0
			return writeDocAtomically(specPath, &doc)
		}
	}
	// Append if missing.
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "updated"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!timestamp", Value: stamp})
	return writeDocAtomically(specPath, &doc)
}

func writeDocAtomically(path string, doc *yaml.Node) error {
	var buf yamlByteBuf
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

type yamlByteBuf struct{ b []byte }

func (y *yamlByteBuf) Write(p []byte) (int, error) {
	y.b = append(y.b, p...)
	return len(p), nil
}

func (y *yamlByteBuf) Bytes() []byte { return y.b }

// specHasBeadFilter reports whether the on-disk spec.yaml still has a
// bead_filter key.
func specHasBeadFilter(specPath string) bool {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return false
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return false
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Kind == yaml.ScalarNode && root.Content[i].Value == "bead_filter" {
			return true
		}
	}
	return false
}

// attachedBeadCount returns the number of beads attached to the work, given
// the current state of its spec.yaml. Falls back to (0, false) when `br` is
// unavailable, when the spec cannot be parsed, or when the resolved filter
// is invalid. Best-effort; the delta is purely informational.
//
// Note: deliberately re-reads spec.yaml from disk (rather than reusing a
// pre-edit *spec.SpecYAML) so the same code path serves both the "before"
// and "after" sites.
func attachedBeadCount(cn, specPath, projectID string, _ interface{}) (int, bool) {
	_ = projectID
	s, err := spec.Read(specPath)
	if err != nil {
		return 0, false
	}
	all, err := beads.List()
	if err != nil {
		return 0, false
	}
	resolved := beads.Resolve(s.BeadFilter, nil)
	if resolved == nil {
		return 0, false
	}
	if err := resolved.Validate(); err != nil {
		return 0, false
	}
	matches := beads.ForWorkWithFilter(all, cn, resolved)
	return len(matches), true
}
