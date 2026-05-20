# Plan 017 dogfood — `kerf doctor` + `kerf localize`

Date: 2026-05-18
Binary: `/Users/gb/go/bin/kerf`
Test project: `/tmp/kerf-test-doctor` (fresh git repo, project id `kerf-test-doctor`)
Real repo touched read-only: `/Users/gb/github/kerf` (project `gregberns-kerf`)

---

## F1 — `kerf doctor` crashes before `br init` (RED)

- **Timestamp:** 2026-05-18T17:11
- **Command:** `kerf init && kerf doctor` in a fresh repo (no `br init`).
- **Expected:** Doctor returns findings — at worst a YELLOW/RED naming the missing bead store.
- **Observed:** Hard crash with `BEADS_TOOL_ERROR ... NOT_INITIALIZED`, exits 1, no findings emitted. Other detectors short-circuited (no green output).
- **Verdict:** **BUG.** `kerf init` does not run `br init`, but the very first thing the init brief tells the agent to do (`kerf doctor`) crashes with a `br`-flavored error instead of a doctor finding. This is the failure mode plan 017 was supposed to remove. The bead-filter-coverage detector should degrade to a single YELLOW/RED finding ("bead store not initialized — run `br init`"), not bubble the raw tool error.
- **Reproducer:**
  ```
  mkdir /tmp/kt && cd /tmp/kt && git init -q && kerf init && kerf doctor
  ```

## F2 — `kerf doctor` also crashes on the real kerf repo (RED)

- **Timestamp:** 2026-05-18T17:24
- **Command:** `kerf doctor` in `/Users/gb/github/kerf`.
- **Expected:** Findings or a controlled error.
- **Observed:** `JSON_ERROR: missing field 'jsonl_export' at line 7` from the `br` tool — doctor exits 1. Other detectors run individually and all report green.
- **Verdict:** **BUG.** Same root cause as F1 — bead-filter-coverage doesn't trap `br` errors and lets the whole command die. Means `kerf doctor` is unusable on this repo today.
- **Reproducer:** `cd /Users/gb/github/kerf && kerf doctor`

## F3 — All five detectors run independently and produce expected severities

- **Timestamp:** 2026-05-18T17:13–17:21
- **Command:** `kerf doctor --detector <name>` for each of `project-yaml`, `storage-drift`, `symlink-integrity`, `archive-orphans`, `bead-filter-coverage`.
- **Expected:** Each detector emits its own finding independently.
- **Observed:** All five run cleanly in isolation. Induced failures:
  - **project-yaml:** Corrupted YAML → RED with file path and parser error. Hint: "fix the YAML syntax in project.yaml". Good.
  - **storage-drift (bench):** Created `.kerf/works/rogue` → YELLOW. **Hint is wrong direction:** says "run `kerf localize`" which migrates to local, but the canonical location for bench mode is the bench — the rogue dir should be moved off the repo. (The local-mode flavor of this hint, observed later, is correctly worded.)
  - **symlink-integrity:** Removed symlink in local mode → RED "bench symlink: missing", hint "kerf localize  (recreate the bench symlink)". Good.
  - **archive-orphans:** Created archive entry with codename `alpha` matching live work → YELLOW "1 codename collision with live works". Good.
  - **bead-filter-coverage:** Work with no filter → RED "1 of 1 works unwired" (matches plan 019 / kerf-7lq). Good.
- **Verdict:** PASS, with one wording bug noted (storage-drift hint in bench mode).

## F4 — Bogus detector errors cleanly

- **Command:** `kerf doctor --detector bogus`
- **Observed:** `Error: unknown detector 'bogus'. Known detectors: archive-orphans, bead-filter-coverage, project-yaml, storage-drift, symlink-integrity`, exit 1.
- **Verdict:** PASS.

## F5 — JSON shape

- **Command:** `kerf doctor --format json` (clean state, post `br init`).
- **Observed:** `{project_id, storage_mode, findings: [{detector, severity, summary, items, hint}, ...]}`. Matches plan 017 spec sketch.
- **Verdict:** PASS.

## F6 — `kerf doctor --strict` exit semantics

- **Command:** `kerf doctor --strict` while alpha was unwired (RED).
- **Observed:** Exit 1; without --strict, exit 0. Confirmed both with and without RED present.
- **Verdict:** PASS.

## F7 — `kerf new` fenced trailer

- **Command:** `kerf new alpha --jig spike`
- **Observed:** Output ends with:
  ```
  working directory: /Users/gb/.kerf/projects/kerf-test-doctor/alpha
  repo-side files:   .kerf/project-identifier (committed); add agent instructions to your config file (CLAUDE.md, AGENTS.md, etc.)
  ```
- **Verdict:** PASS — fenced trailer present and correctly identifies the working dir + repo-side files (per kerf-57u).

## F8 — `kerf localize --check` / `--dry-run` previews without mutating

- **Command:** `kerf localize --check`
- **Observed:** Printed the four planned actions (move works, move project.yaml, replace bench dir with symlink, write storage:local to config.yaml). Filesystem unchanged afterwards.
- **Verdict:** PASS. `--dry-run` is documented as an alias (confirmed in `--help`).

