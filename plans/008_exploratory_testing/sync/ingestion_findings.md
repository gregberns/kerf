# Ingestion & Drift Findings — 2026-05-15

Workspace: `/tmp/kerf-ingest/` (copied `.beads/` from gregberns/harmonik then `bd init --force`,
imported 30 synthetic beads across labels `backend`, `ui`, `docs`, `bug`, `research`).
Binary: `/tmp/kerf` (current main).

## Cold ingest experience

| Step | Observation | Severity |
|------|-------------|----------|
| `kerf` (bare, before init) | Auto-resolved to a stale "last touched" project and printed `Total active works: 24` — those works belong to **other** projects, not the cwd. Bare `kerf` ignores cwd entirely when no `.kerf/project-identifier` exists. | **High** — actively misleads a triage agent into thinking it found work that does not exist. |
| `kerf init` | Created `.kerf/project-identifier` and `project.yaml` with default jigs. **Did not detect the populated `.beads/` directory next door.** No mention of beads, no proposed `bead_filter`, no offer to scan labels. Output is purely jig boilerplate. | **High** — `kerf init` is the natural entry point for a triage agent; this is where label discovery should live. |
| `kerf next` (post-init, pre-works) | `No actionable works for project 'kerf-ingest'.` — silent about the 30 untriaged beads sitting in `.beads/`. A triage agent has zero hint that beads even exist. | **Critical** — defeats the entire "triage agent" workflow. |

## Drift-surfacing test results

Set up 4 works (`backend-cleanup`, `ui-polish`, `docs-pass`, `bug-triage`) and hand-edited
`bead_filter` into each `spec.yaml` (no CLI flag exists for this — see gap below).
The 5 `research`-labeled beads match no filter from the start.

| # | Mutation | Did `kerf next` surface it? | Did `kerf show` surface it? | Did `kerf map` surface it? |
|---|----------|----------------------------|----------------------------|---------------------------|
| A | `bd label remove` — relabel 2 `backend` beads off | No | No | No |
| B | `bd create` new bead with orphan label `unbinned` | No | No | No |
| C | `bd close` two beads externally | No | No | No |
| D | `bd delete` one bead | No | No | No |
| E | Reopen externally-closed bead | No | No | No |

`kerf next` output is **byte-identical** across mutations — it only cares about work-level state
(`creation order` ties).

Beyond the mutation list, two structural blind spots:

- **Initial unassigned coverage** — `research` beads (5 of 30) never matched any filter; `kerf next`
  never said "5 beads have no work." A triage agent has to know to ask `bd label list-all` itself.
- **`kerf show <codename>`** — does not list the attached beads, their statuses, or counts. Generic
  jig instruction text only. There is no way to ask kerf "what beads are in this work right now?".

## Severity-ranked gaps

1. **Critical — no surface for untriaged beads.** `kerf next` / `kerf init` / `kerf show` never
   mention beads that match no work's filter. Triage agent is blind unless it bypasses kerf.
2. **Critical — `bead_filter` is invisible to the CLI.** No `kerf new --bead-filter`, no
   `kerf config bead_filter`, no `kerf <codename> bead add`. Filters must be hand-edited into
   YAML, which is a non-starter for an agent expected to wire works up at scale.
3. **High — bare `kerf` ignores cwd.** Reports a global bench count that's wrong for the project
   the agent is sitting in.
4. **High — `kerf init` does not scan `.beads/` for label suggestions.** This is the only place
   in the workflow where auto-detection would compound; today it does nothing.
5. **Medium — drift is invisible.** Re-labels, externals closes, deletes, reopens, and new
   off-filter beads produce identical `kerf next` output. No drift report exists.
6. **Medium — `kerf show` does not render bead attachment.** Attached bead IDs, statuses,
   completion ratio, rework count — none of it appears, despite being core to the
   coordination spec (specs/coordination.md L168 `momentum`, L170 `rework`).

## Suggested fixes (smallest viable)

- **`kerf init --scan-beads`** (or unconditional): on init, query `bd label list-all` + `bd list --json`
  and write a `project.yaml` `proposed_bead_filters:` block grouped by label frequency. Print a
  one-liner: "Found 30 beads across 5 labels — none attached to any work. Run `kerf triage`."
- **New command `kerf triage`** (or extend `kerf next --triage`): emit a single report:
  - Beads matching no work's filter (with label histogram)
  - Beads matching multiple works (ambiguity)
  - Beads whose status changed externally since last `kerf` invocation
  - New beads created since last invocation
  Cheap implementation: persist `last_seen_bead_ids` and `last_seen_filter_assignments` in
  `.kerf/sync-cache.json`; diff on demand.
- **CLI surfaces for filters.** `kerf new --bead-filter 'label=backend'` and
  `kerf work edit <codename> --bead-filter ...` so works can be wired without hand-YAML.
- **`kerf show` enrichment.** Append an `Attached beads (N open / M closed)` block enumerating
  IDs, statuses, and any drift since last view.
- **`kerf next` headline counters.** Before the work ranking, print warnings:
  `! 6 untriaged beads · ! 2 beads match multiple works · ! 1 bead deleted externally`.
  Spec coordination.md L157 already names `warning` as a render block — wire it.
- **Fix bare `kerf`'s `Total active works`.** Scope to the inferred project, or label it
  "across all projects" if intentional.
