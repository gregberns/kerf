# Plan 020 — Jig Review-Gate and Pass-Loop Fixes

> **Status: baked.** Spawned from Plan 015 (harmonik beta-feedback triage). Theme 7 (spec-jig pass loop) plus three command-UX items from theme 9 that share the same surface.

## Intent

The spec jig is kerf's flagship multi-pass authoring loop (`problem-space → decompose → research → change-design → spec-draft → integration → tasks → ready`). Today its pass instructions assume sub-agent primitives that aren't universally available — the Agent tool for reviewer dispatch, sub-agent file writes for pass-3 research — and when those primitives aren't present, the jig silently falls back to weaker self-review or the orchestrator has to improvise. Pass instructions also ship no output templates, leave file-naming conventions ambiguous between full names and abbreviations, duplicate the "what done looks like" block as both prose and a checklist, and don't pre-create the next pass's output directory. This plan makes the jig harness-agnostic and template-driven so a fresh agent can walk the loop on any harness without manual workarounds.

## Background

All items come from `plans/015_harmonik_beta_feedback/triage.md`:

- Theme 7 (spec-jig pass loop): items 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7, 7.8.
- Theme 9 (command-UX gaps), the three items that touch the spec jig's command surface: 9.6 (`kerf status --quiet`), 9.7 (`kerf preview <next-status>`), 9.8 (`kerf show --compact`).

The 2026-05-18 dogfood verification block in `triage.md` confirms every theme-7 sub-item is still live in `internal/jig/builtin/spec.md` and `cmd/show.go` as of commit `48aad35`.

## Scope

In scope:

- Document fallback paths for the review gate inside the jig's markdown body — fresh-context re-read and parent-orchestrator review named explicitly, no assumption that the Agent tool is loaded by the harness.
- Pass-3 (research) instructions tell the parent orchestrator to collect inline returns from research sub-agents and own the write step. Sub-agents return text; parent persists.
- New `kerf review <codename>` command that emits the canonical reviewer prompt for the work's current pass — the harness then dispatches whatever reviewer primitive it has.
- Per-pass output templates shipped with the jig binary (`01-problem-space.md.template`, `02-components.md.template`, and siblings for the remaining content passes).
- `kerf show` prints the canonical pass filename in a stable location: one line per pass, `Pass N: <name> → Output: NN-<filename>.md`.
- Collapse "What done looks like" and "Review Criteria" into one normative block per pass — `Done when reviewer approves on:` followed by the criteria list.
- Pass-N status advance creates the pass-N output directory if it doesn't exist (no manual `mkdir -p` from the agent).
- Declare a default "one design decision per file" convention in pass-4 instructions, with a noted aggregate alternative for very small changes.
- New `kerf preview <codename> <next-status>` to read the next-pass instructions without advancing.
- `kerf show --compact` — status + next-pass name + file count + last-session marker.
- `kerf status --quiet` for scripted transitions.

Out of scope:

- Rewriting the spec jig's pass topology or status list.
- Authoring new jigs.
- Touching the `plan`, `bug`, `implementation`, `retrofit`, or `spike` jigs except where the review-gate fallback paths and templates pattern is generalized via `specs/jig-system.md`.
- The reviewer-prompt content itself for non-spec jigs (this plan ships the spec-jig prompts; other jigs adopt the pattern as a follow-up).

## Design notes

**Review-gate fallback paths.** Today `specs/jig-system.md` §Review Pattern says "spawn a review sub-agent with fresh context." Three concrete primitives can satisfy that contract, and the jig spec should name all three so any harness has a path:

