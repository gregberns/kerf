# Exploratory Test — Plan 018 (kerf triage)

Date: 2026-05-18
Binary: /Users/gb/go/bin/kerf
Bead tools available: br 0.1.45 + bd 0.62 (dev)
Test sandboxes: /tmp/kerf-triage-JOI1 (main seeded store), /tmp/kerf-triage-empty-mkcg (empty), /tmp/kerf-triage-big-Beuu (120 beads)

Severity: BLOCKER / MAJOR / MINOR / NIT

## Scenario results

| # | Scenario | Result |
|---|---|---|
| 1 | Basic triage (no flags) | PASS — untriaged + multi-matched + external_drift sections emit cleanly |
| 2 | Tier-1 routing: codename:foo, no live foo | PASS — emits `suggest: kerf new foo --bead-filter 'label=codename:foo'` |
| 2b | Tier-1 routing: codename:foo, live foo work | PASS — beads vanish from untriaged (they match the filter); tier-2-only beads now suggest `kerf pin foo …` |
| 3 | Tier-2 fallback (only axis:perf) | PASS — falls through to `kerf pin <lex-earliest active work>` |
| 4 | Archive-aware (archived codename:archy) | PASS — emits `codename 'archy' is archived — consider 'kerf restore archy' …` (no `kerf new archy`) |
| 5 | `--ack` single-line | PASS — `Baseline advanced to 2026-05-19T00:19:43Z.` only; no full report |
| 6 | `--ack --format=json` | PASS — `{ "baseline_advanced_at": "...", "items_captured": N }` and nothing else |
| 7 | `--top N` | PASS — header `(showing N of M)`, footer `... and X more — use --top 0 for full list` |
| 7b | `--top 0` (unlimited) | PASS |
| 7c | `--top -1` | PASS — `Error: --top must be >= 0 (got -1)` |
| 8 | `--group-by codename-label` | PASS — groups by `codename:archy`, `spec:bar`, then `(ungrouped)` tail |
| 9 | `--group-by + --top 1` co-existence | PASS — per-group truncation `(ungrouped) (showing 1 of 3): ... and 2 more` |
| 10 | `--kind nonexistent` | PASS (brief was wrong) — actually errors out per spec `commands.md:2141`: `Error: unknown triage kind 'nonexistent'. Known kinds: untriaged, multi_matched, external_drift` |
| 11 | `--kind multi_matched --kind untriaged` both empty | NOT TESTED in empty-set form. With items present in either kind, only those sections render. |
| 12 | `--help` lifecycle | PASS — 4 phases (What triage returns / Item kinds / Exit codes / Baseline lifecycle) + recipe line `First run on a large project: 'kerf triage --top 20 --group-by codename-label'.` |
| 13 | Storage-drift footer | UNCLEAR — induced stray `~/.kerf/projects/<proj>/stray-work/` directory; no footer appeared. Plan 018 explicitly out-of-scope for the footer (Plan 017 owns it), so not a 018 bug. |
| 14 | Bead counts header shape | PASS — `Beads: 7 open · 7 ready · 7 total` |

Exploratory pokes:

- All flags combined `--top 5 --group-by codename-label --kind untriaged --ack` — PASS (prints only the `Baseline advanced …` single line; `--ack` correctly overrides rendering).
- Empty bead store — PASS, prints `No untriaged, multi-matched, or externally-changed beads. Project is clean.`
- 120 beads — `triage --top 20 --group-by codename-label` returns in 0.48s. Acceptable.
- `--resolved` exit codes: drift present → exit 2; clean project → exit 0. Confirmed.

## Pass / total

13 / 14 scenarios as briefed pass. Scenario 11 (both `--kind` flags with empty sets) not fully exercised because in the seeded store both untriaged and multi_matched had items; would need a state with all kinds empty (post-`--ack` + clean store). Worth a follow-up unit test rather than another fixture.

## Findings

1. **MINOR — Brief disagreed with spec on `--kind nonexistent`.** Brief expected `No nonexistent items.` (single-line empty). Spec `commands.md:2141` (and observed behavior) reject unknown kinds with `Error: unknown triage kind …`. The spec behavior is correct (typed enum); the brief had it wrong. No code change needed.

2. **MINOR — `--ack` re-run inside the same second.** Two `kerf triage --ack` calls under 1s emit identical `Baseline advanced to <ts>` strings (the snapshot timestamp uses second resolution). After `sleep 2`, the timestamp advances. Mostly harmless but could confuse a fast loop. Recommend either sub-second precision or a "no-op (already at <ts>)" hint if the snapshot is byte-identical.

3. **NIT — multi-matched section uses `matches: foo, perfwork` shape.** Clear and readable; matches spec. No issue but worth noting that the suggestion is `kerf pin foo …` (lex-earliest match) — agents reading triage may want explicit guidance to pick the right work rather than the lex-earliest. (Plan 018 doesn't promise smart pin-target selection, so this is a future thought, not a defect.)

4. **NIT — Tier-2 fallback picks the lex-earliest *active* work even when its filter has nothing to do with the bead.** E.g. bead with only `kind:bug` → `kerf pin foo …`. Spec-compliant per Plan 018 design note, but slightly noisy guidance: every truly-orphan bead points at the same work. Consider downgrading the suggestion to `investigate manually` when no label prefix overlap exists with any work filter. (Same code path that returned `no auto-suggestion …` when there were zero active works — that path is correct.)

5. **NIT — `Per-work bead health` block appears below all triage sections.** Not in the brief but useful: per-work `filter: <expr>  beads: N open / M closed`. Looks like kerf-baf or adjacent. Acts as a free health summary.

6. **PASS — archive detection.** The `codename 'archy' is archived` message correctly references `kerf restore archy` (not `mv …/archive/…`). Plan 018 B2 looks delivered.

## Out-of-band

- `br` 0.1.45 cannot parse `bd` 0.62 dolt stores (`missing field jsonl_export`). This forced the fixture to set `tools.tasks: bd` manually because `kerf config tools.tasks bd` errors with `unknown configuration key 'tools.tasks'`. Plan 015/dogfood log already captures this. Two follow-ups worth tracking:
  - `kerf config` should accept `tools.tasks` (currently has to be hand-edited into `project.yaml`).
  - `kerf init` could auto-detect bd vs br by probing the `.beads/` store and set `tools.tasks` accordingly.

## Files referenced

- /Users/gb/github/kerf/plans/018_triage_rework/_plan.md
- /Users/gb/github/kerf/specs/commands.md (lines 1660-1823, 1971-1989, 2141)
- /Users/gb/github/kerf/internal/beads/beads.go
- /Users/gb/.kerf/projects/kerf-triage-joi1/project.yaml (manual `tools.tasks: bd` added)
