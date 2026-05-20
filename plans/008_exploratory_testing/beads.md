# Plan 008 — Implementation Beads

## Overview

Plan 008 lands the validated findings from the exploration cycle. **15 beads
across 3 phases:**

- **Phase 0 (Now, 8 beads, real max 4 parallel workers):** code-drift
  fixes the spec already covers. No spec edit required. `cmd/next.go`
  is serialized B3 → B4 → B6 (and B10-code later); B7 runs last. ~2
  engineer-days wall-clock at 4 workers.
- **Phase 1 (Next, 5 spec→code pairs spanning 6 beads):** each item has a
  spec bead that must merge before the code bead starts. ~2–3 days code
  plus spec writing time.
- **Phase 2 (Investigation gate, 1 bead):** diagnose the sim-integrity
  metric anomaly. 1–2 days; **starts day 1 in parallel with Phase 0**
  (longest single bead). Output is a written diagnosis; any code fix
  spawns a follow-on bead outside this plan.

Triage workflow (plan 009) and concurrency (plan 010) are intentionally
out of scope.

## Dependency Graph

```
Phase 0 — Now (parallel; one ordered pair noted)
  ├── B1  cmd/show.go — replace bd-shell with internal/beads.List
  │       ↳ (must merge before B5)
  ├── B2  cmd/square.go — same fix
  ├── B3  cmd/next.go + internal/feed — B:F4 structural attachment fix
  ├── B4  cmd/next.go — keep unknown-status works visible (Invariant 5)
  ├── B5  cmd/show.go + cmd/map.go — migrate off legacy ForWork
  │       depends: B1 (file ownership of cmd/show.go)
  ├── B6  cmd/next.go — recompute unmatched-bead count after open filter
  ├── B7  testdata + cmd — help-text snapshot for `kerf next --help`
  │       and bare `kerf` (run terminology audit pass first inside this bead)
  └── B8  cmd/root.go — root help "Available commands" list

Phase 1 — Next (spec → code pairs)
  ├── B9-spec   specs/commands.md — kerf init re-run rule
  │   └── B9-code   cmd/init.go — idempotency + detectBeadFilter repair
  ├── B10-spec  specs/commands.md + specs/coordination.md — corrupt_spec
  │             and no_project_yaml warning kinds
  │   └── B10-code  internal/feed/warning.go + cmd/next.go (and
  │                 list/map/show error swallow sites)
  │                 depends: B10-spec, B5 (file ownership of
  │                 cmd/show.go and cmd/map.go)
  ├── B11-spec  specs/coordination.md — relabel drift hash scope
  │   └── B11-code  internal/feed/warning.go (or sibling) — relabel detector
  ├── B12-spec  specs/simulator.md — top_of_queue_churn single-candidate
  │             clarification (in-place tightening; no code change required
  │             unless current behavior diverges)
  └── B13-spec  spec sweep: bd→br residual text across specs +
                internal/jig/builtin/implementation.md (no code change)

Phase 2 — Investigation gate
  └── B14  diagnose priority_inversions = 0 / rework_p50_wait = 0; write
           diagnosis to plans/008_exploratory_testing/sim_integrity_diagnosis.md
```

## File Ownership

No two beads modify the same file. Where the same file appears more than
once below, the beads are sequenced (B1 → B5 on `cmd/show.go`; B3 → B4 →
B6 → B10-code on `cmd/next.go`, each landing before the next starts).

| File | Owning beads (in order) |
|------|-------------------------|
| `cmd/show.go` | B1, then B5, then B10-code (swallow-site sweep) |
| `cmd/map.go` (swallow-site sweep) | B5, then B10-code |
| `cmd/square.go` | B2 |
| `cmd/next.go` | B3, then B4, then B6, then B10-code |
| `cmd/root.go` | B8 |
| `cmd/init.go` | B9-code |
| `internal/feed/feed.go` (BeadSource + Input) | B3 |
| `internal/feed/warning.go` | B10-code, then B11-code |
| `testdata/golden/*` | B7 |
| `specs/commands.md` | B9-spec, then B10-spec, then B13-spec |
| `specs/coordination.md` | B10-spec, then B11-spec, then B13-spec |
| `specs/simulator.md` | B12-spec |
| `specs/architecture.md`, `specs/jig-implementation.md`, `specs/jig-system.md`, `specs/verification.md`, `internal/jig/builtin/implementation.md` | B13-spec |

## Cross-Cutting Concerns

