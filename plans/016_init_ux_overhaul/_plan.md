# Plan 016 — Init UX Overhaul

> **Status: baked.** Expanded from the Plan 015 (harmonik beta-feedback triage) stub. Ready for the spec stage of the `plan-implementation` flow.

## Intent

`kerf init` is the first command a fresh agent runs against a new project, so its output sets the agent's mental model for everything that follows. Today it issues an interactive y/N prompt with no escape hatch (agent harnesses can't answer prompts), claims to have set fields that never land in `project.yaml`, fires a stale "100% of beads use `kerf:*` labels" detection, prints two overlapping `AGENT SETUP INSTRUCTIONS` blocks (one from `kerf init` directly, one from the embedded `kerf setup` call), omits the daily-driver commands (`kerf next`, `kerf triage`, `kerf pin`, `kerf map`, `kerf areas`, `kerf work edit`), and never mentions that the authoritative state lives on the bench (`~/.kerf/projects/<id>/`, kerf's global store) rather than in the repo. This plan reworks init so a single non-interactive run produces an unambiguous, complete project setup that needs no manual repair.

## Background

All items trace to `plans/015_harmonik_beta_feedback/triage.md` themes 1 (init / first-run UX) and 9 (command-UX gaps). The harmonik tester (Claude Opus 4.7) bootstrapped a fresh kerf install on the harmonik repo over 2026-05-15 → 2026-05-18 and flagged init as the single biggest friction source. Triage verified items 1.3, 1.4, and 1.5 against `cmd/init.go` HEAD as still-live.

## Scope

**In scope.** The agent-facing behavior of `kerf init` from invocation to final output: prompt removal, flag surface, state-change reporting, detector accuracy, instruction-block consolidation, and the `project.yaml` fields init advertises.

- Make `kerf init` non-interactive by default; introduce `--yes` and `--no` flags that resolve the `bead_filter` decision without input. Keep `--force` distinct (it controls overwrite, not prompt answers).
- Emit one consolidated state-change summary per run: each artifact (`project.yaml`, `.kerf/project-identifier`, default-jig setting, bead_filter) reports `created`, `updated`, or `unchanged`. Stop printing claims that don't reflect what landed on disk.
- Fix the label-prefix detector: sample the current `.beads/issues.jsonl`, report only when score and absolute count both clear a confidence threshold, stay silent otherwise. No more 100% claim on empty corpora.
- Collapse the two `AGENT SETUP INSTRUCTIONS` blocks into one canonical source. The init flow either calls `kerf setup` and skips the inline block, or owns the block and skips the `kerf setup` call — the report flags which is cleaner.
- Update the agent-setup instruction text to include `kerf next`, `kerf triage`, `kerf pin`, `kerf map`, `kerf areas`, `kerf work edit`, and the exact `.gitignore` two-line pattern (`.kerf/` + `!.kerf/project-identifier`).
- Either persist `default_jig` and any pass-schedule fields init advertises into `project.yaml`, or stop advertising them. Symptom and on-disk state agree.

**Out of scope.** Storage drift detection and `kerf doctor` (Plan 017). Filter-bootstrap from existing labels (Plan 019). Triage suggester rework (Plan 018). The bench-location callout in the instruction block is owned by Plan 017 (item 1.10); this plan leaves a one-line stub that 017 fills in.

## Design notes

The interactive prompt is the load-bearing problem: removing it forces every other piece of init to become declarative. Default resolution for `bead_filter` when no flag is given: apply the detector's top prefix if confidence clears the threshold, otherwise leave bead_filter unset and print a one-line note explaining how to set it later (`kerf config bead_filter ...` or hand-edit `project.yaml`). `--yes` accepts whatever the detector suggests (silent on low confidence — i.e., still unset); `--no` skips the detector entirely and leaves bead_filter unset; `--bead-filter <expr>` (already implied by the existing config path) lets the caller name an exact value.

Alternative considered: keep the prompt but add `--yes`/`--no` only. Rejected — the y/N prompt is the actual blocker for agent harnesses; flag-only is a partial fix that leaves the no-flag default broken for the very callers who matter.

The state-change summary takes the shape of one fenced block at the end of normal output, with one line per artifact. This makes the output diffable across runs and gives the agent a stable place to look.

For the duplicate `AGENT SETUP INSTRUCTIONS` collapse, the cleaner factoring is to make `kerf setup` the single owner of the instruction block, and have `kerf init` call it once. Init's own inline block (currently in `bootstrapInstructions` in `cmd/init.go`) becomes a thin wrapper that defers to `kerf setup`'s output.

## Spec changes proposed

- **`specs/commands.md` (`kerf init` section, ~line 1160).** Document the non-interactive default. Add the `--yes` / `--no` flag rows alongside `--force` and `--jig`. Replace the current "auto-detect bead_filter" prose with the confidence-threshold rule and the three flag resolutions. Document the single state-change summary as part of `### Output`. Remove or rewrite the line that says init prints `Set default_jig: spec`. Note that init delegates the agent-setup instruction block to `kerf setup` (no inline copy).
- **`specs/commands.md` (`kerf setup` section).** Confirm `kerf setup` is the single source of the instruction text. Add the missing commands (`kerf next`, `kerf triage`, `kerf pin`, `kerf map`, `kerf areas`, `kerf work edit`) and the exact gitignore pattern to the canonical block. Leave a one-line placeholder for Plan 017's bench-location section.
- **`specs/cli.md`.** Add a short paragraph under output philosophy describing the state-change summary shape used by `kerf init`. Scoped to init in this plan; whether the pattern generalizes to every state-changing command is a separate cross-cutting decision and is explicitly not proposed here.
- **`specs/architecture.md`.** If `default_jig` is to land in `project.yaml`, document the field in the project-config schema. If pass schedules for non-`implementation` jigs are advertised by init, document those too. If the decision is the opposite (drop the claim from init's output), this spec is untouched.

No spec edits in this plan — those are the spec stage's job.

## Beads outline

Rough, dependency-aware decomposition; final shapes determined during plan-implementation's bead-creation pass.

1. **B1 — Drop interactive prompt, add `--yes` / `--no` flags.** Refactor `detectBeadFilter` in `cmd/init.go` to consume a resolution mode instead of `stdin`.
2. **B2 — Detector confidence threshold.** Add an absolute-count floor and a score-floor to `DetectFilterPrefix` in `internal/beads/heuristic.go`; return "no suggestion" instead of a low-confidence prefix.
3. **B3 — State-change summary block.** Build a small artifact-change tracker that init's steps report into; emit one fenced summary at the end.
4. **B4 — `default_jig` persistence (or claim removal).** Decide based on the spec-change resolution; either wire the write or strip the misleading log line.
5. **B5 — Collapse AGENT SETUP blocks.** Remove the inline `bootstrapInstructions` block from init's output; rely on the embedded `kerf setup` call. Verify there is no remaining double-print.
6. **B6 — Update `kerf setup` instruction text.** Add the six missing commands, the exact gitignore two-line pattern, and the one-line bench-location placeholder (filled by Plan 017).
7. **B7 — Tests.** Update `init_test.go` and `init_bead_filter_test.go` for the new non-interactive paths, flag combinations, detector silence, and state-summary shape.
8. **B8 — Snapshot/help regen.** Refresh `help_snapshot_test.go` and any golden outputs that name the init flag set.

Eight beads, roughly half implementation and half test/documentation. B2 and B5 are independently shippable. B4 depends on B3 (the state-change tracker needs to know whether `default_jig` is a tracked artifact before it can report on it); both depend on the spec-stage resolution of the `default_jig` open question. B6 depends on the spec-stage decision about the bench-location placeholder. The existing `--jig` flag is preserved unchanged; `--yes` / `--no` only affect the bead_filter resolution path, not jig selection.

## Items absorbed from Plan 015

- 1.1 — non-interactive default plus `--yes` / `--no` flags
- 1.2 — single state-change summary (no "lying about state")
- 1.3 — stale `kerf:*` label-prefix detector
- 1.4 — `default_jig` claim must match `project.yaml` shape
- 1.5 — duplicate `AGENT SETUP INSTRUCTIONS` blocks
- 1.6 — instruction text missing current-generation commands
- 1.7 — exact `.gitignore` two-line pattern (folded into B6)
- 1.8 — `project.yaml` shape matches init's claims
- 9.1 — `--yes` / `--no` flags (same surface as 1.1)

## Open questions

- Should `--yes` / `--no` be init-specific or a global convention for any future interactive prompt? Recommend init-specific in v1; promote to a global convention only if a second command needs it.
- When the detector clears the score floor but not the count floor (e.g., 100% of a 1-bead corpus), does init stay silent or print a one-line "corpus too small for confident detection"? Recommend the silent path; the agent can re-run later.
- Does `default_jig` land in `project.yaml` or get dropped from init's output entirely? The latter is smaller; the former is what users seem to expect. Spec stage decides.
- Does init keep calling `kerf setup` (collapse-by-deletion of the inline block), or does init own the block and `kerf setup` becomes purely standalone? Recommend the former — fewer code paths, single source of truth.
- Does the bead_filter resolution need a third explicit flag (`--bead-filter <expr>`) alongside `--yes` / `--no`, or is the existing `kerf config bead_filter` post-init path sufficient? Spec stage decides.

## Conflicts with other plans

This plan touches `cmd/init.go`, `specs/commands.md` (init + setup sections), and possibly the `project.yaml` schema in `specs/architecture.md`. Plans 017 (storage reconciliation) and 019 (filter bootstrap) both also touch init and `project.yaml`:

- **Plan 017** wants to add a bench-location section to the instruction block (its absorbed item 1.10) and may add drift-related fields to `project.yaml`. This plan leaves a one-line placeholder slot for 017 in the instruction block; sequence 016 first so 017 fills a known gap rather than fighting over the block's structure.
- **Plan 019** wants `kerf new` to always emit a `bead_filter:` key in `spec.yaml` and may extend `project.yaml` with filter-bootstrap config. Plan 019's `spec.yaml` changes don't overlap with this plan's `project.yaml` changes, but both touch the bead-filter story in init's output. Sequence 016 first so 019 builds on top of a working non-interactive detector path rather than rewriting it.

Flag for orchestrator: spec stage should sequence 016 → 017 → 019 so each plan's spec edits land on a stable base.
