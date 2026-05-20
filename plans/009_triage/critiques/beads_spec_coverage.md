# Plan 009 — Beads vs. Spec Coverage

## Headline gap

`beads.md` line 9 asserts: *"Specs are already merged on `main` (commands.md,
coordination.md, works.md, architecture.md), so there are no spec beads."*
This is **false on main as of 2026-05-15** (commit `c6e178d`). None of the
following appear in any of the four spec files:

- `kerf triage` section in `commands.md` (no `--resolved`, `--ack`, `--format`,
  `--kind` flags; no exit-code table; no `not_initialized` kind).
- `kerf pin` section in `commands.md` (steps 1–7 referenced by B9 do not
  exist).
- `kerf work edit` section in `commands.md` (steps 1–7 referenced by B10,
  including step 7 "do not advance the drift baseline").
- `kerf show` "Attached beads" block at line 251 (B7 cites a line that points
  to the existing Commands block, not an attached-beads block).
- `kerf new --bead-filter` flag in step 6 (B11a cites it; step 6 in main is
  "Initialize `spec.yaml`").
- `kerf next` drift-summary headline at line 1520 (file is 1679 lines but the
  drift-summary text is not in it).
- `coordination.md` §"Pin layer", §"Drift detection", §"Sync cache",
  §"Snapshot shape", §"Hash scope", §"Baseline advancement", §"Composition
  with other detectors" — none exist.
- `works.md` `pinned_beads:` row in the schema table or schema YAML.
- `architecture.md` `.kerf/sync-cache.json` entry.

Every per-bead `Specs:` reference points at sections that do not yet exist.

## Per-bead status (all 12)

| Bead | Spec section cited | Exists on main? |
|---|---|---|
| B1 | `commands.md` §`kerf pin` step 5; `commands.md` §`kerf work edit` step 3; `works.md` `pinned_beads:` row | no / no / no |
| B2 | `coordination.md` §"Sync cache", §"Snapshot shape", §"Hash scope", §"Drift detection on every read" | no |
| B3 | `works.md` §"spec.yaml schema" `pinned_beads` row; `commands.md` §`kerf new` step 6 | no / no |
| B4 | `coordination.md` §"Composition with other detectors"; `commands.md` §`kerf next` warning header | no / no |
| B5 | `coordination.md` §"Pin layer", §"Drift detection" | no |
| B6 | `architecture.md` §"In the repo, inside git" sync-cache entry; `coordination.md` §"Sync cache", §"Baseline advancement" | no / no |
| B7 | `commands.md` §`kerf show` "Attached beads" block (line 251) | no |
| B8 | `commands.md` §`kerf triage` (full section, flags, exit codes 0/1/2/3) | no |
| B9 | `commands.md` §`kerf pin` (steps 1–7); `coordination.md` §"Pin layer" single-owner rule | no |
| B10 | `commands.md` §`kerf work edit` (full section, step 7) | no |
| B11a | `commands.md` §`kerf new` step 6 | no (extant step 6 unrelated) |
| B11b | `commands.md` §`kerf next` drift-summary line at line 1520 | no |
| B12 | `_plan.md` "Triage agent workflow (canonical)" | yes (plan, not spec) |

## Behavior beads claim that no spec authorizes

- **Pin conflict warning** (B5, judgment call #6): emits `pin_conflict` warning
  with lexicographically-earliest winner. No spec text covers this; it is
  defense-in-depth invented by the bead plan.
- **`last_resolved_counts` snapshot field** (B8, judgment call #4):
  acknowledged in the plan as "kerf-internal metadata, not part of the
  canonical snapshot shape." Without a spec entry it has no normative home.
- **Exit code 3 "made progress" semantics** (B8): the `_plan.md` table
  defines it; `commands.md` does not.
- **`--kind` flag on `kerf triage`** (B8): present in beads.md, absent from
  any spec.
- **`UntriagedBeads` rename of plan-006 `UnmatchedBeads`** (B4, judgment
  call #3): a renaming decision that touches plan-006 callers without a
  spec entry; today `coordination.md` line 252 still calls them "Unmatched
  beads."

## Bottom line

Beads claim 12 code beads against an unwritten spec surface. The plan's
Sequencing section (`_plan.md` step 1) calls for spec writes first, but
beads.md skipped that step on the assumption it had already happened.

## Counts

- **Spec gaps:** 12 of 12 beads cite at least one missing spec section.
- **Unspecced writing behavior:** B1 mutators, B6 cache writes, B8 `--ack`
  advance, B9 single-owner cross-file pin removal, B10 filter mutators,
  B11a `pinned_beads: []` default emit — all write to disk against spec
  text that is not yet on main.
- **Biggest:** the entire `kerf triage` and `kerf pin` command surfaces,
  the `coordination.md` Pin layer + Drift detection subsections, the
  `works.md` `pinned_beads:` schema row, and the `architecture.md`
  sync-cache entry are absent. Every other gap descends from these.
