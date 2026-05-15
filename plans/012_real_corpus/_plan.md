# Plan 012 — Real-Workload Corpus for the Simulator

> **Status: DRAFT.** Builds the "real data" half of the simulator that was deferred from Plan 007 ("Phase 2/3" in the May 14, 2026 design proposals). Independent of Plan 011 and may run in parallel.

## Intent

The kerf simulator (Plan 007) ships with synthetic scenarios and a placeholder lognormal duration distribution. We want to drive it with real workloads from kerf's and harmonik's actual development history, so weight tuning and policy comparisons reflect how AI agents actually work, not how a hand-authored YAML imagines they do.

Two distinct ingredients, two distinct sources:

1. **Workload shape** — what depends on what, how many beads per work, area touch sets. Source: harmonik's bead YAMLs at `~/github/harmonik/docs/decompose-to-tasks/*.yaml` (eight pilots: ar, bi, cp, ev, em, hc, on, pl, rc, sh, wm) plus kerf's own `plans/006_*` through `plans/009_*` history.
2. **Per-phase durations** — how long real beads actually take across distinct phases (spin-up, task work, reviewer round-trip, merge, conflict). Source: Claude Code session transcripts at `~/.claude/projects/-Users-gb-github-{kerf,harmonik}/`.

## Why

- Weight tuning against unrealistic synthetic distributions is worthless. Plan 008 already showed kerf's rework metric is structurally zero in most synthetic scenarios — partly a metric bug (fixed in Plan 011 pillar E) and partly because synthetic durations don't generate enough saturation pressure for rework to queue.
- The May 14, 2026 design proposals (`SIM-PROPOSALS-2026-05-14.md`, reconstructed in `source/sim_proposals_reconstruction.md`) explicitly named a fourth canned scenario `real-export.yaml`, plus a Phase 2/3 roadmap entry: "Claude-log-derived duration distributions (extract bead-to-timestamp pairs from session logs, build empirical distributions, scale wall-clock to ticks)." That work was deferred and never returned to.
- Tier 1 and Tier 2 of the May 14 framework already proved harmonik bead YAMLs can be loaded into a kerf-shaped bead store and produce meaningful scoring output. A scratch Python loader processed 234 beads across four pilots. This plan formalizes that path.

## Pillars

### A — Bead-shape ingestion (`kerfsim import`)

**Goal:** convert a directory of harmonik-style bead YAMLs (or a kerf `plans/NNN_*/` plan) into a simulator scenario YAML that the existing `kerfsim run` can execute.

**Approach:**
- New subcommand `kerfsim import <source> --out <scenario.yaml>`.
- Source detection: YAML directory (harmonik pilots), kerf plan dir, or `bd` export JSON.
- Output: scenario YAML with `works:` populated from the import — codename, areas, deps, bead_count. No agent model or arrival generator wired by default (those come from pillar B or remain synthetic).
- Validation: emit a report of how many beads / works / edges were imported, plus any dropped entries with reasons.

**Open question:** harmonik bead YAMLs carry rich metadata (cite tags, forward-deferred edges, kind taxonomies) that the simulator does not need. Decision: drop everything outside `{id, depends_on, area, kind}` on import. Document in spec.

### B — Per-phase duration extraction

**Goal:** produce a fitted duration distribution per phase, derived from real Claude Code session transcripts, suitable as a drop-in replacement for the simulator's placeholder lognormal.

**Phases to model (per user 2026-05-15):**
- **Spin-up** — TaskCreate-dispatch timestamp → first tool call in the sub-agent transcript. Includes reconnaissance (`bd show`, `grep`, initial reads).
- **Task work** — first tool call in sub-agent → final `git commit` in that sub-agent.
- **Reviewer round-trip** — implementer commit → reviewer sub-agent's last assistant message → orchestrator's next dispatch. Captured if separable.
- **Merge** — `git commit` timestamp → `git log` commit-date on integration branch. May be near-zero for direct-to-main workflows.
- **Conflict-merge delta** — addressed inside the simulator as a synthetic injectable (see pillar C). No real data needed.

**Pipeline (hybrid, per methodology agents' verdict 2026-05-15):**
1. **Programmatic extractor** (Go binary or Python script) walks `~/.claude/projects/<proj>/<sessionId>/subagents/*.jsonl` plus the parent JSONL. Joins via tool-use-id and bead-id text matching. Reads `<usage><duration_ms>` as a sanity check. Emits one CSV row per bead with the phase deltas.
2. **LLM clean-up pass** (Sonnet 4.6, ~$50 one-time, cache-friendly) handles the ambiguous cases the programmatic pass cannot resolve confidently — primarily spin-up vs task-work boundary, and orchestrator-vs-worker time on interleaved sessions.
3. **Distribution fit** — for each phase, fit lognormal / gamma / weibull, pick by KS or AIC, emit as a reusable `agent_model.duration` block referenced by scenarios.

**Definitional choices (user confirmed 2026-05-15):**
- Reconnaissance counts as spin-up.
- Reviewer round-trip is its own phase, not part of task-work.
- Abandoned `worktree-agent-*` branch work is filtered out of duration stats but recorded in a separate "wasted-effort" counter.

### C — Synthetic conflict-merge model

**Goal:** since direct-to-main workflows produce no real merge-conflict data in either corpus, build the conflict behavior into the simulator itself.

**Approach:**
- Add scenario fields `merge_model.conflict_probability` and `merge_model.conflict_duration` (distribution).
- On bead completion, with probability `p`, draw an extra duration from `conflict_duration` and add it to the merge phase. Optionally make `p` depend on `area_collisions` count at completion time.
- Default values seeded from intuition until real conflict data exists; explicitly flagged as synthetic in `summary.json`.

### D — Real-corpus scenario generation

**Goal:** produce a small set of real-data-driven scenario YAMLs ready for the Plan 011 D (weight tuning) pillar.

**Approach:**
- Run pillar A on each harmonik pilot (eight YAMLs) → eight scenarios.
- Run pillar A on kerf's plans 007–009 → three scenarios.
- Attach pillar B's fitted duration distribution to all of them.
- Smoke-run each with `kerfsim run --runs 3` to verify they execute and produce sane metrics.

## Sequencing

- **L0:** A (importer) and B-step-1 (programmatic extractor) — independent, can start immediately.
- **L1:** B-step-2 (LLM clean-up) — depends on L0 programmatic output.
- **L2:** B-step-3 (distribution fit), D (scenario generation), C (synthetic conflict model).

## Specs touched

- New `specs/sim_corpus.md` — describes the importer's source-format contract and the phase-extraction definitions.
- `specs/simulator.md` — `merge_model` block; reference to fitted duration distributions.
- `specs/commands.md` — `kerfsim import` subcommand.

## Out of scope

- Live agent instrumentation. We work with what's already on disk.
- Sending transcripts to third-party services. Structural metadata extraction only.
- Real merge-conflict data acquisition. Plan 012 punts to the synthetic model in pillar C.

## Source material

- `source/sim_proposals_reconstruction.md` — May 14, 2026 design proposals (reconstructed; original `SIM-PROPOSALS-2026-05-14.md` never committed).
- `source/methodology_programmatic.md` — programmatic-extraction viability report.
- `source/methodology_llm.md` — LLM-classification pipeline report.
- `source/2026-05-15_design_discussion.md` — captured discussion that produced this plan.
