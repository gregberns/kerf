package contracttest

// optOuts records commands that are exempt from a given contract.
//
// Key shape:  "<dotted.command.path>::<contract-id>"
//
// Value:      one-line rationale ending with the bead id that tracks the
//             exception, e.g. "deliberate; tracked by kerf-XXXX".
//
// Rules:
//   - Every entry MUST cite a bead id. A future audit pass greps this file
//     and chases each bead.
//   - Adding an entry is a normative claim about kerf's behaviour and
//     should be accompanied by a bead, a plan note, or a spec edit.
//   - Empty by default: the audit underpinning plan 023 asserts the empty
//     set for the five seed contracts (no command legitimately violates
//     subprocess-exit-symmetry, filter-clause-roundtrip,
//     config-key-roundtrip, show-agreement, or bead-filter-slot).
//
// Contract ids in use (defined by each contract's test file):
//   subprocess-exit-symmetry   — plan 023 / B2 (kerf-gro2)
//   filter-clause-roundtrip    — plan 023 / B3 (kerf-hl13)
//   config-key-roundtrip       — plan 023 / B4 (kerf-sh29)
//   show-agreement             — plan 023 / B5 (kerf-k117)
//   bead-filter-slot           — plan 023 / B6 (kerf-l60u)
var optOuts = map[string]string{
	// --- subprocess-exit-symmetry exemptions (plan 023 / B2, kerf-gro2) ---
	//
	// These commands deliberately treat a failing bd/br subprocess as a
	// non-fatal condition (silent degrade, best-effort enrichment, or a
	// soft "RED finding" rather than a hard error). The contract test in
	// contract_exit_symmetry_test.go skips them. Each exemption names the
	// reason and the bead that owns it; flipping any of these to "must
	// hard-fail" is a future spec edit, not a code-only change.
	"kerf.new::subprocess-exit-symmetry":     "kerf new only consults bd for best-effort bead-filter detection (cmd/new.go uses `if lerr == nil`); subprocess failure must not block work creation. Tracked by kerf-gro2.",
	"kerf.init::subprocess-exit-symmetry":    "kerf init's detector (detectBeadFilter, cmd/init.go) is best-effort: subprocess failure leaves prior filter unchanged and init continues. Tracked by kerf-gro2.",
	"kerf.map::subprocess-exit-symmetry":     "kerf map uses `_, _ = beads.ListNamed` (cmd/map.go): subprocess failure degrades silently, the dependency map still renders from spec.yaml. Tracked by kerf-gro2.",
	// kerf.show and kerf.work.edit were previously registered here under
	// kerf-gro2 (silent-degrade on bd failure). Plan 022 / kerf-cz2t flipped
	// both to surface subprocess errors (cmd/show.go getBeadSummary /
	// getAttachedBeadsBlock now return the error; cmd/work_edit.go
	// attachedBeadCount returns it too). Plan 023 / kerf-61oi removes those
	// entries and asserts both leaves under contract_exit_symmetry_test.go's
	// shellOutLeaves set.
	"kerf.pin::subprocess-exit-symmetry":    "kerf pin only consults bd to validate that the bead-id argument exists (cmd/pin.go); subprocess failure skips validation but pinning succeeds. Tracked by kerf-gro2.",
	"kerf.square::subprocess-exit-symmetry": "kerf square's bead-summary footer is best-effort (cmd/square.go: `if err != nil || len(bs) == 0 { return }`); subprocess failure suppresses the summary line, not the command. Tracked by kerf-gro2.",
	"kerf.doctor::subprocess-exit-symmetry": "kerf doctor without --strict degrades a bd failure to a RED finding and exits 0 by design (specs/commands.md §`kerf doctor` §Exit codes). The --strict path does hard-fail, but its exit hook is package-private to cmd/ and cannot be safely intercepted from this package; the real-binary scenario test for plan 022 (kerf-cz2t) covers --strict end-to-end. Tracked by kerf-gro2.",
}
