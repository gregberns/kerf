# Agent Confusion — kerf orientation & directions

Fresh-agent reading: `kerf` (no args), `kerf next --help`, `kerf new --help`, `kerf init --help`, plus a few probes.

Impact key: **mild** (small friction) · **moderate** (could cause wrong action) · **blocks** (agent gets stuck).

---

## 1. `kerf next --help` is wildly out of sync with the spec

- **Where**: `cobra` help for `kerf next` vs `specs/commands.md:1408-1536` (`kerf next` section, especially the "Help text" subsection at 1525-1536).
- **Confusion**: Spec literally says: "`kerf next --help` is part of the spec — the agent's contract. A fresh agent running it once must come away knowing the full loop." The current help advertises only `--area` and `--limit`. It is missing the entire item-kinds model (`bead`/`cleanup`/`warning`), the flags that gate them (`--only`, `--include`, `--kinds`, `--format`), the "read top item, do it, re-run" loop, and the pointer to coordination.md scoring. The example `kerf next --area api` references a flag that, per the spec, should not exist in this form (filtering is by kind not by area name). An agent reading only `--help` will misuse the command.
- **Impact**: **blocks** — directly violates the spec's stated agent contract.
- **Suggested fix**: Regenerate `kerf next --help` to match `commands.md` §"Help text". Make this a test (snapshot) so it cannot drift again.

## 2. `kerf next` empty result gives no guidance

- **Where**: `/tmp/kerf next` with no works prints `No actionable works for project '…'.` and exits.
- **Confusion**: An agent that was told "run `kerf next` to start" gets one sentence and no next step. Spec says every state-changing command emits next steps (invariant 7 in `_index.md`); `next` is read-only but is the documented entry point.
- **Impact**: moderate.
- **Suggested fix**: When empty, append the same `Get started: kerf new` hint that `kerf list` shows, plus a hint to run `kerf init` if `project.yaml` is absent.

## 3. Top-level `kerf` orientation omits half the commands

- **Where**: Output of bare `kerf`.
- **Confusion**: The "Available commands" block lists `new`, `list`, `show`, `status`, `resume`, `shelve`, `finalize`, `square`, `snapshot`, `history`, `restore`, `archive`, `delete`, `config`, `jig`. It does **not** list `init`, `setup`, `localize`, `next`, `map`, `areas`. But `init` is the documented entry point ("Run this once per project"), and `next` is described in the spec as "the agent's primary pull signal". A fresh agent reading the orientation never learns these exist.
- **Impact**: **blocks** — the agent will not run `kerf init` or `kerf next` because it has never heard of them.
- **Suggested fix**: Add a dedicated "Onboarding" section at the top (`kerf init`, `kerf setup`) and an "Agent loop" section (`kerf next`, `kerf map`, `kerf areas list`) before the work-CRUD commands.

## 4. `kerf` orientation step 2 is hand-wavy

- **Where**: Bare `kerf` output: `2. Work through jig passes    Write artifacts, advance status`.
- **Confusion**: "Work through jig passes" is not actionable. There's no pointer to `kerf jig show <name>` or `kerf status <codename> <new-status>`. The agent doesn't learn how the loop actually advances.
- **Impact**: moderate.
- **Suggested fix**: Expand step 2 to: "Read the pass instructions in the `kerf new` output (or `kerf show <codename>`); write the pass artifact; run `kerf status <codename> <next-status>` to advance."

## 5. `kerf init` help omits jig-selection prompt

- **Where**: `kerf init --help` vs `commands.md:1144-1212`.
- **Confusion**: Help text says only "creates the project identifier, sets the default workflow, and prints instructions". The spec says step 7 prompts the user to **select active jigs** and step 8 auto-detects `bead_filter`. Neither is mentioned in `--help`. An agent running `kerf init` non-interactively (the common case) won't know whether to expect a prompt, an error, or silent defaulting.
- **Impact**: moderate.
- **Suggested fix**: Mention the interactive jig-selection step and the non-interactive fallback ("If stdin is not a TTY, defaults are used and no prompt is shown") in the help body.

## 6. `kerf setup` error is unactionable when no project-identifier exists but bench does

