# Plan 017 — Storage Reconciliation and `kerf doctor`

> **Status: stub.** Spawned from Plan 015 (harmonik beta-feedback triage). Expansion handled by the `plan-implementation` flow.

## Intent

kerf splits state between the repo's `.kerf/` and the global bench at `~/.kerf/projects/<id>/`. The split is a real architectural choice, but today agents can't tell which side is canonical, drift between the two accumulates silently, and there is no single "is my project healthy?" surface. This plan introduces the doctor / reconciliation primitives: a health check command, drift surfacing on the routing commands, clearer bench-location output from `kerf new`, and the documentation cleanup that lets the storage model be understood by a fresh-context agent.

## Background

Items come from `plans/015_harmonik_beta_feedback/triage.md` themes 2 (storage layout) and 9 (command-UX gaps), plus one cross-cutting item from theme 1. The dogfood session repeatedly produced orphan files in the repo because the bench path appeared once mid-output of `kerf new` and was never re-surfaced.

## Scope

- New `kerf doctor` command (or `kerf status --project`) that checks `project.yaml` completeness, repo `.kerf/` vs. bench sync, per-work `bead_filter` coverage, and archive orphans; prints a green / yellow / red summary.
- Drift surfacing on every `kerf next` / `kerf triage` run as a one-line footer when drift exists, with a hint pointing at the doctor command.
- `kerf new` ends with a clearly fenced "working directory:" line so agents writing files relative to the repo notice the bench path.
- Documentation cleanup: every reference to `work.yaml` becomes `spec.yaml`; `kerf work edit --help` names the file path it edits.
- Optional: `kerf localize --check` non-destructive preview of what reconcile would do.
- Out of scope: the actual reconcile semantics if they need to change (assume current `kerf localize` is correct); init's instruction text (Plan 016).

## Items absorbed from Plan 015

- 1.10 — instruction block should mention bench location and `kerf localize` (cross-listed with Plan 016, but the bench-surfacing work lands here)
- 2.1 — `.kerf/` ↔ bench silent drift
- 2.2 — no reconciliation tool surfaced from `kerf init` / `kerf next`
- 2.3 — `kerf new` doesn't make the bench path obvious
- 2.4 — `work.yaml` vs. `spec.yaml` doc drift
- 9.5 — `kerf doctor` / `kerf status --project`

## Specs likely touched

- `specs/architecture.md` — bench layout, repo `.kerf/` semantics, drift definition
- `specs/commands.md` — new `kerf doctor` (or `kerf status --project` extension); `kerf new` output; `kerf work edit --help`
- `specs/finalization.md` — possibly, depending on how localize interacts
- TBD during plan-implementation for which command surface owns drift detection

## Open questions

- Is the health command `kerf doctor` (new top-level verb) or `kerf status --project` (extension of an existing command)?
- What counts as "drift"? Differing work dirs only, or also differing `spec.yaml` contents, history depth, archive state?
- Should the drift footer on `kerf next` / `kerf triage` be opt-out (env var or config flag) for users who don't want it?
