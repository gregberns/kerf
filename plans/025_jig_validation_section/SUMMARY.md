# Plan 025 — Jig Validation Section (Planning Summary)

**Date:** 2026-05-19 (revised after independent review)
**Status:** planning phase complete; awaiting user decisions before bead creation.
**Source:** harmonik kerf-feedback 2026-05-19, item #1.

## Scope

Promote harmonik's local `.kerf/jigs/` override upstream — but as a *checklist extension* of existing acceptance-criteria machinery, not as a new section. Every built-in jig that produces a normative planning artifact (plan, spec, bug, implementation) gains two extra items inside the existing "What done looks like" checklist of each affected pass: one **scenario-test** item ID and one **exploratory-test** item ID. The work and its downstream implementation beads cannot close until both items close. The canonical requirement text lives once in `specs/jig-system.md`; per-jig specs delegate. A new `kerf doctor` detector surfaces missing IDs as a yellow finding.

## Motivation

Three 2026-05-18 harmonik dogfood incidents:
- **hk-37zy8** — handler-pause goroutine: unit-tested, reviewer-LGTM, committed; never wired into the composition root.
- **hk-aievp** — daemon paste-injects into stale pane (prior-session `lastHandle` pollution).
- **hk-ry3be** — daemon emits heartbeats ~15 hours after claude pane disappears.

All three: unit-tested + reviewer-LGTM + no integration-surface test.

## Design (after independent review)

- **No new heading.** Two checklist items inside the existing "What done looks like" block. Canonical paragraph in `specs/jig-system.md`. (Was: standalone `### Validation / Acceptance Tests` subsection per pass — rejected as duplicated surface.)
- **`kerf doctor` detector, not `kerf square` check.** Square is structural (verification.md L7); doctor is the canonical host for warn-only content-quality findings (commands.md L1571–1604).
- **Honest non-guarantee paragraph** in the canonical spec text. A checked-box scenario bead can still be implemented as a unit-against-fake test; the mechanism's value is naming the integration-surface gap at planning time, not structurally guaranteeing the test kind.
- **5 beads, not 14.** Per-jig delegation collapses spec edits; doctor host collapses the square check; jig-body edits merge into one PR.
- **Test items may be referenced by ID** across later passes. Implementation Pass 1 = creation point; Pass 4 = closure-check point.
- **Tracker-agnostic body**, `br` named as one example. Per "kerf serves users" memory.

## Beads (5 total)

1. spec: canonical requirement in `jig-system.md` + glossary in `_index.md` + per-jig delegations + retrofit/spike exclusion.
2. spec: `validation-section-coverage` detector in `kerf doctor` (commands.md).
3. code: jig markdown bodies updated for plan / spec / bug / implementation.
4. code: doctor detector implementation.
5. self-dogfood: combined scenario + exploratory test bead.

See `beads.md` for the dependency graph and spec-sentence mapping.

## User decisions needed

1. **OQ1** — yellow vs red doctor finding. Default: yellow (warn-only, preserves invariant #6).
2. **OQ2** — tracker-agnostic language vs `br`-specific. Default: tracker-agnostic with `br` example.
3. **OQ3** — recommended title conventions (`scenario:` / `explore:` prefixes). Default: recommend, don't enforce.
4. **OQ4** — retrofit / spike scope. Default: excluded with one-line justification.

All defaults shippable if no user redirect.

## What changed in the 2026-05-19 revision

Applied `critiques/independent_review.md` (verdict: proceed-with-changes). Findings #1, #4, #5, #6 implemented wholesale; finding #2 absorbed as an explicit non-guarantee paragraph in the canonical spec text; finding #3 adopted alternative (a) (doctor detector), rejected alternative (b) (impl-jig label-only policy) — rationale in `_plan.md`. Bead count dropped from 14 to 5.

## Adjacent work

`br_followups.md` enumerates three br/beads_rust upstream issues from the same feedback file. Out of kerf scope.
