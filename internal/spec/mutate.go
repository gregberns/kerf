// Package spec mutators — comment-preserving in-place edits to spec.yaml.
//
// Plan 009 / B1. These mutators decode spec.yaml into a yaml.v3 node tree,
// surgically edit the requested keys, and re-encode. Head/foot/line comments
// on untouched nodes survive. The mutators do NOT validate the whole
// document — they touch the keys named in their signature only. Callers
// (cmd/pin.go, cmd/work_edit.go, cmd/new.go) are responsible for any
// surrounding business rules (single-owner pins, "at least one of add/remove"
// flag, etc.).
//
// API surface (introduced by this bead):
//
//	AddPinnedBead(path, beadID) error
//	RemovePinnedBead(path, beadID) error
//	AddBeadFilterClause(path, clause string) error
//	RemoveBeadFilterClause(path, clause string) error
//
// Each function is idempotent: adding a value already present, or removing
// one that is absent, is a no-op (no error).
package spec

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/gberns/kerf/internal/beads"
)

// readYAMLNode reads a YAML document and returns its root mapping node
// (skipping the implicit DocumentNode wrapper). Returns the DocumentNode
// for re-encoding and a pointer to the mapping for editing.
func readYAMLNode(path string) (*yaml.Node, *yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, nil, fmt.Errorf("%s: empty or invalid YAML document", path)
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("%s: root is not a mapping", path)
	}
	return &doc, root, nil
}

// writeYAMLNode encodes the document and writes atomically to path.
func writeYAMLNode(path string, doc *yaml.Node) error {
	enc := newEncoder()
	out, err := enc.encode(doc)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return fmt.Errorf("writing temp file %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// nodeEncoder wraps yaml.Encoder so we can keep a single indent setting
// in one place and produce deterministic byte-for-byte output across
// round-trips when no semantic change occurred.
type nodeEncoder struct{}

func newEncoder() *nodeEncoder { return &nodeEncoder{} }

func (e *nodeEncoder) encode(doc *yaml.Node) ([]byte, error) {
	var buf yamlBuffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// findMapValue returns the value node for a given key in a mapping, plus
// the index of the key node (so callers can splice it out). Returns
// (nil, -1) when the key is absent.
func findMapValue(mapping *yaml.Node, key string) (*yaml.Node, int) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, -1
	}
	for i := 0; i < len(mapping.Content); i += 2 {
		k := mapping.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			return mapping.Content[i+1], i
		}
	}
	return nil, -1
}

// scalarNode returns a fresh scalar node holding the given string value
// (rendered as a plain YAML scalar; no surrounding quotes unless yaml.v3
// chooses to add them).
func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

// AddPinnedBead appends beadID to the pinned_beads list in path's spec.yaml.
// Creates the key (as an empty flow list, then appended) if it does not
// exist. No-op if beadID is already present. Comments on surrounding nodes
// are preserved.
func AddPinnedBead(path, beadID string) error {
	doc, root, err := readYAMLNode(path)
	if err != nil {
		return err
	}
	list, _ := findMapValue(root, "pinned_beads")
	if list == nil {
		// Create the key with a single-element block list.
		k := scalarNode("pinned_beads")
		v := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		v.Content = append(v.Content, scalarNode(beadID))
		root.Content = append(root.Content, k, v)
		return writeYAMLNode(path, doc)
	}
	if list.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s: pinned_beads is not a sequence", path)
	}
	for _, item := range list.Content {
		if item.Kind == yaml.ScalarNode && item.Value == beadID {
			return nil // already present
		}
	}
	// Adding an item: switch a flow-style empty list to block style so the
	// document reads sensibly with one or more entries.
	if list.Style == yaml.FlowStyle && len(list.Content) == 0 {
		list.Style = 0
	}
	list.Content = append(list.Content, scalarNode(beadID))
	return writeYAMLNode(path, doc)
}

