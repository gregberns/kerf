# Final Plan — Plan 008 Exploration Output

> Merged from action list + 3 critiques (spec-conformance, agent-UX, scoping/risk).
> bd→br configurability shipped via commit d965b9e. Specs still drift; see "Spec sweep" below.
> See `ACTION_LIST.md` for the full enumerated lists — this document is the *operating plan*.

## Now — quick wins, ship within a week

Pure code-drift, agent-visible, scoped. Listed in commit order (dependencies first).

1. **P0#3 — replace `cmd/show.go:278` shell-out to `bd list` with `internal/beads.List()`.**
   `bd list` has no `--project` flag and the binary is `br`; bead summary silently fails. Spec-covered (`architecture.md:237`). **Sequence first** — P1#5 (legacy `ForWork` migration) depends on this rewrite. Add `cmd/show_test.go` regression. Estimate: 0.5 day.

2. **P0#4 — same fix for `cmd/square.go:214`.** Silent zero counts. Add `cmd/square_test.go`. Estimate: 0.25 day.

3. **P0#1 / B:F4 — fix `work_codename: null` + spurious `work_no_attached_beads` cleanup.**
   *Scoping critique correction: this is **structural**, not "medium afternoon".* `feed.BeadSource` (feed.go:72) keys on `b.Epic`; resolved filter from `cmd/next.go:168` is discarded. Cleanest fix: build `BeadToWork map[string]string` in `cmd/next.go` (~20 LOC), thread through `feed.Input` (~10 LOC), add `TestBeadSource_WithLabelFilter`. Risky touch — load-bearing JSON contract. Estimate: 1 day. **Highest agent-UX value of the P0 bucket** (wrong-action loop today).

4. **P0#2 — emit warning when `spec.yaml` is corrupt** (cmd/next.go:135-138 + list/map/show). Silent disappearance is the single worst agent failure mode. *Note:* spec-conformance critique says the warning kind needs a spec line in `commands.md` first — see "Next". Code change is small (one warning detector in `internal/feed/warning.go`) once spec lands. Estimate: 0.25 day after spec.

5. **P1#6 — unknown statuses should remain visible** (`cmd/next.go:134-144`). Invariant 5; pure drift. Add unit test. Estimate: 0.25 day.

6. **P1#5 — migrate `cmd/show.go` / `cmd/map.go` off legacy `ForWork`, then tighten/delete it.** Spec is case-sensitive (`coordination.md:232`). **Must follow #1 above** (P0#3 owns the show.go rewrite). *Agent-UX note: invisible to working agents — lower priority than action list implied.* Estimate: 0.5 day.

7. **P1#1 + P1#2 combined — `cmd/init.go` idempotency + `detectBeadFilter` repair.** Both touch the same file. *Gated on spec rule for re-run idempotency — see "Next".* Estimate: 0.5 day after spec.

8. **P1#4 — `kerf next` unmatched-bead counter staleness.** Recompute count after the open-bead filter step. Snapshot test. Estimate: 0.25 day.

9. **Workflow #3 — snapshot-test `kerf next --help` and bare `kerf` against `specs/commands.md`.** Locks the agent contract. **Run terminology audit (Workflow #4) first** so the snapshot doesn't lock in drift. Estimate: 0.5 day (audit) + 0.25 day (snapshot).

10. **P2 — root help "Available commands" includes `init/setup/localize/next/map/areas`** (`cmd/root.go:119-132`). `commands.md:42` already requires this list. Highest-leverage-per-byte agent visibility fix. Estimate: 0.1 day.

**Now bucket total: ~4 engineer-days.** Lower-priority P2 polish (force/yes alias, negative `--limit`, etc.) deferred — agents don't re-read help once oriented.

## Next — needs spec change first

CLAUDE.md spec-first rule applies. Code follows after spec lands.

