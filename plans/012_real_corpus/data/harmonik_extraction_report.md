# Harmonik corpus extraction report

- Top-level sessions available: 84
- Sessions processed: 60 (newest-first, MAX=60)
- Beads extracted (clean rows): 150 (capped from 384 candidates)
- Unmatched/skipped: 44
- Wasted (abandoned worktrees, no commit): 123

## Per-phase distribution (seconds)

| phase | n | min | median | p95 | max |
|---|---|---|---|---|---|
| spin_up_seconds | 150 | 1.8 | 3.1 | 5.0 | 5.6 |
| task_work_seconds | 150 | 16.2 | 267.2 | 1219.3 | 6559.3 |
| merge_seconds | 68 | -51.5 | -0.3 | 165.0 | 362.5 |
| reviewer_seconds | 0 | - | - | - | - |
| total_seconds | 150 | 21.0 | 277.7 | 1366.4 | 6562.1 |

## Bead-density per session

- Sessions yielding >=1 bead: 16 of 60
- Beads/session: min=1, median=9, max=31
- Distribution: [1, 1, 3, 4, 4, 5, 6, 7, 9, 10, 12, 12, 13, 14, 18, 31]

## Structural notes vs kerf corpus

- Sub-agents dispatched via `Agent` tool (not `Task`/`TaskCreate`). Linkage is via tool_result text `agentId: a...`, not a structured field.
- Sub-agent transcripts at `<session>/subagents/agent-<agentId>.jsonl` with sibling `.meta.json` (agentType, description).
- Bead IDs: `hk-<slug>(.<subnum>)*`. Pattern matched in dispatch description.
- Implementers run in worktrees (isolation=`worktree`). SHA recovery: parse `[branch sha]` from the sub-agent's own `git commit` tool_result; fallback to orchestrator `Updating old..new` merge output.
- Commit messages are free-form (no enforced bead-id prefix), so message-grep is not a reliable linkage path.
- `reviewer_seconds` is 0 here: the 16 productive sessions in the newest-first window do not dispatch per-bead reviewer agents. Older sessions (e.g. `d26d3203-...`) do dispatch `Review hk-X` Agents and would populate the column; they fell outside the 60-session cap.
- `merge_seconds` distribution is bimodal: a sub-second cluster (negative-to-zero) is clock skew between the tool_use emission ts (API side) and the git author-date set locally when `git commit` actually runs; the p95=165s tail is real queueing — parallel agents serializing on the index lock / pre-commit hooks.
- 123 sub-agents filtered to `harmonik_wasted_effort.csv` (dispatched, ran in a worktree, never committed).
