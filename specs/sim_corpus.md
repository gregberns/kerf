# Simulator Corpus Ingestion

> Status: DRAFT. Companion to Plan 012 pillar A.

This spec describes how the kerfsim toolchain converts real-world bead
and work definitions into scenario YAMLs that the existing
[kerfsim run](simulator.md) loop can execute.

The simulator's native scenario format is small and synthetic-friendly.
Real workloads live in two places today:

1. **Harmonik pilot YAMLs.** Eight files under
   `~/github/harmonik/docs/decompose-to-tasks/*-pilot-data.yaml`. Each
   one is one pilot (`cp`, `hc`, `on`, `pl`, `rc`, `sh`, `wm`, `meta`)
   carrying a `beads:` list, an `edges:` list, and a rich body of
   discipline / cite / forward-defer metadata.
2. **Kerf plan directories.** `plans/NNN_name/{_plan.md,beads.md}`. The
   `beads.md` markdown format is not yet machine-readable in a stable
   schema; Phase 1 stubs this code path.

The `kerfsim import` subcommand reads either source and emits a single
scenario YAML.

## Source contract — harmonik pilot YAML

The importer reads only these fields, dropping everything else:

| Field             | Meaning                                                          |
|-------------------|------------------------------------------------------------------|
| `epic.mnem`       | Pilot prefix (e.g. `cp`). Becomes the scenario work's codename.  |
| `beads[].mnem`    | Bead identifier. Used only to count beads.                       |
| `beads[].kind`    | One of `req`, `invariant`, `schema`, `error-taxonomy`, `test-infra`, ... Becomes one of the work's areas. |
| `edges[].from`    | Citing bead's mnem.                                              |
| `edges[].to`      | Prerequisite bead's mnem.                                        |

All other harmonik fields (`description`, `req:` cite lists,
`extra_labels`, `cite:*` tags, the `forward:*` deferred placeholders,
the `epic.description`, etc.) are dropped. The importer's job is to
extract structural workload shape, not to preserve harmonik semantics.

## Granularity

**One pilot YAML becomes one `work` in the output scenario.** Per-bead
detail is collapsed to a `bead_count`; per-kind detail is collapsed to
an unordered, deduplicated `areas` list.

If `<source>` is a directory, every `*-pilot-data.yaml` file in that
directory becomes one work. If `<source>` is a single file, the output
scenario contains exactly one work.

## Cross-pilot dependencies

The harmonik `edges:` block carries edges in both intra-pilot
(`cp-001 → cp-004`) and cross-pilot (`cp-001 → ar-039`) form. The
importer:

1. Drops the `forward:` placeholders (they have no destination bead).
2. Drops edges whose `to` prefix is not a pilot present in the import
   batch (e.g. an `em-*` edge when `em-pilot-data.yaml` is not on
   disk).
3. Aggregates the surviving cross-pilot edges to work-level deps:
   pilot `cp` depends on pilot `ar` iff any `cp-*` bead has an edge to
   any `ar-*` bead.

The result frequently produces cyclic work-level dependencies because
harmonik pilots cite each other in both directions at the bead level
(e.g. `cp` cites `wm` for a hook contract; `wm` cites `cp` for a
control-point definition). The simulator requires a DAG, so the
importer **greedily breaks cycles**: candidate work-level edges are
evaluated in `(codename, dep)` lexicographic order and an edge is
accepted iff it does not close a cycle in the already-accepted
subgraph. Dropped edges are reported in the import notes.

## Defaults for non-workload fields

`kerfsim import` does not attempt to fit a duration distribution or
arrival generator. The output scenario carries placeholders:

| Field                                 | Default                              |
|---------------------------------------|--------------------------------------|
| `seed`                                | `42`                                 |
| `ticks`                               | `5000`, scaled to `total_beads * 25` for large imports |
| `agents`                              | `4`                                  |
| `bead_arrivals.generator.rework_rate_per_tick` | `0.001`                     |
| `bead_arrivals.generator.target_works` | First half of the works              |
| `agent_model.duration`                | `{kind: lognormal, median_ticks: 25, sigma: 0.9}` |

A header comment in the emitted YAML notes that the duration and
arrival blocks should be replaced by Plan 012 pillar B fitted
distributions before drawing scoring conclusions.

## CLI

```
kerfsim import <source> --out <scenario.yaml>
```

`<source>` is auto-detected:

- A file ending in `-pilot-data.yaml` is treated as a harmonik pilot.
- A directory containing any `*-pilot-data.yaml` is treated as a
  harmonik pilot batch.
- A directory containing `_plan.md` is treated as a kerf plan (NOT YET
  IMPLEMENTED — returns an error).

`--out` is required and names the path the emitted scenario YAML is
written to. The command validates the result through
`scenario.Validate` before writing.

## Known limitations (Phase 1)

- **Generator dep collisions.** The kerfsim generator currently draws
  random "older sibling" edges for any work with an empty `deps:`
  list (other than the index-0 work). When a corpus contains multiple
  dep-less works, those random draws can collide with explicit deps
  on other works and produce a cycle. Single-pilot imports are
  unaffected. The directory case may fail `kerfsim run` with
  `generate: generator: dag: cycle detected` until the generator
  treats explicit `deps: []` as authoritative; tracked separately.
- **Kerf plan ingestion** is stubbed and returns an error.
- **`bd` export JSON** ingestion is not yet implemented.
