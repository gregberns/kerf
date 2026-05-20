# Implementation Jig

> Built-in jig for implementing a spec: break down into tasks, dispatch to agents, execute with review gates.

This spec defines the `implementation` jig that ships with kerf. It is the primary jig for taking a completed spec or plan and turning it into working code. This is a **composable** jig — projects select which passes to include based on their workflow. See [jig-system.md](jig-system.md) for jig file format, composable passes, and the review pattern.

> Beads-CLI binary name comes from `project.yaml` `tools.tasks` (default `br`). If you have configured a different binary, translate the argv shape — this spec uses the `br` CLI syntax verbatim.

## When To Use

The `implementation` jig applies when:

- A spec or plan work has reached `ready` status and been finalized
- The work has a clear task list or can be decomposed into implementation tasks
- Code needs to be written, tested, and verified against a spec

It does not apply to:

- Exploration where the approach is unknown (use the [`spike`](jig-spike.md) jig)
- Work that has already been done without specs (use the [`retrofit`](jig-retrofit.md) jig)
- Writing or updating specs (use the [`plan`](jig-plan.md) or [`spec`](jig-spec.md) jig)

## Frontmatter

```yaml
---
name: implementation
description: Implement a spec. Break down, dispatch, execute, verify.
version: 1
phase: implementation
tools:
  - br
  - ntm
  - agent-mail
aliases: [impl, implement]
composable: true
status_values:
  - breakdown
  - dispatch
  - implementing
  - verify
  - complete
passes:
  - name: "Breakdown"
    status: breakdown
    output: ["01-breakdown.md"]
    tools: ["br"]
  - name: "Dispatch"
    status: dispatch
    output: ["02-dispatch.md"]
    tools: ["ntm"]
  - name: "Implement"
    status: implementing
    output: []
    tools: ["ntm", "agent-mail"]
  - name: "Verify"
    status: verify
    output: ["03-verify.md"]
  - name: "Complete"
    status: complete
    output: []
file_structure:
  - spec.yaml
  - SESSION.md
  - 01-breakdown.md
  - 02-dispatch.md
  - 03-verify.md
---
```

## Composable Pass Configurations

This jig has `composable: true`. Projects select which passes to include via [project configuration](architecture.md). Common configurations:

**Full orchestrated** (all passes):
```
Breakdown → Dispatch → Implement → Verify → Complete
```
For multi-agent workflows using ntm/Kilroy with beads. The default when all passes are active.

**Single-agent** (breakdown + implement):
```
Breakdown → Implement → Complete
```
For solo development. No dispatch (no orchestrator), no separate verify pass (review gates are inline in the implement pass).

**Minimal** (implement only):
```
Implement → Complete
```
For small changes where the task list is obvious and doesn't need formal breakdown.

## Status Progression

```
breakdown -> dispatch -> implementing -> verify -> complete
```

The `complete` status indicates all implementation tasks are done and verified against the spec. The work can then be archived.

## Passes

### Pass 1: Breakdown (breakdown)

**Output:** `01-breakdown.md`

Decompose the spec into implementation tasks (beads) with dependencies.

#### Agent Instructions

You are turning a finalized spec into an actionable task list. Each task becomes a bead — an atomic unit of work that one agent can complete in one session.

**What to do:**

1. Read the source spec or plan work. Find the finalized `SPEC.md` or equivalent document. If the implementation work has a `depends_on` referencing a plan work, read that work's artifacts.
2. Read `07-tasks.md` from the plan work if it exists — the plan jig already produced a task breakdown. Use it as a starting point, not as gospel. The breakdown pass may need to split, merge, or reorder tasks based on implementation considerations.
3. Create beads using `br`:
   - `br create "Task title" --description "Full description with acceptance criteria"`
   - Include the spec reference in the description so the implementing agent knows what to check against
   - Set dependencies between beads: `br dep add <issue> <depends-on>` (i.e. "issue depends on depends-on")
