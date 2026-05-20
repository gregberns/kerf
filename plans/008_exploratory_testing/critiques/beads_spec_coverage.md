# Plan 008 — Beads vs. Spec Coverage

Verdict: Phase 1 spec→code pairing is **correct**. Phase 0 contains two
beads whose deliverables outrun their cited spec.

## P0 — Spec-debt blockers (must resolve before code lands)

**None.** Every Phase 1 code bead correctly lists its spec bead as a hard
dependency (B9-code←B9-spec, B10-code←B10-spec, B11-code←B11-spec).
Confirmed gaps (`kerf init` re-run rule, `corrupt_spec`/`no_project_yaml`
warning kinds, relabel-drift hash scope) are real — none of these
sections exist in `specs/commands.md` or `specs/coordination.md` today,
and B9/B10/B11 spec beads are the right shape to close them.

## P1 — Deliverable outruns cited spec

1. **B7 — Help-text snapshot.** Bead promises an "exact byte match
   against `commands.md` example output" for both `kerf next --help` and
   bare `kerf`. `commands.md` §`kerf next` §Help text (L1525) gives a
   six-element **content contract**, not verbatim text; `kerf` (no
   arguments) §Output (L37–45) gives a bullet list, not example output.
   Fix: scope B7 to assert the contract elements are present (one regex
   per element), OR add a "Reference output" block to the spec under
   both sections first. As written, the test would either lock in
   whatever the implementation emits (drift, not contract) or fail on
   day one.

2. **B4 — Unknown statuses remain visible.** Spec basis is
   `coordination.md` "Invariant 5" — that invariant lives in
   `specs/_index.md` L75, not in `coordination.md`. Behavior is covered;
   just the citation is wrong. One-line bead edit. Also: bead suggests
   "optionally surface a per-work warning … no new warning kind defined
   here" — keep that path off; a new warning kind without a spec row
   would be a B10-style gap.

## P2 — Minor

3. **B1 file-ownership note.** B1 cites `architecture.md` L237 which
   still says `bd` (B13-spec fixes it). The bead correctly flags this
   ("the *behavior* is already `br`") so no action — call out only that
   B13-spec is independent of code and could land first to remove the
   ambiguity for B1/B2 reviewers.

4. **B12-spec contingency.** Bead notes that if the chosen
   `top_of_queue_churn` rule diverges from current
   `internal/sim/metrics` behavior, a follow-on code bead is needed.
   Plan 008 should pre-name that bead (B12-code-if-needed) so the
   investigation gate (B14) doesn't accidentally absorb it.

## Confirmed correct

- B1, B2, B3, B5, B6, B8: cited spec sections (`kerf show`, `kerf
  square`, `kerf next` output / cleanup detectors / Bead Attachment,
  coordination L232 case sensitivity, unmatched-bead warning header,
  Discoverability L42) all exist and cover the deliverables.
- B14: cites `simulator.md` §Metrics — all four metrics and the warmup
  window are present (L267–301). Diagnosis-only output is correctly
  scoped outside plan 008.
- Phase 1 spec→code ordering: every dependency is explicit and acyclic.