- **Where**: `kerf setup` → `Error: project not initialized. Run 'kerf init' first`.
- **Confusion**: Bench has 19 works for project `gregberns-kerf` (per `kerf` orientation), but `kerf setup` says project is not initialized. The error doesn't explain that project-identifier resolution failed at the cwd, only at the bench. Agent does not know whether to `kerf init` (will it clobber the 19 works?) or `cd` somewhere.
- **Impact**: moderate.
- **Suggested fix**: Distinguish "no `.kerf/project-identifier` in cwd" from "no `project.yaml` for the resolved project". Include the resolved project ID (when one exists) and clarify that `kerf init` is safe to re-run.

## 7. `kerf status --help` is one line; no examples, no warning behavior

- **Where**: `kerf status --help`.
- **Confusion**: Help is "Get or set a work's status" with no examples. Spec (`commands.md:530-599`) describes a meaningful warning when the new status isn't in the jig's `status_values` and a read-mode output with the progression arrow. An agent has no idea that this is the command used to advance jig passes, nor that it can also read.
- **Impact**: moderate.
- **Suggested fix**: Add examples (`kerf status blue-bear`, `kerf status blue-bear research`) and note the "open string, warn on unrecognized" behavior.

## 8. "Bench summary" shows global count, not project-scoped count

- **Where**: Bare `kerf` output: `Total active works: 19`.
- **Confusion**: Spec (`commands.md:44`) says output should show "number of active works in the current project (if inside a repo), total active works across all projects". Running inside the kerf repo, project `gregberns-kerf` is inferred, but only the global count is shown. Agent has no per-project orientation.
- **Impact**: mild — but it's a spec mismatch.
- **Suggested fix**: Print both lines: `Active works in {project}: N` and `Total across all projects: M`.

## 9. Standard-workflow block omits the `square` and `status` steps

- **Where**: Bare `kerf` output, "Standard workflow" lines 1-5.
- **Confusion**: The 5-step recipe goes new → "work through jig passes" → shelve → resume → finalize. There is no mention of `kerf square` (verification) or `kerf status` (advancing). The spec treats square as a precondition of finalize. An agent following the recipe literally will go from "wrote some files" to `kerf finalize` and fail the pre-flight.
- **Impact**: moderate.
- **Suggested fix**: Insert a step 3 "advance status with `kerf status`" and a step 5 "verify with `kerf square` before finalize".

## 10. `kerf new --help` doesn't surface the first-run onboarding error

- **Where**: `kerf new --help` vs `commands.md:115-138`.
- **Confusion**: A fresh agent calling `kerf new` with no `default_jig` and no `--jig` flag will hit the "No default workflow configured" error. The help text doesn't warn that `--jig` is effectively required until config is set. The example `kerf new auth-rewrite` will fail on first invocation.
- **Impact**: moderate.
- **Suggested fix**: Add a note: "If no default jig is configured, one of `--jig plan` / `--jig spec` is required. See `kerf init --jig <name>`."

## 11. Error responses include cobra "Usage:" boilerplate

- **Where**: `kerf show foo` → emits error then prints full `Usage: kerf show <codename>` + flag table.
- **Confusion**: For an agent parsing structured errors, the extra cobra noise after the `Error:` line is awkward and inconsistent with the spec's lean error-message format (`commands.md` error tables show single-line messages).
- **Impact**: mild.
- **Suggested fix**: Disable cobra's `SilenceUsage` for runtime errors so help is shown only on flag-parse errors.

## 12. No way to discover the "first pass" instructions after `kerf new` output scrolls

- **Where**: `kerf new` output (per spec) includes the jig's agent instructions for the first pass; spec doesn't define a command that re-prints them. `kerf show <codename>` includes "Jig context: the pass corresponding to the current status, with the jig's agent instructions" — but a fresh agent doesn't know that.
- **Confusion**: An agent that clears context mid-pass needs to know how to retrieve the current pass's instructions. The standard workflow doesn't tell them.
- **Impact**: moderate.
- **Suggested fix**: Top-level help should call out `kerf show <codename>` as "rehydrate pass instructions". `kerf status <codename>` (read mode) could also include them.

## 13. `kerf list` empty output suggests `kerf new` but not `kerf init`

