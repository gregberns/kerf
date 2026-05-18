# Plan 016 — Init UX Overhaul

> **Status: stub.** Spawned from Plan 015 (harmonik beta-feedback triage). Expansion handled by the `plan-implementation` flow.

## Intent

`kerf init` is the first command a fresh agent runs against a new project, so its output sets the agent's mental model for everything that follows. Today it issues an interactive prompt with no escape hatch (agent harnesses can't answer prompts), claims to have set fields that never land in `project.yaml`, prints two overlapping `AGENT SETUP INSTRUCTIONS` blocks, omits the daily-driver commands (`kerf next`, `kerf triage`, `kerf pin`, `kerf map`, `kerf areas`, `kerf work edit`), and never mentions that the authoritative state lives on the bench (`~/.kerf/projects/<id>/`) rather than in the repo. The plan reworks init so a fresh-context agent runs it once, reads the output, and ends up with an unambiguous, complete project setup that needs no manual repair.

## Background

All items come from `plans/015_harmonik_beta_feedback/triage.md` themes 1 (init / first-run UX) and 9 (command-UX gaps). The harmonik tester (Claude Opus 4.7) bootstrapped a fresh kerf install on the harmonik repo over 2026-05-15 → 2026-05-18 and logged init as the single biggest source of friction.

## Scope

- Non-interactive default behavior; introduce `--yes` / `--no` flags; keep `--force` distinct from both.
- Single state-change summary per run (created / updated / unchanged for each artifact); stop printing `Set default_jig: spec` or `Created project.yaml` when nothing actually changed.
- Repair the label-prefix detector to sample the current `.beads/issues.jsonl` (not stale data); stay silent when confidence is low.
- Collapse the two `AGENT SETUP INSTRUCTIONS` blocks into one canonical source.
- Add `kerf next`, `kerf triage`, `kerf pin`, `kerf map`, `kerf areas`, `kerf work edit` to the instruction text.
- Mention the bench location (`~/.kerf/projects/<id>/`) and `kerf localize` in the instruction block.
- Ensure `default_jig` and pass-schedule fields advertised by init actually land in `project.yaml`.
- Out of scope: storage reconciliation logic itself (Plan 017), triage rework (Plan 018), filter bootstrap (Plan 019).

## Items absorbed from Plan 015

- 1.1 — non-interactive default + `--yes` / `--no` flags
- 1.2 — single state-change summary (no "lying about state")
- 1.3 — stale `kerf:*` label-prefix detector
- 1.4 — `default_jig` claim must match `project.yaml` shape
- 1.5 — duplicate `AGENT SETUP INSTRUCTIONS` blocks
- 1.6 — instruction text missing current-generation commands
- 1.8 — `project.yaml` shape matches init's claims
- 9.1 — `--yes` / `--no` flags (same surface as 1.1)

## Specs likely touched

- `specs/commands.md` — `kerf init` flags and output shape
- `specs/cli.md` — output philosophy (single state-change summary)
- `specs/architecture.md` — `project.yaml` schema if fields are added or moved
- Agent-setup instruction template (location TBD during plan-implementation)

## Open questions

- Should `--yes` / `--no` apply only to the bead_filter prompt, or to all future interactive prompts as a global convention?
- When the label-prefix detector has low confidence, does it stay silent or print a one-line "couldn't determine" note?
- Does the canonical instruction block live in a single source file the binary embeds, or in a doc the binary points to?
