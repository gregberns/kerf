# Plan 025 — Jig Validation / Acceptance-Tests Section

> **Status: drafting (revised 2026-05-19).** Input: harmonik feedback `/Users/gb/github/harmonik/docs/kerf-feedback/2026-05-19.md` § "KERF-UPSTREAM: Jig templates must require a Validation / Acceptance Tests section". The three `br` follow-ups from the same file remain out-of-scope (see `br_followups.md`).

## 2026-05-19 revision

Restructured after `critiques/independent_review.md` (verdict: proceed-with-changes). Key deltas vs. the prior draft:

1. **No new standalone heading.** The prior design added a parallel `### Validation / Acceptance Tests` subsection to the jig markdown bodies, sitting next to the existing `Acceptance criteria` / `Verification` subsections and the `What done looks like` checklist. The reviewer flagged this as duplicated surface and an ergonomics regression. Instead, the canonical requirement paragraph lives once in `specs/jig-system.md`, and each affected pass gains two checklist items inside its existing **What done looks like** block ("Scenario-test item filed with ID `<id>`" / "Exploratory-test item filed with ID `<id>`"). Per-jig specs delegate to `jig-system.md` rather than restating.
2. **Detector lives on `kerf doctor`, not `kerf square`.** `verification.md` L7 calls square explicitly structural; doctor (commands.md L1571–1604) is the canonical home for warn-only content-quality findings, integrates with the existing `kerf next` warning footer, and already hosts the related `bead-filter-coverage` detector. This collapses the prior square-check beads.
3. **5 beads, not 14.** Per-jig delegation makes the four per-jig spec deltas a single edit; jig markdown body updates merge into one or two code beads; the square-check work folds into doctor.
4. **"What this does not guarantee" is now stated explicitly.** A checked-box scenario bead can still be implemented as a unit-against-fake test — `hk-37zy8` is the cautionary example. The mechanism's power is "name and shame the missing integration test at planning time," not "structurally guarantee a twin-substrate test exists." This honest framing lives in the canonical `jig-system.md` paragraph so future readers don't read the section as a guarantee it isn't.
5. **Alternatives weighed.** The independent review proposed (a) doctor-only detector, (b) implementation-jig-only bead-label policy. The revised design adopts (a) wholesale; (b) is rejected because the three motivating incidents would not all have surfaced from impl-jig labels alone — the gap is at the planning artifact, not at the dispatch boundary.

## Intent

Every built-in kerf jig that produces a normative planning artifact (plan, spec, bug-fix-spec, implementation breakdown) must record, inside its existing acceptance-criteria machinery, at least two tracked test items: one scenario-level (end-to-end against a runnable substrate) and one exploratory (operator-facing surface). The work and its downstream implementation beads are gated on those items closing. The point is to close the "unit-tested, reviewer-APPROVED, but never wired into the composition root" gap at the planning stage, with an honest accounting of what the mechanism does and does not guarantee.

## Motivation

Three incidents from the harmonik dogfood session on 2026-05-18, all referenced in the feedback file:

- **hk-37zy8** (handler-pause policy goroutine) — passed unit tests + reviewer approval; never wired into the composition root. Required follow-up bead hk-c8k4c.
- **hk-aievp** (DOGFOOD-BLOCKER: daemon paste-injects task into a stale pane carrying prior-session state) — discovered only in live dogfood.
- **hk-ry3be** (DOGFOOD-BLOCKER: daemon emits heartbeats for ~15 hours after the claude pane disappears) — discovered only in live dogfood.

Common pattern: unit correctness + reviewer LGTM + zero integration-surface exercise.

## What this mechanism does NOT do

Stated up front so the canonical spec text inherits it:

- It does not structurally guarantee the filed scenario test exercises an integration surface. A planning agent can still satisfy "scenario-test item filed" by writing a unit-against-fake test. `hk-37zy8` is the cautionary case — a checkbox alone would not have caught it.
- It does not detect missing exploratory coverage on a live binary; it only detects that two IDs were filed in the artifact.
- The doctor detector is warn-only and is read-only: it does not query the tracker, only the artifact file.

The value is "force the agent to name the integration-surface gap at planning time, before code is written," which is a 30–60% intervention, not a 100% one.

## Affected specs

Verified by reading `specs/_index.md`, `specs/jig-system.md`, the four per-jig specs, `specs/commands.md` (kerf doctor), and `specs/verification.md`.

