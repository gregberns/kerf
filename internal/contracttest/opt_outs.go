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
	// No real exemptions yet. Example shape (uncomment / replace when a
	// real exemption lands):
	// "kerf.next::subprocess-exit-symmetry": "deliberate; tracked by kerf-XXXX",
}
