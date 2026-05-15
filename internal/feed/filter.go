package feed

// KindSelection is the resolved set of kinds to include in the feed,
// computed from the `--kinds`, `--only`, and `--include` flags on
// `kerf next`. Bead B6 owns flag parsing; B3 owns the data type and
// the precedence rules.
//
// Precedence (per specs/commands.md §"kerf next"):
//  1. --kinds=a,b sets the BASE set. Default (no --kinds) = all known kinds.
//  2. --only=X (repeatable) INTERSECTS the current set with the union of
//     all --only values.
//  3. --include=X (repeatable) UNIONS the additional kinds into the result.
//  4. Repeated identical flags act as union — no last-wins semantics.
type KindSelection map[Kind]bool

// AllKinds returns a KindSelection containing every known kind.
func AllKinds() KindSelection {
	sel := make(KindSelection, len(KnownKinds()))
	for _, k := range KnownKinds() {
		sel[k] = true
	}
	return sel
}

// Has reports whether the kind is in the selection.
func (s KindSelection) Has(k Kind) bool { return s != nil && s[k] }

// Clone returns a deep copy of the selection.
func (s KindSelection) Clone() KindSelection {
	out := make(KindSelection, len(s))
	for k, v := range s {
		if v {
			out[k] = true
		}
	}
	return out
}

// ResolveKindSelection applies the documented precedence rules to produce
// the final selection. Each slice may be empty/nil; identical entries
// within a slice are deduplicated (union, not last-wins).
//
//   - kinds: comma-pre-split values from `--kinds=a,b` (may be empty -> all)
//   - only:  values from each `--only=X` occurrence (may be empty)
//   - include: values from each `--include=X` occurrence (may be empty)
//
// Unknown kind tokens are returned as the second value (error). The first
// returned KindSelection is always non-nil and safe to range over.
func ResolveKindSelection(kinds, only, include []string) (KindSelection, error) {
	var base KindSelection
	if len(kinds) == 0 {
		base = AllKinds()
	} else {
		base = make(KindSelection)
		for _, raw := range kinds {
			k, err := ParseKind(raw)
			if err != nil {
				return AllKinds(), err
			}
			base[k] = true
		}
	}

	if len(only) > 0 {
		onlySet := make(KindSelection)
		for _, raw := range only {
			k, err := ParseKind(raw)
			if err != nil {
				return AllKinds(), err
			}
			onlySet[k] = true
		}
		// Intersect base with onlySet.
		for k := range base {
			if !onlySet[k] {
				delete(base, k)
			}
		}
	}

	for _, raw := range include {
		k, err := ParseKind(raw)
		if err != nil {
			return AllKinds(), err
		}
		base[k] = true
	}

	return base, nil
}

// ApplyKindFilter returns items whose Kind is in sel, preserving order.
// A nil/empty sel filters everything out.
func ApplyKindFilter(items []Item, sel KindSelection) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if sel.Has(it.Kind) {
			out = append(out, it)
		}
	}
	return out
}
