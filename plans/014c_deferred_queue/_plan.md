# Plan 014c — Deferred Queue

> **Status: STUB.** Spun out from Plan 014 on 2026-05-19 after the independent review's "cut roughly in half" verdict. Do NOT start until the gate conditions below are satisfied.

## Origin

This plan absorbs what was originally Plan 014 beads B5 (deferred-queue data model) and B10 (deferred-queue UX). The independent review (§5) and critique C both flagged this as the single largest bead by surface area, crossing `internal/store`, `internal/queue`, `internal/dep`, `specs/coordination.md`, `specs/dependencies.md`, and adding a new `kerf defer` command.

Crucially, it is **conceptually orthogonal to the weight-derivation reframe**: a user could want a deferred bucket without ever caring about coordination profiles, and vice versa. It deserves its own user-flow design rather than riding inside the scheduler reframe.

## What was deferred from Plan 014

- **Deferred-queue data model** — a bead in the `deferred` state is excluded from `kerf next` candidates and treated as incomplete for must-complete-first dependency gating.
- **Deferred-queue UX** — `kerf defer <bead-id>` moves a bead into the deferred state; `kerf next` excludes deferred beads; `kerf triage` lists them under a dedicated section, suppressed by default and shown with `--include-deferred`.
- **Subdivision option carried forward**: schema bead / dep-gating bead / UX bead, if the reviewer flags it as too large during this plan's own decomposition phase.

## Gate conditions — do not start until satisfied

1. **A documented user need.** The original justification was "the agent has been observed taking on too many parallel works rather than finishing existing ones" — but that is a momentum-weight problem, not a deferred-bucket problem. Before drafting this plan, capture a concrete user scenario where `mark complete`, `close as won't-do`, and a label like `defer:true` are all insufficient.
2. **Plan 014 v1 landed.** v1's collision-tolerance refactor and `maturity` field may interact with deferred-state semantics (does `frozen` maturity defer aggressively? does `experimental` defer not at all?). Decide that ordering after v1 has shipped.
3. **A user-flow design pass** independent of the weight-derivation reframe. Review §5 was explicit: this deserves its own user-flow design.

## Open design questions (carry into this plan when started)

- Where does the deferred state live? New bead status? New queue-membership flag? New plan-level field?
- How is it surfaced in `kerf next` and `kerf triage`?
- How does it interact with `IsComplete` for dependency gating? (The original B5 spec sentence answered "treated as incomplete" — but verify this is what users actually want; "treated as complete-for-blocking" is the alternative.)
- Does deferring a bead transitively defer its dependents, or only itself?
- Re-activation flow: explicit `kerf undefer`, or just clear the flag inline?

## Specs likely touched (forward look)

- `specs/coordination.md` — deferred-state semantics in the scheduler.
- `specs/dependencies.md` — interaction with `IsComplete` gating.
- `specs/commands.md` — `kerf defer` command; `kerf next` filter; `kerf triage --include-deferred`.
