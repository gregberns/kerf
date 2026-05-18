# Workflow Gap Analysis — kerf surface vs. this project's actual flow

kerf describes itself as a tool for *structuring whatever workflow a project chose*, not for prescribing one. This memo asks a narrower question: how well does the existing jig and command surface cover the workflow we just used to bake Plans 016 through 020 in this very repo? That workflow has six stages — plan, plan review, spec edits, spec review, beads, beads review — followed by implementation. The short answer: the **spec jig** ("maintain a living spec, spec is always right") is a close conceptual match for stages 1 through 4 but is structured as a bench-resident multi-pass walk rather than a thin assist over our in-repo `plans/NNN/` + `specs/` layout, so most of our flow is currently *conventional hand-rolling* rather than tool-assisted. The bead stages have zero direct kerf support today and lean entirely on the external `bd` (beads) CLI.

## Jig inventory

Built-in jigs live in `internal/jig/builtin/`. Each has a YAML frontmatter (status_values, passes, file_structure) and a markdown body.

| Jig | Purpose (one line) | Passes | Key artifacts |
|---|---|---|---|
| `plan` | Plan a change to an existing codebase, end with an implementation-ready task list. | problem-space → analyze → decompose → research → change-spec → integration → tasks → ready | `01-problem-space.md`, `02-analysis.md`, `03-components.md`, `04-research/{c}/findings.md`, `05-specs/{c}-spec.md`, `06-integration.md`, `SPEC.md`, `07-tasks.md` |
| `spec` | Maintain a living system specification; spec changes flow to code. | problem-space → decompose → research → change-design → spec-draft → integration → tasks → ready | `01-problem-space.md`, `02-components.md`, `03-research/...`, `04-design/...`, `05-spec-drafts/{component}.md` (1:1 to target `specs/` files), `05-changelog.md`, `06-integration.md`, `07-tasks.md` |
| `bug` | Investigate a defect, specify a fix. | reported → research → reproducing → root-cause → fix-spec → ready | report, research, reproduction, root-cause, fix-spec docs |
| `implementation` | Break down → dispatch → implement → verify (composable; uses `br`/`bd`, `ntm`, `agent-mail`). | breakdown → dispatch → implementing → verify → complete | `01-breakdown.md`, `02-dispatch.md`, `03-verify.md` |
| `retrofit` | Reconcile code with specs when code changed without the workflow. | capture → rationale → spec-sync → square | capture, rationale, spec-sync docs |
| `spike` | Structured exploration when the approach is unknown. | frame → explore → converge → align → squared | frame, explore, exploration-log, alignment docs |

The `spec` jig in particular already encodes plan-style problem framing, per-area research/design, drafted spec files mapped 1:1 to system spec files, an integration cross-check, and a tasks pass. That is uncannily close to what we just did by hand for Plans 016–020.

## Command inventory (surface only)