| Concern | Beads | Spec section |
|---------|-------|--------------|
| Bead-to-work attachment join key (`work_codename` correctness) | B3 (impl), B7 (golden) | `specs/commands.md` §`kerf next` output, §Item shape |
| Case sensitivity in label matching | B5 (migrates callers off `EqualFold` path) | `specs/coordination.md` L232 |
| Warning kinds enum extensibility | B10-spec (defines), B10-code (implements), B11-code (uses) | `specs/commands.md` §Warning kinds |
| Spec drift `bd` → `br` | B13-spec, plus mirror to `internal/jig/builtin/implementation.md` | n/a (pure text sweep) |
| Snapshot test as agent-contract guard | B7 | `specs/commands.md` §Help text |

## Per-Bead Specification

### Phase 0 — Now

#### Bead 1 — `cmd/show.go`: drop `bd list` shell-out
**Specs to read:** `specs/architecture.md` §"Tools" (esp. L237 — note this
still says `bd`; B13-spec fixes it but the *behavior* is already to use
`br` via `internal/beads`), `specs/commands.md` §`kerf show`.
**Package / files:** `cmd/show.go`, new `cmd/show_test.go`.
**Deliverables:**
- Remove `exec.Command("bd", "list", "--json", "--project", ...)` at
  `cmd/show.go` ~L278.
- Call `internal/beads.List(...)` with the project's resolved filter.
- Behaviour unchanged on success; on `br` unavailable, the existing
  graceful-degradation path applies.
**Tests:**
- `TestShow_BeadsAttached_RendersCounts` against a stub `br` source.
- `TestShow_BeadToolUnavailable_DegradesGracefully`.

#### Bead 2 — `cmd/square.go`: same fix
**Specs to read:** `specs/commands.md` §`kerf square`.
**Package / files:** `cmd/square.go`, new `cmd/square_test.go`.
**Deliverables:** identical pattern to B1 at `cmd/square.go` ~L214.
**Tests:** `TestSquare_BeadCounts_NonZero` against stub source.

#### Bead 3 — B:F4 structural fix (~1 day)
**Specs to read:** `specs/commands.md` §`kerf next` output (especially the
`work_codename` field contract), §Cleanup detectors
(`work_no_attached_beads`); `specs/coordination.md` §Bead Attachment.
**Package / files:** `cmd/next.go` (caller), `internal/feed/feed.go`
(adds `BeadToWork` field to `feed.Input` + threads through `BeadSource`).
No new files.
**Deliverables:**
- In `cmd/next.go`, after resolving filters and calling
  `internal/beads.ForWorkWithFilter` per work, build
  `beadToWork map[string]string` mapping bead ID → matching work codename
  (a bead matching N works gets N entries; see note below).
- Multi-match handling: when a bead matches multiple works,
  `BeadSource` emits one item per match (consistent with
  `internal/beads.Match` semantics). Document in `feed.go` godoc.
- Extend `feed.Input` with `BeadToWork map[string][]string` (slice to
  preserve multi-match).
- Update `feed.BeadSource` to consult `BeadToWork` for the join, not
  `bead.Epic`.
**Tests:**
- New `TestBeadSource_WithLabelFilter` — labelled bead with no `Epic`
  field surfaces with the correct `work_codename`.
- New `TestBeadSource_MultiMatch` — one bead, two matching works → two
  items.
- Update existing `internal/feed/feed_test.go` cases.
- `TestCleanup_NoSpuriousWorkNoAttachedBeads` — a work whose filter
  matches at least one bead does not emit the cleanup item.
- New `TestShow_WorkCodename_MultiMatch` in `cmd/show_test.go` — JSON
  contract test asserting `work_codename` is populated correctly when a
  bead matches multiple works (exercises the contract at the caller
  boundary, not only in `feed_test.go`).

#### Bead 4 — Unknown statuses remain visible
**Specs to read:** `specs/_index.md` L75 (system-wide invariants — Invariant 5).
**Package / files:** `cmd/next.go` (~L134–144 status-exclusion block).
**Deliverables:** Treat unrecognized statuses as actionable; do not drop
the work from the feed. Optionally surface a per-work warning (use
existing `warning` kind if a spec hook exists; otherwise just keep
visible — no new warning kind defined here).
**Tests:** `TestNext_UnknownStatus_RemainsVisible` with a work whose
`status` is a string not in the known set.

#### Bead 5 — Migrate `cmd/show.go` + `cmd/map.go` off legacy `ForWork`
**Specs to read:** `specs/coordination.md` L232 (case sensitivity).
**Depends on:** B1 (B1 owns the `cmd/show.go` rewrite first; this bead
swaps the bead-lookup API after).
**Package / files:** `cmd/show.go`, `cmd/map.go`, `internal/beads/beads.go`
(tighten or remove `ForWork`), `internal/beads/beads_test.go`.
**Deliverables:**
- Replace `beads.ForWork` call sites in `cmd/show.go` and `cmd/map.go`
  with `beads.ForWorkWithFilter` (resolving the filter via the same path
  `cmd/next.go` uses).
