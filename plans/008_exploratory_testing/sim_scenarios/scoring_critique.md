# Scoring critique (kerf's `internal/queue.Compute`)

> Based on static analysis of `internal/queue/queue.go` plus the five
> adversarial scenarios in `adversarial.md`. **Pending empirical
> confirmation** when the simulator becomes end-to-end runnable.

## Score factors and their weights

| Factor    | Weight | Range observed | Effective max |
|-----------|--------|----------------|---------------|
| Fan-out   | 10     | count of transitive dependents | unbounded |
| Momentum  | 5      | complete/total ∈ [0,1]         | 5 |
| Rework    | 15     | rework bead count, additive    | **unbounded** |
| Creation  | 0.1    | (n−1−rank)                     | ~3 for n=30 |

## Predicted worst kerf losses (from adversarial.md)

1. **adv-rework-swamp.** Six stale rework beads beat a fan-out-4 work
   with a real downstream tree. Predicted loss on `goal_completion_3d`
   for the productive work. **Highest-confidence loss.**
2. **adv-fanout-trap.** Counting trivial leaves as equal to deep ones
   inflates `wall_ticks` and `agent_ticks_total`.
3. **adv-momentum-lock.** Rework drip onto an in-progress big work
   keeps it on top long after a more urgent work becomes actionable.
4. **adv-area-collisions.** Pure signal gap — no scoring change fixes
   this without a new factor.

## Concrete weight-tuning recommendations

### 1. Cap rework's contribution

Today `score += reworkCount * 15` is unbounded and additive per bead. A
stale work with 6+ rework beads outranks anything else. Two options:

- **A. Saturating cap.** `score += min(reworkCount, 3) * 15` — capped at
  +45. Preserves the "rework is hot" signal but stops a runaway
  rework pile from drowning everything.
- **B. Logarithmic.** `score += log2(reworkCount+1) * 15` — yields 0,
  15, 23.8, 30, 34.8, 38.8 for 0..5. Smooth and intrinsically bounded.

Option A is simpler and easier to reason about. Recommend A.

### 2. Weight fan-out by downstream bead count, not work count

The structural issue in adv-fanout-trap is that 15 trivial leaves =
15 deep ones to the score. Replace `fanOut` (count of transitive
dependents) with `effortFanOut` = sum of dependent works' `Total` beads.

This is a **bigger change** than tuning a constant — it changes the
meaning of the fan-out factor — and probably needs the multiplier to
drop from 10 to ~2 to keep score magnitudes sensible. Worth a sweep
once the simulator runs.

### 3. Drop creation-order weight (or make it pure tiebreaker)

At 0.1 across n=30 works, creation contributes at most +2.9 — well
below a single rework bead's +15 or one fan-out unit's +10. It's not
strong enough to matter in policy outcomes but adds a confusing third
sort dimension. Two cleaner options:

- Set Creation to 0 and use creation time only as `sort.SliceStable`
  tiebreaker after scoring.
- Bump it to ~1.0 if "age" should actually move the queue.

Recommend the first: it removes a useless decimal that's currently
just noise in `reasons` strings.

### 4. Momentum is too weak to matter

Capped at +5, momentum is dominated by even a single rework bead.
Either kill it (status quo behavior would barely change) or bump to
~15–20 so a half-done work really does outrank a fresh one. Without
the simulator we can't tell which is right. Predict: killing it has
near-zero effect on metrics in 3 of 5 adversarial scenarios.

## Score-signal gaps

Signals kerf does not measure today but probably should:

| Gap | Why it matters |
|-----|----------------|
| **Area-collision pressure.** | adv-area-collisions hits this directly. Agents working the same area block each other at merge/integration time. Today scoring is area-blind. A simple version: `score -= (numActiveAgentsInSharedArea) * areaPenalty`. |
| **Downstream effort, not just count.** | See recommendation 2. Fan-out by work-count is a proxy that breaks under the trivial-leaf trap. |
| **Staleness penalty for rework.** | A rework bead that's been in queue for N ticks without being picked up is probably one no one cares about. Today rework's score is constant in time. Possible fix: decay rework score by half-life T_rework after first arrival. |
| **Per-work concurrency cap.** | Even when a work has 10 actionable beads, dispatching 3 agents to it concurrently produces merge churn. Scoring doesn't see "how many agents are already on this work." Could be in policy rather than score. |
| **External priority signal.** | No way to say "this work matters more" outside the structural signals. A pinned-priority field on the work would short-circuit scoring for the rare case where it's needed. |

## Open meta-question

Three of the five hypotheses route their loss through the rework
weight. If empirical runs confirm this, the rework weight is the
dominant lever in the entire system — which is fine, but argues for
**recommendation 1 (cap rework) being the single highest-value
change** to test first when the simulator is wired up.
