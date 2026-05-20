# Critique A — Architecture / Spec Coherence

Scope: does Plan 014's reframe sit coherently against the existing spec corpus, and does the T=0/T>0 architecture compose with what already ships?

## Strengths

1. **Naming the right invariant.** The reframe correctly identifies that the existing `specs/coordination.md` §"Scoring weights" treats weights as the user-facing knob. Reframing them as an internal control surface derived from declarative inputs is a coherent restructuring, not a rewrite — `coordination.md` already says "configurable over time" (§161), which leaves room for derivation to live above it.

2. **Two-layer decomposition matches the data flow.** T=0 (graph-only) and T>0 (live observations) map cleanly onto inputs that already exist: the bead graph is in the store today; live observations are exactly what Plan 013 collects. No new persistence layer is required at T=0.

3. **Borrows rather than invents.** Pointing at List Scheduling / CPM / HEFT instead of inventing a graph-analysis algorithm is the right call. These are 50-year-old well-characterized algorithms.

## Coherence problems

1. **The "likely follow-on plans" numbering collides with shipped reality.** Plans 015–020 shipped under different scopes (harmonik beta feedback). The follow-ons named in Plan 014 — declarative inputs schema, T=0 analyzer, derived weights, deferred queue, adaptive wiring, profile surface, collision tolerance — need re-numbering to ≥ 025. This is a documentation hazard: a future reader will be confused by "Plan 015 — declarative inputs schema" referring to a plan that does something else.

2. **Spec-write ownership not specified.** The plan says "follow-on plans will translate the reframe into spec changes" but doesn't say which spec gains the new sections. Candidates:
   - `coordination.md` already houses scoring weights — natural home for derivation.
   - A new `process-management.md` would isolate the new framing.
   - `simulator.md` would need a paragraph because `kerfsim` consumes weights — if the source-of-weights changes, the simulator's weight-input contract changes.
   - Recommendation: add a paragraph to the plan naming `coordination.md` as the primary spec owner, with a `process-management.md` carve-out if the surface grows beyond ~200 lines.

3. **`kerf advise` candidacy is under-argued.** The plan lists five candidate surfaces for the T=0 output (advise, `kerf next` "why" field, `kerf plan-report`, etc.) without picking. This is fine for a reframe but the bead decomposition needs a default. `kerf advise --explain` is the strongest candidate because it composes with the existing `kerf show` / `kerf preview` family without overloading `kerf next`'s payload (Plan 019 just shrunk that payload to leading with ranked items; adding "why" rationale rows back into it would undo 019's work).

4. **Interaction with the simulator (`kerfsim`) not stated.** The simulator reads a weights YAML (`specs/simulator.md` §44). If weights move from "user-supplied" to "derived," the simulator needs either: (a) a way to feed declarative inputs and ask kerf to derive the weights, or (b) continued ability to feed weights directly (for sweeps). Both should be possible; the plan should name this explicitly so the bead doesn't accidentally remove the direct-weights path.

5. **"Profiles" are introduced twice.** Once as "downstream of declarative inputs, not the top-level knob" (§"Don't expose weights to users") and once as "preset or derived" (open question 4). The plan should pick one framing — recommendation: profiles ARE the user-facing surface, declarative inputs are the orchestrator's way of selecting/blending profiles, T=0 graph signals can override the profile choice with a recommendation. This makes "profile" the noun a user sees and "declarative inputs + T=0" the way profiles are picked.

## Recommendations

- Add a "spec ownership" subsection naming `coordination.md` as the home; carve out `process-management.md` only if the new content exceeds ~200 lines.
- Pick `kerf advise --explain` as the default T=0 surface; leave `kerf next` payload alone (don't undo Plan 019).
- Add an explicit "kerfsim compatibility" paragraph: weights-YAML input path stays, declarative-inputs input path added.
- Resolve the profile / declarative-inputs framing collision: profiles are the user noun.
- Re-number follow-ons to ≥ 025 in the plan body so future readers don't search for nonexistent plans.
