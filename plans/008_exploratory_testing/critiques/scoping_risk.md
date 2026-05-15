# Scoping & Risk Critique

## Mis-sized items (estimate looks wrong)

- **P0#1 (B:F4 attachment) — claimed "medium", actually structural.** `cmd/next.go:168-169` resolves the filter and uses the result for `EpicSummary` counts only. The resolved filter is **never** threaded into `feed.BeadSource`. `BeadSource` (feed.go:59-91) reads `b.Epic` and ignores labels. Fix is a contract change: either (a) extend `feed.Input` with a per-work matched-bead map and rewrite `BeadSource` to consume it, or (b) set `WorkCodename` from a label-derived lookup. Touches `internal/feed/feed.go`, `internal/feed/feed_test.go`, `cmd/next.go`, plus likely `cleanup.go`/`warning.go` for consistency. Estimate: 1 day, not "medium afternoon".
- **Sim integrity blocker — claimed "medium investigation", reality 1–2 days.** Wiring in `hooks.go:65-93` and `metrics.go:331` is correct on inspection; tests pass. The 0-inversions result must come from runtime conditions (warmup window dropping all dispatches, generator's rework arrivals never coinciding with new-work dispatch, or `Heap` shape causing rework to dispatch immediately every tick). Diagnosing requires per-scenario dispatch-trace instrumentation, not a code grep. See "Specific diagnoses".
- **Triage #6 (`kerf triage` report) — "Medium" is optimistic.** Aggregating untriaged + multi-matched + external-changes + per-work-health + `--resolved` exit codes + drift-state diff = new command surface, new state file format, new spec section, new tests. Realistically 200+ LOC bead, plus spec changes. Should be a sub-plan, not a single bead.
- **Workflow #1 (`kerf where` / `kerf doctor`) — "Small" understates.** Two commands collapsed into one bullet; doctor implies tool-availability + sentinel checks already overlapping with `verify-tools` (#2). Split.
- **Follow-up sweeps "Consider deduplicating via `go:embed`"** — not small. Build-time embed of a spec file into the jig package can cause import cycles and shifts the source-of-truth contract.

## Hidden dependencies

- **All "Design / Scoring" items depend on sim-integrity blocker.** Action list says so for rework, but momentum, fan-out, creation-order tuning all rely on metrics that may share the rework wiring defect. Treat the whole bucket as gated.
- **P1#5 (migrate `ForWork` callers) depends on P0#3 (`cmd/show.go` rewrite).** P0#3 says "delegate to `internal/beads.List()`" — that rewrite is where the `ForWork`→`ForWorkWithFilter` switch lands. Doing P1#5 first is wasted churn.
- **P1#1 (init clobber) and P1#2 (`detectBeadFilter` never fires)** both touch `cmd/init.go:113`/`:208`. Combine.
- **Triage #1 (`kerf show` renders beads) duplicates work in P0#3.** Action list flags this ("subsumes part of A:F2"). Sequence P0#3 first, then triage #1 becomes "add count column".
- **P1#9 (no-project.yaml warning) depends on P0#2 (corrupt spec.yaml warning)** — both add to `internal/feed/warning.go`. Same detector skeleton; ship together.

## Sequencing issues

- Do **P0#3 + P0#4** (bd→br shell removal) **before** any scoring/sim work; the silent zero counts contaminate any baseline data captured from current binaries.
- Do **sim integrity fix before promoting adversarial scenarios** (Design bullet 10). Validating new scenarios with broken metrics produces noise that survives the fix.
- Do **P0#1 (B:F4) before P1#3 (relabel drift)**. Drift detection layered on top of broken attachment will surface false positives.
- Do **terminology audit (Workflow #4) before help-text snapshot test (Workflow #3)**. Snapshotting current drift just locks it in.

## Risky touches (need extra-careful review)

- **`feed.BeadSource` contract change (P0#1)** — load-bearing JSON contract; every downstream renderer and the JSON test fixtures (`feed_test.go:62`) depend on field shape. Any change risks consumer breakage.
- **`internal/sim/metrics/hooks.go` (sim blocker)** — touches queue-scoring evaluation; a wrong fix invalidates all prior sim runs.
- **`cmd/init.go:113` no-clobber (P1#1)** — first-run vs. re-run divergence is exactly the path that lost a user's `bead_filter`. Merge semantics need a spec line before code.
- **`internal/beads/beads.go:167` `ForWork` deletion (P1#5)** — symbol used by `cmd/show.go`, `cmd/map.go`. Mass rename without callsite audit is the classic break.
- **Jig-template embed (Follow-up sweeps tail)** — single source of truth migration; very easy to ship with stale embedded bytes.

## Test-debt

- **P0#1 (B:F4):** add a `TestBeadSource_WithLabelFilter` covering: bead has `Epic=""` but `work:foo` label, and resolved filter matches by label → `WorkCodename == "foo"`. No such case in `feed_test.go`.
- **P0#2 (corrupt spec.yaml):** add `TestNext_CorruptSpecYaml_EmitsWarning` against a fixture with malformed YAML in one work dir.
- **P0#3/#4 (shelled bd):** add `cmd/show_test.go` and `cmd/square_test.go` asserting bead counts populate from `internal/beads.List()`; without these, the silent-failure regression returns when someone "optimizes" back to exec.
- **Sim integrity blocker:** add `TestRun_BaselineRandom_ProducesInversions` — a constructed scenario where the random policy *must* invert (rework arrives at tick 1, new-work at tick 2, both eligible). If the assertion currently fails, the bug is reproduced; if it passes, the bug is scenario-specific.
- **P1#4 (counter staleness):** snapshot test of header count vs row count after a simulated `br close`.
- **P1#6 (unknown status visibility):** unit covering `spec.SpecYAML` with `status: "foo"` not in `status_values` → still rendered.
- **Workflow #3 (help snapshot):** the snapshot itself is the test; ensure it runs in CI.

## Out-of-scope drift

- **Triage workflow bucket** is labelled "new-plan scope" but interleaved with P0/P1 items. Promote to `plans/009_triage/_plan.md`; otherwise it bleeds capacity from bug-fix sprint.
- **Sim adversarial-scenario promotion** is a research item, not a bug. Belongs in Plan 007 follow-up, not 008.
- **`go:embed` deduplication** of `jig-implementation.md` is an architecture change; out of scope for an exploration synthesis.
- **`kerf status --auto` (Workflow #9)** — new feature with ordering rules; the action list flags "new plan; needs ordering rules", which means it's not actually in scope.

## Specific diagnoses

### Sim-integrity blocker: 1–2 days, not 1 hour

Static read of `hooks.go:65-93`, `hooks.go:188-223`, `metrics.go:325-340` shows the wiring is consistent: `IsRework` derives from labels, `HadOlderRework` walks the store, the dispatch record reaches the counter. The bug therefore lives in **runtime conditions**, one of:

1. **Warmup window swallows everything.** `inWindow(d.Tick)` (metrics.go:331) — if warmup is N=0 and the cap is small, every dispatch may fall outside the window. Check `Config.Deadline1d/3d/7d`.
2. **Rework arrives but never sits in queue.** If kerf policy + baselines both pick rework first when present, `HadOlderRework` is true only at the moment a *new-work* dispatch happens, which may be never if rework supply is ≥ agent capacity. Across 7 scenarios all dominated by `agent-idle ≥ 0.79`, that's plausible — but doesn't explain baselines.
3. **Generator emits `rework:true` but at ticks where the bead's `depsAllClosed` returns false** (the rework beads depend on a non-closed bead, so they're never eligible per `olderReworkEligible`). Worth checking — generator.go:215 likely sets `DependsOn`.

Action: add a `--debug` flag to `cmd sim run` that dumps `DispatchInfo` JSONL, run one scenario, inspect. Realistic estimate: 1 day to instrument + diagnose + fix + regression test. Worst case 2 days if the fix touches generator semantics.

### B:F4 (work_codename: null): structural, not a 1-line fix

Root cause: `BeadSource` (feed.go:72) uses `b.Epic` as the work key, but the filter system matches by **labels** (`work:<codename>`). The resolved filter computed in `cmd/next.go:168` is discarded after summary-counting. A bead matched by label-only (with bd's `epic` field empty or pointing elsewhere) emits with `WorkCodename: nil`.

Minimum fix: in `BeadSource`, for each bead, walk `in.Works` and pick the first work whose resolved filter matches the bead (helper exists: `beads.ForWorkWithFilter`). That requires `Input` to expose `ProjectFilter` (already there) and re-resolve per-bead, OR pre-build a `BeadToWork map[string]string` in `cmd/next.go` and add it to `Input`.

The latter is cleaner: ~20 lines in `cmd/next.go` to build the map, ~10 lines in `feed.go` to consume it, ~30 lines of new tests. Call it a half-day. **Not** a 1-line wiring fix — `BeadSource`'s contract has to change.

### Triage workflow real cost

Items 1-5 are small individually but cumulatively touch: `cmd/show.go`, `cmd/map.go`, `cmd/new.go` (new flag), `cmd/work.go` (new subcommand), `cmd/attach.go` (new command), `internal/spec/*` (safe mutation of `spec.yaml`), specs/commands.md (5 new sections). Item 6 (`kerf triage`) plus item 7 (drift state file) add a new command, a new file format, a new spec section, and CI-loop exit-code contract.

Honest sizing: 1.5 sprint-weeks for one engineer if done as a unit, vs. action list's implicit "afternoon of small beads + one medium bead". Should be Plan 009 with its own breakdown.
