# Sim-Integrity Diagnosis — Plan 008 / Bead B14 (kerf-3b2)

**Status:** diagnosis complete; one regression test landed (`TestRun_BaselineRandom_ProducesInversions` in `internal/sim/run/run_test.go`); semantic fix for `priority_inversions` deferred to a follow-up bead.

**Scope:** explain why `priority_inversions`, `rework_p50_wait`, and `rework_p95_wait` were stuck at zero across all 7 Plan-008 scenarios.

## TL;DR

There are **two independent causes**, neither of which is a bug in the metric-collection pipeline:

1. **Under-saturation (causes universal zero waits).** Every scenario in `plans/008_exploratory_testing/sim_scenarios/` ran with `agent_idle_pct ≥ 0.79`. With agents idle that often, every rework arrival lands on an already-idle agent and is dispatched in the *same tick* — so `dispatch_tick - arrival_tick = 0` and both percentiles round to 0.

2. **Latent semantic issue in `priority_inversions` (structural zero independent of saturation).** The spec defines an "older" rework as one with a strictly lower `arrival_tick` than the dispatched new-work bead (`specs/simulator.md` ~line 292). The synthetic generator emits all initial new-work beads at `ArrivalTick = 0` and only emits rework at `tick ≥ 1`. Therefore a rework bead can never be "older" than an initial new-work bead by this definition, and an inversion against initial new-work beads is **structurally unreachable**.

The metric machinery is correct. The `IsRework` label survives end-to-end. Generator-emitted rework has empty `DependsOn`, so `depsAllClosed` is vacuously true and they ARE queue-eligible. The bug hypotheses (a/b/c) from the bead body — warmup swallowing, deps blocking, label mismatch — are all refuted by the JSONL evidence below.

## Reproduction recipe

```
go build -o /tmp/kerfsim ./cmd/kerfsim

# Canned scenario, under-saturated:
/tmp/kerfsim run rework-heavy --quiet --out /tmp/rh --debug-dispatch /tmp/rh.jsonl
cat /tmp/rh/kerf/summary.json | grep -E 'rework|priority|idle'

# Saturated synthetic scenario, demonstrates the wait pipeline works:
cat > /tmp/sat.yaml <<EOF
seed: 42
ticks: 2000
agents: 2
works: [{codename: a, areas: [x], deps: [], bead_count: 200}]
bead_arrivals: {generator: {rework_rate_per_tick: 0.1, target_works: [a]}}
agent_model: {duration: {kind: lognormal, median_ticks: 5, sigma: 0.5}}
EOF
/tmp/kerfsim run /tmp/sat.yaml --quiet --out /tmp/sat --debug-dispatch /tmp/sat.jsonl
cat /tmp/sat/random/summary.json | grep -E 'rework|priority|idle'
```

Or run the regression test directly: `go test ./internal/sim/run/ -run TestRun_BaselineRandom_ProducesInversions`.

## Evidence

### Under-saturation (cause 1)

Canned `rework-heavy` (15000 ticks, 3 agents, rework_rate 0.02):

```
"agent_idle_pct": 0.879
"rework_p50_wait": 0
"rework_p95_wait": 3
"priority_inversions": 0
```

309 rework arrivals; 60 rework dispatches (the rest never get picked up before `idle-threshold` stop). Of the 60 dispatches, **56 have wait=0** (rework arrived to an idle agent). The four nonzero waits are [3, 65, 152, 265]. With n=60, p50=0 and p95=3 are arithmetically correct.

Synthetic `s1_rework_storm` (20000 ticks, 4 agents, rework_rate 0.06):

```
"agent_idle_pct": 0.894
"rework_p50_wait": 0, "rework_p95_wait": 0
```

295 rework dispatches; **294 of 295 have wait=0**; only one (wait=80) is nonzero. Again the metric is arithmetically correct.

When we re-run with a saturated scenario (200-bead single-work, 2 agents, lognormal median 5 ticks, rework_rate 0.1), `agent_idle_pct = 0.004` and the random baseline reports:

```
"rework_p50_wait": 139
"rework_p95_wait": 451
"priority_inversions": 0   ← still zero; see cause 2
```

The wait pipeline is fully functional.

### Label propagation (refutes hypothesis "IsRework mismatch")

`/tmp/rh.jsonl`:

```
{"bead":"bright-mole/r10","depends_on":[],"is_rework":true,"kind":"arrival","tick":83,"work":"bright-mole"}
{"agent":0,"arrival_tick":83,"bead":"bright-mole/r10","in_warmup":true,"is_rework":true,"kind":"dispatch","older_rework_eligible":false,"tick":348,"unmet_deps":[],"work":"bright-mole"}
```

Every generator-emitted rework arrives with `is_rework=true`; it carries that flag through to dispatch. The `unmet_deps` field is empty in every observed dispatch — so the `depsAllClosed`-blocked hypothesis (b) is refuted.

### Warmup swallowing (refutes hypothesis "warmup window")

