# Plan 009 — Triage Workflow

## Intent

Give a triage agent a closed loop for reconciling a project's bead store with
kerf's understanding of it. Today kerf renders works and ranks beads, but when
the bead store drifts — beads added, relabeled, closed, deleted externally — kerf
produces wrong output silently. This plan adds the minimum surface so a triage
agent can run `kerf triage`, act on what it surfaces, and re-run until clean.

Two load-bearing pieces:

- **`kerf show <codename>` renders attached beads.** Closes the daily "what's
  left on this work after a context clear" loop.
- **`kerf triage` with a `--resolved` exit code.** A single report of untriaged
  beads, multi-matched beads, external changes since the last triage, and
  per-work bead health. The exit code lets an agent loop on it.

Everything else in this plan is plumbing to make those two useful.

## Why

Plan 008 exploration sat a fresh agent in a project with a populated `.beads/`
store and 30 beads across five labels. Findings:

- `kerf init` does not scan the bead store. The agent never learns beads exist.
- `kerf next` is byte-identical across five different drift mutations (relabel,
  external close, delete, reopen, new off-filter bead). Drift is invisible.
- `kerf show <codename>` does not list attached beads — the one thing an agent
  asks after clearing context.
- Filters can only be wired by hand-editing `spec.yaml`. There is no CLI path.

Net: agents currently bypass kerf and shell out to `bd`/`br` to reason about the
store. That defeats kerf's purpose. The fix isn't a single command — it's a
workflow that surfaces drift, lets an agent bin beads into works, and confirms
the state is clean.

The agent-UX critique (`plans/008.../critiques/agent_ux.md`) is explicit: of the
seven candidate commands, only `kerf show <codename>` with bead rendering
closes a real daily loop on its own. `kerf triage` is only useful with the
`--resolved` exit code; without it, it's a dashboard no one reads. The other
five candidates are config plumbing that agents touch rarely.

## What Changes

### New commands

1. **`kerf triage`** — single-shot drift report. Sections:
   - **Untriaged**: beads matching no work's resolved filter, grouped by label.
   - **Multi-matched**: beads matching two or more works (ambiguity).
   - **External changes since last triage**: closed, reopened, deleted, new.
   - **Per-work bead health**: open/closed counts, filter expression.

   Flags:
   - `--resolved` — exit 0 if no untriaged, no multi-matched, no unacknowledged
     external changes; non-zero otherwise. The load-bearing flag for agent loops.
   - `--format=json` — same shape as `kerf next --format=json` (kind-tagged
     items).
   - `--ack` — record the current bead-store snapshot as the new "last seen"
     baseline without surfacing changes. Used after the agent has acted.

2. **`kerf pin <codename> <bead-id>`** — explicit pin. Escape hatch for the
   case where a filter can't reasonably catch a single bead. Writes the bead ID
   into a `pinned_beads:` list on the work's `spec.yaml`. Renamed from `attach`
   to avoid colliding with the existing filter-driven "bead attached to work"
   mental model and to match the `pinned_beads:` schema field. Demoted from
   headline status — the agent-UX critique notes this is "rarely" needed; the
   right fix is usually to broaden a work's filter (see `kerf work edit`
   below).

   **Multi-match override semantics:** pinning bead B to work A removes B from
   any other work's pin list. Pins are a *single-owner* layer applied after
   filter resolution: if B is pinned to A but B also matches work C's filter, B
   appears under A only and `kerf triage` does *not* flag it as multi-matched.
   This is what makes `kerf triage --resolved` converge — additive pins would
   loop forever on a bead whose filter overlap can't be narrowed.

### Edits to existing commands

3. **`kerf show <codename>`** — append an `Attached beads (N open / M closed)`
   block listing bead ID, status, title, and any drift markers (e.g. `! closed
   externally since last triage`). This is the single highest-value piece.

4. **`kerf new`** — add `--bead-filter '<spec>'` flag. One-shot at creation;
   writes the filter into the new work's `spec.yaml`.

5. **`kerf work edit --bead-filter-add/--bead-filter-remove '<clause>'`** —
   restored from the original 7-command surface after the scope critique noted
   the canonical triage loop has no widen-filter path without it. This is the
   primary remediation for `work_no_attached_beads` (filter too narrow) and the
   path the plan refers to in #2 above ("broaden the filter"). Mutates the
   work's `spec.yaml` `bead_filter:` clause list.

6. **`kerf next`** — add headline counters above the ranking:
   `! 6 untriaged beads · ! 2 beads multi-matched · ! 1 bead deleted externally`.
   The spec already names `warning` as a render block in `coordination.md`; this
   wires it. Agents working a single bead see drift without running `triage`.

### Drift detection model

`kerf` persists `.kerf/sync-cache.json` per project (registered in
`architecture.md` as project-local infrastructure, peer to
`.kerf/project-identifier` — *not* in `works.md`, which is per-work). On every
kerf invocation that touches the bead store, kerf compares the current
snapshot to the cached one and records deltas. `kerf triage` reads and
presents them.

