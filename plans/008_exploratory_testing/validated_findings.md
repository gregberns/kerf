# Validated Findings — Post-Recovery

Validation against current main, tip `9fa7717`. Code reviewed: `cmd/next.go`
(rewritten item-feed), `cmd/init.go` (`detectBeadFilter`), `cmd/show.go`,
`cmd/root.go`, `internal/feed/{feed,cleanup,warning,filter,item}.go`,
`internal/beads/{beads,filter,heuristic}.go`, `internal/config/project.go`.

## Still Valid

- **findings/A:F2** — `cmd/show.go:275-278` still runs `bd list --json --project <id>`; `bd list` has no `--project` flag, so the bead summary silently fails. P0. Fix: drop `--project`, filter in Go using resolved `bead_filter`.
- **findings/A:F4** — `cmd/init.go:113` unconditionally calls `createDefaultProjectConfig` on every re-run; `config.SaveProjectConfig` overwrites with no merge of existing `bead_filter`. P1. Fix: when `project.yaml` exists, either skip or merge.
- **findings/A:F8** — Dangling deps after `bd delete` not surfaced. P2. Fix: optional detector in `internal/feed/warning.go`.
- **findings/A:F11** — `kerf next` excludes works with unknown statuses; invariant 5 says they should remain visible. P1. Fix locus: work filtering in `cmd/next.go` (load loop ~lines 134-144) or `cmd/list.go`. Status is treated as actionable only when in jig's `status_values`.
- **findings/A:F12** — Corrupt `spec.yaml` silently dropped (`spec.Read` error swallowed at `cmd/next.go:135-138`; analogous behavior in list/map/show). P0. Fix: emit warning item or `[corrupt]` row; matches B:F11.
- **findings/A:F13** — Orphan dir after deleted `spec.yaml` causes inconsistency between `kerf list` (absent) and `kerf new` ("already exists"). P1.
- **findings/A:F14** — `--area no-such-area` returns empty with no warning. P2. NOTE: `--area` is no longer a flag on `kerf next` (the rewrite dropped it). Still applies to any other command that accepts `--area`; verify on `cmd/list.go` or `cmd/map.go`.
- **findings/A:F15 / B:F16** — `kerf delete --force` rejected; correct flag is `--yes`. P2/S3. Fix: add `--force` as alias in `cmd/delete.go`.
- **findings/B:F5** — `internal/beads/beads.go:167` `ForWork` still uses `strings.EqualFold` (case-insensitive). S2. The `next` rewrite uses `ForWorkWithFilter` (case-sensitive per spec), so `kerf next` is now spec-conformant, but the legacy `ForWork` is still called from other commands (e.g. `cmd/show.go`, `cmd/map.go`); migrate callers and tighten `ForWork` or remove it.
- **findings/B:F12** — `--limit` negative/zero accepted silently. S3. NOTE: `--limit` no longer exists on `kerf next` (dropped in rewrite). Applies to any other command exposing `--limit`.
- **findings/B:F13** — `--area=''` empty string silently no-op. S3. NOTE: same caveat — verify against commands that still accept `--area`.
- **workflow/agent_confusion #2** — `kerf next` empty output is still terse (`nextEmptyText` is one sentence with no `kerf init` hint when `project.yaml` is missing). Moderate.
- **workflow/agent_confusion #3** — `cmd/root.go:119-132` "Available commands" still omits `init`, `setup`, `localize`, `next`, `map`, `areas`. Blocks fresh agents.
- **workflow/agent_confusion #4** — Bare `kerf` step 2 "Work through jig passes" still hand-wavy at `cmd/root.go:112`. Moderate.
- **workflow/agent_confusion #5** — `kerf init --help` does not mention jig-selection prompt or `bead_filter` auto-detect. Moderate.
- **workflow/agent_confusion #6** — `kerf setup` error unactionable when bench has the project but cwd lacks `.kerf/project-identifier`. Moderate.
- **workflow/agent_confusion #7** — `kerf status --help` one-line, no examples. Moderate.
- **workflow/agent_confusion #8** — Bare `kerf` shows only `Total active works` (global), not per-project. `cmd/root.go:101`. Mild.
- **workflow/agent_confusion #9** — Standard-workflow block omits `square` and `status` steps. `cmd/root.go:110-115`. Moderate.
- **workflow/agent_confusion #10** — `kerf new --help` doesn't warn about first-run `--jig` requirement. Moderate.
- **workflow/agent_confusion #11** — Cobra "Usage:" boilerplate on runtime errors. Mild. Fix: set `SilenceUsage: true` on `rootCmd`.
- **workflow/agent_confusion #12** — No documented way to re-print first-pass instructions. Moderate.
- **workflow/agent_confusion #13** — `kerf list` empty hints `kerf new` but never `kerf init` when project.yaml absent. Moderate.
- **workflow/agent_confusion #14** — Codename / bench terms used before defined. Mild.
- **workflow/agent_confusion #15** — No warning when `project.yaml` missing; "empty" indistinguishable from "uninitialized". Moderate. Fix: add detector in `internal/feed/warning.go`.
- **workflow/agent_confusion #16** — "pass"/"status"/"stage" terminology mixed across CLI and specs. Mild.
- **workflow/agent_confusion #17** — `kerf shelve` "active session" mechanism opaque. Mild.
- **workflow/agent_confusion #18** — Jig-list passes (`breakdown, dispatch, implement, verify, complete`) differ from `commands.md` examples (`breakdown, dispatch, implement, review`). Moderate. Reconcile examples in spec.
- **workflow/agent_confusion #19** — `kerf finalize --branch` provides no naming-convention guidance. Mild.
- **workflow/agent_confusion #20** — `kerf jig show plan` truncation note. Mild.
- **sync/ingestion High** — Bare `kerf` ignores cwd; reports global active count attributed to a stale "last touched" project. `cmd/root.go`.
- **sync/ingestion High** — `kerf init` does not scan `.beads/` to suggest a filter when no `bd init` was run in cwd. The auto-detect in `detectBeadFilter` only runs when `beads.IsAvailable()` returns true via `br`; if the store is a sibling `.beads/` not on PATH, nothing surfaces. Related to A:F3.
- **sync/ingestion Medium** — Drift is invisible: re-labels, external closes, deletes, reopens produce identical `kerf next` output across mutations. No diff state, no `kerf triage`. (See triage_workflow.md.)
- **sync/ingestion Medium** — `kerf show <codename>` does not render attached beads (IDs, statuses, counts). `cmd/show.go`. Subsumed-by/related-to A:F2.
- **sync/triage 1-5** — Triage loop pieces missing: `kerf triage`, `kerf new --bead-filter`, `kerf work edit --bead-filter-{add,remove}`, `kerf attach`, drift state, exit-code signal. New-plan scope.

