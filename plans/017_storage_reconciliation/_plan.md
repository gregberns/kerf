# Plan 017 — Storage Reconciliation and `kerf doctor`

> **Status: baked.** Spawned from Plan 015 (the harmonik beta-feedback triage). Expanded via the `plan-implementation` flow.

## Intent

kerf splits per-project state across two locations: the repo's `.kerf/` directory (committed into git) and the bench at `~/.kerf/projects/<project-id>/` (the global, off-git workspace). The split is an intentional architectural choice — works can be in flight without polluting git, while project identity stays in the repo so worktrees agree on what project they belong to. The friction is that today a fresh-context agent has no obvious way to learn which side is canonical for which file, drift between the two accumulates silently, and there is no single "is my project healthy?" surface. This plan introduces the reconciliation primitives: a health-check command, drift surfacing on the routing commands, a clearly fenced bench path in `kerf new` output, and the documentation cleanup that lets the storage model be understood without spelunking through specs.

## Background

Items come from `plans/015_harmonik_beta_feedback/triage.md` themes 2 (storage layout) and 9 (command-UX gaps), plus one cross-cutting item from theme 1 about surfacing the bench path during init. The harmonik dogfood session (2026-05-15 → 2026-05-18, run by an Opus 4.7 agent) produced orphan files in the repo because the bench path appeared once mid-output of `kerf new` and was never re-surfaced; the agent then could not tell whether `.kerf/works/` or `~/.kerf/projects/<id>/` held the truth.

Storage modes themselves are already specified in `specs/architecture.md` (bench mode versus local mode, with a symlink bridging the two in local mode). This plan does not reopen the mode design — it adds the missing observability and reconciliation surface on top of it.

## Scope

- A new top-level health-check command — working name `kerf doctor` — that inspects the current project and reports green / yellow / red findings on:
  - `project.yaml` shape (declared jigs, default_jig presence, schema completeness).
  - Drift between the repo's `.kerf/` and the bench's `~/.kerf/projects/<id>/` for the current project.
  - Symlink integrity in local mode (target exists, points where the resolver expects).
  - Per-work `bead_filter` coverage (a work is `unwired` if it has no filter declared — see Plan 019).
  - Archive orphans (a `~/.kerf/archive/<id>/` entry whose codename also appears live).
- One-line drift surfacing on `kerf next` and `kerf triage` when drift exists, with a hint pointing at the doctor command. Suppressible via a config flag or env var.
- A `kerf new` output cleanup so the run ends with a clearly fenced `working directory:` line naming the bench (or local) path, plus a second line naming the repo-side files agents should touch.
- A `kerf localize --check` (or `--dry-run`) preview that prints what the migration would move without changing anything.
- Documentation cleanup: every spec, help text, and embedded instruction block referring to `work.yaml` is corrected to `spec.yaml`; `kerf work edit --help` names the file path it edits.
- Out of scope:
  - The init UX itself (Plan 016 owns the instruction block content; this plan owns only the storage-related fragment routed in).
  - The triage rework (Plan 018 owns the `kerf triage` output redesign; this plan owns only the drift-footer addition).
  - The filter-bootstrap primitive (Plan 019 owns the filter slot; this plan only consumes "is this work unwired?" as a doctor signal).
  - Re-designing the storage modes themselves; `kerf localize` semantics stay as today.

## Design notes

**Canonical location, by file class.** The triage's headline pain — "two locations, no clear canonical" — is mostly a documentation problem; the spec already names the canonical side for each file class, but the rules are not surfaced anywhere an agent will reliably read. The doctor command codifies the same rules in a runtime check:

- Project identity (`.kerf/project-identifier`, `.kerf/sync-cache.json`, repo `config.yaml`) — always in the repo.
- Per-project config (`project.yaml`, `areas.yaml`) — on the bench in bench mode, in the repo's `.kerf/` in local mode.
- Work directories (`{codename}/`) — on the bench in bench mode, in `.kerf/works/` in local mode.
- Archive — always on the bench under `~/.kerf/archive/`.

**What counts as drift?** First-pass definition, narrow. The resolved storage mode names a single canonical location for each file class; drift is anything that contradicts that:

