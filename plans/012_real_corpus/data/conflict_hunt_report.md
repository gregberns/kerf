# Conflict-hunt report — Plan 012 real-corpus pass

## Coverage

| Corpus | Sessions scanned | Ephemeral worktree dirs skipped |
|---|---|---|
| kerf | 11 | n/a |
| harmonik | 84 | yes (`*--harmonik-worktrees-*`) |

Both top-level session JSONLs and per-session `subagents/agent-*.{jsonl,meta.json}` files were scanned. Sub-agent start timestamps were recovered from the first JSONL record when the `meta.json` lacked a `createdAt` field.

## Totals by pattern

| Pattern | Count | Notes |
|---|---|---|
| 1 — `git push` rejected | 7 | All in harmonik. None observed in kerf. |
| 2 — `CONFLICT (content)` / `Merge conflict in` | 24 | All in harmonik. Tight clusters within single sessions (e.g. `14c78c5f` had 5 conflict markers within ~25 min). |
| 3 — long session (>2h, >5 sub-agents) | 30 | 2 kerf, 28 harmonik. 9 of these also tripped pattern 1 or 2. |
| 4 — sub-agent retried with identical description | 1 | Identical-description retries are rare; the orchestrator tends to vary the prompt on retry (e.g. add "fix"/"v2"), so this pattern under-counts. |

## Pattern catalog (resolution shapes)

Profile across the 31 P1/P2 incidents, classifying Bash commands within `[incident_start, incident_end + 5 min]`. Token sequence is de-duplicated and order-preserved.

| Shape | n | Median duration | Interpretation |
|---|---|---|---|
| `plain_push` only | 9 | 177 s | Push rejected, agent re-pushed shortly after (likely after a silent fetch/merge that didn't surface in this token set). Fast-resolve case. |
| `stash` (in window) | 5 | ~33 min | Conflict on dirty tree → stash → continue. Long-tail because some `stash`-class incidents bled into next session step. |
| `rebase_other` (rebase started, no continue logged) | 4 | 5 s | Conflict appeared mid-rebase; resolution itself wasn't captured in window (probably manual edits then `--continue` outside the 5-min slice). |
| `rebase_cont` then `plain_push` | 1 | 10 s | Classic rebase resolve → push. Very fast. |
| `rebase_cont` then `rebase_abort` | 1 | 27 s | Tried to continue, gave up, aborted. Recovery rollback. |
| `checkout_path` → `plain_push` | 1 | 396 s | Used `git checkout -- <path>` to discard local then re-push. |
| `plain_push` → `reset_hard` → `stash` → `plain_push` (×3) | 1 | 30 s | Thrash: multiple stash/push churn — diagnostic of an over-reactive recovery loop. |
| `fetch` → `rebase` → `force_push` (×2) | 1 | 1 425 s | Long-form: fetched, rebased, force-pushed; remote diverged again; repeated. |
| `no_resolution_cmds_detected` | 6 | 20 min | Marker fired but agent likely fixed via Edit tool (file rewrite) rather than git CLI. |

**Take-aways for simulator design:**

- **Fast P1 resolves** dominate (~10x cases in <5 min) — the *modal* outcome of a push rejection in this corpus is a quick re-push, suggesting the simulator's conflict model should weight "trivial reject + retry" heavily.
- **Long-tail P2 conflicts** (≥30 min) cluster around 1-3 cases per session and are characterized by either repeated `stash`/`rebase` churn or absence of git CLI activity (Edit-tool resolves).
- **Force-pushes are rare** (only 1 of 31 incidents used `--force` / `--force-with-lease`). Worth noting given the user's earlier guidance about force-push hygiene.
- **35h outlier** (`4c89151e`): one P1 has a 127 610 s window because no clean "successful push" signal followed the rejection — this is a measurement artifact, not a 35-hour conflict. Should be filtered in any distribution fit.

## Bead-of-data sufficiency

31 incidents across patterns 1 and 2 — comfortably above the "~5+" threshold for fitting a rough distribution. **Plan 012 pillar C does NOT need expert-judgment defaults**; we have enough shape data for an empirical bootstrap of (a) the rate of conflicts per long-session hour and (b) the conditional duration given a conflict occurred. The duration distribution is clearly bimodal (sub-5-min trivials vs. 20-min+ long-tail) and the simulator should reflect that.

Pattern 4 (n=1) is unreliable. If the simulator needs a retry-rate prior, derive it from session-level features (long-session × conflict-marker correlates at 9/30 = 30%) rather than the literal P4 signal.

## Useful regex patterns for the eventual extractor

```
# Pattern 1 — push rejected
(non-fast-forward|Updates were rejected|failed to push|! \[rejected\])

# Pattern 2 — merge/rebase conflict
(CONFLICT \(content\)|CONFLICT \(modify/delete\)|Merge conflict in|Automatic merge failed)

# Successful-push end markers (for window closing)
(To github\.com|To gitlab|Everything up-to-date|Successfully rebased)

# Resolution-flavor classification (case-insensitive)
git\s+pull\s+--rebase
git\s+rebase\s+--(continue|abort|skip)
git\s+push.*--force(-with-lease)?
git\s+reset\s+--hard
git\s+stash
git\s+checkout\s+[^-]

# Bead-id linker (kerf project)
kerf-[a-z0-9]{2,6}
```

## Files written

- `/Users/gb/github/kerf/plans/012_real_corpus/data/conflict_incidents.csv` — 62 rows (P1+P2+P3+P4)
- `/Users/gb/github/kerf/plans/012_real_corpus/data/long_sessions.csv` — 30 rows
- `/Users/gb/github/kerf/plans/012_real_corpus/data/conflict_hunt_report.md` — this file

Extractor scripts kept at `/tmp/scan_conflicts.py`, `/tmp/profile_resolutions.py`, `/tmp/annotate_beads.py` — these are scratch scripts, not committed.
