# Plan 006 — Flexible Bead Attachment & Actionable `kerf next`

## Intent

Two changes:

- **A. Flexible bead-to-work attachment.** Replace the hardcoded `work:<codename>` label match with a configurable `bead_filter` (project-wide default + per-work override).
- **B. `kerf next` becomes a feed of actionable items, not just beads.** Beads are one kind of item. Cleanup tasks, warnings, and (later) PR signals are others. The user can filter by kind.

## Why

**A.** Real projects have their own bead conventions — harmonik uses `subsystem:*` labels and `hk-*` ID prefixes. Forcing `work:<codename>` means the scoring pipeline sees nothing and `kerf next` runs blind. The bead store should drive; kerf adapts.

**B.** Today `kerf next` only emits beads. But there are other things an agent should resolve: a work whose beads are all closed but whose jig walk is still owed, a `bead_filter` matching nothing, an open PR with comments. Modeling these as items in the same feed gives the agent one place to look and a filter to choose what to act on.

## A — `bead_filter`

A new optional field at project and per-work level. If neither is set, the built-in default is `label: "work:{codename}"`.

**Project-wide in `project.yaml`:**

```yaml
bead_filter:
  label: "subsystem:{codename}"
```

**Per-work override in `works/<codename>/spec.yaml`:**

```yaml
codename: bridge
bead_filter:
  any:
    - label: "subsystem:bridge"
    - label: "codename:claude-hook-bridge"
    - id_prefix: "hk-cb"
```

**Rules:**

- `any:` is a union. No `all:` in v1.
- Clause types: `label:` and `id_prefix:`.
- One template variable: `{codename}`. (Language-neutral; substituted at match time.)
- Case sensitive.
- Resolution per work: per-work `bead_filter` → project `bead_filter` → built-in default. First hit wins; filters do not merge.
- A bead matching N works counts for each.

### Onboarding

`kerf init` uses kerf's existing bead-read path (same as `kerf next`) to scan the store, tallies label prefixes with at least 3 beads, and scores each prefix `P:` as `match_score = (beads matching some codename via "P:{codename}") / (total beads with prefix "P:")`. Picks the highest match_score above 0.5 and proposes it:

```
Detected: 87% of beads use `subsystem:*` labels.
Set project-wide bead_filter to `subsystem:{codename}`? [Y/n]
```

If the store is empty or no codenames exist yet, skip detection. If no prefix scores above 0.5, fall back to a manual prompt offering the top 5 prefixes by raw count plus a "type your own" option. If the bead tool is unavailable, skip auto-detect silently. The result is written to `project.yaml` and is always editable later. Full algorithm lives in `specs/commands.md` under `kerf init`.

### Unmatched beads

Beads that match no work are surfaced in `kerf next` as a `warning` item (see B), not as an error.

## B — `kerf next` as a feed of actionable items

`kerf next` returns a ranked list. Each item has a **kind** and a **target**:

| kind | target | example |
|------|--------|---------|
| `bead` | a bead | today's behavior — work on this bead |
| `cleanup` | a work | "all beads closed; walk the jig or shelve" |
| `warning` | project-level | "12 beads match no work — check `bead_filter`" |
| `pr` (future) | a work | "PR open; review comments pending" |

### Filtering

```
kerf next                     # default: beads + cleanup, warnings shown once at top
kerf next --only=bead         # beads only (today's behavior)
kerf next --only=cleanup
kerf next --include=warning
kerf next --kinds=bead,pr
```

Default surfacing strategy:

- Show beads as today, ranked by the existing coordination scoring.
- Show cleanup items after all beads, sorted by their parent work's would-be bead score (descending). Cleanups do not mix into the bead ranking in v1 — keep cross-kind scoring simple. A work with all beads closed but status `problem-space` is still visible, just below ready beads.
- Show warnings as a header block, not as ranked items, since they're project-wide and not work-specific.

### How this replaces "effectively-complete" gating

The earlier design used a derived predicate to release dependency gates when all attached beads were closed. Replaced with this:

- **Dependency gating stays strict** on jig status. No derived predicates threading through `queue.go`.
- **The bead-done-but-status-stale case surfaces as a `cleanup` item** so the agent or user resolves it explicitly — walk the jig, or shelve the work. Either action unblocks downstream naturally.

