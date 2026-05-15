# Implement Beads

Execute the bead implementation plan using parallel worker pairs (implementer + reviewer) in isolated worktrees. The controller orchestrates handoffs — it never reads code or reviews diffs itself.

## Roles

| Role | What it does | What it does NOT do |
|------|-------------|---------------------|
| **Controller** | Dispatches beads, monitors progress, triggers merges, manages worktrees | Read diffs, review code, assess spec compliance |
| **Implementer** | Reads spec, writes code, writes tests, commits, responds to review feedback | Self-review, start next bead without approval |
| **Reviewer** | Reads diff + spec, checks compliance, sends targeted feedback via agent-mail | Write code, commit, close beads |

Each worktree has one implementer and one reviewer — two Claude sessions sharing the same directory. They take turns (never write concurrently) and communicate via agent-mail.

## Prerequisites

- `plans/{name}/beads.md` exists with full breakdown (from `plan-implementation` procedure)
- Beads are tracked in `bd` with dependency chains
- `bd ready` shows which beads are unblocked

## Step 1: Set Up Worktrees and Agent Pairs

Each parallel work stream gets a worktree with two agents: an implementer and a reviewer.

```bash
# Spawn with worktree isolation
# For N parallel streams, spawn 2N agents (N implementers + N reviewers)
ntm spawn kerf --cc=N --worktrees

# Add reviewer agents (one per worktree, sharing the same worktree directory)
# Reviewers don't need their own worktrees — they read the implementer's
ntm add kerf --cc=N
```

### Worktree layout (example with 3 streams)

```
main repo (controller)
├── worktree-1/  ←  implementer A + reviewer A
├── worktree-2/  ←  implementer B + reviewer B
└── worktree-3/  ←  implementer C + reviewer C
```

### Manual worktree setup

If workers are already running, create worktrees manually:

```bash
git worktree add ../kerf-worker-N -b bead/BEAD_ID main
```

Point both the implementer and reviewer panes at the same worktree directory.

### Register with agent-mail

Controller and all agents register so they can message each other:

```bash
# Controller registers (use macro_start_session MCP tool)
# Each worker registers on first message (include registration in prompt)
```

### Verify

```bash
ntm worktrees list
ntm activity kerf
```

## Step 2: The Bead Loop

For each bead, the controller orchestrates this cycle:

```
DISPATCH (to implementer)
  → implementer works, commits, signals DONE
TRIGGER REVIEW (to reviewer)
  → reviewer reads diff + spec, sends feedback or APPROVED
    ┌─ if feedback: implementer fixes, reviewer re-reviews (loop)
    └─ if approved: controller merges, resets, sends next bead
```

The controller's job is routing — it never evaluates code quality.

### 2a. Dispatch ONE Bead to Implementer

Send exactly one bead per prompt. Never chain multiple beads.

Write the prompt to a temp file, then send:

```bash
ntm send kerf --pane=N --file /tmp/bead_prompt.md
```

#### Implementer prompt template

```markdown
## Bead {ID}: {title}

**Specs to read:** {spec file paths}
**Package:** {package path}

### Deliverables
{copy from beads.md}

### Cross-cutting concerns
{any from beads.md that apply to this bead}

### Tests
{test requirements from beads.md}

### When done
1. Run `go build ./...` and `go test ./...` in this worktree
2. Commit: `git commit -m "feat: Bead {ID} — {title}"`
3. Send agent-mail to controller:
   - subject: `DONE: Bead {ID}`
   - body: summary of what was implemented, any spec ambiguities encountered
4. Wait for reviewer feedback. Do NOT start another bead.
```

### 2b. Wait for Implementer DONE Signal

Monitor via agent-mail inbox or `ntm logs`:

```bash
ntm logs kerf --panes=N --limit=30
# or poll agent-mail inbox for "DONE: Bead {ID}" message
```

### 2c. Trigger Review

When the implementer signals DONE, send the reviewer a review prompt. The reviewer is a separate Claude session on the **same worktree**.

