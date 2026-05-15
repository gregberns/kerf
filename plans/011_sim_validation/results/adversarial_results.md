# Adversarial scenarios — empirical results

Plan 011 / pillar B. Five adversarial scenarios from plan 008 executed with
3 seeds (42, 43, 44) × 4 policies (kerf, random, fifo-bead, fifo-work).
Metric values reported are **median across the 3 seeds**.

Scenarios live at `plans/011_sim_validation/scenarios/adversarial/`.
Raw output (per-policy `summary.json`, `events.jsonl`) under
`plans/011_sim_validation/results/adversarial/`.

> Note on simulator stop behavior: every adversarial run terminated on
> `stop_reason=idle-threshold` (no eligible beads for the idle window),
> not on `ticks` exhaustion. Several scenarios therefore complete only
> a small fraction of declared `bead_count` totals — they exhaust the
> initial bead pool, the rework generator dribbles slowly, and the
> simulator gives up.

---

## adv-rework-swamp (highest-confidence prediction)

Prediction (scoring_critique §1): kerf prioritises stale rework over
the productive fan-out-4 work; `goal_completion_3d` for `new-hotness`
should be lower under kerf than under fifo.

| metric              | kerf | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| work_completed      | 2    | 3      | 2         | 2         |
| wall_ticks          | 656  | 674    | 656       | 656       |
| agent_idle_pct      | 0.049| 0.045  | 0.049     | 0.049     |
| goal_completion_3d  | 2    | 3      | 2         | 2         |
| priority_inversions | 0    | 0      | 0         | 0         |
| area_collisions     | 11   | 12     | 11        | 11        |

**Verdict: PARTIALLY CONFIRMED.** Random does in fact complete one more
goal than kerf / both fifo flavours (median 3 vs 2). Kerf ties fifo
rather than losing to it because fifo also serializes stale rework
first (the rework arrivals are early — ticks 50–500 — so fifo-bead by
arrival-time picks them up too). Random's stochastic sampling avoids
the trap. The scoring critique's deeper claim — that the rework cap
matters — holds: an alphabetical-rework-first policy underperforms a
random one on the same world.

## adv-fanout-trap

Prediction (§2): kerf treats trivial leaves as full fan-out signal,
inflating `wall_ticks` / `agent_ticks_total`.

| metric              | kerf | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| work_completed      | 2    | 2      | 2         | 2         |
| wall_ticks          | 137  | 144    | 144       | 144       |
| agent_idle_pct      | 0.088| 0.133  | 0.255     | 0.255     |
| area_collisions     | 13   | 13     | 13        | 13        |

**Verdict: REFUTED at this scale.** Kerf actually beats fifo on wall
ticks (137 vs 144) and crushes fifo on idle (0.088 vs 0.255). The
"trap" scenario as written terminates after 2 works are completed by
the idle-threshold guard — there isn't enough sustained traffic for
the bad weight choice to compound. Needs a larger / longer scenario to
expose the predicted loss.

## adv-cascade-chain

| metric              | kerf | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| work_completed      | 1    | 1      | 1         | 1         |
| wall_ticks          | 93   | 93     | 93        | 93        |
| agent_idle_pct      | 0.468| 0.388  | 0.468     | 0.468     |
| area_collisions     | 5    | 5      | 5         | 5         |

**Verdict: INCONCLUSIVE.** Scenario terminates in 93 ticks with only 1
work complete; all four policies tie. Idle-threshold cuts the run
before any policy distinction emerges. Need to either (a) lower the
idle threshold or (b) add more arrivals to keep the queue hot.

## adv-momentum-lock

Prediction (§3): rework drip on an in-progress big work pins it on top
even when another work becomes more urgent.

| metric              | kerf  | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| work_completed      | 139   | 139    | 139       | 139       |
| wall_ticks          | 59979 | 59979  | 59979     | 59979     |
| agent_idle_pct      | 0.914 | 0.914  | 0.914     | 0.914     |
| goal_completion_3d  | 65    | 65     | 65        | 65        |
| priority_inversions | **20**| **16** | **20**    | **18**    |
| area_collisions     | 58    | 54     | 57        | 57        |

**Verdict: WEAKLY CONFIRMED.** Goal completion is identical across
policies, but `priority_inversions` differentiates: random has the
fewest (16), kerf and fifo-bead tie at the worst (20), fifo-work
in-between (18). This is also the **only adversarial scenario that
produces non-zero `priority_inversions`** — direct evidence that the
plan-011 / kerf-3b2 fix removed the structural zero and the metric now
discriminates between policies.

## adv-area-collisions

Prediction (§4): pure signal gap; no current scoring factor avoids
collisions.

| metric              | kerf | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| work_completed      | 5    | 5      | 5         | 5         |
| wall_ticks          | 389  | 396    | 369       | 369       |
| agent_idle_pct      | 0.091| 0.085  | 0.043     | 0.043     |
| area_collisions     | **74**| **54**| **74**    | **74**    |

**Verdict: CONFIRMED.** Random has fewer area collisions (54) than
kerf/fifo (74). Kerf's deterministic top-of-queue choice keeps
funneling agents into the same overlapping areas; random's shuffling
spreads load. Fifo is even slightly faster on wall-ticks because both
agents start at the front and the deterministic-collision penalty is
the same regardless of order. The signal-gap prediction holds — kerf
has no factor that diversifies area choice.

---

## Summary

| Scenario             | Prediction      | Result            | Differentiating metric |
|---|---|---|---|
| adv-rework-swamp     | kerf < fifo     | kerf ≈ fifo < random | goal_completion_3d |
| adv-fanout-trap      | kerf inflated   | refuted (idle-cap) | n/a (run too short) |
| adv-cascade-chain    | —               | tie (idle-cap)     | n/a |
| adv-momentum-lock    | rework pinning  | weakly confirmed   | priority_inversions |
| adv-area-collisions  | signal gap      | confirmed          | area_collisions |

Two of five predictions show empirical signal; two terminated too
early to discriminate; one (rework-swamp) confirms an alphabetical
penalty for both kerf and fifo-bead — not the kerf-loses-to-fifo story
the critique sketched, but still a real failure mode of the rework
weight (random beats both deterministic policies).

**Action items implied for plan 011 / pillar D (weight tuning):**

1. Saturating cap on rework contribution (critique §1A) is justified —
   adv-rework-swamp shows the unbounded additive form lets stale
   rework keep its top-of-queue slot.
2. Add an area-diversity factor (critique score-signal gap) —
   adv-area-collisions shows kerf has zero edge over fifo.
3. adv-cascade-chain and adv-fanout-trap need redesign (longer runs,
   more arrivals) before they can test their hypotheses.
