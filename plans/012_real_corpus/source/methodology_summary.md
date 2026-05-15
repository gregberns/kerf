# Duration-Extraction Methodology — Summary

> Condensed from two parallel investigation agents on 2026-05-15. Their full reports remain in session transcripts; this is the operational summary.

## Verdict: hybrid pipeline, programmatic-heavy

Both investigators converged independently on the same answer.

| Layer | Tool | Cost | Coverage |
|---|---|---|---|
| 1. Skeleton extractor | Go or Python | ~6–10 engineer-hours | All sessions, structural fields only |
| 2. Programmatic phase tagger | Same tool | included | 70–80% of events, deterministic |
| 3. LLM classifier on ambiguous spans | Sonnet 4.6 with cache | ~$50 one-time | The remaining 20–30% (boundary cases) |
| 4. Distribution fitter | scipy or Go gonum | trivial | Per-phase output |

## Where signals come from

- Sub-agent transcripts: `~/.claude/projects/<proj>/<sessionId>/subagents/*.jsonl`
- Per-sub-agent metadata: `<sessionId>/subagents/agent-*.meta.json` — has `description` (de facto bead label).
- Ground-truth duration: every assistant message in the JSONL carries `message.usage.duration_ms`. Cross-check extracted phase sums against this.
- Git history: `git log` for the commit SHA written by the sub-agent, to get the actual integration timestamp.

## Phase signal table (programmatic)

| Phase | Start | End | Reliability |
|---|---|---|---|
| Spin-up | Orchestrator `TaskCreate` ts | First sub-agent `tool_use` ts | High |
| Reconnaissance | (subsumed by spin-up per user decision) | First `Write`/`Edit` ts | High |
| Task work | First `Write`/`Edit` ts | Sub-agent `git commit` Bash ts | High |
| Reviewer round-trip | Implementer commit ts | Reviewer's last assistant ts | Moderate (text-match on bead ID) |
| Merge | Sub-agent `git commit` ts | `git log` integration-branch ts | Mixed (near-zero in this corpus) |
| Conflict-merge delta | n/a | n/a | Not in corpus — synthesize |

## What LLM is needed for

Concrete cases:
1. Spin-up vs task-work boundary inside reconnaissance (e.g., is "Read internal/feed/warning.go" exploration or implementation?)
2. Orchestrator-vs-worker time on interleaved sessions (orchestrator answering meta questions while three sub-agents are in flight)
3. Conflict resolution scoping (which Reads/Edits address the conflict vs the feature?)
4. Bead-attribution when text doesn't carry the bead ID

## Cost math

- kerf corpus: ~44MB, 11–19 JSONL files
- harmonik corpus: ~454MB, 152 JSONL files
- Total raw: ~500MB, ~167M tokens at 3 chars/token
- Skeleton compression: ~10x → ~17M structured tokens
- Sonnet hybrid (ambiguous spans only, ~5K LLM calls × 3K context): **~$50 one-time**
- Pure-LLM would be ~$400 — wasteful for cases regex handles.

## End-to-end verification — bead kerf-665

Trace through `a2ade8c4-…` (orchestrator) + `agent-a2d4dd50690b3b3a3` (implementer):

- Orchestrator `TaskCreate`: 18:08:58.067Z
- First implementer event: 18:27:59.416Z (19m queue delay)
- First sub-agent tool use (`bd show kerf-665`): 18:28:03.296Z → spin-up = **3.9s**
- First `Write` to `cmd/triage.go`: 18:31:40.187Z → reconnaissance = **3.6 min**
- `git commit` Bash call: 18:34:58.072Z → write→commit = **3.3 min**
- Commit on main `a78df33`, author date 18:34:58 UTC → merge delta = **0**
- Total sub-agent wall clock: **7m28s** — matches the JSONL's own `duration_ms` within a second.

## Hard blockers

1. **Merge-under-conflict cannot be measured** in either corpus — direct-to-main workflow. Plan 012 pillar C synthesizes this.
2. **Sub-agent work is opaque from the parent transcript** — only the Agent tool call + return are visible there. Must walk child JSONLs.
3. **Cross-session reviewers** require text-matching on bead ID, ~90% reliable.
4. **Abandoned `worktree-agent-*` branches** — work is in the JSONL but no commit lands. Filter these from duration stats, keep a wasted-effort counter.
