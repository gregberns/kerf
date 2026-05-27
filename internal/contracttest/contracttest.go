// Package contracttest hosts cross-command property contracts that span the
// cobra command tree. Each contract is one invariant ("every command that
// shells out exits non-zero when the subprocess exits non-zero", "every
// documented config key round-trips", etc.) asserted against every leaf
// command, so a new cobra command inherits the contract by default.
//
// Spec: see specs/testing.md, section "Cross-command contracts" (under
// Property-Based Tests). The five recognised contracts are listed there;
// each lives in its own _test.go file in this package.
//
// HOW TO ADD A NEW CONTRACT
//
//  1. Add a TestContract_<Name> function in its own _test.go file.
//  2. Call Walk(t) to get the slice of leaf CommandDef values.
//  3. For each leaf, assert the invariant (skip leaves where Exempt is set
//     for this contract id).
//  4. Add the contract id and a one-line description to specs/testing.md
//     under "Recognised contracts".
//
// HOW TO ADD AN OPT-OUT
//
// A command may legitimately violate a contract (e.g. `kerf next` may
// deliberately swallow subprocess failures — though as of plan 023 the
// audit asserts the empty set). Register the exception in opt_outs.go by
// adding an entry to the optOuts map keyed by `<command-path>::<contract-id>`
// with a value naming the bead that tracks the exception. Each entry MUST
// cite a bead id so future audits can chase the asymmetry.
//
// Open-question resolutions (plan 023):
//
//   - OQ2: opt-out registry shape. Decided: central map in this package
//     (Exemptions(), see opt_outs.go) rather than cobra annotations. The
//     map keeps every exception in one auditable file and forces a bead id
//     reference; annotations would scatter the exceptions across cmd/*.go
//     and lose the audit trail.
package contracttest

import (
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/gregberns/kerf/cmd"
)

// CommandDef describes a single leaf command discovered by the walker.
//
// "Leaf" means: Runnable() (has a Run or RunE) AND has no subcommands.
// Hidden commands are excluded. Parent groups like `kerf areas` or `kerf
// jig` are NOT leaves — only the terminal subcommands are.
type CommandDef struct {
	// Path is the dotted command path, e.g. "kerf.areas.add" or
	// "kerf.next". Used as the map key for the opt-out registry and as a
	// human-readable identifier in failure messages.
	Path string

	// Cmd is the cobra command itself, available to contracts that need to
	// inspect flags, args, or invoke RunE with a stub.
	Cmd *cobra.Command
}

// Walk returns every Runnable, non-hidden leaf command under the assembled
// root, deterministically sorted by Path. The returned slice may be
// filtered by callers using IsExempt for a given contract id.
//
// Walk takes a *testing.T so it can be called inline from contract tests;
// it does not currently fail t, but reserves the right to (e.g. if the
// command tree is empty, which would indicate a build problem).
func Walk(t *testing.T) []CommandDef {
	t.Helper()
	return walk(cmd.Root())
}

// WalkRoot is the same as Walk but takes an explicit root, for use by
// this package's own tests (which need to build synthetic trees).
func WalkRoot(root *cobra.Command) []CommandDef {
	return walk(root)
}

func walk(root *cobra.Command) []CommandDef {
	var out []CommandDef
	var visit func(c *cobra.Command, path []string)
	visit = func(c *cobra.Command, path []string) {
		if c.Hidden {
			return
		}
		segment := commandName(c)
		next := append(path, segment)
		children := c.Commands()
		// Filter to non-hidden children for the "is leaf" decision.
		visibleChildren := 0
		for _, ch := range children {
			if !ch.Hidden {
				visibleChildren++
			}
		}
		if visibleChildren == 0 {
			if c.Runnable() {
				out = append(out, CommandDef{
					Path: strings.Join(next, "."),
					Cmd:  c,
				})
			}
			return
		}
		// Non-leaf: recurse. (We do not also emit the parent even if it
		// has its own Run, because parents-with-Run that also have
		// children are treated as menu commands, not leaves.)
		for _, ch := range children {
			visit(ch, next)
		}
	}
	visit(root, nil)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func commandName(c *cobra.Command) string {
	// Cobra's Use field can include arg hints like "show <codename>".
	// Take just the first whitespace-delimited token.
	use := c.Use
	if i := strings.IndexAny(use, " \t"); i >= 0 {
		use = use[:i]
	}
	return use
}

// IsExempt reports whether the named command is registered as exempt from
// the named contract. Contract ids are short stable strings like
// "subprocess-exit-symmetry" or "config-key-roundtrip".
func IsExempt(path, contractID string) bool {
	_, ok := optOuts[exemptKey(path, contractID)]
	return ok
}

// ExemptionReason returns the recorded rationale (including the bead id
// tracking the exception) for an opt-out, or the empty string if none is
// registered.
func ExemptionReason(path, contractID string) string {
	return optOuts[exemptKey(path, contractID)]
}

func exemptKey(path, contractID string) string {
	return path + "::" + contractID
}
