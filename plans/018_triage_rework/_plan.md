# Plan 018 — Triage Rework

> **Status: baked.** Spawned from Plan 015 (harmonik beta-feedback triage). Theme 4 (triage output + suggestions), with two cross-cutting items pulled in where they share a code path.

## Intent

`kerf triage` is the "what's wrong with this project today" surface — the command an agent leans on to figure out what to fix next. Today it dumps unbounded output (168 items on the harmonik dogfood; that would explode on a 10k-bead project), emits low-quality `kerf new ...` suggestions seeded from cross-cutting label families (`axis:`, `tag:`, `kind:`, `scope:`), ignores archive state (suggests `kerf new imrest` for an archived codename), re-prints the whole report when the agent passes `--ack`, and lets `--kind=multi_matched` with zero matches still emit the full header. This plan tightens triage so an agent can route off it without grepping past noise.

## Background

The verification block in `plans/015_harmonik_beta_feedback/triage.md` confirmed at commit 48aad35 (HEAD `d814f46`) that the suggester logic in `cmd/triage.go:451-481` is unchanged since 2026-05-15 and does not consult archive state or rank label prefixes. The triage spec lives in `specs/commands.md` lines 1660-1823 and already names the three item kinds (`untriaged`, `multi_matched`, `external_drift`); this plan tightens shape, suggester quality, and lifecycle docs without changing the kind taxonomy or the `--ack` data model.

Translation glossary (used throughout):
- **triage items** — entries in the report: `untriaged`, `multi_matched`, `external_drift`.
- **suggester** — the code that emits the `suggest:` line under each `untriaged` item.
- **cross-cutting label** — a label prefix like `axis:`, `tag:`, `kind:`, `scope:` that groups beads orthogonally to work cohorts (rather than defining one).
- **cohort-defining label** — a label prefix like `codename:` or `spec:` that identifies a work cohort.
- **baseline** — the bead-store snapshot at `.kerf/sync-cache.json`, advanced only by `--ack`.

## Scope

In scope (all items absorbed below):
- Suggester refuses to seed new works from cross-cutting label families; prefers `codename:` and `spec:` prefixes.
- Suggester checks archive state and emits `(archived — consider unarchive or re-pin)` instead of `kerf new <archived-name>`.
- `--ack` prints `Baseline advanced to <timestamp>` only; no full re-dump of the report.
- New `--top N` and `--group-by codename-label` flags to bound output for large projects.
- One canonical bead count per run — resolve the 163-vs-168 header discrepancy by either unifying or labeling each count with its filter.
- `--help` documents the baseline / delta / ack lifecycle so a first-time reader does not have to re-derive it.
- `--kind=multi_matched` (or any `--kind`) with zero matches prints `no <kind> items` and exits without the full report header.

Out of scope:
- The `--ack` data model itself (the on-disk shape of `.kerf/sync-cache.json` is correct).
- Ranking-algorithm changes (those belong to Plan 014 — process-management reframe).
- Drift detection itself — `kerf triage` already calls into shared drift code from `internal/drift/`; this plan changes presentation, not detection.
- Storage-reconciliation drift footer on `kerf next` (Plan 017).

## Design notes (high-level)

**Suggester routing.** Two-tier prefix ranking: tier-1 prefixes are cohort-defining (`codename:`, `spec:`) and may seed a `kerf new`; tier-2 prefixes are cross-cutting (`axis:`, `tag:`, `kind:`, `scope:`, plus any prefix that does not appear in tier-1) and never seed a `kerf new`. Note that prefixes seen in the wild like `subsystem:` or `area:` fall to tier-2 by default — tier-1 is a small explicit allow-list rather than a tier-2 deny-list, so the routing rule is conservative when an unfamiliar prefix appears. When all a bead's labels are tier-2, fall back to `kerf pin <codename> <bead-id>` against the lexicographically-earliest active work, or to "no auto-suggestion; investigate manually" when no work exists. Tier-1 list is a constant for v1 — see open question 1.

Alternative considered: a fully data-driven approach that ranks prefixes by entropy across the bead store. Rejected for v1 as harder to reason about; the static prefix list is enough for the failure modes Plan 015 surfaced.

**Archive check.** Before emitting `kerf new <codename>`, the suggester looks up `<codename>` in the archive index. If present, the suggestion becomes a re-pin hint plus an unarchive command pointer. The archive scan needs to be cheap (it runs once per `untriaged` item); precomputed once per triage invocation, not per item.

Alternative considered: archive scan inside the codename generator (so archived names never get proposed in the first place). Rejected because the codename generator already has many callers and a centralized check in the suggester is narrower.

**Bounding output.** `--top N` truncates per-section after sorting; the section header includes both shown and total counts (`Untriaged beads (showing 20 of 168):`). `--group-by codename-label` groups `untriaged` items by the first label whose prefix matches the tier-1 routing list (see suggester routing above); ties broken lexicographically. A grouped section emits one header per group with the items nested under it; the implementer chooses the exact rendering during B5. Truncation never hides `external_drift` items by default (those are the smallest section and the most actionable), unless the user passes `--top N` explicitly.