## Needs Recheck (re-testing agent assignment)

- **findings/A:F1** — Pre-recovery this was "next ignores beads entirely". Post-recovery `cmd/next.go` imports `internal/feed` and wires all detectors. Recheck:
  ```
  /tmp/kerf next
  /tmp/kerf next --only bead
  /tmp/kerf next --kinds bead,cleanup,warning
  /tmp/kerf next --format=json
  ```
  Expect: bead rows, --only/--kinds/--format accepted, no "unknown flag" error. If accepted, F1 is fully invalidated.

- **findings/A:F3** — `detectBeadFilter` exists at `cmd/init.go:208`. Recheck: in a fresh tempdir with ≥3 beads sharing a prefix, delete `project.yaml`, run `kerf init`. Expect: stdout mentions detection (`cmd/init.go:232` "Detected: ..."), and `project.yaml` contains `bead_filter`. If silent, the path that reads beads (via `beads.List()` → `br list --format json --all --limit 0`) failed; verify by checking `beads.IsAvailable()` returns true and the JSON parses. Likely still environment-conditional.

- **findings/A:F5** — Out-of-band `bd close` not reflected. Beads source now feeds queue/cleanup/warning, but bead status mapping in `cmd/next.go:174-183` recognises `"closed", "complete", "completed", "done"`. Recheck: confirm `br` status taxonomy matches; if `bd close` writes a status not in this list, F5 stands.