- Either delete `internal/beads.ForWork` entirely, or tighten it to be
  case-sensitive (matching spec) and mark deprecated. Prefer deletion if
  no remaining callers exist after migration.
- Update `internal/beads/beads_test.go` to reflect the chosen outcome.
**Tests:**
- `TestShow_CaseSensitiveLabelMatching` — `Subsystem:bridge` does not
  match `subsystem:bridge`.
- `TestMap_CaseSensitiveLabelMatching` — same.

#### Bead 6 — Unmatched-bead counter recompute
**Specs to read:** `specs/commands.md` §`kerf next` §Warning header
(unmatched count).
**Package / files:** `cmd/next.go` warning-render path.
**Deliverables:** Recompute the unmatched count from the post-open-filter
bead set, not the pre-filter set. Single-call ordering fix.
**Tests:** `TestNext_UnmatchedHeader_MatchesListed` — after `br close` of
one previously-unmatched bead, the header count and the rendered list
agree.

#### Bead 7 — Help-text snapshot + terminology pass
**Specs to read:** `specs/commands.md` §`kerf next` §Help text (six-element
contract), §"Bare `kerf` invocation" output.
**Package / files:** `testdata/golden/kerf_next_help.txt`,
`testdata/golden/kerf_bare.txt`, `cmd/next_test.go` (or `cmd/root_test.go`)
golden-comparison helpers.
**Deliverables:**
- One-pass audit of pass/status/stage terminology across `cmd/*.go` help
  strings and `specs/commands.md`; reconcile to a single term (recommend
  what `commands.md` already uses; do not invent). Land terminology fixes
  in this bead, *then* snapshot.
