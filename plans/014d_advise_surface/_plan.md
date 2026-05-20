# Plan 014d — `kerf advise --explain` Surface

> **Status: STUB, "open until justified".** Spun out from Plan 014 on 2026-05-19. Do NOT start unless and until the v1 reuse of `kerf next.reason` proves insufficient through dogfooding. This stub exists to capture the design space, not to schedule the work.

## Origin

This plan absorbs what was originally Plan 014 bead B7 (`kerf advise --explain` command). The independent review (§1) flagged it as the largest surface-sprawl risk in the original Plan 014:

- `kerf next` already provides a per-item `reason` field (`specs/commands.md` ~L1873, `internal/queue/queue.go:50`).
- `kerf doctor` already declares itself the read-only diagnostic surface that names the canonical fix command for each finding (`specs/commands.md:1571`).
- Graph health (no momentum, dangerous fan-out, area-collision risk) is a textbook `kerf doctor` finding.

Adding a third read-only diagnostic surface (`kerf advise`) alongside `kerf next` and `kerf doctor` is exactly the "two tools that do similar things" friction the user explicitly worried about during planning. The v1 reframe (Plan 014, B2) sidesteps the problem by appending graph-shape signals to `kerf next`'s existing `reason` field.

## "Open until justified" gate

Do NOT start this plan unless **all three** of the following are true:

1. **Plan 014 v1 has landed and been dogfooded for at least one full cycle.** B2 appends graph-shape signals to `kerf next.reason`; we need to see whether that surface is enough.
2. **A documented case where `kerf next.reason` is insufficient.** Concrete failure mode, not "I think it would be nicer." Candidates that would qualify:
   - Graph-level signals (critical-path length, fan-out width) genuinely don't fit per-item rationale strings — they describe the *graph*, not any one item.
   - The user wants to ask "why is the queue ordered this way?" *before* picking an item, not while reading an item.
   - The derivation trace (declarative inputs -> derived weights) is something the user wants to inspect independently of any specific item.
3. **A check that `kerf doctor` cannot absorb the need.** The review's alternative recommendation was "add a graph-shape detector to `kerf doctor` (single bead)." If the need can be served as a `kerf doctor` finding family (e.g. `graph-shape`, `momentum-pressure`), prefer that over a new top-level command.

## What was deferred from Plan 014

- **`kerf advise` command** — prints the active configuration, the declarative inputs that drove it, the T=0 graph signals that informed it, and (if 014b has shipped) the derived weights.
- **`--explain` flag** — shows the derivation trace.

## Preserved naming from Plan 014

The user confirmed profile names `throughput` / `balanced` / `safety` on 2026-05-19. They were deferred from Plan 014 v1 because v1 commits to `maturity` as the only user-facing declarative noun. If this plan resurrects a "profile" surface, those names are pre-vetted.

## Specs likely touched (forward look)

- `specs/commands.md` — new `kerf advise` section.
- `specs/cli.md` — command listing.
- `specs/coordination.md` — cross-reference from the derivation section.

## Open until closed

If after one to two dogfooding cycles `kerf next.reason` reuse holds up, this plan should be marked **closed without shipping** and moved out of the active plan set.
