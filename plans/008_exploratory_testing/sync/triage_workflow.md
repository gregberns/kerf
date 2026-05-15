# Triage Agent Workflow — Sketch

Greg's framing: "an agent dumps issues in; kerf surfaces them so they can be
organized, binned into works, and prioritized."

## Target loop (what a triage agent should be able to run)

```
1. kerf triage                  # what's new, what's orphaned, what drifted
2. kerf areas list              # what bins already exist
3. kerf new <codename> \         # create a work and attach matching beads
        --jig implementation \
        --bead-filter 'label=<L>'
   # or:
4. kerf work edit <codename> --bead-filter-add 'id_prefix=hk-cb'
5. kerf next                    # confirm queue and priority after binning
6. kerf triage --resolved       # confirm no stragglers remain
```

Steps 1, 3 (`--bead-filter` flag), 4 (`work edit`), and 6 do not exist today.

## Phase-by-phase mapping to what kerf gives you now

| Triage phase | Today | Missing |
|--------------|-------|---------|
| **Discover** new/orphan beads | Nothing. Agent must shell out to `bd list --json` and reconcile against `spec.yaml` files itself. | `kerf triage` report: untriaged, multi-matched, externally-changed, deleted. |
| **Decide** which work each bead belongs to | `kerf areas list` and `kerf map` give the work skeleton, but neither says which beads are in scope. | `kerf show <codename>` should render attached beads. `kerf next --explain` should show why a bead landed (or didn't) in each work. |
| **Bin** beads into works (mutate filters / labels) | Hand-edit `spec.yaml`, or use `bd label add` and hope a filter picks it up. | `kerf new --bead-filter`, `kerf work edit --bead-filter-{add,remove}`, `kerf attach <codename> <bead-id>` (explicit pin). |
| **Prioritize** | `kerf next` ranks works by `creation order` + jig pass; spec mentions `momentum` and `rework` weights but they require attached beads to compute. | Compute and display per-work `momentum`, `rework`, open-bead count, blocked-bead count — none surfaces today. |
| **Confirm clean** | No "everything is binned" signal. | `kerf triage` exit code: 0 = clean, non-zero = drift. Lets the triage agent run idempotently in CI / a loop. |

## Where kerf already helps

- `kerf init` writing `.kerf/project-identifier` is the right anchor for "this repo is the
  triage target" — once project resolution works (see ingestion finding #3).
- `kerf map` gives the area-grouped portfolio view; adding bead counts per row would make it
  a real triage dashboard with no new command.
- The `bead_filter` schema in `specs/works.md` (any/label/id_prefix) is expressive enough — the
  problem is exclusively surfacing and CLI ergonomics, not the data model.
- `specs/coordination.md` already names `warning` as a render block in `kerf next` (L157) — the
  hook for surfacing drift exists in the spec, just not in the code.

## Where kerf leaves the agent guessing

1. **"Is there anything to triage?"** — no answer without shelling out to `bd`.
2. **"Which work owns this bead?"** — no command maps bead → work.
3. **"Did anything change since I last looked?"** — no state to diff against.
4. **"How do I attach beads to a work I just made?"** — must hand-edit YAML.
5. **"Is the queue stable?"** — `kerf next` looks fine even when 6 beads are orphaned.

## Biggest single piece missing

A `kerf triage` (or `kerf next --triage`) command that, in one call, prints:

```
Untriaged: 6 beads
  research(5): hk-2d3 hk-... (suggest: new work or attach to existing)
  unbinned(1): hk-79u

Multi-matched: 0 beads
External changes since last triage:
  closed: hk-3i2 hk-7a3
  deleted: hk-ze5
  reopened: hk-3i2
  new: hk-79u

Per-work bead health:
  backend-cleanup  6 open / 0 closed   (filter: label=backend)
  ui-polish        6 open / 0 closed   (filter: label=ui)
  docs-pass        6 open / 0 closed   (filter: label=docs)
  bug-triage       6 open / 0 closed   (filter: label=bug)
```

With that one report plus a `--bead-filter` flag on `kerf new` and a
`kerf work edit --bead-filter-...` mutator, the triage loop closes end-to-end.
