# kerf real-corpus extraction — findings

## Coverage

- **52 beads extracted**, one row each, no duplicates.
- **0 unmatched** (every committed `kerf-XXX` bead in `git log --all` linked to an orchestrator dispatch + a sub-agent transcript).
- **2 wasted-effort sub-agents** (`kerf-cdh`, `kerf-0kx`) — both ran on a `worktree-agent-*` branch and produced no commit that landed on main.
- **24 of 52 beads (46%)** had a sibling reviewer sub-agent dispatched against the same description token; the other 28 were merged without a separate reviewer pass.
- 215 total sub-agents parsed across 11 orchestrator sessions; 136 of those had descriptions that did not mention a `kerf-XXX` ID (research/planning/spec-edit agents) and were not included.

## Per-phase distribution preview (seconds)

| phase | n | min | median | p95 | max |
|---|---|---|---|---|---|
| spin_up | 52 | 2.4 | 4.0 | 6.0 | 7.4 |
| task_work | 52 | 41.5 | 216.6 | 521.1 | 696.4 |
| merge | 52 | 0.0 | 0.0 | 0.0 | 0.1 |
| reviewer | 24 | 60.6 | 93.4 | 133.2 | 146.1 |
| total | 52 | 44.7 | 220.4 | 525.0 | 700.0 |

Spin-up is very tight (2–8s); task_work spans an order of magnitude (40s–700s) and dominates total. Merge is effectively 0 — this corpus is direct-to-main only.

## Structural surprises

1. **No `duration_ms` field on sub-agent assistant messages.** Only token-usage. The prior agent's "JSONL duration_ms" sanity-check must have come from the orchestrator side. I substituted last-event-minus-first-event wall-clock (ms) in that column.
2. **`git commit` author-date precedes the Bash tool_use timestamp** by 0.1–0.2s consistently. The Bash tool_use timestamp is queue-time; commit runs inside the call. Negative `merge_seconds` were clamped to 0 and flagged in `notes` (`merge_clamped_neg_*`).
3. **Linkage by description string works perfectly** — all 214 meta.json descriptions in the corpus are unique, so meta.description ↔ orchestrator Task.description is an exact join. No fuzzy matching needed.
4. **No `tool_use_id` in sub-agent transcripts.** The connecting key is `description`, not the orchestrator's `toolu_*` id.
5. Several beads (e.g. `kerf-v0n`) show two `git commit` Bash calls (first attempt amended/retried); flagged via `N_commit_calls` note.
