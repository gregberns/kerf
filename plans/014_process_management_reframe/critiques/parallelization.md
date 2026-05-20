# Critique B — Parallelization Opportunities

Scope: when Plan 014 decomposes into beads, which can run concurrently in worktrees and which must serialize?

## Dependency shape of the work

The plan describes seven follow-on areas. Their data-flow dependencies are not all serial:

```
  declarative-inputs-schema (DI)
        |
        v
  T=0 analyzer  ----+
        |          |
        v          v
  derived-weights  profile-surface
        |          |
        +----+-----+
             v
        adaptive-wiring (T>0)  <-- consumes Plan 013
             |
             v
        collision-tolerance refactor
             |
             v
        deferred-queue (DQ)
```

But this serial view is wrong. Closer reading:

1. **Declarative-inputs schema** is a `project.yaml` / `spec.yaml` field addition. No code beyond a loader. Truly upstream.
2. **T=0 analyzer** depends ONLY on the bead graph, which already exists. It does not depend on declarative inputs — the analyzer produces structural signals; declarative inputs are a separate input vector. **These two can be developed in parallel.**
3. **Derived weights** is the joining layer: it consumes (declarative inputs) ∪ (T=0 signals) and emits the internal weights struct. This serializes after both upstreams.
4. **Profile surface** is user-facing presentation of the same derivation. Can be developed against a stub derivation function in parallel with the real one, then wired.
5. **Adaptive wiring (T>0)** depends on Plan 013's detector outputs landing. Plan 013 is in flight; 014's adaptive bead should not block on it but should not start integration until 013 publishes a stable signal schema.
6. **Collision tolerance** is a refactor of a single decision rule (`tools/sweep_decision.go` per the plan). **Independent of everything else** — can ship first, in parallel, as a clean-up move.
7. **Deferred queue** is a data-model change to bead status / membership. Touches `internal/store` and `internal/queue`. **Independent of the weight-derivation work** — the queue mechanism is orthogonal to how weights are derived. Can run in parallel with everything else.

## Parallelization plan

Three wavefronts, with worktree-safe boundaries:

**Wave 1 — independent, parallel (3 worktrees):**
- W1a: Declarative-inputs schema (`project.yaml` field, loader, validation).
- W1b: T=0 static analyzer skeleton (`internal/static` package; CPM + fan-out + area-overlap density; no integration).
- W1c: Collision-tolerance refactor (extract from universal rule, land as profile knob with default at 5%).

**Wave 2 — depends on Wave 1 (2 worktrees, parallel):**
- W2a: Derived-weights joiner (consumes W1a + W1b outputs; emits weights struct).
- W2b: Deferred-queue data model (touches `internal/store`; orthogonal to W2a but touches same package as W1c indirectly — schedule serially with W1c's merge if both touch the same file).

**Wave 3 — depends on Wave 2 + Plan 013 (2 worktrees, parallel):**
- W3a: Profile surface + `kerf advise --explain` (presents W2a output to the user).
- W3b: Adaptive wiring (T>0); consumes Plan 013 detector signals; modifies the W2a joiner to accept live observations.

## Worktree hazards to surface in beads

- **`internal/queue` is a hotspot.** Both derived-weights (W2a) and deferred-queue (W2b) touch it. If both land in parallel worktrees off `main`, the merged state may compile in each worktree but fail integration tests (see the `newTestContext` redeclaration incident in CLAUDE.md). Mitigation: name in the beads.md file that W2a and W2b must run `go test ./internal/queue/...` against the merged state, not just worktree-local.
- **`specs/coordination.md` is a hotspot.** Declarative inputs, derived weights, and profile surface all want to edit it. Spec edits should be serialized; pick one bead to land the section skeleton, then others extend it.
- **kerfsim weights-YAML compatibility.** If W2a changes the in-memory weights struct, `kerfsim` may break silently. Bead: smoke-test kerfsim against derived-weights output before merging W2a.

## Recommendation

Plan 014's bead decomposition should call out three independent first beads (W1a/b/c) that can fan out immediately, rather than treating the seven follow-on areas as a serial list. This unlocks ~50% wall-clock reduction in the planning-to-spec path.