- **`commands.md` §`kerf init`** — add "if `project.yaml` exists, merge / skip / abort" subsection. **Biggest spec gap.** Blocks P1#1 and P1#2 above.
- **`commands.md` §`kerf next` warning table** — add `corrupt_spec` and `no_project_yaml` warning kinds; matching feed-warning rule in `coordination.md`. Blocks P0#2 and P1#9.
- **`commands.md` §`kerf new` + §`kerf list`** — define orphan-dir handling. Blocks P1#7.
- **`commands.md`** — bare `kerf` scopes active-work count to resolved project (explicit line). Blocks P1#8.
- **`simulator.md:290`** — clarify `top_of_queue_churn` when there is a single candidate. In-place tightening.
- **`coordination.md` drift section** — specify relabel-detection hashing scope (P1#3). Underspecified today.

## Investigation gate — diagnose-then-act

**Sim integrity blocker: 1–2 days, not "medium investigation".** `priority_inversions = 0` and `rework_p50_wait = 0` across 7/7 scenarios. Static read of `internal/sim/metrics/hooks.go:65-93` + `metrics.go:331` looks correct — bug is in runtime conditions. Diagnose, do not patch blindly.

Sub-tasks:
1. Add a `--debug DispatchInfo` flag to `cmd sim run` that dumps dispatch records as JSONL.
2. Run one scenario; verify dispatches reach `inWindow(d.Tick)` (warmup window may swallow everything if `Deadline1d/3d/7d` configured tight).
3. Check generator (`generator.go:215`): rework beads' `DependsOn` may keep `depsAllClosed` false so `olderReworkEligible` never fires.
4. Add `TestRun_BaselineRandom_ProducesInversions` with a constructed scenario where random *must* invert (rework arrives tick 1, new-work tick 2, both eligible). Reproduces the bug or proves it's scenario-specific.

Until this resolves, all Design / Scoring items are gated. Treat them as Phase 2 of a future scoring-tuning plan.

## Design / Scoring — frozen until investigation gate clears

All 10 weight-tuning hypotheses (rework cap, momentum cut, effort-weighted fan-out, creation-order tiebreaker, area-diversity penalty, staleness, concurrency cap, external pin, scenario saturation, adversarial promotion) are **blocked on sim integrity**. Any baseline captured with broken metrics is contaminated.

**Spec conflict to surface before tuning:** "Cut momentum 5.0 → 2.0 or 0.0" is **not** a numeric tweak. `coordination.md:149` enshrines momentum as a named principle ("prevents orphaned work"). Lowering it materially conflicts with that principle; the spec **rationale** must be revised first, not just a default. Same applies to demoting `creation` from independent field to tiebreaker (`coordination.md:180`).

All other design items also require `coordination.md` §Computed priority (L167-180) + `simulator.md` weights table (L178-181, L222-228) extensions before code is reviewable.

## Triage workflow — promote to its own plan

Recommend `plans/009_triage/`. Honest sizing: ~1.5 sprint-weeks, **not** "afternoon of small beads". Items 1–5 cumulatively touch `cmd/show.go`, `cmd/map.go`, `cmd/new.go`, `cmd/work.go`, new `cmd/attach.go`, `internal/spec/*` mutators, plus 5 new sections in `commands.md`. Item 6 (`kerf triage`) + item 7 (drift state file) add a new command surface, a new file format, a new spec section, and a CI-loop exit-code contract — sub-plan-sized alone.

Agent-UX verdict: of the 7 commands, **only `kerf show <codename>` with bead rendering closes a daily agent loop on its own.** `kerf triage` is only useful with the `--resolved` exit code (otherwise it's a dashboard nobody reads). Items 3–5 are config plumbing agents touch rarely. Sequence `kerf show` first inside plan 009 — it absorbs the bead-count column reuse and dovetails with P0#3 already in Now.

## Concurrency gap — promote to its own plan

Recommend `plans/010_concurrency/`. **Biggest agent-UX gap not in the existing action list.** `kerf next` does not signal that the top-ranked work has an active session held by a different agent. Two parallel agents will both pick item #1 and step on each other.

Scope candidates:
- `active_session != null AND not mine` surfaces as cleanup/warning row.
- Bead items annotated with "owned by session X".
- Stale `active_session` recovery nudge from `kerf next` (today `kerf resume` returns "active session exists" with no path forward).
- `kerf next` exit-code contract (0 items / 1 empty / 2 warnings-only) for orchestrator loops.
- JSON shape stability contract for `kerf next --format=json`.

Spec touchpoints: `commands.md` §`kerf next`, `coordination.md` session-ownership semantics. Needs its own breakdown.

## Spec sweep — land before any further jig-implementation review

Residual `bd` references in specs (bd→br configurability shipped in code but specs drift):

- `specs/jig-implementation.md` lines 30, 45, 115, 116, 118, 130, 154, 161, 166, 188, 230, 247, 301, 326
- `specs/architecture.md:237` (`tools.tasks: bd` default → `br`)
- `specs/verification.md:50` (narrative `bd list`)
- `specs/commands.md:640, 662, 1320` (example outputs / tool examples)
- `specs/jig-system.md:62` (external-tools example)
- `specs/coordination.md:190, 257` (`beads (bd)` references)
- Mirror fixes into `internal/jig/builtin/implementation.md`.

`plans/006_bead_attachment/_plan.md:183` and `plans/007_simulator/_plan.md:156` are historical footnotes — leave.

**Defer:** `go:embed` deduplication of `jig-implementation.md` ↔ `internal/jig/builtin/implementation.md`. Scoping critique flags it as architecture-change risk (import cycles, source-of-truth shift). If pursued, document the decision in `jig-system.md` first.

## Already shipped / invalidated — for the record

Reviewers can skip:

- A:F1 / B:F1 — `kerf next` is the item feed; `internal/feed` wired.
- B:F2 — `--only`, `--include`, `--kinds`, `--format` flags exist.
- B:F3 — `nextLongHelp` covers all six required elements.
- B:F6 — `work_no_attached_beads` cleanup fires (but spuriously today — see P0#1).
- A:F6 / A:F9 / B:F7 — `unmatchedBeadsDetector` fires above thresholds.
- A:F5 — out-of-band `br close` reflected (counter staleness logged as P1#4).
- Drift mutations: 4 of 5 surfaced (relabel remains in P1#3).
- B:F8/F9/F10/F15/F17 — PASS results.
- A:F10 — superseded by bd→br configurability (commit d965b9e).