**`--ack` quiet mode.** The render step in step 7 of the spec behavior list runs only when `--ack` is absent. With `--ack`, the report is captured to the in-memory state but not printed; the single-line confirmation is what stdout sees. JSON output under `--ack --format=json` emits a one-record summary (`{ "baseline_advanced_at": "...", "items_captured": N }`) instead of the full item stream; this is the v1 decision, not a punt — open question 4 is retained only as a back-out path if a real consumer needs silent mode.

**Count reconciliation.** The 163 vs. 168 discrepancy traces to two callers using different status filters (one excludes `closed`, one does not). The fix is to disambiguate the counts in the header line (`163 open · 168 total`) rather than silently picking one — both numbers are accurate, the issue is that today they appear unlabeled in different places.

**Lifecycle in `--help`.** A new "Baseline lifecycle" paragraph in `kerf triage --help` walks through: first run shows `baseline: never`; subsequent runs without `--ack` show drift accumulating since the previous baseline; `--ack` after the agent investigates advances the baseline; the `--resolved` exit-code loop terminates when drift returns to zero.

## Spec changes proposed (prose)

- `specs/commands.md` (lines 1660-1823, the `kerf triage` section) — add `--top N` and `--group-by` flags; tighten the `--ack` "render then capture" step so render is skipped when `--ack` is set; restate the suggester routing rule (tier-1 vs. tier-2 prefixes); add archive-aware suggestion wording; clarify that empty-section output for `--kind X` is a single line, not the full header; extend `--help` text contract to cover the baseline lifecycle.
- `specs/coordination.md` — possibly a short subsection on the cohort-defining vs. cross-cutting label prefix lists, since the same routing rule could be reused by `kerf work edit --bead-filter-add` suggestions. Decision deferred to plan-implementation; the change might end up entirely in `commands.md`.
- `specs/cli.md` — the "every state-changing command emits next steps" invariant already covers `--ack`; this plan refines the corresponding "informational command output should be bounded" guideline. One sentence at most.

No spec edits in this plan; the lines above are proposed for the implementation phase.

## Beads outline (no `bd` yet)

- B1 — suggester: tier-1 vs. tier-2 prefix routing; refuse `kerf new` for cross-cutting labels.
- B2 — suggester: archive-aware codename check; emit re-pin hint.
- B3 — `--ack` quiet-mode rendering; `Baseline advanced to <ts>`.
- B4 — `--top N` flag with per-section truncation and shown-of-total header counts.
- B5 — `--group-by codename-label` flag for `untriaged` grouping.
- B6 — count-discrepancy fix: label each header count with its status filter.
- B7 — `--kind` with zero matches prints one line, suppresses report header.
- B8 — `--help` text update: baseline / delta / ack lifecycle paragraph.
- B9 — spec update covering B1–B8: primarily `specs/commands.md` (the `kerf triage` section), with a short prefix-routing note in `specs/coordination.md` and a one-sentence bounded-output guideline in `specs/cli.md` if the reviewer agrees they belong there. Single bead so the spec change lands as one diff.

Rough dep graph: B1, B2, B3, B4, B5, B6, B7 are independent on the implementation side; B9 sequences after the implementation beads it documents; B8 can run in parallel with B1–B7.

## Items absorbed from Plan 015

- 4.1 — suggester proposes `kerf new <axis-label>` for cross-cutting tags.
- 4.2 — suggester ignores archive state.
- 4.3 — `--ack` re-prints the full report.
- 4.4 — no `--top` / `--group-by` flags.
- 4.5 — bead count discrepancy (163 vs. 168).
- 4.6 — `baseline: never` semantics undocumented in `--help`.
- 4.7 — `--kind=multi_matched` with zero items prints full report header.

## Open questions

1. Is the cohort-defining prefix list (`codename:`, `spec:`) exhaustive, or should it be configurable per project (for example in `project.yaml`)? v1 default: hard-coded list; revisit if a real project surfaces a need.
2. For the archive-aware check, does the suggester read `~/.kerf/archive/` directly or consult a stored archive index? `internal/feed/cleanup.go:41-51,84` already exposes `feed.Inputs.ArchivedOrFinalized` — likely reusable.
3. Default `--top N` value when the flag is omitted: unlimited (current behavior, preserves agent-loop semantics) or a sensible default like 20? Leaning unlimited-by-default for back-compat; documented `--top 20` as the "first run on a large project" recipe in `--help`.
4. Should `--ack --format=json` emit a one-record `{ "baseline_advanced_at": "...", "items_captured": N }` summary, or stay silent (empty stream)? Leaning summary record so machine consumers have something to parse.
5. Does the prefix-routing rule belong in `specs/coordination.md` (shared with `kerf work edit` suggestion paths) or in `specs/commands.md` next to `kerf triage`? Defer to plan-implementation review.

## Conflicts with neighbor plans

Reviewed Plans 016, 017, 019, 020 stubs at commit 48aad35:
- 016 (init UX) — disjoint; touches `kerf init` output and `project.yaml` shape.
- 017 (storage reconciliation) — overlaps only on the drift footer surfacing on `kerf next` / `kerf triage`, which 017 explicitly owns. This plan does not touch the footer.
- 019 (filter bootstrap) — disjoint; touches `kerf work edit`, `kerf show`, `kerf next` rank labels.
- 020 (jig review gate) — disjoint; touches the spec jig and `kerf show`.

None observed beyond the 017 footer note, which is already disambiguated.