- A work directory (or `project.yaml` / `areas.yaml`) lives in the non-canonical location for the active mode — e.g. a `.kerf/works/<codename>/` directory in a bench-mode project, or a `~/.kerf/projects/<id>/<codename>/` real directory in a local-mode project. This is the harmonik orphan-file failure mode.
- A work directory appears in *both* canonical and non-canonical locations (the agent wrote twice).
- The bench symlink (local mode) is broken, missing, points outside the resolver's expected target, or is a real directory instead of a symlink.
- `project.yaml` or `areas.yaml` exists in both the repo's `.kerf/` and the bench `~/.kerf/projects/<id>/` simultaneously.
- Content-level drift inside files is explicitly **out of scope** for v1; doctor reports presence-level findings only. A later pass can extend to file-hash comparison once the presence-level surface is settled.

**Sync model.** Doctor is read-only by default. It reports findings and names the command that would fix each (most commonly `kerf localize` for bench → local migrations, or a manual move for the inverse). A `--fix` flag is deferred to a later plan once the failure modes are catalogued from real usage; auto-fixing storage state without a recovery path is the kind of destructive primitive worth proving demand for first.

**Doctor output shape (sketch).**

```
kerf doctor — project: harmonik (local mode)

[green]  project-identifier: harmonik
[green]  project.yaml: present, default_jig=spec, 3 jigs declared
[yellow] storage drift: 1 finding
         - work 'phase-3-dot' exists on bench but not in .kerf/works/
           hint: kerf localize --check  (preview what reconcile would do)
[green]  bench symlink: ~/.kerf/projects/harmonik -> /Users/.../.kerf/works
[red]    bead_filter coverage: 2 of 6 works unwired
         - works without bead_filter: phase-3-dot, scratch
           hint: kerf bootstrap-filters  (when Plan 019 lands)
[green]  archive: 1 entry, no live collisions
```

**Drift footer on `kerf next` / `kerf triage`.** When drift exists, append a one-line footer:

```
note: 1 storage finding — run `kerf doctor` for details
```

Suppressible via `kerf config set doctor.footer false` or an env var `KERF_DOCTOR_FOOTER=0`. Default on; opt-out rather than opt-in, since the harmonik session's orphan-file failure mode happened silently.

**Naming alternatives considered.**

- `kerf doctor` (chosen) — new top-level verb, mirrors `go vet` / `brew doctor` / `gh doctor`. Easy to discover; clear "diagnostic, not destructive" connotation.
- `kerf status --project` — extension of an existing command. Lower surface-area cost, but `kerf status` today is scoped to one work and confusing to overload. Rejected for v1.
- `kerf check` — overlaps with `kerf square` (the work-level verification command); rejected.

**Surface alternatives considered.**

- A `--fix` flag that would actively reconcile drift. Deferred; see "Sync model" above.
- Folding drift footer into `kerf list` instead of `kerf next` / `kerf triage`. Rejected: `kerf list` is the inventory surface, the routing commands are the ones an agent runs every loop.

## Spec changes proposed

Prose only — no edits in this plan. The plan-implementation flow will land the actual edits per bead.

- `specs/commands.md`
  - Add a `kerf doctor` section: synopsis, flags (`--format json`, `--detector <id>`, `--quiet`, `--strict`), exit-code semantics (tentative: 0 on green/yellow, non-zero on red findings only when `--strict` is set — exact default pinned during decomposition; see open questions).
  - Extend the `kerf next` and `kerf triage` sections with a drift-footer subsection and the suppression config / env var.
  - Extend the `kerf new` section with the fenced `working directory:` final line.
  - Extend `kerf localize` with `--check` / `--dry-run` semantics.
  - Update `kerf work edit` help-text spec so it names `spec.yaml` and the bench-or-repo path.
- `specs/architecture.md`
  - Add a "Drift detection" subsection (peer to "Symlink Lifecycle") naming the presence-level drift definitions used by doctor. Cross-link to `coordination.md`'s existing drift surface so the two concepts (bead-store drift versus storage-layout drift) are explicitly distinguished.
  - Add a "Where state lives" cheat-sheet table that condenses the existing file-locations table into a single agent-readable block; embed-or-link from the init instruction template.
- `specs/cli.md`
  - Add output-philosophy note for diagnostic commands (read-only by default, name the fix command in each finding).
