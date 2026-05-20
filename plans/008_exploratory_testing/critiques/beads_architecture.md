# Plan 008 — Architecture Critique of beads.md

Scope: file-ownership, package boundaries, sizing of B3 (B:F4), instrumentation
sufficiency of B14.

## 1. File-ownership: `cmd/next.go` parallelism is over-claimed

beads.md asserts "No two beads modify the same file" and sequences
`cmd/next.go` as **B3 → B4 → B6 → B10-code**. The Parallelization Plan then
lists **B1, B2, B3, B4, B6, B7, B8 — up to 7 workers** in Phase 0. B3, B4, B6
all edit `cmd/next.go` and cannot run concurrently. The honest concurrency
ceiling for Phase 0 is **5** (B1, B2, B3-or-B4-or-B6, B7, B8), not 7. The
table at line 345 should read:

- Parallel-safe: B1, B2, B7, B8, plus exactly one of {B3, B4, B6}.
- Serialized after the first `cmd/next.go` lands: the other two.

B3 is also the largest of the three and the most invasive — start it first,
not "one of three." Recommend hoisting B3 above B4/B6 explicitly.

Secondary collision: B10-code lists `cmd/next.go`, `cmd/list.go`,
`cmd/map.go`, `cmd/show.go` as edit sites. B5 also owns `cmd/show.go` and
`cmd/map.go`. The dependency graph mentions neither edge. Add
**B10-code depends on B5** (and on B1, transitively) or scope B10-code's
swallow-site sweep to `cmd/next.go` only.

## 2. Package boundaries / cycles

B3 adds `BeadToWork map[string][]string` to `feed.Input`. `feed` already
imports `beads`, `queue`, `spec` — no new cycle. Fine.

B14 lists edits to `cmd/kerfsim/` and `internal/sim/generator/`. The hooks
file at `internal/sim/metrics/hooks.go` already imports `beads`, `event`,
`loop`, `store`. Adding `--debug DispatchInfo` JSONL in the **command**
layer (not `metrics`) is correct — keep flag plumbing out of `internal/sim/*`
to preserve the loop↔metrics isolation noted in hooks.go's header.

## 3. B3 sizing — confirmed ~1 day, not 1-line

`feed.BeadSource` (feed.go:59–91) hard-codes `work := b.Epic` and emits one
item per bead. The fix requires: (a) a new `Input` field, (b) godoc for
multi-match semantics, (c) iteration shape change (one bead → N items), (d)
caller construction in `cmd/next.go` after the existing `ForWorkWithFilter`
loop, (e) four new tests including the cleanup-detector interaction. The
1-day estimate is right. Anyone scoping this as a join-key rename is wrong.

One omission: `cmd/show.go` and `cmd/map.go` also read `b.Epic` implicitly
via `ForWork`. B5 already migrates them, but the **JSON contract test**
(`work_codename` population under multi-match) should also be exercised in
`cmd/show_test.go`, not only `feed_test.go`. Add to B3's test list.

## 4. B14 instrumentation — under-specified

The DispatchInfo JSONL field list (`tick, agent_id, bead_id, work,
is_rework, eligible_set_size, depsAllClosed, in_warmup`) is missing:

- **`arrival_tick`** — without it you cannot reconstruct
  `olderReworkEligible` post-hoc.
- **`older_rework_eligible`** — the very bit the inversion metric counts;
  emit the boolean from `hooks.go:81` directly, don't re-derive.
- **`warmup_cutoff`** (once per run) — `in_warmup` alone doesn't tell you
  whether the cutoff swallowed the entire scenario.
- **Per-bead `deps`** — `depsAllClosed=false` is uninformative without the
  unmet dep list; rework `DependsOn` wiring is the prime suspect.

Recommend B14 also dump `ArrivalInfo` records (not only dispatches) so
arrivals that never become eligible are visible. Otherwise B14 risks
producing a diagnosis that says "metric is zero" without a root cause.
