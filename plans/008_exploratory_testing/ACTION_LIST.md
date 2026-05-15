# Action List — Plan 008 Exploration Synthesis

> Generated from 6 exploration agents' findings, validated + rechecked against main post-recovery. bd→br configurability already shipped (commit d965b9e).

## P0 — Real bugs in shipped code

1. **Work-level `bead_filter` resolved but attachment broken.** `kerf next` reads project/work filters and reaches the resolution path, but matching beads still come back with `work_codename: null` and the `work_no_attached_beads` cleanup fires even when the store contains matching labels. Locus: `cmd/next.go:168-169` (`beads.Resolve` / `ForWorkWithFilter`) + attachment loop. Fix idea: trace the resolved filter through `ForWorkWithFilter` and confirm the join key on the returned bead matches the work's codename. Scope: medium. (B:F4)

2. **Corrupt `spec.yaml` silently drops the work.** `cmd/next.go:135-138` (and analogous list/map/show) swallow `spec.Read` errors, making the affected work invisible. Fix idea: emit a `warning` feed item or `[corrupt]` row via `internal/feed/warning.go`. Scope: small. (A:F12 / B:F11)

3. **`cmd/show.go:278` shells `bd list --json --project <id>`.** `bd list` has no `--project` flag and the binary is `br`. Bead summary on `kerf show` silently fails. Fix idea: delegate to `internal/beads.List()` with the resolved filter; drop the manual exec. Scope: small. (A:F2, also called out in bd→br audit)

4. **`cmd/square.go:214` shells `bd list`.** Same root cause; `bd` not on user PATH so square's bead counts silently zero. Fix idea: `exec.Command("br", "list", ...)` or route through `internal/beads`. Scope: small. (bd→br audit)

## P1 — Important but not blocking

1. **`cmd/init.go:113` clobbers existing `project.yaml` on re-run.** `createDefaultProjectConfig` is unconditional; any user-set `bead_filter` is lost. Fix idea: skip or merge when the file exists. Scope: small. (A:F4)

2. **`detectBeadFilter` never fires in practice.** Recheck with 12 prefix-sharing beads produced no detection line and no `bead_filter` key. Locus: `cmd/init.go:208`. Fix idea: verify `beads.IsAvailable()` gating and the JSON parse; loosen the trigger threshold. Scope: small-medium. (A:F3)

3. **Relabel drift is invisible.** Out of 5 mutations, only `br label` changes don't alter `kerf next` output. Fix idea: hash labels-per-bead into the warning detector or surface a label-delta row. Scope: medium. (sync/s5 residual)

4. **`kerf next` unmatched-bead counter staleness.** A:F5 recheck showed header says "13" while listing 12 after `br close`. Fix idea: recompute count after the open-bead filter step. Scope: small. (A:F5 sub-bug)

5. **`internal/beads/beads.go:167` `ForWork` uses `EqualFold`.** Spec is case-sensitive. `kerf next` migrated to `ForWorkWithFilter`, but `cmd/show.go`/`cmd/map.go` still call the legacy path. Fix idea: migrate callers, then tighten or delete `ForWork`. Scope: small. (B:F5)

6. **`kerf next` excludes works with unknown statuses.** Invariant 5 says they should remain visible. Locus: `cmd/next.go:134-144`. Fix idea: treat unrecognized statuses as actionable with a warning. Scope: small. (A:F11)

7. **Orphan-dir inconsistency between `kerf list` (absent) and `kerf new` ("already exists").** Fix idea: detect and warn or auto-clean on `new`. Scope: small. (A:F13)

8. **Bare `kerf` reports global active-work count attributed to a stale "last touched" project.** Cwd is ignored. Locus: `cmd/root.go:101`. Fix idea: scope to current project when `.kerf/project-identifier` resolves. Scope: small. (sync ingestion)

