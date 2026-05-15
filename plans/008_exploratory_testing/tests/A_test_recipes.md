# Integration-Test Recipes — Failure & Out-of-Band

Format per recipe: setup → action → expected observable. Each is small enough to be a single Go integration test against a tempdir + a `bd`-initialized `.beads/`.

---

## R1 — `kerf next` surfaces bead items when work has matching beads

Setup: kerf init, `kerf new foo`, `bd create -l work:foo "Task 1"`, `bd create -l work:foo "Task 2"`.
Action: `kerf next`.
Expect: output contains a `bead` row for each of the two open beads, attributed to `foo`.

---

## R2 — `kerf next --only bead` and `--kinds` flags exist and filter

Setup: as R1 plus an unmatched bead `bd create -l rogue:x "Other"`.
Action: `kerf next --only bead`, then `kerf next --kinds warning`.
Expect: `--only bead` returns only bead rows, no warnings; `--kinds warning` returns only the unmatched-beads warning.

---

## R3 — `kerf show` displays bead summary

Setup: as R1 plus `bd close <id>` for one bead.
Action: `kerf show foo`.
Expect: output contains a line like `Beads: 2 total, 1 closed, 1 open`.

---

## R4 — `kerf init` re-run preserves `bead_filter`

Setup: init, manually add `bead_filter: { id_prefix: "kex-" }` to project.yaml.
Action: `kerf init` again.
Expect: project.yaml still contains the bead_filter; exit code 0; output explicitly says "already initialized, no changes" (not "Created project.yaml").

---

## R5 — `kerf init` auto-detect proposes a filter when ≥3 beads share a prefix

Setup: `bd init`, create 5 beads labeled `subsystem:foo`, then `kerf init`.
Expect: init output includes a "Bead-filter detection summary" line and project.yaml contains `bead_filter: { label: "subsystem:{codename}" }` (non-interactive auto-write per spec line 1185).

---

## R6 — Out-of-band `bd close` reflects in `kerf next` and `kerf show`

Setup: as R1.
Action: run `kerf show foo`, capture bead summary; `bd close <id>`; run `kerf show foo` again.
Expect: closed count increments; the corresponding `bead` row is gone from `kerf next`.

---

## R7 — Deleting a work leaves unmatched beads → `kerf next` emits a warning

Setup: `kerf new foo`, create 2 beads `work:foo`. `kerf delete foo --yes`.
Action: `kerf next`.
Expect: a `warning` item naming "2 unmatched beads (work:foo)" or similar.

---

## R8 — Case-mismatched labels surface as warning

Setup: project default filter `work:{codename}`. Create work `foo`. Create bead with label `Work:Foo`.
Action: `kerf next`.
Expect: a `warning` item suggesting case-mismatch with the project's filter.

---

## R9 — Brand-new label prefix used by many beads is flagged

Setup: kerf init with project_filter `work:{codename}`. Create 10 beads labeled `subsystem:newthing` (no kerf work matches).
Action: `kerf next`.
Expect: warning naming the unmatched prefix and the count.

---

## R10 — Bead tool unavailable / incompatible is surfaced, not silently swallowed

Setup: a bead store that the kerf bead reader cannot parse (e.g. `br list --format json` returns a JSON error like `missing field jsonl_export`).
Action: `kerf init`, `kerf next`, `kerf show <work>`.
Expect: a clear warning ("could not read bead store: ...") in each command's output, not a silent zero-bead result.

---

## R11 — Custom (non-jig) status keeps work visible in `kerf next`

Setup: `kerf new foo --jig spec`, `kerf status foo custom-status` (CLI warns but accepts).
Action: `kerf next`.
Expect: `foo` still appears in next-actions, possibly with a `warning` row noting unrecognized status. (Invariant 5.)

---

## R12 — Corrupt `spec.yaml` produces a clear error, not silent disappearance

Setup: create work `foo`. Overwrite `spec.yaml` with invalid YAML.
Action: `kerf list`, `kerf show foo`, `kerf map`.
Expect: each command shows `foo` with a `[corrupt]` marker or emits a project-level warning. `kerf show foo` exits non-zero with "spec.yaml is malformed at line N", not "work not found".

---

## R13 — Missing `spec.yaml` directory state is consistent across commands

Setup: create work `foo`, delete its `spec.yaml` (leave dir).
Action: `kerf list`, `kerf new foo --jig spec`.
Expect: consistent behavior — either both treat the dir as orphan and allow recreate, or both treat it as existing and `list` shows it as `[broken]`.

---

## R14 — `kerf next --area <undefined>` warns

Setup: project has area `api`; no area `web`.
Action: `kerf next --area web`.
Expect: explicit "no area named `web` in project" error/warning, not empty success.

---

## R15 — `kerf delete --yes` honors a documented force flag

Action: `kerf delete foo --yes`, then for parity `kerf delete bar --force`.
Expect: either both flags work, or `--force` returns a hint pointing to `--yes`. Today `--force` is "unknown flag".
