# Adversarial simulator scenarios

> **Status: scenarios designed and parse-validated; not yet runnable end-to-end.**
>
> The simulator code in `internal/sim/...` has the event loop, store, scenario
> loader, generator, output writer, and metrics — but **no `Policy`
> implementations** (kerf-scoring, FIFO-bead, FIFO-work, random) and **no
> `cmd/kerfsim` binary** wiring them together. See `plans/007_simulator/` —
> this is bead B10 territory. Until those land, scenarios below cannot
> produce measured metrics.
>
> Each scenario YAML below loads + validates against `internal/sim/scenario`
> (verified via a one-shot `go run` against `scenario.Load`).
> "Results" sections record **predictions** derived from reading
> `internal/queue/queue.go`. Verdicts are deferred.

Scenario YAMLs live in `/tmp/kerfsim-adversarial/scenarios/`.

## Scoring summary (from `internal/queue/queue.go`)

```
score = fanOut * 10            // transitive must-complete-first dependents
      + (complete/total) * 5   // momentum: completion ratio, max 5
      + reworkCount * 15       // per-bead, additive — unbounded
      + (n-1-creationRank) * 0.1
```

- Fan-out is a **count**, not effort-weighted. One trivial leaf = one deep
  12-bead dependent.
- Rework is **per bead, additive, unbounded**. Six rework beads add +90 —
  more than fan-out of 9.
- Momentum is capped at 5 (small effect).
- Creation order is capped at ~3 for typical work counts (~30) (tiny effect).
- **Areas play no role in scoring.** They show up only in display, not
  ordering or collision avoidance.

---

## 1. adv-fanout-trap

**Hypothesis.** kerf prefers `trunk-trivial` (fanOut=15, all dependents are
1-bead trivial leaves) over `trunk-deep` (fanOut=3, dependents are 12-bead
real work). Total downstream effort is far larger behind trunk-deep, but
fan-out is a *count*. Result: kerf finishes work_completed at roughly the
same rate as FIFO but inflates `wall_ticks` and `agent_ticks_total` because
it lets agents pile onto trivial leaves while the long-tail deep beads sit.

**Scenario.** `/tmp/kerfsim-adversarial/scenarios/adv-fanout-trap.yaml`

**Results (predicted).**
- kerf finishes trunk-trivial + 15 leaves before trunk-deep is even started.
- FIFO-by-work alternates trunk-trivial and trunk-deep, so deep beads start
  earlier and the critical path shortens.
- Predicted kerf loss: `wall_ticks` ~10–20 % worse than FIFO-by-work at
  agents=3.

**Hypothesis verdict.** Pending (sim not runnable end-to-end).

---

## 2. adv-rework-swamp

**Hypothesis.** Six explicit rework beads land on `stale-work` early
(ticks 50–500). Each adds +15, so stale-work scores ~90 from rework alone —
larger than `new-hotness`'s fan-out of 4 (= +40). kerf will starve
new-hotness even though no one cares about the stale work. FIFO-by-bead
mixes by arrival time; FIFO-by-work ignores rework signal entirely.

**Scenario.** `/tmp/kerfsim-adversarial/scenarios/adv-rework-swamp.yaml`

**Results (predicted).**
- `goal_completion_3d` for new-hotness: kerf < both FIFO variants.
- `priority_inversions` for kerf will be **low** (kerf's rework-first
  behavior is "correct" per its own definition) — which exposes that the
  metric measures policy self-consistency, not real-world value.

**Hypothesis verdict.** Pending.

**Note.** This is the highest-confidence loss in the set: rework
multiplier of 15× per bead, additive, unbounded, is a structurally
strong signal that's easy to weaponize.

---

## 3. adv-momentum-lock

**Hypothesis.** `marathon` (30 beads) collects rework + momentum bonuses
as it progresses. At tick ~5000 an `urgent` work (fanOut=6) becomes
actionable. urgent's score is fanOut*10=60. marathon's score is
fanOut*10 + completion*5 + rework_count*15 — by the time it's half done
with a steady rework drip, easily ≥ 60. kerf keeps grinding marathon.

**Scenario.** `/tmp/kerfsim-adversarial/scenarios/adv-momentum-lock.yaml`

**Results (predicted).**
- `top_of_queue_churn` is **artificially low** for kerf — it sticks on
  marathon. Low churn looks "good" but here means "missed the pivot".
- `goal_completion_1d` for urgent: kerf < FIFO-by-work.
- Caveat: momentum alone caps at +5, so the lock-in is really driven by
  the rework drip. Without the rework, the bonus is too small to lock in.
  This blurs the line between #2 and #3.

**Hypothesis verdict.** Pending.

---

## 4. adv-area-collisions

**Hypothesis.** Four top-scoring works all share area `core`. Kerf's
scoring has zero area-awareness, so it will happily dispatch two
`core` works to two agents simultaneously. The four isolated `iso-*`
works (fanOut=0) are deprioritized despite being collision-free.

**Scenario.** `/tmp/kerfsim-adversarial/scenarios/adv-area-collisions.yaml`

**Results (predicted).**
- `area_collisions` is the worst-case metric for kerf here: 4 agents
  pulling from 4 same-area works.
- FIFO-by-bead happens to spread better only by accident (arrival
  interleaving across works).
- This is not a weight-tuning problem; it's a **signal gap**.

**Hypothesis verdict.** Pending.

---

## 5. adv-cascade-chain

**Hypothesis.** A 15-work linear chain has ≤1 actionable work at any
tick. All policies converge to the same dispatch sequence; kerf has no
parallelism leverage to exploit. With 3 agents, idle_pct ≈ 67 % across
the board.

**Scenario.** `/tmp/kerfsim-adversarial/scenarios/adv-cascade-chain.yaml`

**Results (predicted).**
- kerf == FIFO on completion metrics.
- kerf "loses" only in that it spends per-`next` compute on
  `queue.Compute` with one actionable work in the set — pure overhead.
- Useful as a sanity baseline: a scenario where kerf cannot win.

**Hypothesis verdict.** Pending. Not really a loss case — included to
calibrate the "no-headroom" floor.