This is simpler (no new gating logic), more honest (the user sees what's owed), and extensible (PR signals, doctor findings, lint warnings can all become items later without re-plumbing the queue).

### Item shape

A single struct, kind-tagged. Rough shape (Go field names; JSON emits snake_case — see `specs/commands.md` for the canonical JSON shape):

```
Item {
  Kind         string   // valid kinds come from the current build (v1: bead, cleanup, warning)
  Score        float64
  Title        string   // human one-liner
  Action       string   // suggested command or next step
  WorkCodename string   // omitted/null for project-level items
  BeadID       string   // omitted/null for non-bead items
  Reason       string   // why this surfaced
}
```

### Output

Default output is compact text, optimized for an agent reading it at the top of every cycle. Text is for humans and prose-reading agents — it is not a parsing contract. Agents that need stable structured output use `--format=json`.

```
$ kerf next
1. bead   hk-cb-042  "wire retry into adapter"        work: bridge
2. bead   hk-cb-051  "extract header parser"           work: bridge
3. clean  bridge      all beads closed; walk jig or shelve

run with --format=json for machine output, --help for filters
```

`--format=json` emits the full `Item` stream for automation. No other formats in v1.

### Help text (agent-facing)

Agents will run `kerf next` more than any other command. The `--help` output is the contract — it should make the loop self-teaching, so a fresh agent reads it once and knows everything it needs for the cycle.

`kerf next --help` must clearly state, in this order:

1. **What it returns**: a ranked feed of things to act on right now.
2. **The item kinds and what each means** (one line each): `bead` = work on this; `cleanup` = resolve this on a work; `warning` = project-level issue, fix config.
3. **The default action loop**: read top item → do it → re-run `kerf next`.
4. **The filter flags** with concrete examples: `--only=bead`, `--include=warning`, `--kinds=bead,cleanup`.
5. **Machine output**: `--format=json` for scripts.
6. **How scoring works in one sentence** + pointer to the spec for detail.

The text is part of the spec, not boilerplate — changes to it require a spec change.

### Cleanup item triggers (v1)

- `work_no_attached_beads` — work exists and resolved `bead_filter` matches zero beads (attached_count == 0). Suggested action: edit `spec.yaml` or check the project filter.
- `work_beads_done_status_open` — attached_count > 0, every attached bead is closed, and jig status is not terminal. Suggested action: `kerf status <codename> <next-stage>` or `kerf shelve <codename>`.

The two detectors are mutually exclusive by construction.

PR/lint/doctor items are explicitly out of scope for v1 but the kind enum and item shape are designed to absorb them later.

## Specs Affected

| Spec file | Change |
|-----------|--------|
| `specs/coordination.md` | `bead_filter` config + resolution. Replace any reference to `work:<codename>` with the resolved filter. |
| `specs/works.md` | Add `bead_filter` to per-work `spec.yaml`. |
| `specs/commands.md` | Rewrite `kerf next` to describe the item feed, kinds, and filter flags. Update `kerf init` to describe the auto-detect step. |
| `specs/dependencies.md` | No change — gating stays strict on jig status. |
| `specs/cli.md` | No change. |

## Implementation Notes

1. **`internal/beads/beads.go` carries A.** Add `Filter` struct and `Match(bead, codename)`. `ForWork` resolves the filter (per-work → project → default) and calls `Match`. Filter resolution is per-call; no caching.

2. **A new `internal/feed` package carries B.** Replaces the bead-only return path of `kerf next`. The feed is built from: (i) ranked beads from `internal/queue`, (ii) cleanup detectors that scan works, (iii) warning detectors that scan project state. Each detector returns zero or more `Item`s.

3. **Cleanup detectors are pure functions over current state.** No persistence, no caching. Called on every `kerf next`.

4. **Filter flags are parsed once in `cmd/next.go`.** Default to `bead,cleanup` for items, with warnings rendered as a header. JSON output emits everything unless `--only` narrows it.

5. **Auto-detect heuristic.** Use kerf's existing bead-read path; skip silently if unavailable. Tally label prefixes appearing on at least 3 beads. For each prefix `P:`, compute `match_score = (beads matching some existing codename via "P:{codename}") / (beads with prefix "P:")`. Pick the highest match_score above 0.5; fall back to a top-5 by raw count plus "type your own" if no candidate qualifies. Skip auto-detect entirely if zero codenames exist yet.

6. **Sequencing.** A first (filter + resolution + tests). Then the feed scaffolding (`Item`, kinds, render, JSON). Then the two v1 cleanup detectors. Auto-detect on `kerf init` last — easiest to defer if scope grows.

7. **Tests.**
   - A: filter resolution truth table; `any:` union; template substitution; multi-work matches.
   - B: feed assembly with mixed kinds; filter flag behavior; JSON shape stable across kinds; cleanup detectors fire and clear correctly as state changes.

## Implementation Beads

See [beads.md](beads.md) for the full implementation task breakdown — 9 beads across 4 layers, with dependency graph and parallelization plan. Beads are tracked in `bd`; see [/plans/bead-id-map.md](../bead-id-map.md) for the bd ID mapping.
