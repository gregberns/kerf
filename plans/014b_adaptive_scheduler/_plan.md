# Plan 014b — Adaptive Scheduler (T>0)

> **Status: STUB.** Spun out from Plan 014 on 2026-05-19 after the independent review's "cut roughly in half" verdict. Do NOT start until both gate conditions below are satisfied.

## Origin

This plan absorbs what was originally Plan 014 beads B4 (derived-weights joiner), B6 (kerfsim compatibility for derived weights), B9 (adaptive T>0 wiring), and B11 (derivation-symmetry property contract). It was carved out because:

1. T=0 (Plan 014 v1, B2) is shippable now on data at rest, with no new persistence and no dependence on Plan 013.
2. T>0 depends on Plan 013 publishing a stable signal-schema contract, requires new persistence (running observed-signal aggregates), modifies the queue's weight source dynamically (concurrency + invalidation concerns), and needs a story for "what does the user see when the weights silently change mid-session."
3. Shipping these together meant the T=0 work was gated on Plan 013's signal schema and on the T>0 plumbing decisions. They are two plans, not one.

## What was deferred from Plan 014

- **Derived-weights joiner** — replaces the global weight vector with a function of declarative inputs + T=0 graph signals. v1 of Plan 014 ships only the T=0 *signals*; this plan ships the *joiner* that turns signals + declarative inputs into weights and routes them through `internal/queue`.
- **Adaptive (T>0) wiring** — consume Plan 013's diagnostic findings (observed rework rate, merge-conflict rate, abandoned-dispatch rate, phase durations) to refine the derived weights live.
- **kerfsim symmetry** — `kerfsim run --declarative` path; the property contract that `kerf` and `kerfsim` produce identical derived weights for the same `(declarative inputs, bead graph)` pair.

## Preserved inputs from Plan 014

`correctness` and `work_shape` enum values were confirmed by the user on 2026-05-19 but dropped from Plan 014 v1 (review found the fields don't earn their seats in v1). If 014b finds the joiner needs more declarative inputs than `maturity` alone, the candidate enums are:

- `correctness` — `untested` / `tested` / `verified`
- `work_shape` — `feature` / `bug` / `refactor` / `spike` / `infra`

Pick up the rationale-against in Plan 014's `_plan.md` "deferred fields" section before re-litigating: `work_shape` overlaps existing `rework:` / `finding:` label vocabulary in `specs/coordination.md` §79; `correctness` is aspirational unless derived from CI / test presence.

## Gate conditions — do not start until ALL satisfied

1. **Plan 013 (self-diagnostics) has closed and published its signal schema.** This plan's T>0 layer consumes those signals; without a stable contract this work designs against a moving target.
2. **Plan 014 v1 has landed and been dogfooded for at least one cycle.** v1 commits to `maturity` as the only user-facing declarative noun and to `kerf next.reason` as the T=0 surface. 014b should know whether those decisions held up before extending them.
3. **A concrete user scenario showing the before/after delta** the adaptive layer produces on a representative bead graph (review §6 — "if the user-visible delta is 'kerf next ordering occasionally differs,' the abstraction is not earning its keep").

## Design notes to carry forward

- Review §7: before any bead touches `internal/queue`, this plan must specify the new layering (weight source becomes an interface; adaptive adjustments live in a separate WeightSource implementation). Otherwise parallel worktrees will each pick a different decomposition.
- kerfsim `BeadSource` divergence risk: derivation must either run on the simulator's side too or be fed in as a separate input. The original B11 property contract addresses this.

## Specs likely touched (forward look)

- `specs/coordination.md` — derivation paragraph, T>0 adaptive section.
- `specs/simulator.md` — additive `--declarative` input path; preserve `--weights` path.
- `specs/testing.md` — derivation-symmetry property contract.
