# Critique C — Spec Coverage

Scope: does Plan 014 identify every spec sentence its reframe will require, and are there spec gaps the plan leaves unaddressed?

## Specs that will need edits

Walking the reframe section-by-section against `specs/_index.md`:

### `specs/coordination.md`

- **§Scoring weights (~lines 165–180).** Currently names momentum / fan_out / creation / rework as user-config under `queue:`. The reframe moves these from user-facing config to derived internal state. This section needs either (a) replacement with a "derived weights" paragraph that points at the new declarative-inputs surface, or (b) demotion to an advanced-override surface ("if you must, you can still override; here's how"). The plan does not say which. **Gap.**
- **§Completion momentum / fan-out / rework (lines 147–149).** These prose paragraphs explain *why* each weight exists. They should stay — they explain the underlying signal. They need a forward-reference to the new derivation layer. **Plan does not specify.**

### `specs/works.md` and `specs/architecture.md`

- **Declarative inputs need a home in the schema.** Candidates: `project.yaml` (project-wide defaults) and `spec.yaml` (per-work overrides). The plan's open question 1 surfaces this but doesn't commit. The spec edit will be in `works.md` (spec.yaml schema) and `architecture.md` (project.yaml schema) — both will need new field definitions. **Gap: the plan should name the candidate field set and shape (enum vs. struct).**

### `specs/commands.md`

- **`kerf advise` (new command).** Needs a full section — flags, output format, exit codes, agent-readable shape. The plan mentions it as one of several candidates; if it's the canonical T=0 surface, it needs a spec section in this plan's deliverable list.
- **`kerf next` output shape.** Plan 019 just locked the payload-first ordering and the rank-label vocabulary (`empty` / `unwired` / `broken`). If 014 adds a `(why this is #1)` rationale row, Plan 019's spec edits to `commands.md` will collide. **Conflict surface — should be in the plan's "interactions" list.**
- **`kerf triage` output.** Same as `kerf next` — Plan 018 reshaped this. Any 014 additions to triage's output must be additive, not overwriting.

### `specs/simulator.md`

- **Weights input format.** `kerfsim run --weights w.yaml` currently takes a weights YAML directly. If the reframe makes that the "low-level" path and adds a "declarative inputs" path, `simulator.md` needs a flag addition (`--declarative inputs.yaml`?) or a documented "for simulation sweeps, use the low-level weights YAML; for derived runs, use the kerf binary's `kerf advise` output as input." The plan does not address this. **Gap.**
- **§Relationship to kerf (~line 25).** Says the simulator's in-memory store implements the same `BeadSource` interface so divergence is impossible. If derived weights are computed by kerf-binary code that the simulator doesn't run, the simulator gains a NEW way to diverge. The interface contract may need extension to cover the weight-derivation step too. **Gap with serious correctness implications.**

### `specs/coordination.md` — deferred queue

- The "later" bucket is a new lifecycle state for a bead. `specs/dependencies.md` §"Determine completeness" treats status values as the gate. A deferred bead is neither "in flight" nor "complete" nor "blocked" — it's "consciously not now." Needs:
  - A new status value or a new orthogonal flag.
  - Interaction with `IsComplete` (probably: deferred ≠ complete).
  - Interaction with `kerf next` ordering (probably: deferred = excluded from candidate set entirely).
  - **Open question 3 surfaces this; the spec answer is missing.**

### `specs/testing.md`

- Property contracts (Plan 023) — if 014 adds new symmetry guarantees (e.g., "every command's weight output equals what `kerf advise` would compute"), those belong as new entries in the property-contract list. Not blocking; should be a sentence in the plan.

## Spec sentences each follow-on bead must satisfy (pre-draft)

A bead is only meaningful if it can point at a spec sentence it satisfies. For the seven follow-on areas:

| Area | Candidate spec sentence (to be written before/with the bead) |
|---|---|
| Declarative inputs schema | `works.md` / `architecture.md`: "Projects MAY declare a `process:` section in `project.yaml` with fields `maturity`, `correctness`, `work_shape`. Per-work overrides live under the same key in `spec.yaml`." |
| T=0 analyzer | `coordination.md` (new §): "kerf computes static graph signals (critical-path length, fan-out width, area-overlap density) on the bead graph and surfaces them via `kerf advise`." |
| Derived weights | `coordination.md` §Scoring weights, replacement paragraph: "Scoring weights are derived from declarative inputs and T=0 graph signals. Explicit `queue:` overrides retain priority for advanced users." |
| Profile surface | `commands.md` (new `kerf advise` section + addition to `cli.md` output philosophy): "kerf exposes named profiles (`throughput`, `balanced`, `safety`) that resolve to weight vectors. `kerf advise --explain` shows the derivation." |
| Adaptive wiring | `coordination.md` (new §): "Once execution begins, observed signals (rework rate, merge-conflict rate, abandoned-dispatch rate from Plan 013 detectors) refine the derived weights." |
| Collision tolerance | `coordination.md` (move, not add): "The 5% area-collisions floor is a default within the `safety` profile, not a universal rule." |
| Deferred queue | `coordination.md` + `dependencies.md`: "A bead may be in the `deferred` state. Deferred beads are excluded from `kerf next` candidates and treated as incomplete for dependency gating." |

## Recommendations

- Add a "specs touched" subsection to the plan listing every file above with one-line scope.
- For each follow-on area, draft the candidate spec sentence (above table is a starting point) — the bead decomposition then has something to satisfy.
- Surface the `kerfsim` BeadSource-interface coupling explicitly: deriving weights outside the interface is the kind of divergence the simulator's spec was written to prevent.
- The deferred-queue spec edits cross two specs (coordination + dependencies) and three implementation packages (`internal/queue`, `internal/dep`, `internal/store`); this is the biggest single bead by surface area.