// RemovePinnedBead removes beadID from the pinned_beads list. Leaves the
// (possibly empty) list intact so its head/foot comments survive. No-op if
// the key is absent or the value is not present.
func RemovePinnedBead(path, beadID string) error {
	doc, root, err := readYAMLNode(path)
	if err != nil {
		return err
	}
	list, _ := findMapValue(root, "pinned_beads")
	if list == nil || list.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]*yaml.Node, 0, len(list.Content))
	removed := false
	for _, item := range list.Content {
		if !removed && item.Kind == yaml.ScalarNode && item.Value == beadID {
			removed = true
			continue
		}
		out = append(out, item)
	}
	if !removed {
		return nil
	}
	list.Content = out
	// An empty list renders as flow `[]` per the works.md schema row.
	if len(list.Content) == 0 {
		list.Style = yaml.FlowStyle
	}
	return writeYAMLNode(path, doc)
}

// AddBeadFilterClause parses `clause` (label=<v> or id_prefix=<v>) and adds
// it to the work's bead_filter. If bead_filter is absent, the clause becomes
// a direct leaf clause. If it is a direct leaf clause already, the existing
// clause and the new one are lifted into an `any:` list. If it is already an
// `any:` list, the clause is appended. Idempotent: a clause whose value
// already matches an existing one is a no-op.
func AddBeadFilterClause(path, clause string) error {
	parsed, err := beads.ParseFilterClause(clause)
	if err != nil {
		return err
	}
	doc, root, err := readYAMLNode(path)
	if err != nil {
		return err
	}
	bf, bfIdx := findMapValue(root, "bead_filter")
	if bf == nil {
		// Create as a direct clause.
		k := scalarNode("bead_filter")
		v := buildLeafClauseNode(parsed)
		root.Content = append(root.Content, k, v)
		return writeYAMLNode(path, doc)
	}
	// Present-but-null is the canonical empty form `kerf new` emits (Plan 019,
	// kerf-3ac). Replace the null slot with a direct leaf clause.
	if bf.Kind == yaml.ScalarNode && (bf.Tag == "!!null" || bf.Value == "") {
		root.Content[bfIdx+1] = buildLeafClauseNode(parsed)
		return writeYAMLNode(path, doc)
	}
	if bf.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: bead_filter is not a mapping", path)
	}
	// Inspect: any present, or direct leaf?
	anyNode, _ := findMapValue(bf, "any")
	if anyNode != nil {
		if anyNode.Kind != yaml.SequenceNode {
			return fmt.Errorf("%s: bead_filter.any is not a sequence", path)
		}
		if clauseExistsInSeq(anyNode, parsed) {
			return nil
		}
		anyNode.Content = append(anyNode.Content, buildLeafClauseNode(parsed))
		return writeYAMLNode(path, doc)
	}
	// Direct leaf form — read the existing clause and lift to `any:`.
	existing := readLeafClause(bf)
	if existing == nil {
		// Unrecognized direct form: treat as empty and write the new clause directly.
		bf.Content = buildLeafClauseNode(parsed).Content
		return writeYAMLNode(path, doc)
	}
	if beads.FilterClauseEquals(existing, parsed) {
		return nil
	}
	// Lift: replace the mapping content with a single `any:` entry.
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	seq.Content = append(seq.Content, buildLeafClauseNode(existing))
	seq.Content = append(seq.Content, buildLeafClauseNode(parsed))
	bf.Content = []*yaml.Node{scalarNode("any"), seq}
	return writeYAMLNode(path, doc)
}