## F9 — `kerf localize` real run

- **Observed:** Moved 1 work (`alpha`), wrote `.kerf/config.yaml` storage:local, created bench symlink. Output includes git-add hints. `kerf doctor` afterwards reports `local mode` green for storage-drift and symlink-integrity. Good.
- **Verdict:** PASS.

## F10 — `kerf localize --check` is idempotent after localize

- **Command:** `kerf localize --check` (run twice after the project is already local).
- **Observed:** Both runs: `Already using local storage for project 'kerf-test-doctor'.`, exit 0.
- **Verdict:** PASS.

## F11 — Drift footer on `kerf next` and `kerf triage`

- **Setup:** Local mode, induced 2 storage findings (extra-work dir in bench location + real dir where symlink expected).
- **Observed (text):** Both commands appended `note: 2 storage findings — run 'kerf doctor' for details`. Good.
- **Negative case:** With only an `unwired` RED (no storage finding), no footer appears on `next` or `triage`. **Confirmed:** the footer is gated on *storage*-class findings only, not all RED findings. This matches the plan-017 wording ("drift footer"), but worth noting since the doctor command itself groups them under one report — an agent may expect "a RED exists ⇒ footer fires".
- **Verdict:** PASS for the storage-drift surface; potential UX gap that bead-filter REDs do not surface on routing commands.

## F12 — Suppression matrix (3×2)

Drift induced as in F11. Matrix:

| Case | next text | triage text | next json | triage json |
|------|-----------|-------------|-----------|-------------|
| default (footer on) | shown | shown | (no surface) | n/a |
| `KERF_DOCTOR_FOOTER=0` | suppressed | suppressed | (no surface) | n/a |
| `doctor.footer: false` in project.yaml | suppressed | suppressed | (no surface) | n/a |
| config:false + env=1 (env wins) | shown | n/t | — | — |
| config:true + env=0 (env wins) | suppressed | n/t | — | — |

- **Verdict:** PASS for text; **gap:** the JSON output of `kerf next` (only mode tested) contains no `footer` / `storage_findings` field, so a JSON consumer cannot see the drift signal at all. This is an observability hole — JSON callers (the obvious automation target) are blind to drift unless they separately call `kerf doctor --format json`.

## F13 — Doctor on the real kerf repo

- **Command:** `kerf doctor` in `/Users/gb/github/kerf` (and `--detector` per-detector to work around F2).
- **Observed:** All four non-`br` detectors green: bench mode, project.yaml present, no storage drift, no archive entries, symlink-integrity n/a. The full `kerf doctor` invocation crashes (F2).
- **Verdict:** Real repo is clean per layout; doctor itself is broken on it.

## F14 — Double-presence of `project.yaml` (bench + repo, local mode)

- **Setup:** Local mode with bench symlink intact pointing into `/tmp/kerf-test-doctor/.kerf/works`. Wrote a second `project.yaml` *outside* the symlinked works dir into a sibling location, then placed a real `project.yaml` at both `/Users/gb/.kerf/projects/<id>/project.yaml` (bench) and `.kerf/project.yaml` (repo).
- **Observed:** RED finding: `project.yaml: present in both <bench path> and <repo path>`, hint references `specs/architecture.md` 'Where state lives'. Good wording.
- **Caveat:** Because in local mode the bench path is normally a symlink to the repo's works dir, this detector may need to be careful not to flag the "same file via symlink" case. In this test the bench location was a sibling dir to the symlink target, not the same file, so the detection was genuine.
- **Verdict:** PASS, with a note for the implementer to confirm the symlink-equivalent case is handled.

---

## Cleanup performed

Restored the bench symlink for the test project; left `/tmp/kerf-test-doctor` and its bench dir in place (gitless throwaway). **Did not mutate** anything under `~/.kerf/projects/gregberns-kerf/` (the real kerf repo's bench dir) — all real-repo doctor runs were read-only and no findings reported drift.

## Top issues (prioritized)

1. **F1/F2 — Doctor crashes on `br` errors instead of degrading to a finding.** Hits the exact failure path plan 017 was supposed to insulate against. Currently `kerf doctor` is unusable in (a) any fresh repo before `br init` and (b) the actual kerf repo today. Fix bead-filter-coverage to trap `br` errors and emit a single RED finding pointing the user at the underlying tool issue.
2. **F3 — Storage-drift hint wording is wrong in bench mode.** Says `kerf localize` for a drift that should be resolved by moving works *off* `.kerf/works/` *onto* the bench. The local-mode counterpart is worded correctly.
3. **F12 — JSON drift surface is missing.** `kerf next --format json` has no footer/drift field; automation consumers are blind to storage findings without an extra doctor call.
4. **F11 — Routing-command footer is storage-only.** Bead-filter REDs (the most common "your project is in a weird state" finding) do not produce a footer. Either expand the trigger or document the storage-only scope so agents know to also run doctor on unwired-work suspicions.