- **findings/A:F6** — Unmatched-beads-after-work-delete warning. `internal/feed/warning.go:unmatchedBeadsDetector` exists with abs/frac thresholds (abs ≥ 10 OR frac ≥ 5%). Recheck: with 6 unmatched, abs threshold (10) is not met; with default 6 the detector intentionally does not fire. Decide whether this is "still valid: threshold too high" (design) or "ok by design". Recipe: create `kerf new foo`, 11 beads `work:foo`, then `kerf delete foo --yes`, `kerf next`. Expect: warning. If absent, F6 stands.

- **findings/A:F7** — Mixed-case label warning (`filter_case_mismatch`). The detector exists in `internal/feed/warning.go`. Recheck:
  ```
  kerf new foo
  br create -l "Work:Foo" "x"
  kerf next --kinds=warning
  ```
  Expect: warning about case mismatch (per spec). Confirm whether detector fires for label-on-bead vs. filter-vs-config case mismatch.

- **findings/A:F9** — 39 beads under `subsystem:` prefix → expected warning naming the unmatched prefix. The `unmatchedBeadsDetector` triggers on frac/abs thresholds (39 well over 10). Recheck same as F6 but with 39 unmatched.

- **findings/A:F10** — `br` vs `bd` dolt incompatibility (infra). Genuinely a `br`/`bd` mismatch (see Bd/Br section below). Worth reclassifying.

- **findings/B:F1** — "`kerf next` is not the item feed". Post-recovery the rewrite IS the item feed. Re-running B:F1 on current main is expected to PASS. Recheck:
  ```
  /tmp/kerf next
  ```
  Expect: rows with `bead`/`clean`/`warn` kind tokens, footer hint about `--format=json`, six-element help.

- **findings/B:F2** — Same as B:F1; flags now exist (`--only`, `--include`, `--kinds`, `--format`). Recheck by running each.

- **findings/B:F3** — Help text contract. `nextLongHelp` in `cmd/next.go:47-70` lists kinds, loop, flags, json, scoring. Recheck: `kerf next --help` and verify six bullets in order against `specs/commands.md` §"Help text".

- **findings/B:F4** — Project-wide `bead_filter` ignored. `cmd/next.go:168-169` now calls `beads.Resolve(w.BeadFilter, projectFilter)` and `ForWorkWithFilter`. Recheck: set project `bead_filter: label: "codename:{codename}"`, create work matching real codename label, run `kerf next`. Expect: bead counts contribute to score. Likely INVALIDATED but confirm.

- **findings/B:F6** — `work_no_attached_beads` cleanup detector. Now wired (`feed.NewCleanupDetectors`). Recheck: `kerf new lonely`, no matching beads, `kerf next --kinds=cleanup`. Expect: cleanup row.

- **findings/B:F7** — Unmatched-bead warning. Now wired. Same recheck as A:F9.

- **findings/B:F11** — Malformed `spec.yaml` silently dropped. `cmd/next.go:135-138` still continues on `spec.Read` error without surfacing. Tentatively STILL VALID, but add a detector in feed/warning rather than ad-hoc print. Recheck: confirm no project-level warning emitted.

- **sync/ingestion s5** — Drift mutations (relabel, close, delete, reopen). Beads source now refreshes per call so close/delete are reflected in counts. Recheck: do A→E mutations from the table and confirm whether `kerf next` output changes.

## Invalidated by Recovery

- **findings/A:F1** — `kerf next` is now the item feed; `internal/feed` is wired. (Confirm via recheck above.)
- **findings/B:F1** — Same as A:F1.
- **findings/B:F2** — `--only`, `--include`, `--kinds`, `--format` flags now registered in `cmd/next.go:82-85`.
- **findings/B:F3** — `nextLongHelp` now contains the six elements (kinds, loop, flags, json, scoring). Spot-check against spec.
- **findings/B:F4** — `cmd/next.go:168-169` now uses `Resolve(w.BeadFilter, projectFilter)` + `ForWorkWithFilter`. Project filter is honored.
- **findings/B:F6** — `feed.NewCleanupDetectors` invoked in `cmd/next.go:241-243` and includes `work_no_attached_beads`.
- **findings/B:F7** — `feed.NewWarningDetectors` invoked at `cmd/next.go:245-247` and includes `unmatchedBeadsDetector`.
- **findings/B:F8, F9, F10, F15, F17** — PASS results, not bugs.
- Phrasings throughout the reports of the form "feed package never imported", "--only doesn't exist", "no JSON format", "bead summary uses wrong flag in next" all refer to the pre-recovery state.

