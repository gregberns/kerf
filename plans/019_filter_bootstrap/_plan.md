# Plan 019 — Filter Bootstrap and `kerf show` Filter Slot

> **Status: stub.** Spawned from Plan 015 (harmonik beta-feedback triage). Expansion handled by the `plan-implementation` flow.

## Intent

Each kerf work attaches to its beads via a `bead_filter` field on `spec.yaml`. Without that field, the work is silently disconnected from the bead store and shows up as "clean" in `kerf next` — which sounds positive but actually means "needs attention." During the harmonik dogfood session, closing the 168-bead untriaged gap took four manual `kerf work edit` invocations; `kerf show <codename>` doesn't display the filter slot at all; the `clean` rank label conflates three different states (filter right but empty, no filter declared, filter wrong); and bead-label conventions are split between prefixed (`codename:foo`) and bare (`foo`). This plan builds the one-shot bootstrap primitive and cleans up the surfaces around the filter slot.

## Background

Items come from `plans/015_harmonik_beta_feedback/triage.md` themes 5 (work bead-filter bootstrap) and 6 (filter-syntax / convention drift), plus two adjacent items from theme 3 (`kerf next` ranking) that share the same root cause. Existing bead `hk-43ate` already tracks the `clean` vs. `empty` vs. `unwired` distinction.

## Scope

- New `kerf bootstrap-filters` command (or `kerf work edit --infer-from-labels`) that samples existing labels, proposes filters for every work in one pass, and applies with confirmation.
- `kerf show <codename>` prints `bead_filter:` slot — current value or `(none)`.
- Distinct rank labels in `kerf next`: `empty` (filter declared, no matches), `unwired` (no filter declared), `broken` (filter present but malformed); cross-ref existing bead `hk-43ate`.
- `kerf work edit` count message disambiguates open vs. closed: `Now matches: N (M open / K closed). Previously: 0.`
- New `kerf work show <codename>` command (single-work dump without parsing yaml).
- `kerf new` always emits a `bead_filter:` key in `spec.yaml`, even when empty.
- `kerf next` warns when a `clean` filter has a near-match under a different prefix (for example, `codename:bridge-integration` filter exists alongside a bare `bridge-integration` label) and suggests the swap.
- `kerf list --created-by self` (or per-session attribution) so multi-agent works don't appear unannounced.
- Out of scope: the underlying bead query language; the ranking algorithm itself (Plan 014).

## Items absorbed from Plan 015

- 3.2 — `clean` is the wrong rank label for zero-bead works
- 3.3 — no beads in feed when no work has a filter
- 5.1 — no one-shot bootstrap from existing labels
- 5.2 — `kerf show` doesn't display `bead_filter`
- 5.3 — `kerf work edit` count includes closed beads
- 5.4 — no `kerf work show <codename>` command
- 5.5 — multi-agent works appear unannounced in `kerf list`
- 6.1 — bead-label convention split (`codename:foo` vs bare `foo`)
- 6.2 — `clean` conflates three different states
- 6.3 — `phase-3-dot` had no `bead_filter` field at all

## Specs likely touched

- `specs/commands.md` — `kerf bootstrap-filters` (or extended `kerf work edit`); `kerf work show`; `kerf list` flags; `kerf next` rank labels
- `specs/works.md` — `spec.yaml` schema (always-present `bead_filter` key); session attribution on `kerf list`
- `specs/coordination.md` — rank-label semantics for `kerf next`
- `specs/sessions.md` — per-session work attribution if `--created-by self` lands here

## Open questions

- Is `kerf bootstrap-filters` a new top-level command or a flag on `kerf work edit` (`--infer-from-labels`)?
- How interactive is the bootstrap confirmation — single yes/no for all proposed filters, per-work confirmation, or non-interactive with a dry-run preview?
- For the near-match warning on `kerf next`, what counts as "near-match"? Exact label vs. filter clause minus a prefix? Levenshtein distance?
- Per-session attribution: does kerf already record which session created a work, or does this require a schema addition?