- **`specs/jig-system.md`** — adds a single canonical subsection "Validation-test requirement" defining: the term "normative planning artifact"; the two required test-item kinds (scenario + exploratory) with recommended title conventions; tracker-agnostic phrasing; the close-gating rule; the explicit non-guarantee paragraph; the `retrofit` / `spike` exclusion rationale.
- **`specs/_index.md`** — two glossary entries: "scenario-test bead" and "exploratory-test bead", cross-linking to `jig-system.md`.
- **`specs/jig-plan.md`, `specs/jig-spec.md`, `specs/jig-bug.md`, `specs/jig-implementation.md`** — each affected pass's existing "What done looks like" checklist gains two items: "Scenario-test item filed with ID `<id>`" / "Exploratory-test item filed with ID `<id>`", with a one-line cross-reference to `jig-system.md` for the canonical shape. No new headings, no new subsections. Plan Pass 5 / Pass 7; spec Pass 5 / Pass 7; bug Pass 5; implementation Pass 1 (creation point) / Pass 4 (closure-check point).
- **`specs/commands.md`** § `kerf doctor` — new detector `validation-section-coverage`: scans active works whose jig is in {plan, spec, bug, implementation} for the affected-pass artifacts, warns when those artifacts do not list scenario+exploratory test-item IDs in their "What done looks like" block. Warn-only severity (yellow). Hint line names the backfill: "add the two items to `<file>` § What done looks like".
- **`specs/verification.md`** — no change. Square remains structural; the new check explicitly does not belong here.

Out of scope: `specs/jig-retrofit.md`, `specs/jig-spike.md`.

## Open questions for the user

Same defaults as the prior draft, retained because the user has not redirected:

- **OQ1 — Detector severity.** Default: yellow (warn). Rejected: red. Rationale: invariant #6 (jigs are guidance, not gates).
- **OQ2 — Tracker-agnostic phrasing.** Default: tracker-agnostic body, `br` named as one example. Per "kerf serves users" memory.
- **OQ3 — Title conventions.** Default: recommended (`scenario: <codename> — <brief>` / `explore: <codename> — <brief>`), not enforced.
- **OQ4 — Retrofit / spike scope.** Default: excluded, with one-line justification in `jig-system.md`.

OQ1 was previously "warn vs fail in `kerf square`"; with the move to doctor, it becomes "yellow vs red doctor finding" — same answer.

## Source material

- `/Users/gb/github/harmonik/docs/kerf-feedback/2026-05-19.md` § KERF-UPSTREAM
- `/Users/gb/github/harmonik/.kerf/jigs/{plan,spec,bug,implementation}.md` — reference local override
- `/Users/gb/github/kerf/internal/jig/builtin/{plan,spec,bug,implementation}.md` — upstream jig markdown bodies
- `/Users/gb/github/kerf/specs/jig-system.md`, `specs/_index.md`, `specs/commands.md` § `kerf doctor`
- `plans/025_jig_validation_section/critiques/independent_review.md` — the review this revision applies

## Reconciliation notes

Most of the prior draft's accepted critique items survive; what changed is the *shape* of the surface they apply to.

**Surviving accepted items:**

- Canonical text in `jig-system.md`; per-jig specs delegate (architecture #4).
- Tracker-agnostic body + `br`-flavored example (ergonomics; OQ2).
- Test items may be referenced by ID across later passes (ergonomics).
- Implementation jig: Pass 1 = creation; Pass 4 = closure-check (workflow).
- Backfill rule for plans that pre-date this change (workflow).
- Glossary entries in `_index.md` (architecture).
- `retrofit` / `spike` excluded with documented reasoning.

**Newly accepted from independent review:**

- Drop the standalone heading; use the existing "What done looks like" checklist (independent review finding #6).
- Move detector to `kerf doctor` (independent review finding #4).
- Explicit "what this does not guarantee" paragraph (independent review finding #2).
- Bead count collapses to 5 (independent review finding #5).
- Doctor detector is the chosen alternative; impl-jig label-only is rejected with rationale (independent review finding #3).

**Pushed back / unchanged:**

- The mechanism is still required at plan/spec/bug/impl jigs (not implementation-only). Rationale: hk-aievp and hk-ry3be both surfaced at planning-artifact stages where no impl bead existed yet; impl-only would not have caught them.
- Self-dogfood test beads remain. Reviewer praised them ("plan eating its own tail").