## Bd/Br Confusion (route to migration audit)

- **findings/A:F2** — `cmd/show.go` shells to `bd list --json --project <id>`. Kerf is supposed to use `br` (Rust/SQLite). This is the legacy `bd` path that survived. Route to migration audit; the show-bead-summary should be reimplemented over `internal/beads.List()` (which uses `br`) plus filter resolution.
- **findings/A:F10** — `br` incompatible with `bd` dolt store. Real `bd→br` migration question: is `bd` deprecated? If `br` is canonical, kerf should warn when it detects a `bd`-shape store (presence of `metadata.json` without `jsonl_export` field). Route to migration audit. (Could also produce a feed warning.)
- **tests/A:R10** — Test recipe assumes bead-tool failures surface as warnings. Tied to the `bd`/`br` migration choice — what's the canonical tool?
- Note: several findings in A use `bd create`/`bd close`/`bd delete`/`bd list` in their setup steps. The kerf code path now uses `br` exclusively (`internal/beads/beads.go:48` runs `br list ...`). If a user follows A's setup verbatim with `bd` against a dolt store, kerf's `br`-based reader cannot see those beads at all. Migration audit should decide whether `bd` is still supported.

## Weight-Tuning / Design Notes

- **sim_scenarios/scenarios.md (all 7)** — Empirical results across seeds. Design data, not bugs.
- **sim_scenarios/findings #Top finding** — Suspected `rework_p50_wait`/`rework_p95_wait`/`priority_inversions` zeros across policies. Needs sim-side investigation (generator/metric bug or scoring detector path). Probably a real bug in the simulator; flag as NEEDS RECHECK against `internal/sim/metrics` and the rework label path in the generator.
- **sim_scenarios/findings #Where kerf loses** — `area_collisions` deficit, momentum oscillation; design.
- **sim_scenarios/findings #Weight-tuning hypotheses 1-4** — Cut momentum, cut rework, build a fan-out scenario, add `area_diversity`. Tuning.
- **sim_scenarios/scoring_critique 1-4 + signal gaps** — Cap rework, effort-weighted fan-out, drop creation weight, momentum sizing, area-collision pressure, downstream effort, staleness penalty, per-work concurrency cap, external priority. All design/tuning.
- **sim_scenarios/adversarial 1-5** — Adversarial scenarios. Hypotheses pending empirical runs (now that sim is wired post-recovery, these are runnable). Could be promoted to scenarios.
- **workflow/improvements 1-10** — All ten are enhancement proposals (`kerf where`/`kerf doctor`, `--explain`, snapshot labels, `--write`, terminology audit, project.yaml-missing warning, auto-pass-from-FS, `verify-tools`). Design.
- **sim_scenarios/findings methodology** — 5/7 scenarios bottleneck on bead supply, not policy. Scenario-design feedback.
- **sim_scenarios/findings spec ambiguity** — `top_of_queue_churn` in linear scenarios. One-line spec clarification.

---

## Notes for the re-testing agent

1. Many `STILL VALID` items are workflow-text confusions that don't need re-running kerf; they need spec/help-text edits.
2. The biggest concentration of `NEEDS RECHECK` is the bead-pipeline findings from agent A (F3, F5–F9) — every one of these depended on the pre-recovery `feed`-unwired state. Re-run them against the current binary in one tempdir session.
3. `cmd/init.go` overwrite (A:F4), `cmd/show.go` `bd --project` (A:F2), corrupt-spec swallow (A:F12, B:F11), `--force` alias (A:F15/B:F16), and the bare-`kerf` orientation gaps (workflow #3, #8, #15) are STILL VALID without needing re-testing.
4. The `bd→br` migration story is the cross-cutting hot spot. Several A findings only make sense under `bd`-with-dolt; current kerf only supports `br`. Decide the policy before deciding the fixes.
