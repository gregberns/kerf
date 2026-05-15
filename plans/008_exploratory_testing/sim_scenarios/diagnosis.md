# Diagnosis — `priority_inversions = 0` across all scenarios

Observed in `findings.md`: `priority_inversions` was 0 in 7/7 scenarios across all four policies (`kerf`, `random`, `fifo-bead`, `fifo-work`). Even adversarial scenarios in `adversarial.md` (notably `adv-rework-swamp`) failed to fire the metric. Two independent root causes explain this universal-zero pattern.

## Root cause 1 — under-saturation

The current definition (`specs/simulator.md` §Metric Definitions) requires a *concurrent contention* moment: at the instant of dispatch, a rework bead must be queue-eligible (dependencies met, not in-progress) **and** the selected bead must be new-work. The 7 scenarios in `scenarios.md` mostly run with `agent_idle_pct ≥ 0.79` — i.e. agents are starved for beads, not the other way around. There is rarely a queue snapshot at dispatch time that holds both a rework bead and a competing new-work bead. With no contention, no inversion can be observed regardless of policy.

This is a *scenario-design* shortfall, not a metric-definition shortfall. The metric correctly reports "no inversions occurred" when the queue never contained two competing beads at once. But it makes the metric useless as a policy discriminator in the canned scenario set.

## Root cause 2 — structural unreachability of the inequality

The current definition demands the rework bead have a **lower `arrival_tick`** than the new-work bead. In the simulator's scenario generator:

- Initial new-work beads are emitted with `ArrivedAt = 0` (seeded at scenario start).
- Rework beads arrive later, with `ArrivedAt ≥ 1` (any positive tick at which the rework event fires).

Therefore `rework.arrival_tick > new_work.arrival_tick` is **always true** for the initial new-work cohort. The inversion the metric is defined to catch — "a rework bead older than the new-work bead was passed over" — is structurally impossible against any tick-0 new-work. It can only fire against new-work that itself arrives after tick 0, which the current scenarios do not produce.

This is a *metric-definition* shortfall. Even a maximally saturated scenario cannot fire the metric as written.

## Proposed redefinitions

Two candidate redefinitions of `priority_inversions`, addressing each root cause:

### Redefinition A (preferred) — drop the arrival-order constraint

Count dispatch events where:
- the selected bead is new-work, **and**
- at least one rework bead was queue-eligible (dependencies met, not in-progress) at the moment of dispatch.

Rationale: the policy claim under test is "rework should be preferred over new-work when both are eligible." Arrival-tick order is not part of that claim — it is a tiebreaker the original definition borrowed from FIFO semantics. Dropping it lets the metric fire whenever the policy actually faces the choice, which is what we want to measure. This directly addresses root cause 2 and, in combination with denser scenarios, root cause 1.

### Redefinition B (alternative) — invert the arrival-order asymmetry

Count dispatch events where:
- the selected bead is new-work with `arrival_tick ≤ current_tick`, **and**
- at least one rework bead was queue-eligible at the moment of dispatch.

Functionally identical to A in nearly all cases (any eligible bead has `arrival_tick ≤ current_tick` by construction). Included only to make explicit that the `arrival_tick` comparison in the original spec was the bug, not the eligibility predicate.

**Recommendation: adopt Redefinition A.** It is the simplest expression of the policy claim, eliminates the structural-unreachability failure mode, and remains a faithful count of "kerf chose new-work over an available rework." Tie-breaker language can be dropped entirely — the metric is a count of events, not a ranking.

## Scope of the spec change

- `specs/simulator.md` §Metric Definitions — rewrite the `priority_inversions` bullet to Redefinition A.
- `specs/simulator.md` §Metrics — the one-line table summary should also drop "older" to avoid implying an arrival-order constraint.
- No code changes in this follow-up. Implementation in `internal/sim/metrics` (or wherever `PriorityInversions` is computed) is a separate code bead.

## Open questions

- Should the metric also fire when a *higher-rework-count* new-work bead is selected over a *lower-rework-count* one? That is a different claim ("kerf prefers more-rework"), and arguably belongs in a separate metric (`rework_weight_consistency` or similar). Out of scope here.
- Saturation fixes (root cause 1) are scenario-level — a denser-arrival scenario or higher agent count. Tracked separately, not in this bead.
