# Agent-UX Critique

Lens: fresh autonomous agent runs `kerf` → `kerf next` → tries to work. What unsticks it?

## Highest agent value (ship these first regardless of technical priority)

- **P0.2 corrupt spec.yaml silently drops the work** — silent disappearance is the single worst agent failure mode; an agent cannot reason about what it cannot see. Emit a warning row.
- **P0.1 work_codename: null + spurious `work_no_attached_beads`** — agent sees "no attached beads" cleanup item and follows the wrong remediation (edits filter) when the real bug is attachment. Wrong-action loop.
- **P1.9 no `project.yaml` warning** — uninitialized vs empty is *the* fresh-agent question. `kerf next` returning "no actionable items" in an uninitialized repo sends the agent down the wrong branch (writes code) instead of running `kerf init`. Same root as workflow #15.
- **Workflow #1 (top-level orientation omits init/setup/next/map)** — fresh agent never learns `kerf next` exists from bare `kerf`. Highest leverage per byte of fix. (workflow improvements item, but spec-mandated.)
- **Workflow #3 snapshot-test `kerf next --help` against spec** — spec says help is the agent's contract; it currently lies. Block drift forever.
- **P1.6 unknown statuses exclude works** — invisible work is unfixable work.
- **P1.8 bare `kerf` global count vs project count** — agent in repo orients on wrong project.

## Looks important, low agent value

- **P0.3 / P0.4 `bd list --project` / `bd list` shells failing silently** — technically correct fix, but agents already query beads via other paths; the visible bead counts on `show` / `square` are not in the agent's daily decision loop (they use `kerf next`). Low daily impact.
- **P1.5 `EqualFold` in legacy `ForWork`** — case-sensitivity correctness; agent will not notice until it deliberately constructs a mixed-case label experiment.
- **P1.2 `detectBeadFilter` never fires** — auto-detect is one-shot at init; once configured, agents don't re-encounter it. Useful, not load-bearing.
- **P2 polish items as a class** (help-text omissions, `--force`-vs-`--yes` alias, negative `--limit`) — agent re-reads help rarely; once oriented, polish doesn't move the needle.
- **Sim integrity items** — invisible to a working agent entirely; matters for tuning scoring, not for the loop.
- **Design/scoring bucket** — same: ranking quality is downstream of "does the agent see the right items at all". Fix visibility first.

## Agent-UX gaps in the list (missing items)

- **Concurrency / "another agent owns this work"** — `kerf next` does not signal that a top-ranked work has an active session held by a different agent. Two parallel agents will both pick item #1. Proposed: surface `active_session != null AND not mine` as a cleanup/warning item, or annotate bead items with "owned by session X".
- **"What pass am I on and what does it want?" rehydration command** — workflow #12 is in the list but ranked low. For an agent that just cleared context, this is the #1 daily question. `kerf where` (P1.10/improvement) partially covers it but the action list buries it.
- **`kerf next` JSON shape stability contract** — agents will script against `--format=json`; no item in the list locks the field set or version-bumps on changes. Drift here silently breaks agent loops.
- **Stuck-loop detection** — an agent that runs `kerf next` → does action → re-runs and gets the *same* top item has no signal it's looping. No "you attempted this bead N times" surfacing.
- **Bead → work attribution when one bead matches many works** — spec says "attributed to every work it matches"; output shows only one `work:` column. Agent picks wrong work to advance.
- **Exit codes for `kerf next`** — agent orchestrators need "0 = items, 1 = empty, 2 = warnings-only" to drive loops. Not specified.
- **Stale `active_session` recovery is not in `kerf next`** — agent gets `Error: active session exists` from `resume` with no item nudging it toward `--force`.
- **`kerf init` non-interactive behavior under stdin-not-TTY** — workflow #5 mentions it; agents *always* hit this path. Deserves its own action item: verify non-interactive default works end-to-end and is what fresh agent gets.

## Triage workflow agent test

For each of the 7 proposed commands:

1. **`kerf show <codename>` renders attached beads** — **daily yes**. Closes "what's left on this work after context clear" loop. High value.
2. **`kerf map` adds bead counts per row** — **sometimes**. Portfolio view; agents working a single work won't hit it daily. Closes orchestrator-overview loop.
3. **`kerf new --bead-filter '<spec>'`** — **rarely**. One-shot at creation; agent sets and forgets.
4. **`kerf work edit --bead-filter-add/remove`** — **rarely**. Remediation for the `work_no_attached_beads` cleanup item — useful when triggered, not daily. Closes filter-misconfig loop.
5. **`kerf attach <codename> <bead-id>`** — **rarely**. Escape hatch for filter not catching a specific bead. Bureaucratic unless the filter is wrong (then fix the filter).
6. **`kerf triage` report** — **sometimes** (orchestrator); **rarely** (worker). Closes "what needs human attention" loop. The `--resolved` exit code is the load-bearing piece for CI/agent loops; without it, this is a dashboard nobody reads.
7. **Drift state file** — **never directly**; infra for #6. Worth doing only if #6 ships.

Verdict: ship #1 first (closes a real loop), ship #6 only with the `--resolved` exit code (otherwise bureaucratic), #2 is nice, #3–#5 are config plumbing that agents rarely touch.

## Workflow improvements re-ranked (for agent value)

1. **#3 snapshot-test `kerf next --help`** — locks the agent contract.
2. **#2 `kerf where` / `kerf doctor`** — single-shot rehydration after context clear; replaces 3 commands. Pair with workflow #12 (rehydrate pass instructions).
3. **#5 `kerf init` copy-pasteable agent-instruction block** — fresh-agent onboarding artifact.
4. **#10 `kerf verify-tools`** — catches missing `br`/`ntm` before the agent silently fails.
5. **#7 reconcile pass/status/stage terminology** — agents will parrot whatever they read; consistent vocabulary reduces hallucinated commands like `kerf stage`.
6. **#4 `kerf next --explain <rank>`** — only useful once an agent distrusts the ranking. Day-30 feature.
7. **#6 `kerf shelve --session-file path`** — convenience; agents can already write SESSION.md.
8. **#9 `kerf status --auto`** — risky (infers state from files); agents prefer explicit. Lowest agent value.
9. **#1 (already #1 above)**.

Day-1-agent ranking: 1, 2, 3, 4, 5. Rest are "useful once you know kerf".
