# Independent Review — Plan 014 (Process-Management Reframe)

**Reviewer:** fresh-eyes pass, 2026-05-19. Has not read the three in-thread critiques except as inputs to confirm framing — this review pushes against them, not with them.

**Verdict: proceed-with-changes, but cut the plan roughly in half before any code lands.**

Plan 014 makes one valuable reframe (weights are an internal control surface; users describe their work, not their weights) and then loads onto that reframe a much larger pile of new surface area than the reframe earns. The user's worry — "two tools that do similar things, building crap just to build it" — is well-founded. Specifics below.

---

## TL;DR findings

1. **Surface sprawl.** Plan adds: 3 bead fields (`maturity`/`correctness`/`work_shape`), 3 named profiles (`throughput`/`balanced`/`safety`), 1 new command (`kerf advise`) with an `--explain` flag, a T=0 static analyzer (new `internal/static` package), a T>0 adaptive scheduler, a deferred-queue lifecycle state, plus a new `kerf defer` command, plus a new property contract, plus a kerfsim `--declarative` flag. That is the surface of three plans, not one. The reframe itself only requires *one* of these surfaces to be testable.
2. **`kerf advise` overlaps `kerf next` and `kerf doctor` in ways the plan papers over.** `kerf next` already emits a `reason` field per item (`specs/commands.md:1873`). `kerf doctor` already names itself "the read-only diagnostic surface" (Plan 013's freshness note explicitly re-routes diagnose into doctor). `kerf advise` as drafted answers "what to work on and why at the graph level" — that is exactly the gap `kerf next --why` could fill at item level, or that `kerf doctor` could fill as a graph-health detector. Critique A's claim that advise "composes with `kerf show`" doesn't address the duplication; it just names a different family.
3. **Bead fields don't earn their seats.** The plan justifies `maturity`/`correctness`/`work_shape` as "things the agent already reasons about." Two are aspirational tags that nobody will keep current (`correctness: verified` is a wish, not a fact), and `work_shape` overlaps existing label conventions (`rework:true`, `finding:`) already in `specs/coordination.md` §79–84. **`work_shape` should be derived from labels, not declared.**

---

## Detailed findings

### 1. Feature duplication is real, not theoretical

**`kerf advise --explain` vs. `kerf next` reason field.**
`kerf next` already provides a `reason` field per ranked item in JSON (`specs/commands.md:1873`) and includes Reasons in the Entry struct (`internal/queue/queue.go:50`). The reasons today are item-local: "unblocks 3 works (+30.0)", "completion 4/5 (+4.0)". What `kerf advise` adds — graph-level signals (critical path length, fan-out width) — could be either:
  - a) a new JSON object at the top of `kerf next --format=json` output (graph summary, then ranked items), or
  - b) a new `kerf doctor` detector family (`graph-shape`, `momentum-pressure`), which the user already paid for.

Either is strictly less surface than a new command. Plan 014 should justify the new command or pick one of the existing surfaces. Critique A waves this off ("don't undo Plan 019") but Plan 019 trimmed the *payload row*; it didn't forbid a top-level summary object.

**`kerf advise` vs. `kerf doctor` overlap.**
`kerf doctor` already declares itself the read-only diagnostic surface that names the canonical fix command for each finding (`specs/commands.md:1571`). Graph health (no momentum, dangerous fan-out, area-collision risk) is a textbook doctor finding. The fact that Plan 013 is routing into `kerf doctor` (per its 2026-05-19 freshness note) and Plan 014 is opening *another* read-only diagnostic surface is exactly the friction the user worries about.

**Recommendation:** kill `kerf advise` as a separate command. Add a graph-shape detector to `kerf doctor` (single bead). Add an optional `graph_signals` block to `kerf next --format=json` output if there's an actual consumer that needs it (and only if there's a consumer — otherwise defer).

### 2. Bead fields — only one of the three pulls its weight

| Field | Verdict | Reason |
|---|---|---|
| `work_shape` | **cut, derive from labels** | `feature` / `bug` / `refactor` / `spike` / `infra` is exactly label vocabulary. `specs/coordination.md` §79–84 already treats `rework:true` and `finding:` as bead labels with semantic weight. Adding a structured field for the same information creates two sources of truth. |
| `correctness` | **cut or defer** | `untested` / `tested` / `verified` is aspirational. Either the codebase has tests (observable from the bead store / CI) or it doesn't. A static enum the user must remember to update will drift to "untested" forever or be set to "verified" once and never revisited. If the signal is real, derive it from test presence; if it's not, don't ship it. |
| `maturity` | **keep, but make project-level only** | `experimental` / `stable` / `frozen` IS a real declarative dial — it changes how aggressively to land changes, how much rework is tolerable, whether the safety-collision floor is loose or tight. Per-work overrides (Plan 014 currently proposes both) are gold-plating; project-level alone covers the 80% case. |

This cuts B1's scope by roughly two-thirds and removes two of the three risk surfaces (schema-validation, drift-from-reality) that the field set introduces.

### 3. Profiles vs. declarative inputs — pick ONE user-facing noun

Critique A correctly identifies that the plan introduces profiles twice. The plan accepts "profile is the user noun, declarative inputs pick/blend it" — but the bead list still ships both surfaces (B1 declarative inputs schema AND B8 profile surface). If profile is the user noun, the user picks `safety` in `project.yaml` and never sees declarative inputs. Declarative inputs become an *internal* mechanism — not a `project.yaml` schema. **In which case B1 collapses into B8 and we save a bead.**