- Doc-drift sweep: search every spec and embedded instruction template for `work.yaml`, replace with `spec.yaml`. Same sweep for any remaining references to legacy paths.

## Beads outline

Not yet entered into `bd`. Rough sequencing, smallest shippable units first:

1. Doc-drift sweep: `work.yaml` → `spec.yaml` across all specs and embedded templates; `kerf work edit --help` names `spec.yaml`.
2. `kerf new` output cleanup: fenced `working directory:` final line plus repo-side hint.
3. `specs/architecture.md` cheat-sheet ("Where state lives") and drift-detection subsection.
4. `kerf localize --check` (dry-run preview) implementation + spec.
5. Doctor scaffold: command, registry, output formatter, `--format json` shape, exit codes.
6. Doctor detector — project.yaml shape.
7. Doctor detector — storage-layout drift (presence-level, both modes).
8. Doctor detector — symlink integrity (local mode).
9. Doctor detector — archive orphans.
10. Doctor detector — unwired works (consumes `bead_filter` presence; gated behind Plan 019 landing for the fixer hint, but the detector itself is independent).
11. Drift footer on `kerf next`.
12. Drift footer on `kerf triage`.
13. Suppression config / env var for the drift footer.
14. `specs/cli.md` diagnostic-output philosophy note (read-only by default, each finding names the fix command).

Doctor's symlink-integrity check should reuse the helper logic in `cmd/localize.go` (`ensureLocalSymlink`) rather than re-implementing — pinned during decomposition.

Roughly 13 beads, several mergeable in pairs (footers 11 + 12, detectors 7 + 8). Plan-implementation can re-shape during decomposition.

## Items absorbed from Plan 015

- 1.10 — instruction block should mention bench location and `kerf localize`. The instruction-block edit itself lives in Plan 016; the underlying cheat-sheet that the instruction block embeds (or links to) is owned here.
- 2.1 — `.kerf/` ↔ bench silent drift.
- 2.2 — no reconciliation tool surfaced from `kerf init` / `kerf next`.
- 2.3 — `kerf new` doesn't make the bench path obvious.
- 2.4 — `work.yaml` vs. `spec.yaml` doc drift.
- 9.5 — `kerf doctor` / `kerf status --project`.

## Conflicts flagged (for the orchestrator)

- **Plan 016 (init UX overhaul)** also rewrites the agent-setup instruction block and touches `project.yaml` shape. The cheat-sheet this plan adds to `specs/architecture.md` is the source the 016 instruction block should embed; if 016 lands first, the embed becomes a forward reference. Suggested order: 017's cheat-sheet lands before 016's instruction-block rewrite, or the two coordinate on one PR.
- **Plan 019 (filter bootstrap)** owns the `bead_filter` slot and the `unwired` / `empty` / `broken` rank labels. The doctor's "unwired works" detector is independent code but its hint text references the bootstrap command that Plan 019 introduces. If 019 lands later, the doctor's hint reads as a TBD until 019 ships.
- **`project.yaml` shape and presence.** Plan 016 owns the schema and write-correctness of `project.yaml`; this plan's "project.yaml shape" doctor detector only checks presence + key completeness against whatever schema 016 settles on. If 016's schema is still in flight when doctor detectors land, this detector ships with a thin first-pass check.
- No suggested resolution from this plan — flagged for the orchestrator.

## Open questions

- Is the health command `kerf doctor` (a new top-level verb) or `kerf status --project` (an extension)? Plan currently picks `doctor`; see "Naming alternatives." Worth one user check.
- Should the drift footer's default be opt-in or opt-out? Plan picks opt-out (default on). The risk is footer fatigue if the false-positive rate is high; the upside is the harmonik silent-orphan failure mode.
- Should v1 include any auto-fix behavior, or stay read-only? Plan picks read-only. Revisit after one release cycle of doctor in the wild.
- Doctor's exit-code semantics: does a red finding fail CI (non-zero) by default, or only with `--strict`? Plan-implementation should pin this during decomposition.
- Does presence-level drift detection need a hash-level extension to be useful in practice, or is the presence layer enough for v1? Open until the first detector ships and we see real failure modes.
- For users running multiple worktrees of the same repo (local mode), the bench symlink can only point at one worktree. Should doctor flag this explicitly, or treat it as expected per `specs/architecture.md`'s "Git Worktrees" note?
