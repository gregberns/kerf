# Plan 018 — Triage Rework

> **Status: stub.** Spawned from Plan 015 (harmonik beta-feedback triage). Expansion handled by the `plan-implementation` flow.

## Intent

`kerf triage` is the "what's wrong with this project today" surface — the command agents lean on to figure out what to fix next. Today it dumps unbounded output (168 entries on harmonik, would explode on a 10k-bead project), emits low-quality suggestions that propose seeding new works from cross-cutting label families (`axis:`, `tag:`, `kind:`, `scope:`), ignores archive state (suggests `kerf new imrest` when `imrest` is archived), and re-prints the full report when the agent passes `--ack`. This plan tightens triage so agents can use it for routing without grepping past noise.

## Background

Items come from `plans/015_harmonik_beta_feedback/triage.md` theme 4 (triage output + suggestions). Several items are flagged "appears possibly fixed — verify before action" in the source triage; verification is part of this plan's first pass.

## Scope

- Suggester refuses to seed new works from `axis:`, `tag:`, `kind:`, `scope:` label prefixes; prefers `codename:` and `spec:`.
- Archive-aware suggestions: emit `(archived)` and a re-pin hint instead of `kerf new <archived-name>`.
- `--ack` prints `Baseline advanced to <timestamp>` only (no full re-dump of the report).
- `--top N` and `--group-by codename-label` flags to bound output.
- One canonical bead count per run (resolve the 163-vs-168 discrepancy by stating each header's filter or unifying).
- `--help` documents the baseline / delta / ack lifecycle so first-time readers learn it without re-deriving.
- `--kind=multi_matched` with zero items prints `no multi_matched items` and exits; no full report header.
- Out of scope: the `--ack` data model itself, which is correct; the ranking algorithm changes that belong to Plan 014.

## Items absorbed from Plan 015

- 4.1 — suggester proposes `kerf new <axis-label>` for cross-cutting tags
- 4.2 — suggester ignores archive state
- 4.3 — `--ack` re-prints the full report
- 4.4 — no `--top` / `--group-by` flags
- 4.5 — bead count discrepancy (163 vs 168)
- 4.6 — `baseline: never` semantics undocumented in `--help`
- 4.7 — `--kind=multi_matched` with zero items prints full header

## Specs likely touched

- `specs/commands.md` — `kerf triage` flags, output shape, `--ack` behavior
- `specs/coordination.md` — possibly, if the suggester rules live near label-prefix conventions
- `specs/cli.md` — help-text conventions for lifecycle commands

## Open questions

- Which label prefixes are "cohort-defining" vs. "cross-cutting"? `codename:` and `spec:` clearly cohort; `axis:`, `tag:`, `kind:`, `scope:` clearly cross-cutting — but is the list exhaustive or should it be user-configurable?
- For archive-aware suggestions, does the suggester check `~/.kerf/archive/` by codename, or by a stored archive index?
- Default `--top N` value when the flag is omitted: unlimited (current behavior) or a sensible default (say 20)?
- Verify which of 4.1 / 4.2 are still live before implementation — the 2026-05-18 entry in the source log suggests some were resolved by an intermediate release.
