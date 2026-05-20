# Plan 014 — Bead Decomposition (v2, 2026-05-19 revision)

Three beads, one wave. The 2026-05-19 independent review cut the plan from eleven beads to a minimum-viable slice. Six former beads spun out to follow-on plans 014b–d. See `_plan.md` "2026-05-19 revision" section for the rationale.

Translation glossary:
- **T=0** — static, pre-execution graph analysis (CPM, fan-out, area overlap).
- **maturity** — project-level declarative input (`experimental` / `stable` / `frozen`); the only v1 user-facing declarative dial.
- **`kerf next` reason field** — the existing per-item rationale string in `kerf next --format=json` output (`specs/commands.md` ~L1873). v1's T=0 surface piggy-backs on this rather than introducing a new command.

## Wave 1 — independent, parallel

### B1 — `maturity` field on `project.yaml`
- **Spec sentence (seed):** `architecture.md` (or `works.md` if the project-level config lives there) — "A project MAY declare `maturity: experimental | stable | frozen` at the top level of `project.yaml`. Default is `stable` when unset."
- **Depends on:** none.
- **Touches:** `specs/architecture.md` (or `specs/works.md`), `internal/config` (loader + validation), `internal/spec` if needed for plumbing.
- **Definition of done:** schema loads round-trip; validator rejects unknown enum values; default documented; no per-work override surface (deferred — see `_plan.md` deferred-fields section).
- **Out of scope for v1:** `correctness` field; `work_shape` field; per-work override.

### B2 — T=0 static analyzer skeleton, surfaced via `kerf next` reason
- **Spec sentence (seed):** `coordination.md` — "kerf computes static graph signals — critical-path length, fan-out width, area-overlap density — on the bead graph. The signals append to each ranked item's existing `reason` field in `kerf next --format=json`."
- **Depends on:** none.
- **Touches:** new `internal/static` package (CPM + fan-out + area-overlap); `internal/queue` (read-only consumer that decorates `Reasons`); `specs/coordination.md`; minor `specs/commands.md` note that the `reason` field may include graph-shape signals.
- **Definition of done:** analyzer unit-tested on a toy 5-bead graph; `kerf next --format=json` reasons include at least one graph-shape signal where applicable; no new command surface introduced; Plan 019's payload trim respected (signals append to existing `reason` strings, do not add new top-level fields).
- **Antidote to surface sprawl:** the review's concern about "two tools that do similar things" is answered here — we reuse `kerf next`'s reason field instead of adding `kerf advise`. A separate `kerf advise --explain` surface is deferred to Plan 014d behind an "open until justified" gate.

### B3 — Collision-tolerance refactor in `internal/queue/`
- **Spec sentence (seed):** `coordination.md` — "The 5% area-collisions floor is a default driven by project `maturity`, not a universal decision rule. `frozen` tightens it; `experimental` loosens it; `stable` preserves today's 5%."
- **Depends on:** B1 (consumes the `maturity` field).
- **Touches:** `internal/queue/` (extract the floor from universal decision logic; thread `maturity` through), `tools/sweep_decision.go` if it owns the v2 rule, `specs/coordination.md`.
- **Definition of done:** floor no longer applied to every weight comparison; `maturity`-driven default replaces the hard-coded 5%; `stable` default preserves current behaviour as a non-regression baseline.
- **Hotspot note:** `internal/queue` is 245 lines and intentionally legible. B3 is the only Wave-1 bead that modifies it; B2 only reads from it (decorator pattern). Reviewer must run `go test ./internal/queue/...` against merged state per CLAUDE.md.

## Dependency graph

```
B1 ── B3
B2 (independent)
```

B1 and B2 can dispatch in parallel worktrees. B3 dispatches after B1 lands.

## Hotspot warnings (for orchestrator merge order)

- `internal/queue`: B3 is the only writer in v1; B2 is read-only decorator. Run integrated `go test ./internal/queue/...` after merging both.
- `specs/coordination.md`: B2 and B3 both touch it. Recommend B2 lands the new "Graph signals" subsection skeleton first; B3 extends with the collision-tolerance paragraph.

## What is NOT in this plan (deferred)

The independent review's `cut roughly in half` verdict. Each below now has its own plan:

- **B4 (was: derived-weights joiner), B9 (was: adaptive T>0)** → `plans/014b_adaptive_scheduler/`. Gated on Plan 013 (self-diagnostics) closing.
- **B5 (was: deferred-queue data model), B10 (was: deferred-queue UX)** → `plans/014c_deferred_queue/`. Orthogonal to weight derivation; deserves its own user-flow design.
- **B7 (was: `kerf advise --explain`)** → `plans/014d_advise_surface/`. Gated on B2 reuse of `kerf next.reason` proving insufficient after dogfooding.
- **B6, B11 (kerfsim compatibility + symmetry property contract)** → folded into 014b when adaptive layer lands, since v1 has no derivation to be symmetric about.
- **B8 (profile surface — `throughput` / `balanced` / `safety`)** → deferred indefinitely. v1 ships `maturity` as the only user-facing declarative noun; profile framing revisits only if B2's signal proves useful and `maturity` proves too coarse.
- **`correctness` and `work_shape` bead fields** → deferred indefinitely (see `_plan.md` deferred-fields section). Review found `work_shape` overlaps existing `rework:`/`finding:` labels in `specs/coordination.md` §79, and `correctness` is aspirational and will drift.
