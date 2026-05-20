# Plan 025 — Bead Decomposition (revised 2026-05-19)

Not yet filed in the tracker. Listed here for orchestrator review. The 14-bead draft has been collapsed to 5 per `critiques/independent_review.md` finding #5.

## Bead list

| # | Title | Depends on | Spec sentence it satisfies |
|---|-------|------------|----------------------------|
| 1 | spec: canonical Validation-test requirement in `specs/jig-system.md` + glossary entries in `specs/_index.md` + per-jig delegations in `jig-plan.md` / `jig-spec.md` / `jig-bug.md` / `jig-implementation.md` + retrofit/spike exclusion rationale | — | `specs/jig-system.md` § Validation-test requirement: "Every pass that produces a normative planning artifact MUST list, in its 'What done looks like' checklist, two tracked test-item IDs — one scenario-level and one exploratory — and the artifact's downstream beads MUST be gated on those items closing. The mechanism does not structurally guarantee the test exercises an integration surface; see 'What this does not guarantee'." Per-jig specs: each affected pass's "What done looks like" gains the two checklist items, with a one-line delegation to `jig-system.md`. |
| 2 | spec: add `validation-section-coverage` detector to `kerf doctor` in `specs/commands.md` | 1 | `specs/commands.md` § `kerf doctor` § Detectors: "`validation-section-coverage` — reports each active work using a plan / spec / bug / implementation jig whose affected-pass artifact does not list both a scenario-test item ID and an exploratory-test item ID in its 'What done looks like' checklist. Severity yellow. Hint names the backfill file and section." |
| 3 | code: update `internal/jig/builtin/{plan,spec,bug,implementation}.md` markdown bodies to match the per-jig spec deltas from bead 1 (two checklist items per affected pass, tracker-agnostic with `br` example) | 1 | Implementation of the per-jig spec deltas from bead 1. Implementation jig gets both Pass 1 (creation) and Pass 4 (closure-check) wording. |
| 4 | code: implement the `validation-section-coverage` doctor detector per bead 2 (read artifact files, heading- and checkbox-match for the two IDs, emit yellow finding with backfill hint, integrate with `kerf next` warning footer like the existing `bead-filter-coverage` detector) | 2 | Implementation of bead 2. |
| 5 | scenario-test + exploratory-test (self-dogfood for plan 025): (a) scenario — create a plan work via `kerf new --jig plan <codename>`, walk to Pass 5, write an artifact without scenario/exploratory IDs, run `kerf doctor`, assert a yellow `validation-section-coverage` finding with the expected hint and exit 0; then add the two IDs and assert the finding clears. (b) exploratory — run `kerf doctor` and `kerf next` on a real project carrying a mix of affected and excluded jigs, confirm the finding renders sensibly, the hint is actionable, no truncation, and retrofit/spike works are correctly excluded | 3, 4 | Validates that the spec text "warns but does not fail" and "is excluded for retrofit/spike" both hold against the shipped binary. |

## Notes for the orchestrator

- Beads 3 and 4 can be filed and worked in parallel once beads 1 and 2 land. They touch different packages (`internal/jig/builtin/` vs. the doctor detectors package).
- Bead 5 is the self-dogfood Validation block for plan 025 itself — the plan eating its own tail. Per the requirement this plan defines, bead 5 MUST close before plan 025 closes. Recommended title conventions:
  - scenario portion: `scenario: jig-validation — doctor warns on missing validation IDs`
  - exploratory portion: `explore: jig-validation — doctor and next render the finding sensibly`
- The reviewer of beads 3 and 4 must run `go test ./...` against the **merged** state, per the project memory note on integrated-state tests — both touch packages that have recently seen parallel-agent collisions.
- The reviewer of bead 1 must audit all four per-jig specs in one pass; a partial delegation is worse than none (drift target).
- No tracker beads filed yet; orchestrator to file when implementation begins.

## What changed from the prior 14-bead version

- Prior beads 1, 2, 3, 4, 5, 6 (canonical spec + glossary + four per-jig deltas) → new bead **1** (single edit; per-jig deltas are now one-line checklist additions).
- Prior beads 7, 12 (square check spec + code) → new beads **2** and **4**, relocated to `kerf doctor`.
- Prior beads 8, 9, 10, 11 (four jig-body code edits) → new bead **3** (single PR across four markdown bodies).
- Prior beads 13, 14 (scenario + exploratory self-dogfood) → new bead **5** (combined; both are small).
