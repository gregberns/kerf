# Plan 014 — Process-Management Reframe (Planning Summary)

**Date:** 2026-05-19 (revised same day after independent review).
**Status:** v1 slice ready for implementation — 3 beads, one wave. Six former beads spun out to follow-on plans 014b / 014c / 014d.

## Scope (v1, post-revision)

Ship the smallest slice of the process-management reframe that lets us evaluate whether the reframe pays off:

1. A single user-facing declarative dial — `maturity` at the project level.
2. A T=0 static analyzer that surfaces graph-shape signals via `kerf next`'s existing `reason` field (no new command).
3. A refactor that moves the 5% area-collisions floor from universal decision logic into a `maturity`-driven default.

Then stop and evaluate before building more. See `_plan.md` "2026-05-19 revision" section.

## What the independent review changed

The fresh-eyes review (`critiques/independent_review.md`) returned **proceed-with-changes, cut roughly in half**. Accepted in full. Concretely:

- Eleven beads -> three.
- `kerf advise` command -> deferred to Plan 014d behind an "open until justified" gate. v1 reuses `kerf next`'s existing per-item `reason` field instead.
- `work_shape` field -> dropped (overlaps existing `rework:` / `finding:` labels).
- `correctness` field -> dropped (aspirational; will drift).
- Named profile surface (`throughput` / `balanced` / `safety`) -> deferred. v1 commits to one user-facing noun: `maturity`.
- T>0 adaptive layer -> Plan 014b (gated on Plan 013 closing).
- Deferred queue -> Plan 014c (orthogonal scope).

User-confirmed enums for `correctness`, `work_shape`, and the profile names are preserved in `_plan.md` under "deferred fields" so 014b-d can pick them up without re-litigating.

## Beads (v1)

See `beads.md`.

- **B1** — `maturity` field on `project.yaml` (loader + validation, no per-work override).
- **B2** — T=0 static analyzer skeleton (`internal/static`); signals append to `kerf next` reason field.
- **B3** — Collision-tolerance refactor in `internal/queue/`, driven by `maturity`.

B1 and B2 dispatch in parallel worktrees. B3 dispatches after B1.

## Follow-on plans created (stubs)

- `plans/014b_adaptive_scheduler/` — adaptive T>0 + derived weights + kerfsim symmetry.
- `plans/014c_deferred_queue/` — deferred-queue data model + UX.
- `plans/014d_advise_surface/` — `kerf advise --explain` (only if v1's reuse of `kerf next.reason` proves insufficient).

## Critique headlines (3 original in-thread critiques, preserved for context)

- **Architecture / spec coherence** — `specs/coordination.md` is the primary spec home. The original plan named `kerf advise --explain` as the T=0 surface; the independent review overrides this for v1, choosing `kerf next.reason` reuse instead.
- **Parallelization** — three independent Wave-1 beads. `internal/queue` is a hotspot; v1 still respects this (only B3 writes to it).
- **Spec coverage** — six specs touched in the original plan. v1 touches three: `architecture.md` (or `works.md`), `coordination.md`, `commands.md` (minor note only).

## Process note

The original planning sub-agent could not spawn parallel critique sub-agents (Agent tool not available). The three in-thread critiques were under-weighted as a result. The independent review on 2026-05-19 provided the fresh-eyes pass the planning phase lacked and is what drove the cut.

## User decisions (2026-05-19)

- Profile names confirmed: `throughput` / `balanced` / `safety`.
- Enum values confirmed:
  - `maturity` — `experimental` / `stable` / `frozen`
  - `correctness` — `untested` / `tested` / `verified`
  - `work_shape` — `feature` / `bug` / `refactor` / `spike` / `infra`
