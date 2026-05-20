# Plan 008 — Exploratory Testing Follow-Up

## Intent

Cleanup pass that lands the actionable findings from the plan 008 exploration
cycle. Three buckets:

- **Now**: code-drift bugs the spec already covers — fix the code.
- **Next**: spec gaps surfaced by the exploration — write the spec line, then
  fix the code that depends on it.
- **Investigation gate**: diagnose the simulator metric anomaly
  (`priority_inversions = 0`, `rework_p50_wait = 0` across 7/7 scenarios)
  before any scoring/weights work proceeds.

Plan 008 does **not** carry triage workflow (`plans/009_triage/`) or
concurrency / multi-agent session signalling (`plans/010_concurrency/`).
Both are flagged here for separate planning.

## Why

The exploration produced six independent-agent reports, three critiques, and a
recheck against `main`. The validated bugs are spec-covered code drift; they
ship in days, not weeks. A handful of items need a spec line before code can
land (per CLAUDE.md: spec first, code second). And the simulator's flat-zero
inversion/rework-wait metrics make every scoring hypothesis unreviewable
until we know why the metrics never fire — that's the gate.

The exploration also re-priced two items the action list under-scoped:

- The `work_codename: null` / spurious `work_no_attached_beads` cleanup
  (B:F4) is **structural** (`feed.BeadSource` discards the resolved filter
  and joins on `b.Epic`), not a one-line patch — budget ~1 day.
- The sim-integrity investigation is 1–2 engineer-days, not "medium."

## What Changes

### Now — code only, no spec change required (~4 engineer-days)

Listed in commit order; some items have a sequencing constraint noted.

1. Replace `cmd/show.go` shell-out to `bd list` with `internal/beads.List()`
   (the binary is `br`, and there is no `--project` flag). Sequence first;
   the legacy `ForWork` migration below depends on it.
2. Same fix for `cmd/square.go` — silent zero counts today.
3. **B:F4 structural fix.** Build a `BeadToWork map[string]string` in
   `cmd/next.go` and thread it through `feed.Input` so the resolved filter
   actually flows to attachment. Eliminates `work_codename: null` and
   spurious `work_no_attached_beads` cleanups. Highest agent-UX value in
   the Now bucket.
4. Stop excluding works with unknown statuses from `kerf next`
   (Invariant 5 in `specs/coordination.md`).
5. Migrate `cmd/show.go` and `cmd/map.go` off the legacy
   `internal/beads.ForWork` (which uses `EqualFold`) to
   `ForWorkWithFilter`, then tighten `ForWork` to case-sensitive or remove
   it. **Must follow item 1.**
6. Recompute `kerf next` unmatched-bead count after the open-bead filter
   step (A:F5 sub-bug — header count diverges from listing).
7. Snapshot-test `kerf next --help` and bare `kerf` against
   `specs/commands.md`. Run the pass/status/stage terminology audit first
   so the snapshot does not lock in drift.
8. Root help "Available commands" — add `init`, `setup`, `localize`,
   `next`, `map`, `areas` per `specs/commands.md` §"Discoverability".

### Next — spec edit first, then code

Each entry below is a (spec-bead → code-bead) pair.

- **`commands.md` §`kerf init` re-run rule.** Define merge / skip / abort
  behaviour when `project.yaml` exists. Unblocks the
  `cmd/init.go` idempotency fix + `detectBeadFilter` repair.
- **`commands.md` §`kerf next` warning kinds.** Add `corrupt_spec` and
  `no_project_yaml` to the warning table (and mirror in
  `coordination.md`). Unblocks the `internal/feed/warning.go` detector
  that today swallows `spec.Read` errors silently.
- **`coordination.md` §"Drift detection".** Specify the hash scope used to
  detect relabel drift (the one mutation of five not currently surfaced).
- **`simulator.md` §`top_of_queue_churn`.** Clarify behaviour when there
  is a single candidate — in-place tightening, no caller-visible change.