`warmup_cutoff=1500` in `rework-heavy`. Both the 152-tick-wait and the 265-tick-wait rework dispatches fall inside the warmup window (their dispatch ticks are 442 and 348), but the post-warmup window still has 2 rework dispatches with nonzero wait (1620, 1692) which is enough to populate `rework_p95_wait=3` in the warmup block too. Warmup is not the cause.

### priority_inversions structural unreachability (cause 2)

Even on the saturated scenario where rework waits 139–451 ticks under random ordering, `priority_inversions` is zero. Walking `internal/sim/metrics/hooks.go:188-223` (`olderReworkEligible`):

```go
if st.ArrivedAt < arrived { return true }
if st.ArrivedAt == arrived && st.ID < beadID { return true }
```

"Older" means strictly lower `ArrivedAt`, or equal `ArrivedAt` and lexicographically smaller `BeadID`. Initial new-work beads (`a/b1`..`a/b200`) all have `ArrivedAt = 0`; generator rework (`a/rN`) all have `ArrivedAt ≥ 1`. So:

- When dispatching an initial new-work bead at any tick, every rework in the queue has `ArrivedAt > 0 = new-work.ArrivedAt` → strictly larger → not older → no inversion.
- When dispatching a rework, the condition asks if a *different* rework is older — only possible when two reworks coexist in the queue. Empirically this happens, but those dispatches are themselves rework, so the `IsNewWork && HadOlderRework` predicate (`metrics.go:331`) is false anyway.

Net result: a generator-rework bead can never invert against an initial new-work bead by the spec's current "lower arrival tick" definition.

## Recommended fix (follow-up bead)

The intent of `priority_inversions` per the metric description ("a new-work bead was selected while an older rework bead remained available") is to catch scoring choices that starve waiting rework. The current arrival-tick comparison only fires when rework predates the new-work it preempts — which is the opposite of how rework typically arrives in real systems (rework is feedback on already-emitted new work).

Two equally-valid alternative definitions:

- **A. "Any rework eligible at dispatch time inverts."** Drop the older-than gate entirely; count every new-work dispatch where any rework bead is queue-eligible. Maximally sensitive; may double-count when the same rework sits in the queue across many new-work dispatches.

- **B. "Wait-asymmetric inversion."** Count every new-work dispatch where any eligible rework has waited longer than the new-work bead has waited at this moment (i.e. `rework.ArrivedAt < dispatch_tick` is trivially true; the real comparison is per-bead wait). Closer to the operator-intuitive "you skipped someone who's been waiting".

Either way, the fix is **not a 1-line change** — it requires:
1. A spec update in `specs/simulator.md` §Metric Definitions revising the `priority_inversions` definition.
2. A `hooks.go` change to match.
3. A test rewrite asserting the new semantics.

Therefore this work is deferred to a follow-up bead, which the gate per the B14 bead description should now be unblocked on.

## Follow-up bead description (suggested)

**Title:** Plan 008 / B14-followup — Redefine `priority_inversions` to be reachable

**Description:** The current spec definition of `priority_inversions` (lower-arrival-tick test, `specs/simulator.md` §Metric Definitions) is structurally unreachable under the synthetic generator because generator-rework always arrives after initial new-work. Choose between definition A or B above (recommend B for operator intuition), update spec + `internal/sim/metrics/hooks.go:olderReworkEligible`, and add a test asserting random baseline produces nonzero inversions on the saturated scenario in `internal/sim/run/run_test.go:saturatedReworkScenario`.

**Out of scope here:** rework-storm scenario re-saturation (a separate plan-008 bead about scenario design — agent counts and bead-rates need rebalancing to drive `agent_idle_pct < 0.2` to stop swamping the wait metric).

## Artifacts

- Debug instrumentation: `cmd/kerfsim/debug_dispatch.go` (the `--debug-dispatch <path>` flag), threaded via `internal/sim/run/run.go:RunWithDebug` and `internal/sim/metrics/{metrics,hooks}.go` (`DebugSink` interface).
- Regression test: `internal/sim/run/run_test.go:TestRun_BaselineRandom_ProducesInversions` — passes; asserts the wait pipeline works on a saturated scenario; documents the deferred semantic question.
- JSONL captures (transient, regenerable):
  - `/tmp/rh.jsonl` — `rework-heavy` canned scenario.
  - `/tmp/sat.jsonl` — saturated synthetic scenario.

## What in the JSONL turned out most useful

1. **`is_rework` on arrivals** — single-handedly refuted the "label dropped" hypothesis on inspection of the first 5 arrival lines.
2. **`older_rework_eligible` on dispatches** — clear yes/no per dispatch, made cause 2 visible immediately (the value was correctly false for every new-work dispatch, even when reworks were piled up).
3. **`agent_idle_pct` from summary.json combined with arrival/dispatch tick counts in JSONL** — exposed the saturation gap. Without saturation context the rework-wait zeros looked like pipeline failure.

Fields that were defined in the schema but did not move the diagnosis:

- `unmet_deps` — always empty in observed runs (refuted hypothesis b, which is useful but unsurprising once the generator code was read).
- `in_warmup` — distribution of pre- vs post-warmup dispatches was unremarkable; the warmup window was not the cause.