1. Harness Agent tool (today's default).
2. Parent-orchestrator review — the orchestrator that dispatched the pass-execution sub-agent reads the artifact and applies the review criteria itself.
3. Fresh-context re-read — the same agent compacts its own context and re-reads the artifact alongside the criteria.

Listed in preference order. The jig spec describes them once; the per-jig markdown bodies reference the pattern rather than re-stating it.

**Reviewer primitive: `kerf review` vs. tightening the template alone.** Two alternatives considered:

- *Tighten the template only.* Edit `internal/jig/builtin/spec.md` so each pass's review block lists the fallback paths inline. Cheaper, but every jig has to repeat the same block, and the harness still has to know which review criteria apply to which pass.
- *Ship `kerf review <codename>`.* Emits the reviewer prompt for the work's current pass — criteria, artifact paths, prior-pass references. The harness dispatches whatever reviewer primitive it has against that prompt. Single source of truth in the jig markdown; the command surface stays harness-agnostic.

The plan takes the `kerf review` route. Item 7.6 (review-criteria duplication) is resolved at the source: today the jig markdown body carries both a "What done looks like" prose block and a "Review Criteria" checklist per pass, and `kerf show` surfaces both. Collapsing to a single "Done when reviewer approves on:" block in the jig markdown fixes the duplication everywhere it appears downstream — `kerf show`, `kerf review`, and the on-disk jig file.

**Pass-3 write ownership.** Item 7.2's root cause is that some harnesses block `.md` writes from sub-agents. The fix is editorial: the pass-3 instructions tell the parent to collect inline returns from each research sub-agent and own the write step, and the example output template names the parent as the writer. No mechanism change — kerf doesn't track who held the pen, only that the file exists.

**Pre-creating pass directories.** `kerf status <codename> <next>` already advances the status; it doesn't currently touch the filesystem beyond `spec.yaml`. The fix is to call into the resolved jig's pass list, look up the next pass's output paths, and `mkdir -p` any directory prefix that doesn't yet exist. Idempotent; safe to re-run. Note the non-trivial case is pass-3 (`03-research/{component}/findings.md`), where one directory per affected spec area must be created from the `02-components.md` output — the implementation reads the components list and creates each subdirectory; if components haven't been enumerated yet, only `03-research/` itself is pre-created.

**Review-gate coverage across passes.** Triage 7.1 named passes 2 and 4 but `internal/jig/builtin/spec.md` carries Review Criteria blocks on passes 2, 4, 5, 6, and 7. All of them get the same collapse + fallback-paths treatment; the per-pass edits in beads outline #3 cover the full set, not just the two named in the triage entry.

**Filename surfacing.** `kerf show` already prints pass metadata. Adding a stable `Output:` line per pass is a one-line change in the formatter and resolves item 7.5. The jig spec captures the convention (`NN-<short-name>.md` for content passes) so future jigs match without re-deriving it.

**Templates.** Per-pass templates live in the binary alongside `internal/jig/builtin/spec.md`. `kerf show` references them by path; `kerf new` and `kerf status` advance copy the template into place when the pass directory is created (idempotent — only writes if the file is absent). Templates are skeletons, not boilerplate — headings and `<TODO: ...>` markers, not narrative prose.

**Compact / preview / quiet surfaces.** Three small flags / commands; mostly formatter work in `cmd/show.go` and `cmd/status.go`. `kerf preview <codename> <next-status>` reuses the same renderer as `kerf show`, scoped to a non-current pass and marked read-only in the header.

**Relation to existing jig phases.** No status-list changes; no pass renames; no new pass. Every change lands in either (a) the jig markdown body, (b) the jig-system spec's review-pattern section, or (c) command surfaces around `kerf show` / `kerf status` / a new `kerf review` / `kerf preview`.

## Spec changes proposed (prose)

- `specs/jig-system.md` — extend the Review Pattern section to name the three fallback primitives in preference order and the role of `kerf review`. Add a brief subsection on pass-directory pre-creation as part of status advance. Note the `Pass N: <name> → Output: NN-<filename>.md` convention for `kerf show` output.
- `specs/jig-spec.md` — pass-3 instructions describe the parent-owns-the-write pattern explicitly. Pass-4 instructions declare the per-decision-file convention as default. Each pass's "What done looks like" + "Review Criteria" pair collapses into a single "Done when reviewer approves on:" list. Each pass references the per-pass template by name.
- `specs/commands.md` — add `kerf review <codename>` (emits the current pass's reviewer prompt), `kerf preview <codename> <status>` (read-only render of a future pass), `kerf show --compact`, `kerf status --quiet`. `kerf status` documents the pass-directory pre-creation behavior.
- `specs/cli.md` — note the `--quiet` and `--compact` conventions so other commands can adopt the same flag names without re-deriving the shape.

No new spec files. No changes to `architecture.md`, `works.md`, or `verification.md`.

## Beads outline

Sequenced as far as dependencies require; siblings can run in parallel.

1. Extend `specs/jig-system.md` Review Pattern with the three fallback primitives and `kerf review` reference. (Spec edit.)
2. Add the pass-directory pre-creation paragraph to `specs/jig-system.md` and the corresponding entry under `kerf status` in `specs/commands.md`. (Spec edit.)
3. Edit `specs/jig-spec.md`: collapse the per-pass "what done" + "review criteria" blocks into a single normative block; pass-3 parent-owns-write rewrite; pass-4 per-decision-file convention. (Spec edit; depends on #1.)
4. Spec out per-pass templates in `specs/jig-spec.md` — name them, define the skeleton sections each must contain, point at the binary location. (Spec edit; depends on #3.)
5. Spec out `kerf review <codename>` in `specs/commands.md` — flags, output shape, exit codes. (Spec edit; depends on #1.)
6. Spec out `kerf preview <codename> <status>` in `specs/commands.md`. (Spec edit.)
7. Spec out `kerf show --compact` and `kerf status --quiet` in `specs/commands.md`; note the convention in `specs/cli.md`. (Spec edit.)
8. Spec out the `Pass N: <name> → Output: NN-<filename>.md` line in `kerf show` output, in `specs/commands.md`. (Spec edit; independent of #4 — filename surfacing reads the jig's pass list, not the templates.)
9. Implement: extend `internal/jig/builtin/spec.md` markdown body for the per-pass changes from #3, #4, #5. (Code; depends on #3, #4, #5.)
10. Implement: per-pass template files under `internal/jig/builtin/templates/` (or equivalent), embedded into the binary. (Code; depends on #4.)
11. Implement: `kerf review` command. (Code; depends on #5, #9.)
12. Implement: `kerf preview` command. (Code; depends on #6.)
13. Implement: `kerf show --compact` flag and the `Output:` line. (Code; depends on #7, #8.)
14. Implement: `kerf status --quiet` flag and pass-directory pre-creation + template copy on status advance. (Code; depends on #2, #7, #10.)
15. Tests: unit + golden output for the new flags and the `kerf review` prompts; regression test confirming pass-directory pre-creation is idempotent. (Tests; depends on #11–#14.)

## Items absorbed from Plan 015

- 7.1 — reviewer-sub-agent assumes Agent tool availability
- 7.2 — pass-3 research tells sub-agents to write files
- 7.3 — no `kerf review <codename>` command
- 7.4 — pass-1 ships no output template
- 7.5 — pass file-naming convention not surfaced by `kerf show`
- 7.6 — review criteria duplicated in `kerf show`
- 7.7 — `04-design/` not pre-created on pass-4 entry
- 7.8 — no convention for "one design decision per file" vs. monolithic
- 9.6 — `kerf status --quiet` flag
- 9.7 — `kerf preview <next-status>`
- 9.8 — `kerf show --compact`

## Open questions

- Does `kerf review <codename>` print the reviewer prompt to stdout for the harness to pipe into whatever reviewer it has, or does it dispatch directly when an Agent tool is detected? Defaulting to print-only keeps it harness-agnostic, but means harnesses with native dispatch have to wire the glue themselves.
- Per-pass templates: embedded in `internal/jig/builtin/spec.md` as code-fenced blocks, or shipped as sibling `.md.template` files? Sibling files make `kerf jig show` cleaner; embedded blocks keep the jig as one self-contained artifact. Leaning sibling files.
- Is `--quiet` a `kerf status`-only flag, or a global convention any state-changing command can opt into? If global, the convention belongs in `specs/cli.md`; if local, only `specs/commands.md` needs the entry.
- Should pass-directory pre-creation also copy the template into place, or only create the directory and leave the agent to invoke a separate "scaffold" step? Leaning copy-on-advance — fewer steps for the agent, idempotent if the file already exists.
- The per-decision-file pass-4 convention defaults to one file per decision. Should `spec.yaml` carry an optional `pass4_aggregate: true` override for projects that want a single `04-design/design.md`? Leaning yes, but small enough that it can fall out of implementation rather than be specified up front. If it does become a config field, coordinate with Plan 016's `project.yaml` shape work.

## Conflict check vs. Plans 016–019

No overlap observed. Plans 016 (init UX), 017 (storage / doctor), 018 (triage), 019 (filter bootstrap) all touch `cmd/init.go`, `cmd/triage.go`, `cmd/next.go`, `cmd/show.go`, and the bench/storage surface. Plan 020 touches the jig markdown body (`internal/jig/builtin/spec.md`), the jig-system spec, and adds `kerf review` / `kerf preview` + `--compact` / `--quiet` flags — disjoint command surfaces. The one near-touch is `cmd/show.go` (Plan 019 adds the `bead_filter:` slot; Plan 020 adds the `Pass N → Output:` line and `--compact` mode). These are independent renderer changes and will not collide if landed in either order.
