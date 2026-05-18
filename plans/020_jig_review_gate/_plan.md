# Plan 020 — Jig Review-Gate and Pass-Loop Fixes

> **Status: stub.** Spawned from Plan 015 (harmonik beta-feedback triage). Expansion handled by the `plan-implementation` flow.

## Intent

The spec jig is kerf's flagship multi-pass authoring loop (problem-space → decompose → research → design → ... → ready). Today its pass instructions hard-require sub-agent primitives that aren't universally available — the Agent tool for reviewer dispatch, sub-agent file writes for pass-3 research — and when those primitives aren't present, the jig falls back to structurally weaker self-review. Pass instructions also ship no output templates, leave file-naming conventions ambiguous between full names and abbreviations, and don't pre-create the output directory for the next pass. This plan makes the jig harness-agnostic and template-driven so a fresh agent can walk the loop on any harness without manual workarounds.

## Background

Items come from `plans/015_harmonik_beta_feedback/triage.md` themes 7 (spec-jig pass loop) and 9 (command-UX gaps). The 2026-05-18 dogfood session did not re-exercise passes 1–4, so the items are assumed still live unless re-tested during implementation.

## Scope

- Review-gate fallback paths documented in jig instructions: fresh-context re-read and parent-orchestrator review named explicitly, no assumption that the Agent tool is loaded.
- Pass-3 (research) instruction template tells the parent orchestrator to collect inline returns from research sub-agents and own the write step — sub-agents return text, parent persists.
- New `kerf review <codename>` command (or explicit acknowledgment that the harness owns the primitive and the jig wires to whatever is available).
- Per-pass output templates shipped with the jig (`01-problem-space.md.template`, `02-components.md.template`, and siblings).
- `kerf show` prints the canonical pass filename in a stable location (`Pass N: <name> → Output: NN-<filename>.md`).
- Deduplicate the "What done looks like" and "Review Criteria" blocks — one normative source per pass.
- Pass-N status advance creates the pass-N output directory (no manual `mkdir -p`).
- Declare the "one design decision per file" convention (or its aggregate alternative) in pass-4 instructions.
- `kerf preview <next-status>` to peek at the next-pass instructions without advancing.
- `kerf show --compact` mode (status + next-pass name + file count + last-session marker only).
- `kerf status --quiet` for scripted transitions.
- Out of scope: rewriting the spec jig's pass topology; new jigs (this plan tightens the existing spec jig).

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

## Specs likely touched

- `specs/jig-spec.md` — review-gate fallback paths; pass-3 write ownership; templates; file-naming; "one design decision per file"
- `specs/jig-system.md` — harness-agnostic reviewer primitive; pre-create pass output directory on status advance
- `specs/commands.md` — `kerf review`, `kerf preview`, `kerf show --compact`, `kerf status --quiet`
- `specs/cli.md` — quiet / compact / preview conventions across commands

## Open questions

- Does `kerf review <codename>` ship a canned reviewer prompt that the harness then executes, or does it just emit the prompt text for the orchestrator to dispatch?
- One design decision per file vs. aggregated `04-design/design.md` — which is the default, and is it configurable per-project?
- For per-pass templates: do they live alongside the jig spec in `specs/jig-spec.md` as embedded blocks, or as separate template files the binary ships?
- Does `--quiet` apply only to `kerf status`, or is it a global convention any command can opt into?