- **Spec sweep (`bd` → `br`).** Residual `bd` references in
  `jig-implementation.md`, `architecture.md:237`, `verification.md:50`,
  `commands.md:640/662/1320`, `jig-system.md:62`,
  `coordination.md:190/257`. Mirror into
  `internal/jig/builtin/implementation.md`. Pure text edits.

### Investigation gate — diagnose before tuning

The simulator reports `priority_inversions = 0` and `rework_p50_wait = 0`
across every scenario. Static reads of `internal/sim/metrics/hooks.go` and
`metrics.go` look correct, so the bug is in runtime conditions
(warmup-window swallow, generator rework `DependsOn` keeping
`depsAllClosed` false, etc.). One dedicated bead:

- Add a `--debug DispatchInfo` flag to `cmd sim run` that emits dispatch
  records as JSONL.
- Run one scenario; verify dispatches reach `inWindow(d.Tick)`.
- Inspect `internal/sim/generator/generator.go` rework dependency wiring.
- Add `TestRun_BaselineRandom_ProducesInversions` with a constructed
  scenario where random ordering **must** invert.

**All design/scoring hypotheses (rework cap, momentum cut, effort-weighted
fan-out, etc.) are blocked behind this gate.** Any baseline captured with
broken metrics is contaminated.

## Specs Affected

| Spec file | Change |
|-----------|--------|
| `specs/commands.md` | Add re-run rule for `kerf init`; add `corrupt_spec` + `no_project_yaml` warning kinds; bare-`kerf` active-work count scoped to resolved project; bd→br residual text. |
| `specs/coordination.md` | Drift hashing scope for relabel detection; bd→br residual text (lines 190, 257). |
| `specs/simulator.md` | Clarify `top_of_queue_churn` single-candidate case. |
| `specs/architecture.md` | bd→br residual (line 237 default). |
| `specs/jig-implementation.md` | bd→br residual (multiple lines per ACTION_LIST.md). Mirror into `internal/jig/builtin/implementation.md`. |
| `specs/jig-system.md` | bd→br residual (line 62). |
| `specs/verification.md` | bd→br residual (line 50). |

## Implementation Notes

1. **B:F4 is the one structural fix.** `feed.BeadSource` currently keys on
   `bead.Epic`; the resolved filter from `cmd/next.go` is discarded after
   `beads.ForWorkWithFilter`. Build the bead→work map at the call site,
   thread it through `feed.Input`, and join on that map inside the bead
   source. Touches a load-bearing JSON contract — needs explicit test
   coverage on `work_codename` population and on the
   `work_no_attached_beads` detector clearing.

2. **Show-shell-out lands before `ForWork` migration.** Same file
   (`cmd/show.go`). One worker, two sequential beads — the second cannot
   start until the first is merged.

3. **No two beads modify the same file.** The Now bucket is split so that
   `cmd/show.go`, `cmd/square.go`, `cmd/next.go`, `cmd/root.go`,
   `cmd/init.go` each have a single owner per bead.

4. **Spec beads precede their code beads.** The plan does not promise
   simultaneous landing; the code bead in each pair lists its spec bead
   as a hard dependency in `beads.md`.

5. **Sim investigation is a single bead.** Output is a written diagnosis
   (root cause + proposed fix) checked into `plans/008_exploratory_testing/`.
   Any code fix lands as a follow-on bead created from that diagnosis —
   not committed inside this plan.

6. **Out of scope.**
   - Triage workflow (`kerf show` bead rendering, `kerf triage`, drift
     state file, attach/edit commands) → `plans/009_triage/`.
   - Concurrency / session ownership signalling on `kerf next`
     (top-ranked work has active session held by another agent) →
     `plans/010_concurrency/`.
   - Scoring/weights tuning (all 10 hypotheses) → future plan after the
     investigation gate clears.
   - `go:embed` deduplication of `jig-implementation.md` ↔
     `internal/jig/builtin/implementation.md` — flagged as
     architecture-change risk; defer.

## Implementation Beads

See [beads.md](beads.md) for the breakdown. Grouped into Phase 0 (Now),
Phase 1 (Next), Phase 2 (Investigation gate).