- **Where**: `kerf list` with no works → `Get started: kerf new`.
- **Confusion**: A truly fresh agent should run `kerf init` first (it sets `default_jig`, selects active jigs, writes `project.yaml`). Otherwise `kerf new` hits the first-run-onboarding error. The hint sequences the agent into the failure path.
- **Impact**: moderate.
- **Suggested fix**: Detect "no `project.yaml` for this project" in the empty-list branch and suggest `kerf init` first.

## 14. Codename and project terminology used before defined

- **Where**: Bare `kerf` mentions `codename` 7 times and `bench` once; neither is glossed in the orientation output.
- **Confusion**: A fresh agent has to read `specs/_index.md` glossary to learn that codename is an `adjective-noun` slug and bench is `~/.kerf/`. The orientation is supposed to be self-contained (`commands.md:21-24`: "an agent with zero prior context can use kerf effectively after reading this output").
- **Impact**: mild.
- **Suggested fix**: One-line gloss under "Available commands": `<codename>` = work identifier (e.g., `blue-bear`); bench = `~/.kerf/`.

## 15. `kerf next` doesn't differentiate "no project initialized" from "no actionable items"

- **Where**: `kerf next` in repo without `project.yaml` still prints "No actionable works for project 'X'".
- **Confusion**: Spec defines warning-detector items for misconfigurations. A bare project with no jigs activated should surface a `warning` item directing the agent to `kerf init`. As implemented, the result is indistinguishable from a healthy "you're caught up" state.
- **Impact**: moderate.
- **Suggested fix**: Emit a warning item or footer when `project.yaml` is missing.

## 16. "Pass" vs "status" vs "stage" terminology mixed in outputs

- **Where**: `kerf jig show plan` lists "Passes" with `status:` labels; bare `kerf` says "Work through jig passes"; `kerf next` cleanup detector text (per spec line 1454) says "advance status (`kerf status <codename> <next-stage>`)" — note "stage", a third word.
- **Confusion**: Three terms for the same concept. An agent will wonder whether stage and pass are different things.
- **Impact**: mild.
- **Suggested fix**: Pick one (the spec mostly uses "pass" and "status value"). Audit and replace "stage" usage.

## 17. `kerf shelve` without codename relies on "active session" but agents may not know what that means

- **Where**: `kerf shelve --help`.
- **Confusion**: Help says "infers the active work in the current project". The mechanism — there is exactly one work with `active_session != null` — is invisible. When an agent has shelved+resumed multiple works, "active" is ambiguous to them.
- **Impact**: mild.
- **Suggested fix**: Mention in help that "active" means "currently has an open session (set by `kerf resume` or `kerf new`)".

## 18. Spec drift: `jig list` shows 4 passes for implementation, spec example shows different ones

- **Where**: `kerf jig list` shows implementation passes `breakdown, dispatch, implement, verify, complete`. `commands.md:243-246` example shows `breakdown, dispatch, implement, review`. `commands.md:641` example shows `breakdown, dispatch, implement, review`.
- **Confusion**: Two different pass lists in the codebase. An agent comparing tooling output to spec will not be able to tell which is canonical.
- **Impact**: moderate.
- **Suggested fix**: Reconcile `specs/jig-implementation.md` with the example outputs in `commands.md`. Source of truth is the jig file; spec examples must match.

## 19. `kerf finalize` requires `--branch` but examples in main help don't show it being chosen by anything

- **Where**: Main `kerf` help: `kerf finalize <codename> --branch <name>  Complete and hand off`.
- **Confusion**: An agent that has been planning all session doesn't know what branch name conventions are expected. The spec says "the agent chooses the name based on work context" but doesn't surface any guidance at the CLI surface.
- **Impact**: mild.
- **Suggested fix**: `kerf finalize --help` could suggest `feature/<codename>` or `kerf/<codename>` as a default pattern.

## 20. `kerf jig show plan` truncates `File structure` without "..."

- **Where**: `kerf jig show plan` output ends after partial file structure (mid-list) in 30-line view — but my probe shows the section continuing. Cobra/pager behavior could mislead an agent into thinking the list is complete when it isn't. Not strictly a confusion, but worth a note.
- **Impact**: mild.
- **Suggested fix**: Ensure full structure is always emitted with a trailing newline; no implicit paging.