4. Verify the dependency graph forms a DAG (no circular dependencies).
5. Verify completeness: every section of the spec is covered by at least one bead. Every acceptance criterion appears in at least one bead's description.
6. Write `01-breakdown.md` documenting:
   - The bead list with IDs, titles, and dependency graph
   - Mapping from spec sections to beads (traceability)
   - Any decisions made during decomposition (tasks split, merged, reordered from the plan)
   - Estimated parallelism — which beads can run concurrently
7. **Save to disk.** Advance status to `dispatch`.

**What done looks like:**

- Beads exist in the `br` database with descriptions and dependencies
- `01-breakdown.md` exists with the full task list, dependency graph, and spec traceability
- Every spec section and acceptance criterion maps to at least one bead
- Dependencies form a DAG

### Pass 2: Dispatch (dispatch)

**Output:** `02-dispatch.md`

Set up agent coordination and begin dispatching tasks to workers.

#### Agent Instructions

You are the controller agent. You set up the orchestration environment and begin dispatching beads to worker agents. This pass contains the full instructions for how to use the orchestration tool — follow them exactly.

**What to do:**

1. **Set up the session:**
   - Create an ntm session: `ntm create <session-name> --dir <project-dir>`
   - Add worker panes: `ntm add <session-name> --agent cc --count <N>`
   - Start agent-mail if using file reservations: verify agent-mail MCP is available
   - Register agents if using agent-mail coordination

2. **Dispatch beads:**
   - Query ready beads: `br list --status open` or `br ready`
   - Use `ntm assign` for dependency-aware dispatch, or manually send to specific panes:
     `ntm send <session> --panes=<N> -- "<bead instructions>"`
   - Each dispatch message must include:
     - The bead ID and title
     - The full bead description with acceptance criteria
     - The spec reference (which spec section this implements)
     - Instructions to update bead status when starting (`br update <id> --status in_progress`) and when done
   - If using `ntm assign --watch`, enable continuous auto-assignment of unblocked beads

3. **Monitor progress:**
   - Check session status: `ntm status <session>`
   - Check bead status: `br list` or `br status`
   - Poll for completion — check periodically, don't busy-wait
   - Handle failures: if a worker agent fails or goes off-script, capture the output, reset the bead, and re-dispatch

4. **File reservation (multi-agent):**
   - When multiple agents may edit the same files, use agent-mail file reservations to prevent conflicts
   - Reserve files before dispatching: agents claim files they'll modify
   - Release reservations when beads complete

5. **Write `02-dispatch.md`** documenting:
   - Session setup (session name, number of workers, agent types)
   - Dispatch plan (which beads dispatched to which agents)
   - Tool configuration used

6. **Save to disk.** Advance status to `implementing`.

**Validated ntm patterns:**

These patterns have been validated across multiple projects. Follow them:

- **Captures:** Use `ntm view <session>` to see agent output. Do not rely on agent memory — check actual output.
- **Polling intervals:** Check every 60-120 seconds during active work. Don't busy-wait.
- **Context resets:** If a worker agent's context is exhausted, the bead state is in `br`. Start a new session for that agent and have it pick up the bead by ID.
- **One bead per prompt:** Never batch multiple beads into a single agent prompt. One bead, one dispatch.
- **File reservations:** Always use agent-mail file reservations when 2+ agents may touch the same files.

**What done looks like:**

- ntm session is running with worker agents
- Beads are being dispatched and worked on
- `02-dispatch.md` exists documenting the session setup and dispatch plan
- Agent coordination is active

### Pass 3: Implement (implementing)

**Output:** (none — this is a process pass. State is tracked in the bead database.)

Execute implementation tasks with mandatory review gates. This pass runs until all beads are complete and reviewed.

#### Agent Instructions

You are orchestrating the implementation loop. Each bead goes through: implement → review → feedback → next. This pass is the core execution loop.

**The implementation loop:**

For each bead:

1. **Dispatch** (if not already dispatched in Pass 2):
   - Send the bead to a worker agent with full context (bead description, spec reference, acceptance criteria)
   - The worker implements the bead and reports completion

2. **Review gate (mandatory — never skip):**
   - When a worker reports a bead complete, review the output
   - Compare the implementation against the spec's acceptance criteria
   - Check: does the code match what the spec says? Not "is the code good" — "does it match the spec"
   - If the spec and code disagree, **the spec wins**