```bash
ntm send kerf --pane=M --file /tmp/bead_review.md
```

#### Reviewer prompt template

```markdown
## Review: Bead {ID} — {title}

You are reviewing an implementation in this worktree. The implementer has
committed their work. Your job is to verify it matches the spec.

### Specs to check against
{same spec file paths as the implementer got}

### What to check

1. **Diff review:** Run `git diff main` to see all changes. Read every changed file.

2. **Spec compliance:**
   - Every requirement in the spec sections is implemented
   - No behaviors exist that aren't in the spec
   - Edge cases from the spec are handled

3. **Cross-cutting concerns:**
   {list from beads.md that apply to this bead}
   Verify each is wired in correctly.

4. **Test quality:**
   - Tests verify spec compliance, not just code correctness
   - Tests cover the deliverables listed in beads.md
   - Run `go test ./...` and confirm all pass

5. **Build:** Run `go build ./...`

### Your output

Send agent-mail to the implementer (cc: controller):

**If issues found:**
- subject: `REVIEW: Bead {ID} — changes needed`
- body: For each issue, quote the spec requirement, show what the code does
  instead, and state what needs to change. Be specific — line numbers, function
  names, exact spec language.

**If clean:**
- subject: `APPROVED: Bead {ID}`
- body: Brief confirmation of what was checked.

Do NOT write code or commit. Your only output is the review message.
```

### 2d. Review Loop (Implementer ↔ Reviewer)

The implementer and reviewer iterate via agent-mail without controller involvement:

1. **Reviewer sends feedback** → implementer receives it, applies fixes, commits, replies `FIXED: Bead {ID}`
2. **Reviewer checks the new diff** → sends `APPROVED` or more feedback
3. Repeat until `APPROVED`

The controller monitors for the `APPROVED` message. It does not participate in the review conversation.

If the loop exceeds 3 rounds without approval, the controller intervenes — this usually means the spec is ambiguous and needs a decision, not more code changes.

### 2e. Merge to Main

When the reviewer sends `APPROVED`:

```bash
# Merge the worktree branch to main
ntm worktrees merge cc_N

# Or manually
cd /path/to/main/repo
git merge ntm/kerf/cc_N --no-ff -m "feat: Bead {ID} — {title}"

# Verify full suite on main
go build ./...
go test ./...
```

If tests fail on main after merge (integration issues with other merged beads), send the implementer a fix prompt with the failure output.

### 2f. Reset and Send Next Bead

1. **Reset the worktree** to updated main:
   ```bash
   git -C <worktree-path> fetch origin  # if needed
   git -C <worktree-path> reset --hard main
   ```

2. **Clear both agents' contexts** — mandatory before the next bead:
   ```bash
   ntm send kerf --pane=N "/clear"   # implementer
   ntm send kerf --pane=M "/clear"   # reviewer
   ```

3. Wait for clears to complete.

4. Check what's unblocked:
   ```bash
   bd ready
   ```

5. Pick the next bead and go back to step 2a.

## Step 3: Layer Transitions

When all beads in a layer are done and merged to main:

1. Run full test suite on main: `go test ./...`
2. Run full build on main: `go build ./...`
3. Reset all worktrees to updated main:
   ```bash
   git -C <worktree-path> reset --hard main
   ```
4. Clear all agent contexts
5. Check `bd ready` — next layer's beads should now be unblocked
6. Redistribute agent pairs across the new layer's beads

## Step 4: Sizing

- 2–3 parallel streams is the sweet spot. Each stream = 1 implementer + 1 reviewer on the same worktree.
- One bead per prompt. Never chain. `/clear` between beads.
- Prefer non-overlapping files across streams; merge handles overlap but conflicts cost time.

## Step 5: Completion

When all beads are closed and merged:

```bash
# Verify zero remaining
bd list

# Full test suite on main
go test ./...

# Full build on main
go build ./...

# Review overall git history
git log --oneline | grep "feat: Bead"

# Clean up worktrees
ntm worktrees clean
# or
git worktree prune
```

Update TASKS.md to mark the implementation phase complete.