// RemoveBeadFilterClause removes a matching clause from bead_filter.
// Collapsing rules:
//   - 0 clauses remain → bead_filter key retained with an empty value, so the
//     work-edit path canonicalises on the same present-but-empty form that
//     `kerf new` emits (per specs/works.md bead_filter row and Plan 019).
//   - 1 clause remains in an `any:` list → collapse to direct leaf form.
//   - direct leaf form matches → bead_filter key retained with empty value.
//
// No-op if the clause is not present.
func RemoveBeadFilterClause(path, clause string) error {
	parsed, err := beads.ParseFilterClause(clause)
	if err != nil {
		return err
	}
	doc, root, err := readYAMLNode(path)
	if err != nil {
		return err
	}
	bf, bfIdx := findMapValue(root, "bead_filter")
	if bf == nil || bf.Kind != yaml.MappingNode {
		return nil
	}
	anyNode, _ := findMapValue(bf, "any")
	if anyNode != nil {
		if anyNode.Kind != yaml.SequenceNode {
			return fmt.Errorf("%s: bead_filter.any is not a sequence", path)
		}
		kept := make([]*yaml.Node, 0, len(anyNode.Content))
		removed := false
		for _, item := range anyNode.Content {
			c := readLeafClauseMapping(item)
			if !removed && c != nil && beads.FilterClauseEquals(c, parsed) {
				removed = true
				continue
			}
			kept = append(kept, item)
		}
		if !removed {
			return nil
		}
		switch len(kept) {
		case 0:
			clearBeadFilterValue(root, bfIdx)
		case 1:
			// Collapse to direct leaf form.
			c := readLeafClauseMapping(kept[0])
			if c == nil {
				// Malformed sole survivor — keep as-is in any: list.
				anyNode.Content = kept
			} else {
				bf.Content = buildLeafClauseNode(c).Content
			}
		default:
			anyNode.Content = kept
		}
		return writeYAMLNode(path, doc)
	}
	// Direct leaf form.
	existing := readLeafClause(bf)
	if existing == nil || !beads.FilterClauseEquals(existing, parsed) {
		return nil
	}
	clearBeadFilterValue(root, bfIdx)
	return writeYAMLNode(path, doc)
}

// clearBeadFilterValue replaces the value node at root.Content[keyIdx+1] with
// a null scalar, leaving the `bead_filter:` key in place with an empty value.
// Used when removing the last clause so the work-edit path matches the
// canonical present-but-empty form `kerf new` emits.
func clearBeadFilterValue(root *yaml.Node, keyIdx int) {
	if root == nil || keyIdx < 0 || keyIdx+1 >= len(root.Content) {
		return
	}
	root.Content[keyIdx+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"}
}

// buildLeafClauseNode produces a mapping node containing a single key
// (label or id_prefix) → scalar value. Returns an empty mapping if both
// fields are empty (defensive — ParseFilterClause won't produce one).
func buildLeafClauseNode(f *beads.Filter) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if f.Label != "" {
		m.Content = append(m.Content, scalarNode("label"), scalarNode(f.Label))
	} else if f.IDPrefix != "" {
		m.Content = append(m.Content, scalarNode("id_prefix"), scalarNode(f.IDPrefix))
	}
	return m
}

// readLeafClause extracts label/id_prefix from a bead_filter mapping that is
// expected to be in direct-leaf form (no `any:` key). Returns nil if neither
// field is present.
func readLeafClause(m *yaml.Node) *beads.Filter {
	return readLeafClauseMapping(m)
}

func readLeafClauseMapping(m *yaml.Node) *beads.Filter {
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

// clauseExistsInSeq reports whether the `any:` sequence already contains a
// clause value-equal to `clause`.
func clauseExistsInSeq(seq *yaml.Node, clause *beads.Filter) bool {
	for _, item := range seq.Content {
		c := readLeafClauseMapping(item)
		if c != nil && beads.FilterClauseEquals(c, clause) {
			return true
		}
	}
	return false
}

// removeMappingKey removes the key/value pair at index keyIdx from a
// mapping node's content slice.
func removeMappingKey(mapping *yaml.Node, keyIdx int) {
	if mapping == nil || keyIdx < 0 || keyIdx+1 >= len(mapping.Content) {
		return
	}
	mapping.Content = append(mapping.Content[:keyIdx], mapping.Content[keyIdx+2:]...)
}

// yamlBuffer is a tiny io.Writer wrapper so we can call yaml.Encoder on
// an in-memory buffer without importing bytes in callers.
type yamlBuffer struct{ b []byte }

func (y *yamlBuffer) Write(p []byte) (int, error) {
	y.b = append(y.b, p...)
	return len(p), nil
}

func (y *yamlBuffer) Bytes() []byte { return y.b }