| Category | Commands | Notes |
|---|---|---|
| Bench / project bootstrap | `kerf init`, `kerf setup`, `kerf localize`, `kerf config`, `kerf doctor` (spec'd, not in cmd/) | `init` bootstraps kerf in a project; `setup` regenerates agent-facing instructions from active jigs; `localize` migrates bench → in-repo storage. |
| Work lifecycle | `kerf new`, `kerf list`, `kerf show`, `kerf status`, `kerf resume`, `kerf shelve`, `kerf archive`, `kerf restore`, `kerf delete` | Create / browse / advance / pause / archive a work. |
| Verification & handoff | `kerf square`, `kerf finalize`, `kerf review` (spec'd), `kerf preview` (spec'd) | `square` checks artifacts against the jig's `file_structure`; `finalize` packages a `ready` work for handoff to implementation; `review`/`preview` are spec'd but not yet in `cmd/`. |
| Coordination / planning views | `kerf map`, `kerf next`, `kerf triage`, `kerf pin`, `kerf work edit`, `kerf areas {init,list,add,remove}`, `kerf bootstrap-filters` (spec'd) | These are the bead-aware views: which areas exist, what beads are ready next, drift between bd and the bench, pinning a bead to a single work owner. |
| Jig admin | `kerf jig {list,show,save,load,sync}` | Inspect and customize jigs. |
| Versioning | `kerf snapshot`, `kerf history` | The `.history/` auto-versioning machinery. |

Where each fires in our six-stage flow: nothing fires at Plan stage (we hand-write `plans/NNN/_plan.md`), nothing fires at Plan review or Spec review (those are sub-agent passes), nothing fires at Spec edits (we edit `specs/` files directly), and the bead stages use the external `bd` CLI — kerf's bead-aware commands (`next`, `triage`, `pin`, `map`) operate over an already-populated `bd` store rather than helping to create beads from a plan.

## Six-stage workflow → kerf capability

| Workflow stage | Existing kerf support | Gap | Severity |
|---|---|---|---|
| 1. Plan (`plans/NNN/_plan.md` hand-write) | None directly. The `spec` jig's passes 1–2 (problem-space, decompose) and the `plan` jig's passes 1–3 cover this conceptually, but they expect a *kerf work* (bench-resident, status-walked) rather than a free-form file under `plans/`. | No `kerf plan new <num> <name>` scaffolder; no template; users invent the structure of `_plan.md` per plan. | MINOR — works fine by convention; would be a small ergonomic win. |
| 2. Plan review (fresh-context critique pass) | None. The spec jig has per-pass *review criteria* baked into the markdown body but no command to invoke a review against a `plans/NNN/_plan.md`. | No `kerf review` against a plan doc; no captured critique artifact. (Spec'd `kerf review` exists in `commands.md` but isn't implemented and targets bench works, not `plans/`.) | MAJOR — review-gate is core to the working style; absence means it's "remember to spawn a reviewer." |
| 3. Spec edits (edit `specs/*.md`) | None directly. The `spec` jig's pass 5 ("spec-draft") produces `05-spec-drafts/{component}.md` mapped 1:1 to target `specs/` files; the `spec` jig's finalize then copies those drafts in. That is the right shape but it presumes the bench-jig walk has been done. We instead edit `specs/` in place from a plan. | No "apply a plan's spec changes" affordance; no `kerf spec diff <plan>` to show what changed in `specs/` since a plan started. | MAJOR — spec edits are the most consequential step and have no tooling support. |
| 4. Spec review (verify edits match plan prose) | None. Same shape as stage 2; reviewer is hand-spawned. | No `kerf review --plan NNN --against specs/` style command. | MAJOR — same review-gate concern. |
| 5. Beads (`bd create` with intra- and cross-plan deps) | Partial. `kerf next`, `kerf triage`, `kerf pin`, `kerf map`, `kerf work edit` all *read* the `bd` store. `kerf bootstrap-filters` (spec'd) writes filter config. The `implementation` jig has a `breakdown` pass that produces `01-breakdown.md`, which is the precursor to creating beads. | No "translate a plan's beads outline into actual `bd create` calls"; no plan-aware bead labelling; no dependency wiring tool. The `plan`/`spec` jig `07-tasks.md` is the right artifact but nothing flows it into `bd`. | MAJOR — this is where most of our manual labour lives. |
| 6. Beads review (verify beads match plan outline) | None. `kerf triage` detects drift between the bead store and bench works but does not verify *against a plan's beads outline*. | No `kerf beads verify --plan NNN`. | MINOR–MAJOR — partly covered by `triage`; full coverage missing. |

## Recommendations

### Extend the existing `spec` jig

The `spec` jig is reusable. Its passes already mirror what we do by hand: problem framing, decomposition into affected spec files, per-area research, change design, spec-drafts mapped 1:1 to `specs/`, integration cross-check, and tasks. The mismatch is structural, not semantic: we hand-write under `plans/NNN/` and edit `specs/` in place, whereas the `spec` jig expects a bench work with numbered artifact files. A pragmatic extension is to make the `spec` jig's `file_structure` *projectable* — let a project declare "my plans live at `plans/{codename}/`, my drafts are the live `specs/` files themselves, skip the staging copy." That preserves the spec jig's review criteria and pass structure without forcing a parallel bench tree.

### Propose one new lightweight jig: `plan-spec-bead`

A new jig that fuses the three composable phases we actually run — `plan` (write `_plan.md`) → `spec` (edit `specs/`) → `beads` (create `bd` items) — with explicit review gates between each. Passes would be: `plan-draft → plan-review → spec-edit → spec-review → beads-create → beads-review → ready`. Artifacts are pointers (path to `_plan.md`, list of changed `specs/` files, list of `bd` bead IDs) rather than staged copies. The review passes invoke `kerf review` (see below). This is closer to "an opinionated guide for the workflow this project actually has" than the bench-resident `spec` jig.

### Commands that would have helped during 016–020

- `kerf plan new <num> <name>` — scaffolds `plans/NNN_name/_plan.md` with the section template we keep re-inventing (intent, scope, design notes, spec changes, beads outline, open questions). One-shot ergonomic win.
- `kerf review <target>` — already spec'd in `commands.md` but unimplemented. Implementing it to dispatch a fresh-context reviewer against a plan, a spec edit, or a beads outline would put the review gate inside kerf rather than the orchestrator.
- `kerf plan status NNN` — show which of the six stages have artifacts on disk (`_plan.md` exists? `specs/` files dirty since plan started? beads exist with this plan label?). Useful mid-bake to know what's left.
- `kerf beads from-plan NNN` — read the plan's "beads outline" section and emit `bd create` calls (or shell them out). Verifies the outline against what's already in `bd` and reports drift.

## Routing — what goes where

| Gap | Route |
|---|---|
| `kerf plan new` scaffolder | New small plan (021) — pure ergonomics, no spec ambiguity. |
| `kerf review` implementation | Already in `commands.md` (spec'd, not built). Fold into **Plan 020 (jig review gate)** since they share the review-gate motivation, or extract into Plan 022. |
| `kerf plan status` | New plan (023) once the plan-level lifecycle is decided. Could be deferred. |
| `kerf beads from-plan` | New plan (024). Requires deciding the canonical "beads outline" format inside `_plan.md` — this is a content-shape decision worth its own plan. |
| `spec` jig projectability (file_structure pointers, not copies) | Could ride along with **Plan 017 (storage reconciliation)** since both touch where artifacts live. |
| Verify beads against plan outline | Fold into **Plan 018 (triage rework)** — `triage` already does drift detection; this is one more drift dimension. |
| Six-stage `plan-spec-bead` jig | New plan (025). The biggest of the candidates; only worth opening once `kerf review` lands and the in-repo storage story is settled. |

Workarounds in the meantime: keep hand-writing `_plan.md`, keep dispatching reviewer sub-agents manually, keep running `bd create` by hand. The current 016–020 cycle proves the workflow is viable without tooling; the tooling would mainly remove repeated typing and missed-review-gate risk.