9. **No `project.yaml` warning.** "Empty" and "uninitialized" are indistinguishable. Fix idea: warning detector in `internal/feed/warning.go`. Scope: small. (workflow #15, improvements #8)

## P2 — Polish & nits

- `cmd/root.go:119-132` "Available commands" omits `init`, `setup`, `localize`, `next`, `map`, `areas`. (workflow #3)
- `kerf init --help` doesn't mention jig prompt or `bead_filter` auto-detect. (workflow #5)
- `kerf setup` error unactionable when bench has the project but cwd lacks `.kerf/project-identifier`. (workflow #6)
- `kerf status --help` one-liner, no examples. (workflow #7)
- Standard-workflow block on bare `kerf` omits `square` and `status`. (workflow #9)
- `kerf new --help` doesn't warn about first-run `--jig` requirement. (workflow #10)
- Cobra "Usage:" boilerplate prints on runtime errors; set `SilenceUsage: true`. (workflow #11)
- No documented way to re-print first-pass instructions. (workflow #12)
- `kerf list` empty hint should suggest `kerf init` when `project.yaml` absent. (workflow #13)
- `kerf finalize --branch` no naming-convention guidance. (workflow #19)
- `kerf jig show plan` truncation note. (workflow #20)
- `kerf delete --force` rejected; add as alias of `--yes`. (A:F15 / B:F16)
- Dangling deps after `bd delete` not surfaced. (A:F8)
- `--area no-such-area` returns empty with no warning (any cmd still accepting `--area`). (A:F14)
- `--limit` negative/zero accepted silently on cmds that still expose it. (B:F12)
- `--area=''` empty-string silent no-op. (B:F13)
- Mixed-case label warning (`filter_case_mismatch`) detector not implemented; only generic unmatched fires. (A:F7)
- Codename/bench terms used before defined; "pass"/"status"/"stage" terminology mixed. (workflow #14, #16)
- `kerf shelve` "active session" mechanism opaque. (workflow #17)
- Jig pass list (`breakdown, dispatch, implement, verify, complete`) differs from `commands.md` example (`breakdown, dispatch, implement, review`). (workflow #18)
- `kerf next` empty output should hint `kerf init` when `project.yaml` missing. (workflow #2)
- Bare `kerf` step 2 "Work through jig passes" is hand-wavy. (workflow #4)

## Design / Scoring (separate bucket)

Hypotheses, not bugs. Empirical confirmation pending the sim-metric fix below.

- **Cap rework contribution.** Today `score += reworkCount * 15` is unbounded; recommend `min(reworkCount, 3) * 15` (saturating). Highest-value scoring change. (scoring_critique #1)
- **Cut momentum weight from 5.0 toward 2.0 or 0.0.** Direct cause of area-collision deficit (kerf vs random −30–90%). (sim findings, critique #4)
- **Effort-weighted fan-out.** Replace `fanOut` (count of dependents) with sum of dependent works' bead totals; drop multiplier from 10 to ~2. (scoring_critique #2)
- **Drop creation-order weight, use as stable tiebreaker only.** Currently +0.1·rank ≈ noise. (scoring_critique #3)
- **Add `area_diversity` / area-collision penalty.** `score -= activeAgentsInArea * areaPenalty`. (sim findings #4, critique signal-gap)
- **Staleness penalty for rework.** Decay rework score by half-life T_rework. (critique signal-gap)
- **Per-work concurrency cap** (policy-layer, not score). (critique signal-gap)
- **External priority pin** on works. (critique signal-gap)
- **`top_of_queue_churn` spec clarification.** Does "head unchanged because only candidate" count as no-change? (sim findings, spec ambiguity)
- **Saturate scenarios with bead supply.** 5/7 current scenarios bottleneck on bead supply, not policy — agent-idle ≥ 0.79. Future scenarios need higher bead-rate. (sim findings methodology)
- **Promote 5 adversarial scenarios from `adversarial.md` to runnable YAMLs** now that sim is wired.

### Sim integrity (treat as bug-adjacent — blocks design work)

- **`priority_inversions` = 0 in 7/7 scenarios, `rework_p50_wait` = 0 in 7/7, `rework_p95_wait` = 0 in 5/7.** Suspect the generator's rework label isn't reaching the queue scorer, the in-memory `BeadSource` shape diverges from `br list --format json`, or metrics drop `IsRework`. Locus: `internal/sim/metrics` + rework-label path in generator. **Validate before tuning the rework weight.** Scope: medium investigation.

## Workflow / Agent UX improvements

Ranked by impact / time-to-fix. Items overlapping P0/P1 are omitted.

1. **`kerf where` / `kerf doctor`** — one-shot "project: X, active work: Y (status), next action" report. Replaces 3 invocations after context clear. Small. (improvements #2)
2. **`kerf verify-tools`** — probe declared tool list at init, report availability. Small. (improvements #10)
3. **Snapshot-test `kerf next --help` and bare `kerf` against `specs/commands.md`.** Prevents help-text drift. Small. (improvements #1)
4. **Reconcile "pass"/"status"/"stage" terminology across specs and tooling.** One audit, glossary entry. Small. (improvements #7, workflow #16)
5. **`kerf init` prints a delimited copy-pasteable agent-instruction block.** Small. (improvements #3)
6. **Auto-name `kerf status` snapshots** as `before-<pass>` / `after-<pass>`. Small. (improvements #5)
7. **`kerf shelve --session-file path`** — atomic shelve + SESSION.md write. Medium. (improvements #6)
8. **`kerf next --explain <rank>`** — score breakdown per factor. Medium; requires scorer trace plumbing. (improvements #4)
9. **`kerf status --auto` infers pass from artifact files.** New plan; needs ordering rules. (improvements #9)

## Triage Workflow (kerf triage / new commands)

New-plan scope. Order by precondition.

1. **`kerf show <codename>` renders attached beads** (IDs, status, counts). Small; subsumes part of A:F2 fix. (sync ingestion Medium)
2. **`kerf map` adds bead counts per row.** Small; reuses bead resolver. (triage_workflow)
3. **`kerf new --bead-filter '<spec>'`** at creation time. Small.
4. **`kerf work edit --bead-filter-add / --bead-filter-remove`.** Small-medium; mutates `spec.yaml` safely.
5. **`kerf attach <codename> <bead-id>`** — explicit pin alongside filter. Small.
6. **`kerf triage` report** (untriaged / multi-matched / external-changes / per-work health) + `--resolved` exit-code signal for CI loops. Medium; needs persisted last-seen state for diffs.
7. **Drift state file** (last-seen bead manifest) to power triage diffs. Small.

## Follow-up sweeps (small)

bd→br configurability shipped, but these were flagged by the audit and remain:

- `specs/jig-implementation.md` lines 30, 45, 115, 116, 118, 130, 154, 161, 166, 188, 230, 247, 301, 326 — still reference `bd`, `--desc`, `--status in-progress`, `bd dep <child> <parent>`. Mirror the fixes in `internal/jig/builtin/implementation.md`.
- `specs/architecture.md:237` — `tools.tasks: bd` default → `br`.
- `specs/verification.md:50` — narrative `bd list` → `br list`.
- `specs/commands.md:640, 662, 1320` — example outputs and tool examples still mention `bd`.
- `specs/jig-system.md:62` — external-tools example.
- `specs/coordination.md:190, 257` — `beads (bd)` references.
- `plans/006_bead_attachment/_plan.md:183`, `plans/007_simulator/_plan.md:156` — historical, footnote-only.
- Consider deduplicating `specs/jig-implementation.md` and `internal/jig/builtin/implementation.md` via `go:embed` to prevent future drift.

## Already Fixed (for the record)

Recheck against current main confirmed these — don't re-raise:

- A:F1 / B:F1 — `kerf next` is the item feed; `internal/feed` wired.
- B:F2 — `--only`, `--include`, `--kinds`, `--format` flags exist on `kerf next`.
- B:F3 — `nextLongHelp` has six required elements in spec order.
- B:F6 — `work_no_attached_beads` cleanup detector fires.
- A:F6 / A:F9 / B:F7 — `unmatchedBeadsDetector` fires above thresholds.
- A:F5 — Out-of-band `br close` reflected in listing (counter staleness logged in P1).
- Drift mutations: 4 of 5 (`create`, `close`, `delete`, `reopen`) now alter `kerf next` output (relabel remains in P1).
- B:F8, F9, F10, F15, F17 — PASS results, never bugs.
- A:F10 (`bd` dolt incompatibility) — superseded by bd→br configurability; route any residual to migration plan.