**Baseline advancement is explicit only.** `kerf triage --ack` advances the
baseline to the current snapshot. Neither `kerf new`, `kerf pin`, nor
`kerf work edit` advance it implicitly — surfaces stay sticky until the agent
acknowledges them. This resolves the L86 / Open Question #2 contradiction the
scope critique flagged: implicit advancement is a footgun (agent runs `new`,
drift silently rebaselines, next `triage` shows clean without the agent ever
seeing closed beads).

Snapshot shape:

```json
{
  "snapshot_id": "<sha256 of sorted bead records>",
  "captured_at": "2026-05-15T12:34:56Z",
  "beads": {
    "hk-cb-042": { "status": "open", "labels": ["subsystem:bridge"], "hash": "<sha>" }
  },
  "filter_assignments": {
    "hk-cb-042": ["bridge"]
  }
}
```

Per-bead hash covers status + sorted labels + title. Cheap to compute, easy to
diff, no schema migration risk if `bd`/`br` add fields.

Open question for design step: do we hash the full bead record (forces re-triage
on any bd metadata change) or just the fields kerf consumes (status, labels,
title, dependencies)? Recommendation: the latter — fewer false positives.

### `--resolved` exit-code semantics

| State | Exit |
|-------|------|
| Untriaged == 0 AND multi-matched == 0 AND unacked external changes == 0 | 0 |
| Non-zero drift, but resolved-count strictly decreased since last run (`--made-progress`) | 3 |
| Non-zero drift, no progress since last run | 2 |
| Bead store unreadable, or project not initialized (`kind: not_initialized`) | 1 |

Same 0/1/2 convention as proposed for `kerf next` in plan 010 (concurrency).
Exit 3 is a loop-progress signal so `until kerf triage --resolved; do <act>;
done` terminates on hard cases: two consecutive exit-3 runs with identical
drift sets indicate stuck progress and the agent should break out and ask for
human help, rather than loop forever.

**Not-initialized handling.** When `project.yaml` is absent, `kerf triage`
exits 1 with `kind: not_initialized` and a single-line directive
(`run kerf init first`), not a generic error. The agent must not treat a
non-zero exit from an uninitialized project as "drift exists" and try to act.

`cli.md` does not currently have an exit-code table. The triage exit codes are
documented inline in the `commands.md` `kerf triage` section; promoting them
into a project-wide `cli.md` table is out of scope for this plan and tracked
as separate spec debt.

### Triage agent workflow (canonical)

```
cd <project>
kerf init                                                  # if not initialized; scans bead store
kerf triage                                                # surfaces untriaged + multi-matched + drift
# for each untriaged bucket, one of:
kerf new <codename> --bead-filter 'label=<L>'              # new bucket
kerf work edit <codename> --bead-filter-add 'label=<L>'    # widen an existing bucket
kerf pin <codename> <bead-id>                              # explicit single-bead pin
kerf triage --ack                                          # acknowledge surfaced drift
kerf triage --resolved                                     # confirm clean; loop until exit 0 (or exit 3 stuck)
```

Per-bead suggested next action is rendered inline in each `kerf triage`
section: each untriaged bucket prints a templated, ready-to-paste command
(e.g. `kerf new <suggested-codename> --bead-filter 'label=bridge'`) so agents
copy literally instead of synthesizing.

## Specs Affected