3. **Feedback:**
   - If the review finds issues, send specific feedback to the implementing agent
   - Reference the spec section and acceptance criteria that aren't met
   - The agent fixes and re-reports. Review again.
   - Up to 3 feedback rounds per bead. After that, flag for human review.

4. **Close the bead:**
   - When review passes: `br close <id>`
   - Move to the next bead

5. **Clear context between beads:**
   - Each bead gets a fresh prompt/context
   - Don't carry implementation context from one bead to the next — it leads to drift
   - The spec and bead description are the context, not the previous bead's implementation

**Key rules:**

- **One bead per prompt.** Never batch multiple beads. One bead, one agent prompt, one review.
- **Mandatory review gate.** Every bead gets reviewed before closing. No exceptions.
- **Clear context between beads.** Fresh context per bead. The spec is the anchor, not the previous bead.
- **Spec wins.** If code and spec disagree, fix the code. If the spec is wrong, stop and fix the spec first (this may require a retrofit or plan amendment).

**What done looks like:**

- All beads are closed (`br list` shows 0 open)
- Every bead was reviewed against the spec
- No unresolved review feedback remains

### Pass 4: Verify (verify)

**Output:** `03-verify.md`

Final verification that the complete implementation matches the spec. This is a holistic review — not per-bead, but the whole thing.

#### Agent Instructions

You are doing a final check that the implementation, taken as a whole, satisfies the spec. Individual beads were reviewed during the implement pass. This pass checks that the pieces fit together correctly.

**What to do:**

1. Read the source spec (`SPEC.md` or equivalent from the plan work).
2. Read the codebase as implemented.
3. Walk through every acceptance criterion in the spec. For each:
   - Verify it is implemented
   - Verify tests exist (if the criterion is testable via automated tests)
   - Note any gaps
4. Run the full test suite. All tests must pass.
5. Check for spec-code divergence:
   - Is there code that was implemented but is not in the spec? (scope creep)
   - Is there spec content that was not implemented? (gaps)
   - Flag both.
6. Write `03-verify.md` documenting:
   - Spec compliance status — which acceptance criteria pass, which have gaps
   - Test results summary
   - Any divergences found (implemented-but-not-spec'd, spec'd-but-not-implemented)
   - Recommendation: complete, or needs more work
7. If gaps are found, return to the implement pass to address them.
8. **Save to disk.** When all criteria are met, advance status to `complete`.

**What done looks like:**

- `03-verify.md` exists and documents full spec compliance
- All acceptance criteria from the spec are verified as implemented
- All tests pass
- No unaddressed divergences between spec and code
- The implementation is complete

### Pass 5: Complete (complete)

**Output:** (none)

The implementation is done. This is a terminal pass.

#### Agent Instructions

**What to do:**

1. Run `kerf square <codename>` to verify all expected artifacts exist.
2. Confirm all beads are closed: `br list` shows 0 open for this work.
3. The implementation work is complete. It can be archived.

**What done looks like:**

- `kerf square` passes
- All beads closed
- `03-verify.md` confirms spec compliance
- The work is done

## File Structure

A work governed by the `implementation` jig contains these files:

```
{codename}/
  spec.yaml
  SESSION.md
  01-breakdown.md
  02-dispatch.md
  03-verify.md
```

`spec.yaml` and `SESSION.md` are managed by kerf. Other files are produced by the passes.

The implement pass (Pass 3) is a process pass — its state lives in the bead database (`br`), not in work directory files. `kerf show` reports bead status for implementation works. See [jig-system.md](jig-system.md) §Process Passes vs. Artifact Passes.

When passes are deactivated (composable configuration), their output files are not expected and not checked by `kerf square`.

## Finalization

Implementation works are typically not finalized in the same way as spec-writing works — the code is already in the repository. The work can be archived directly via `kerf archive <codename>` when complete.

If the project wants to preserve the implementation record (breakdown, dispatch plan, verification report), `kerf finalize <codename>` copies the work artifacts into the repository.
