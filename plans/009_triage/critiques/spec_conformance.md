# Spec-Conformance Critique — Plan 009

Per CLAUDE.md: every behavior change requires the spec first. Below: where each
proposed change lands, and where the plan conflicts with existing spec text.

## Per-change placement

| Plan item | Spec home | Status |
|---|---|---|
| `kerf triage` command | New section in `commands.md` between `kerf next` and `kerf areas list` | OK — net-new |
| `kerf attach` command | New section in `commands.md`, adjacent to `kerf new` | OK — net-new |
| `kerf show` "Attached beads" block | Amend `kerf show` Behavior + Output | OK — extends "Bead status" bullet at lines 245-249 |
| `kerf new --bead-filter` flag | Amend `kerf new` flag table + Behavior step 6 (initialize `spec.yaml`) | OK |
| `kerf next` drift counters | Amend `kerf next` warning detectors list (commands.md §`kerf next` step 3) | OK — three new warning kinds slot into existing list |
| Drift model + `.kerf/sync-cache.json` | New subsection in `coordination.md` under **Integration Points** (peer to "Bead Attachment") | OK; cache-file path also needs an entry in `architecture.md` (project-local files), not `works.md` |
| `pinned_beads:` on work `spec.yaml` | New row in `works.md` §`spec.yaml` Schema field table | OK |
| `--resolved` exit codes | `cli.md` has **no** exit-code table today — plan claims one exists. Must create the section before referencing it | **Spec debt** |

## Conflicts with current spec

1. **Filter resolution "first hit wins, filters do not merge"** (coordination.md
   §Resolution order, lines 236-244). Plan's `pinned_beads:` as "additive"
   (Open Question 4) directly conflicts: pins would have to bypass or layer on
   the filter resolution. The spec must be amended to introduce a pin layer
   that is explicitly additive, distinct from filter resolution. Without this
   amendment, `kerf attach` cannot be implemented spec-consistently.

2. **`work_no_attached_beads` semantics** (commands.md §`kerf next` step 3).
   Today a zero-match filter surfaces as a cleanup item. Plan 009's
   `untriaged_beads` warning operates on the inverse (beads matching no work).
   These are complementary but the plan must clarify they do not double-fire
   when a project has both a missing filter and unmatched beads.

3. **`kerf init` already does bead auto-detect** (commands.md lines 1173-1185).
   Plan 009 says "`kerf init` does not scan the bead store" — false per current
   spec. Plan's "Why" section needs to acknowledge the existing scan or refute
   it as not-yet-implemented behavior; otherwise the rationale is wrong.

4. **`kerf next` warning rendering** (commands.md §"Default kind selection",
   line 1470). Warnings render as a header block; plan's "headline counters"
   must use this existing mechanism, not invent a new render path.

5. **Initial baseline (Open Question 5)**. Empty baseline conflicts with
   `kerf init`'s existing snapshot-on-create pattern (snapshots.md). Pick
   one model and reconcile.

## Drift model fit with Integration Points

Good fit. `coordination.md` §Integration Points already partitions kerf-owns
vs. beads-owns; a "Drift detection" subsection peer to "Bead Attachment" reads
naturally. Snapshot shape, hash scope, and baseline-advance rules all belong
there. The cache file path (`.kerf/sync-cache.json`) is project-local
infrastructure — register it in `architecture.md` alongside
`.kerf/project-identifier`, not in `works.md` (which is per-work).

## Spec-first ordering required before any code

1. `coordination.md` Drift subsection + pin-vs-filter amendment.
2. `architecture.md` cache-file entry.
3. `cli.md` exit-code section (new).
4. `commands.md` new + amended sections.
5. `works.md` `pinned_beads:` field.

Plan 009 §Sequencing step 1 already says this; enforce it.