| Spec file | Change |
|-----------|--------|
| `specs/commands.md` | New sections: `kerf triage`, `kerf pin`. Update `kerf show` (attached-beads block), `kerf new` (`--bead-filter` flag), `kerf work edit` (`--bead-filter-add/-remove`), `kerf next` (drift counters). Document `kerf triage` exit codes inline (0/1/2/3, including `not_initialized` kind). |
| `specs/coordination.md` | New "Drift detection" subsection under **Integration Points** (peer to "Bead Attachment"): snapshot shape, hash scope (status + sorted labels + title + deps + id), `--ack`-only baseline advancement. Extend warning kinds with `untriaged_beads`, `multi_matched_bead`, `external_drift`. Amend §Resolution order: pins are a separate layer applied *after* filter resolution and *override* filter matches (single-owner semantics), so the existing "first hit wins, filters do not merge" rule is unaffected. Add a note that `work_no_attached_beads` (zero-filter-match cleanup) and `untriaged_beads` (unmatched beads) are complementary and must not double-fire on the same work. |
| `specs/architecture.md` | Register `.kerf/sync-cache.json` as project-local infrastructure, peer to `.kerf/project-identifier`. |
| `specs/works.md` | Add `pinned_beads:` list to per-work `spec.yaml` schema. Note pin-override semantics (a bead may be pinned to at most one work; pinning removes from any prior work's list). |

No changes to `specs/dependencies.md`, `specs/sessions.md`, `specs/jig-*`,
`specs/cli.md` (no exit-code table exists there yet; triage exit codes live
inline in `commands.md` for now).

## Open Questions

1. **Hash scope.** Decision: hash kerf-relevant fields — bead id, status, all
   sorted labels (not a curated subset; the project filter is user-defined and
   any label is fair game), title, and sorted dependencies. Spec the exact
   field list in `coordination.md`.
2. **Multi-match resolution suggestion.** When a bead matches two works'
   filters, what default action does `kerf triage` recommend in the templated
   next-action hint — narrow one filter, or `kerf pin`? Pick one for v1.
3. **Initial baseline.** On a fresh `kerf init`, is the baseline the current
   bead store (silent adoption) or empty (everything shows as "new")? Must
   reconcile with `kerf init`'s existing snapshot-on-create behavior
   (snapshots.md). Recommendation: empty baseline so the first `kerf triage`
   is a full inventory pass, but the snapshot side stays untouched.

Resolved (no longer open):

- **Pin vs. filter.** Pin overrides filter (single-owner). Pinning to one work
  removes the bead from any other work's pin list. Required for `--resolved`
  to converge.
- **Baseline advancement.** Explicit `--ack` only. No implicit advancement on
  `kerf new` / `kerf pin` / `kerf work edit`.

## Implementation Notes

Approximate scope per the scoping critique:

- **7 command files touched:** `cmd/show.go`, `cmd/new.go`, `cmd/next.go`,
  `cmd/work.go` (`--bead-filter-add/-remove`), `cmd/triage.go` (new),
  `cmd/pin.go` (new, formerly `attach`).
- **`internal/spec/` mutators** — in-place edits to `spec.yaml` for adding
  filters and pinned beads. Comment-preserving round-trip is now treated as
  required for v1: agents will edit user-authored `spec.yaml` files, and the
  first time a kerf edit silently nukes a comment, it becomes mandatory
  rework. Budget accordingly.
- **New `internal/drift/` package** — snapshot capture, hash, diff, cache file
  read/write. ~150 LOC + tests.
- **`internal/feed/` extension** — three new warning detectors
  (`untriaged_beads`, `multi_matched_bead`, `external_drift`). Reuses the
  existing warning render path from plan 006.
- **5 new/amended `commands.md` sections.** Plus `coordination.md` drift
  subsection + pin-layer amendment, `architecture.md` cache-file entry, and
  `works.md` `pinned_beads:` row.

Rough cost: **1.5–2.5 sprint-weeks** for one engineer. The 1.5 figure is the
floor and assumes comment-preserving YAML mutators land cheaply; the 2.5
figure absorbs spec-write time across four spec files (1–2 days before any
code) plus likely rework on the `internal/spec/` round-trip path.

### Sequencing

1. Spec changes first, in this order (CLAUDE.md spec-first rule applies):
   `coordination.md` drift subsection + pin-layer amendment;
   `architecture.md` cache-file entry; `works.md` `pinned_beads:` row;
   `commands.md` new + amended sections (triage, pin, show, new, work edit,
   next).
2. `kerf show <codename>` bead rendering. Highest standalone agent value;
   absorbs the bead-count column work for `kerf map` if extended later.
3. `internal/drift/` package + `.kerf/sync-cache.json` lifecycle.
4. `kerf triage` command using the drift package + new warning detectors.
5. `kerf new --bead-filter` flag and `kerf work edit --bead-filter-add/-remove`.
6. `kerf pin` command.
7. `kerf next` headline counters (cheap; depends on the warning detectors).

### Dependencies on plan 008 work

- Plan 008 P0#3 (rewrite `cmd/show.go:278` to use `internal/beads.List()`) must
  land before #2 above. The scoping critique notes triage #1 "subsumes part of
  A:F2"; sequence accordingly.
- Plan 008 P0#1 (fix `work_codename: null`) must land before #7 above; drift
  counters built on broken attachment will surface false positives.

## Not in scope

- **Multi-project triage.** v1 is single-project. `kerf triage` operates on the
  resolved project only. Cross-project rollup is a later concern.
- **Auto-fix.** Every triage surface is "show, let an agent decide". No
  `--auto` flag, no implicit relabel, no implicit work creation. If an agent
  wants to script `kerf new --bead-filter` in a loop, fine — but kerf doesn't
  do it for them.
- **Bead creation.** `kerf` does not create beads. The triage agent dumps beads
  via `bd`/`br` first, then runs `kerf triage` to bin them.
- **Bead-tool abstraction beyond what plan 006 already provides.** This plan
  reads beads through the existing `internal/beads` path.
- **PR/lint/doctor items.** Out of scope, same as plan 006.
- **`kerf map` bead counts.** Nice-to-have noted in the exploration; defer
  unless the agent-UX critique's "sometimes" frequency proves load-bearing.