Conversely, if declarative inputs are user-facing, profiles are just labels we slap on common combinations, and B8 is documentation, not a feature. Pick one. Today the plan ships both, which is the worst outcome.

**The user's specific worry maps here:** is `safety` / `balanced` / `throughput` an opinion users asked for, or one we are imposing? The memory note "kerf serves users — flexibility beats forcing users to adopt kerf's opinions" cuts directly against three named presets. Recommendation: ship `maturity: experimental/stable/frozen` as the user-facing dial (it composes with the existing config patterns), derive everything else from it, and don't introduce a separate "profile" noun yet. If users ask for `safety` explicitly, add it then.

### 4. T=0 and T>0 are two plans, not one

The plan groups them under one reframe because they share a target (the weights vector). But:

- T=0 is static graph analysis on data already at rest. No new persistence. No dependence on Plan 013. Can ship cleanly and be evaluated on its own merits before T>0 is even designed.
- T>0 depends on Plan 013 publishing a signal-schema contract, requires new persistence (running observed-signal aggregates somewhere), modifies the queue's weight source dynamically (concurrency + invalidation concerns), and needs a story for "what does the user see when the weights silently change mid-session."

Shipping these together means the T=0 work is gated on Plan 013's signal schema and on the T>0 plumbing decisions. **B9 ("Adaptive T>0 wiring") should not be in this plan at all.** Move it to a sibling plan that explicitly depends on 013 closing.

### 5. Deferred queue is a separate plan too

B5 ("Deferred-queue data model") and B10 ("Deferred-queue UX") together cross `internal/store`, `internal/queue`, `internal/dep`, `specs/coordination.md`, `specs/dependencies.md`, and add a new `kerf defer` command. Critique C correctly notes this is "the biggest single bead by surface area." It is also conceptually orthogonal to the weight-derivation reframe — a user could want a deferred bucket without ever caring about coordination profiles, and vice versa. **Cut from this plan. It deserves its own plan with its own user-flow design.**

### 6. UX of profiles — what concretely changes?

The plan never answers: a user picks `safety`, then what? The likely answer, based on the math, is "kerf next sometimes ranks rework-tagged work higher; sometimes momentum dominates." That is a small, hard-to-feel change. If the user-visible delta is "kerf next ordering occasionally differs," the abstraction is not earning its keep — especially as a top-level `project.yaml` knob.

**Recommendation:** the plan needs a concrete user scenario showing the before/after for each profile, on a representative bead graph, before the profile surface is built. If the deltas are subtle, ship just the math (derived weights) without naming profiles; revisit naming once there are real users with strong preferences.

### 7. `internal/queue` is 245 lines. Three Wave-2 beads (B4, B5, B9) all want to expand it.

The hotspot warning in beads.md acknowledges the merge risk but understates the design risk: this package is small and legible *on purpose* (per its own opening comment: "Weights are named constants at the top so they are obvious and easy to tune"). The current Compute function is one readable pass. Layering derived-weights + deferred-state + adaptive-signal consumption into it without a re-decomposition risks turning a 245-line file into a 600-line one that no one wants to touch.

**Recommendation:** before any bead touches `internal/queue`, the plan should specify the new layering (e.g., weight source becomes an interface; deferred filtering is a pre-filter step; adaptive adjustments live in a separate WeightSource implementation). Otherwise three agents in parallel worktrees will each pick a different decomposition and the reviewer will have to untangle it.

---

## What I would ship as Plan 014 v2

Minimum-viable slice (3 beads, one wave):

1. **B1' — `maturity` field on `project.yaml`** (project-level only, enum `experimental` / `stable` / `frozen`, loader + validation, no per-work override).
2. **B2' — T=0 static analyzer skeleton** (`internal/static` with CPM + fan-out + area-overlap; unit-tested on a toy graph; *no* user-visible surface yet).
3. **B3' — Collision-tolerance refactor** (extract the 5% floor from universal decision logic into a maturity-driven default).

Then **stop and evaluate**. With these three beads landed, the next plan can ask: do users feel any difference? Is the T=0 output useful enough to expose? Is `maturity: frozen` doing real work, or is the default fine?

Deferred to follow-on plans (each independent):
- T>0 adaptive (gated on Plan 013 closing).
- Deferred queue (its own plan, own user-flow design).
- `kerf advise` *only if* `kerf doctor` + `kerf next` reasons prove insufficient.
- Per-work declarative overrides *only if* project-level proves too coarse.
- `correctness` / `work_shape` fields *only if* derivation from existing labels/CI fails.

This is half the beads, with the same reframe captured, and each follow-on plan can be evaluated on its own merit instead of bundled.

---

## Direct answers to the review prompt

1. **Feature duplication / surface sprawl** — confirmed. `kerf advise` overlaps `kerf next` reason field + `kerf doctor` detector pattern. `coordination profile` doesn't duplicate `project.yaml` config but adds a parallel naming axis to it that may confuse users.
2. **Bead field bloat** — confirmed. `work_shape` should derive from labels; `correctness` is aspirational and will drift; `maturity` is the only one that pulls its weight.
3. **UX of profiles** — under-specified. Plan needs a before/after scenario walk before the profile surface ships.
4. **T=0 vs T>0** — two plans. T=0 is shippable now; T>0 should not be a bead in this plan.
5. **Right-size** — 11 beads → 3 beads MVP. Six of the eleven defer cleanly to follow-on plans without losing the reframe's value.
6. **Serves users or kerf-as-product?** — half-and-half. The reframe (weights are internal) serves users. The three named profiles, the three bead fields, the new command — those serve "completeness of the model." User memory says don't impose opinions; this plan imposes several.