- Snapshot tests verify each contract element from `commands.md` §`kerf
  next` §Help text (six-element contract) and §"Bare `kerf` invocation"
  is present, using **per-element regex assertions** — not exact byte
  match. (Exact byte match would either lock in implementation drift or
  require adding a "Reference output" block to the spec first; the
  regex-per-element approach matches the spec's content-contract shape.)
**Tests:** golden fixtures plus regex assertions per contract element.
**Sequencing:** B7 edits multiple `cmd/*.go` files for the terminology
pass and therefore conflicts opportunistically with other Phase 0 beads;
run B7 **last** in Phase 0, not in parallel.

#### Bead 8 — Root "Available commands" list
**Specs to read:** `specs/commands.md` §"Discoverability" (the canonical
command list; specifically the requirement at L42 area).
**Package / files:** `cmd/root.go` (~L119–132).
**Deliverables:** Add missing entries: `init`, `setup`, `localize`,
`next`, `map`, `areas`. Preserve existing ordering convention.
**Tests:** Extend `cmd/root_test.go` to assert each command name appears
in the rendered help output.

---

### Phase 1 — Next (spec → code pairs)

Each spec bead must merge before its code bead starts.

#### Bead 9-spec — `kerf init` re-run rule
**Specs to write:** `specs/commands.md` §`kerf init`, add subsection
"Re-running on an existing project" defining: detect existing
`project.yaml`; default behaviour (recommend: preserve existing fields,
merge any newly-detected ones the user confirms, never overwrite a
user-set `bead_filter`); flags or prompts for skip/abort/overwrite.
**Tests:** spec-only.

#### Bead 9-code — `cmd/init.go` idempotency + `detectBeadFilter` repair
**Depends on:** B9-spec.
**Specs to read:** `specs/commands.md` §`kerf init` (post-B9-spec).
**Package / files:** `cmd/init.go`, `cmd/init_test.go`,
`cmd/init_bead_filter_test.go`.
**Deliverables:**
- Honour the re-run rule per spec: do not unconditionally call
  `createDefaultProjectConfig`.
- Repair `detectBeadFilter` (`cmd/init.go` ~L208): verify
  `beads.IsAvailable()` gating, JSON parse path, and trigger threshold;
  the recheck showed 12 prefix-sharing beads produced no detection.
**Tests:**
- `TestInit_Rerun_PreservesBeadFilter`.
- `TestInit_DetectBeadFilter_FiresAboveThreshold` with the same 12-bead
  fixture used in the recheck.

#### Bead 10-spec — `corrupt_spec` and `no_project_yaml` warning kinds
**Specs to write:**
- `specs/commands.md` §`kerf next` §Warning kinds — add two rows
  (`corrupt_spec`, `no_project_yaml`) with title/action/reason templates.
- `specs/coordination.md` mirror entry in feed-warning rules.
**Tests:** spec-only.

#### Bead 10-code — Implement the two warning detectors
**Depends on:** B10-spec, **B5** (B10-code edits `cmd/show.go` and
`cmd/map.go` swallow sites which B5 owns; chain after B5 to avoid file
collision).
**Specs to read:** `specs/commands.md` §`kerf next` §Warning kinds
(post-B10-spec), `specs/coordination.md` §Feed warnings.
**Package / files:** `internal/feed/warning.go`,
`internal/feed/warning_test.go`, `cmd/next.go`,
`cmd/list.go` / `cmd/map.go` / `cmd/show.go` (error-swallow sites).
**Deliverables:**
- Stop swallowing `spec.Read` errors at `cmd/next.go` ~L135–138 and
  analogous sites in `list/map/show`. Surface a `corrupt_spec` warning
  via the feed-warning path.
- Add `no_project_yaml` detector for the
  "empty vs uninitialised" disambiguation.
**Tests:**
- `TestWarning_CorruptSpec_Surfaces` with a deliberately malformed
  `spec.yaml`.
- `TestWarning_NoProjectYaml_Surfaces` against a directory with no
  `project.yaml`.

#### Bead 11-spec — Relabel drift hash scope
**Specs to write:** `specs/coordination.md` §"Drift detection" — define
exactly what bytes feed the per-bead label hash (sorted labels? raw label
slice? include `bead_id`?). One paragraph + worked example.
**Tests:** spec-only.

#### Bead 11-code — Relabel drift detector
**Depends on:** B11-spec.
**Specs to read:** `specs/coordination.md` §"Drift detection"
(post-B11-spec).
**Package / files:** `internal/feed/warning.go` (add detector) or new
sibling file; `internal/feed/warning_test.go`.
**Deliverables:** Detector that diffs current-store labels against the
last-seen manifest (consume the existing drift-state path if present;
otherwise hash-only, no persistence in this bead — note that persisted
last-seen state is plan 009 scope).
**Tests:** `TestWarning_RelabelDrift_Fires` against a fixture pair
showing one bead's labels changed between snapshots.

#### Bead 12-spec — `top_of_queue_churn` single-candidate clarification
**Specs to write:** `specs/simulator.md` §`top_of_queue_churn` (around
L290) — one sentence clarifying whether a head that stays put because it
is the only candidate counts as no-change. Recommended: not-counted, no
denominator increment.
**Tests:** spec-only. No code change unless the chosen rule diverges
from current `internal/sim/metrics` behaviour — in which case spawn a
follow-on code bead in plan 008 or 009.

#### Bead 13-spec — Spec sweep `bd` → `br`
**Specs to write (pure text edits):**
- `specs/architecture.md` L237.
- `specs/verification.md` L50.
- `specs/commands.md` L640, L662, L1320.
- `specs/jig-system.md` L62.
- `specs/coordination.md` L190, L257.
- `specs/jig-implementation.md` lines 30, 45, 115, 116, 118, 130, 154,
  161, 166, 188, 230, 247, 301, 326 — plus `--desc`, `--status
  in-progress`, `bd dep <child> <parent>` example fixes per ACTION_LIST.
- Mirror into `internal/jig/builtin/implementation.md`.
**Tests:** none (text only). Leave `plans/006_*.md` and `plans/007_*.md`
historical references untouched.

---

### Phase 2 — Investigation gate

#### Bead 14 — Diagnose sim-integrity metric anomaly (1–2 days)
**Specs to read:** `specs/simulator.md` §Metrics
(`priority_inversions`, `rework_p50_wait`, `rework_p95_wait`,
warmup window).
**Package / files:** `cmd/kerfsim/` (new `--debug DispatchInfo` flag),
`internal/sim/metrics/hooks.go`, `internal/sim/metrics/metrics.go`,
`internal/sim/generator/generator.go`,
`internal/sim/run/run_test.go` (new test).
**Deliverables:**
1. Add `--debug DispatchInfo` flag to the `sim run` command. Emits two
   JSONL streams:
   - **DispatchInfo** (one record per dispatch): `{tick, arrival_tick,
     agent_id, bead_id, work, is_rework, eligible_set_size,
     depsAllClosed, unmet_deps (list, populated only when
     depsAllClosed=false), older_rework_eligible (boolean emitted
     directly from hooks.go:81, not re-derived), in_warmup}`.
   - **ArrivalInfo** (one record per arrival, regardless of whether it
     ever becomes eligible): `{arrival_tick, bead_id, work, is_rework,
     deps, depsAllClosed_at_arrival}`. Without this, arrivals that
     never become eligible are invisible.
   - Plus a single **run-level** record: `{warmup_cutoff}` so the
     reader can tell whether the warmup window swallowed the scenario.
2. Run `rework-heavy` scenario; capture dispatch JSONL.
3. Verify each dispatch tick passes `inWindow(d.Tick)`
   (`internal/sim/metrics/metrics.go` ~L331). Warmup window may be
   swallowing all dispatches if `Deadline1d/3d/7d` are configured tight.
4. Inspect `internal/sim/generator/generator.go` ~L215 — rework beads'
   `DependsOn` may be keeping `depsAllClosed` false so
   `olderReworkEligible` never fires in `hooks.go` ~L65–93.
5. Add `TestRun_BaselineRandom_ProducesInversions` with a constructed
   scenario where random ordering **must** invert (rework arrives at
   tick 1, new work at tick 2, both eligible, random picks the new
   work some fraction of the time). If the test does not produce
   inversions, the bug is reproduced.
6. Write a diagnosis to
   `plans/008_exploratory_testing/sim_integrity_diagnosis.md` covering:
   reproduction recipe, root cause(s), proposed fix(es), follow-on bead
   description (the fix bead lives outside plan 008).
**Tests:** `TestRun_BaselineRandom_ProducesInversions` (its existence is
the success signal for this bead — it must run, and either pass or fail
with a clearly-pointed-at root cause documented).

**Gate enforcement:** all design/scoring beads (rework cap, momentum
cut, effort-weighted fan-out, area-diversity penalty, staleness penalty,
concurrency cap, external pin, scenario saturation, adversarial
promotion) are blocked until B14's diagnosis lands.

## Parallelization Plan

Real Phase 0 max parallelism is **4 workers**, not 7. `cmd/next.go` is
serialized B3 → B4 → B6 (and B10-code later). B7 edits multiple
`cmd/*.go` files for the terminology pass and runs **last** in Phase 0,
not in parallel.

| Phase | Beads (parallel stream) | Workers | Depends on |
|-------|------------------------|---------|------------|
| Phase 0 stream A (`cmd/next.go` chain) | B3 → B4 → B6 | 1 | — |
| Phase 0 stream B (`cmd/show.go` + `cmd/map.go`) | B1 → B5 | 1 | B5 after B1 |
| Phase 0 stream C | B2 | 1 | — |
| Phase 0 stream D | B8 | 1 | — |
| Phase 0 (serialized last) | B7 | 1 | runs after A/B/C/D land |
| Phase 2 (parallel with Phase 0, start day 1) | B14 | 1 | — |
| Phase 1 (spec) stream A — `commands.md` + `coordination.md` chain | B9-spec → B10-spec → B11-spec → B13-spec | 1 | sequential (shared file edits) |
| Phase 1 (spec) stream B | B12-spec | 1 | — |
| Phase 1 (code) | B9-code, B10-code, B11-code | up to 3 | matching spec bead each; B10-code also depends on B5 |

Real Phase 1-spec parallelism is **2 workers**, not 5: B9-spec,
B10-spec, B11-spec, and B13-spec all collide on `specs/commands.md`
and/or `specs/coordination.md` and must chain on one worker; only
B12-spec (`specs/simulator.md`) is truly independent.

Total: 15 beads. Critical path through Phase 0 is the `cmd/next.go`
chain (B3 → B4 → B6), not B1 → B5. **B14 starts day 1** in parallel
with Phase 0 — it is the longest single bead (1–2 days). Honest
wall-clock estimate: ~2 days for Phase 0 at 4 workers (plus B7
serialized after), +~1 day for Phase 1 spec at 2 workers, +Phase 1 code
+B14 in parallel.

## Judgment Calls (Resolved)

1. **B:F4 fix shape.** Build `BeadToWork` in the caller, thread through
   `feed.Input`. Alternative (storing the resolved filter on
   `feed.Input` and re-matching inside `BeadSource`) was rejected:
   re-matching duplicates work and risks divergence from the
   already-computed bead lists.
2. **Legacy `ForWork`: delete vs deprecate.** Decide at B5 implementation
   time based on remaining callers. Prefer deletion.
3. **Plan 008 does not commit the sim-metric fix.** B14 produces a
   diagnosis; the fix is a follow-on bead so the investigation gate
   stays bounded and reviewable.
4. **No new warning kinds beyond `corrupt_spec` and `no_project_yaml`
   in this plan.** `filter_case_mismatch` enhancement is plan 009 or
   later; the existing generic-unmatched warning is sufficient for now.
