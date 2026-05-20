# Dogfood test — Plan 020 (review / preview / show / status)

Date: 2026-05-18
Tester: dogfood agent (Opus 4.7)
Binary: `/Users/gb/go/bin/kerf`
Test project: `kerf-dogfood-xxxxx-piaihocgxp` (temp dir, fresh `kerf init`)
Test work: `new-book` (jig: `spec`)

## Summary

- Scenarios run: 14 numbered + 4 exploratory pokes
- Passes: 14 of 14 numbered scenarios pass against spec
- Issues found: 2 (1 minor gap, 1 docs/UX nit)

## Scenario-by-scenario

| # | Scenario | Result | Notes |
|---|----------|--------|-------|
| 1 | `preview <cn> problem-space` | PASS | Header `PREVIEW (read-only)`; spec.yaml mtime unchanged (1779149957 before/after). |
| 2 | `preview <cn> bogus` | PASS | `Error: status 'bogus' is not declared in jig 'spec'. Known statuses: …` — lists all 8 valid statuses. |
| 3 | `preview nonexistent <status>` | PASS | `Error: work 'nonexistent' not found in project '<id>'`. |
| 4 | `review <cn>` (text, current pass = `problem-space`) | PASS-with-caveat | Errors `jig 'spec' declares no review criteria for pass 'Problem Space'`. This is correct per jig — Pass 1 has no review block. But it means default `review` at the starting status is always an error for the spec jig. Acceptable; surfacing it as a feature, not a defect. |
| 5 | `review <cn> --format json` (decompose) | PASS | Clean JSON record: `codename`, `pass`, `artifacts[]`, `criteria[]`. |
| 6 | `review <cn> --pass decompose` text | PASS | Header `Reviewer prompt for <cn> — pass: Decompose`, then `Artifacts to read:`, `Done when the reviewer approves on:`, criteria block, and approval/changes-requested footer. |
| 7 | `review <cn> --pass research` | PASS | Exact error string requested by brief: `Error: jig 'spec' declares no review criteria for pass 'Research'`. |
| 8 | `show <cn> --compact` | PASS | 4 lines exactly: `new-book  status: problem-space → next: Decompose` / `bead_filter: (none)` / `files:       1 in work directory` / `last session: just now (active)`. |
| 9 | `show <cn>` default | PASS | Renders `Pass N: <name> → Output: NN-<filename>.md` lines for each pass. |
| 10 | preview pass with `{component}` (research) | PASS | Output line literally shows `03-research/{component}/findings.md`; placeholder is preserved, not substituted. |
| 11 | `status <cn> decompose` advance | PASS | `02-components.md` pre-created on advance; file content matches `internal/jig/builtin/templates/spec/02-components.md.template` byte-for-byte (verified via diff). |
| 12 | `status <cn> research --quiet` | PASS | Single line: `Status updated: decompose -> research`. No instructions block. |
| 13 | Idempotent re-advance | PASS | Modified `02-components.md` (appended `DIRTY`), re-set status back to `decompose`, re-ran advance: sha1 of file is unchanged. Template not clobbered. |
| 14 | Per-pass templates present | PASS | All 8 templates ship under `internal/jig/builtin/templates/spec/`: `01-problem-space.md.template`, `02-components.md.template`, `03-research.findings.md.template`, `04-design.component-design.md.template`, `05-changelog.md.template`, `05-spec-drafts.component.md.template`, `06-integration.md.template`, `07-tasks.md.template`. Picked up correctly from the built-in (embedded) source — no bench install required. |

## Exploratory pokes

- **End-to-end advance through all 8 statuses (`problem-space → ready`).** Each non-deferred output landed correctly: `02-components.md`, `05-changelog.md`, `06-integration.md`, `07-tasks.md`. Each `{component}` pass created the parent directory (`03-research/`, `04-design/`, `05-spec-drafts/`) but no inner files (deferred until the component is known). Behavior matches Pass-3 deferral expectation. Final work dir listing:
  ```
  02-components.md   03-research/   04-design/
  05-changelog.md    05-spec-drafts/  06-integration.md
  07-tasks.md        spec.yaml
  ```
- **`{component}` deferral on advance to research.** `03-research/` directory exists, is empty. Correct.
- **`review` at terminal status (`ready`).** Errors with `jig 'spec' declares no review criteria for pass 'Ready'`. Consistent — Pass 8 has no work to review, only `kerf square` to verify. Sensible behavior; same error path as Pass 1.
- **`review` vs `preview` for the same pass (decompose).** Differentiated headers — preview starts `PREVIEW (read-only) / Preview for <cn> — pass: <name> (read-only, status unchanged)`; review starts `Reviewer prompt for <cn> — pass: <name>` followed by `Artifacts to read:`. Two distinct surfaces, no confusion.

## Issues

### Issue 1 (minor gap) — Pass 1 template not pre-created on `kerf new`

`kerf new --jig spec` creates `spec.yaml` only. The Pass 1 template (`01-problem-space.md`) is **not** copied into the work directory at creation time, even though Pass 1 ships a template (`01-problem-space.md.template`). All subsequent passes get their template materialized on `kerf status <cn> <next>` advance.

Net effect: the agent on the very first pass has to fabricate the file from scratch (or look at the template path themselves) — every other pass, kerf does it for them. This is an asymmetry, not a blocker.

Repro:
```
kerf new --jig spec --title t   # work starts at problem-space
ls $WORK_DIR                    # only spec.yaml present
```

Suggested fix: copy Pass-N template when the work's status is set to the Pass-N status, including by `kerf new` (initial status set).

### Issue 2 (docs/UX nit) — `review` default-current-pass error is technically correct but unhelpful immediately after `kerf new`

A user calling `kerf review new-book` right after `kerf new` will hit `Error: jig 'spec' declares no review criteria for pass 'Problem Space'`. The error is honest, but it might read as a defect to a first-time user. Consider augmenting the message with `(no review block defined — try 'kerf review <cn> --pass decompose' or another pass with reviewer criteria)`.

## Artifacts

- Work dir final state: `/Users/gb/.kerf/projects/kerf-dogfood-xxxxx-piaihocgxp/new-book/`
- Templates verified at: `/Users/gb/github/kerf/internal/jig/builtin/templates/spec/`
- Diff of pre-created `02-components.md` vs template: only the `DIRTY` line we injected in S13.
