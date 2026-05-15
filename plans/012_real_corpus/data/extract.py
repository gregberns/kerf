#!/usr/bin/env python3
"""Extract per-bead phase durations from Claude Code session transcripts."""
import json, glob, os, re, subprocess, csv, sys
from datetime import datetime
from collections import defaultdict

ROOT = '/Users/gb/.claude/projects/-Users-gb-github-kerf'
REPO = '/Users/gb/github/kerf'
OUT = '/Users/gb/github/kerf/plans/012_real_corpus/data'

BEAD_RE = re.compile(r'kerf-[a-z0-9]{3,}', re.I)
COMMIT_MSG_RE = re.compile(r'-m\s+["\']?([^"\']{5,200})', re.S)


def parse_ts(s):
    if not s: return None
    s = s.replace('Z', '+00:00')
    return datetime.fromisoformat(s).timestamp()


def load_jsonl(path):
    out = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line: continue
            try:
                out.append(json.loads(line))
            except Exception:
                pass
    return out


def iter_tool_uses(events):
    """Yield (ts_epoch, ts_str, tool_name, tool_input, tool_id, event) for each tool_use."""
    for o in events:
        msg = o.get('message') or {}
        content = msg.get('content')
        if not isinstance(content, list): continue
        ts = o.get('timestamp')
        epoch = parse_ts(ts)
        for c in content:
            if isinstance(c, dict) and c.get('type') == 'tool_use':
                yield epoch, ts, c.get('name', ''), c.get('input', {}) or {}, c.get('id', ''), o


def iter_assistant_messages(events):
    for o in events:
        if o.get('type') != 'assistant': continue
        ts = o.get('timestamp')
        yield parse_ts(ts), ts, o


def first_bead_id(s):
    if not s: return None
    m = BEAD_RE.search(s)
    if not m: return None
    bid = m.group(0).lower()
    # skip catch-all like "kerf-corpus"
    if bid in ('kerf-corpus',): return None
    return bid


def collect_git_log():
    """Return: list of (sha, author_epoch, subject), and message->sha index, and bead_id->shas index."""
    res = subprocess.run(
        ['git', '-C', REPO, 'log', '--all', '--format=%H%x09%aI%x09%s'],
        capture_output=True, text=True, check=True)
    rows = []
    bead_index = defaultdict(list)
    for line in res.stdout.strip().split('\n'):
        sha, aiso, subject = line.split('\t', 2)
        epoch = parse_ts(aiso)
        rows.append((sha, epoch, subject, aiso))
        bid = first_bead_id(subject)
        if bid:
            bead_index[bid].append((sha, epoch, subject, aiso))
    return rows, bead_index


def main():
    git_rows, git_bead_index = collect_git_log()
    print(f'git: {len(git_rows)} commits, {len(git_bead_index)} bead-tagged', file=sys.stderr)

    # 1) Build orchestrator description -> (dispatch_ts, session_id) map
    orch_dispatch = {}  # desc -> (epoch, ts_str, session_id, tool_use_id)
    orch_jsonls = sorted(glob.glob(ROOT + '/*.jsonl'))
    for op in orch_jsonls:
        session_id = os.path.basename(op).replace('.jsonl', '')
        ev = load_jsonl(op)
        for epoch, ts, name, inp, tid, _o in iter_tool_uses(ev):
            if name not in ('Task', 'TaskCreate', 'Agent'): continue
            desc = inp.get('description') or inp.get('subject') or ''
            if not desc: continue
            # only earliest dispatch per desc
            if desc not in orch_dispatch or (epoch and epoch < orch_dispatch[desc][0]):
                orch_dispatch[desc] = (epoch, ts, session_id, tid)
    print(f'orchestrator dispatches indexed: {len(orch_dispatch)}', file=sys.stderr)

    # 2) Walk sub-agent meta files
    metas = sorted(glob.glob(ROOT + '/*/subagents/agent-*.meta.json'))
    rows_out = []
    unmatched = []
    wasted = []

    # For reviewer linkage: group sub-agents by bead_id, track which committed.
    # First pass: parse each meta + transcript landmarks.
    parsed = []  # list of dicts
    for mp in metas:
        meta = json.load(open(mp))
        desc = meta.get('description', '')
        bead_id = first_bead_id(desc)
        agent_jsonl = mp.replace('.meta.json', '.jsonl')
        sess_dir = os.path.basename(os.path.dirname(os.path.dirname(mp)))
        sub_agent_id = os.path.basename(agent_jsonl).replace('agent-', '').replace('.jsonl', '')
        if not os.path.exists(agent_jsonl):
            unmatched.append(f'{sub_agent_id}\tno_transcript\t{desc}')
            continue
        ev = load_jsonl(agent_jsonl)
        if not ev:
            unmatched.append(f'{sub_agent_id}\tempty_transcript\t{desc}')
            continue
        # Branch
        branch = None
        for o in ev:
            if o.get('gitBranch'):
                branch = o.get('gitBranch'); break

        # Landmarks
        first_tool_epoch = None
        first_tool_ts = None
        commits = []  # (epoch, ts_str, cmd)
        last_assistant_epoch = None
        last_assistant_ts = None
        first_event_epoch = None
        last_event_epoch = None
        for o in ev:
            te = parse_ts(o.get('timestamp'))
            if te is not None:
                if first_event_epoch is None: first_event_epoch = te
                last_event_epoch = te
        for epoch, ts, name, inp, tid, o in iter_tool_uses(ev):
            if first_tool_epoch is None and epoch is not None:
                first_tool_epoch = epoch; first_tool_ts = ts
            if name == 'Bash':
                cmd = inp.get('command', '') or ''
                if re.search(r'\bgit\s+commit\b', cmd):
                    commits.append((epoch, ts, cmd))
        for epoch, ts, o in iter_assistant_messages(ev):
            if epoch is not None:
                last_assistant_epoch = epoch; last_assistant_ts = ts
        total_duration_ms = int((last_event_epoch - first_event_epoch) * 1000) if first_event_epoch and last_event_epoch else 0

        parsed.append({
            'meta_path': mp,
            'agent_jsonl': agent_jsonl,
            'sub_agent_id': sub_agent_id,
            'session_id': sess_dir,
            'description': desc,
            'bead_id': bead_id,
            'branch': branch,
            'first_tool_epoch': first_tool_epoch,
            'first_tool_ts': first_tool_ts,
            'commits': commits,
            'last_assistant_epoch': last_assistant_epoch,
            'last_assistant_ts': last_assistant_ts,
            'total_duration_ms': total_duration_ms,
        })

    print(f'parsed sub-agents: {len(parsed)}', file=sys.stderr)

    # 3) Determine which sub-agents are "implementer" vs "reviewer" for a bead.
    # Heuristic: an implementer has at least one `git commit` Bash call AND its
    # description does NOT start with "Review" / contain "review".
    by_bead = defaultdict(list)
    for p in parsed:
        if p['bead_id']:
            by_bead[p['bead_id']].append(p)

    # 4) Emit rows
    for p in parsed:
        desc = p['description']
        bead_id = p['bead_id']
        if not bead_id:
            # not a bead implementation sub-agent
            continue
        # Reviewer detection
        is_reviewer = bool(re.search(r'\breview', desc, re.I))

        # Dispatch lookup
        dispatch = orch_dispatch.get(desc)
        if dispatch is None:
            # try less-strict: search across orchestrator descs containing same bead AND distinctive token
            cand = [(d, v) for d, v in orch_dispatch.items() if bead_id in d.lower()]
            if len(cand) == 1:
                dispatch = cand[0][1]

        # Skip reviewers from main row output (we'll link them onto implementers)
        if is_reviewer:
            continue

        commits = p['commits']
        # Wasted-effort filter: on a worktree branch with no main commit for this bead
        bead_main_commits = git_bead_index.get(bead_id, [])
        if not commits or not bead_main_commits:
            if (p['branch'] or '').startswith('worktree-agent-') and not bead_main_commits:
                total_sec = ''
                if dispatch and p['last_assistant_epoch']:
                    total_sec = round(p['last_assistant_epoch'] - dispatch[0], 1)
                elif p['first_tool_epoch'] and p['last_assistant_epoch']:
                    total_sec = round(p['last_assistant_epoch'] - p['first_tool_epoch'], 1)
                wasted.append((bead_id, total_sec, p['sub_agent_id'], desc))
                continue

        if not dispatch:
            unmatched.append(f'{p["sub_agent_id"]}\tno_orchestrator_dispatch\t{desc}')
            continue

        dispatch_epoch, dispatch_ts, dispatch_sess, _tid = dispatch

        # Phase: spin_up
        first_tool_epoch = p['first_tool_epoch']
        spin_up = None
        if first_tool_epoch and dispatch_epoch:
            spin_up = first_tool_epoch - dispatch_epoch

        # Phase: task_work — first_tool -> last `git commit` Bash invocation
        commit_call_epoch = commits[-1][0] if commits else None
        task_work = None
        if first_tool_epoch and commit_call_epoch:
            task_work = commit_call_epoch - first_tool_epoch

        # Match the bead's commit SHA: prefer the commit whose author-date is
        # closest-after the Bash `git commit` invocation.
        commit_sha = ''
        commit_epoch = None
        commit_iso = ''
        notes = []
        if commit_call_epoch and bead_main_commits:
            # closest-after, else closest overall
            after = [c for c in bead_main_commits if c[1] and c[1] >= commit_call_epoch - 5]
            pick = None
            if after:
                pick = min(after, key=lambda c: c[1] - commit_call_epoch)
            else:
                pick = min(bead_main_commits, key=lambda c: abs(c[1] - commit_call_epoch))
                notes.append('commit_sha_match_loose')
            commit_sha, commit_epoch, _subj, commit_iso = pick

        merge_sec = None
        if commit_call_epoch and commit_epoch:
            merge_sec = commit_epoch - commit_call_epoch
            if merge_sec < 0:
                # Bash tool_use ts is when call was queued; git author-date stamped
                # when commit ran (inside that Bash call), so author-date can be
                # slightly earlier. Floor to 0 — direct-to-main merge is effectively 0s.
                notes.append(f'merge_clamped_neg_{round(merge_sec,1)}s')
                merge_sec = 0.0

        # Reviewer: find a sibling sub-agent for same bead with "review" in desc
        reviewer_sec = None
        rev_candidates = [q for q in by_bead[bead_id] if q is not p and re.search(r'\breview', q['description'], re.I)]
        if rev_candidates:
            # pick the one dispatched after our commit (if any), else first
            chosen = None
            for q in rev_candidates:
                qd = orch_dispatch.get(q['description'])
                if qd and commit_call_epoch and qd[0] and qd[0] >= commit_call_epoch - 60:
                    chosen = q; break
            if chosen is None:
                chosen = rev_candidates[0]
                notes.append('reviewer_match_loose')
            if chosen['last_assistant_epoch'] and commit_call_epoch:
                reviewer_sec = chosen['last_assistant_epoch'] - commit_call_epoch

        # Total
        total_sec = None
        end_epoch = commit_epoch or commit_call_epoch or p['last_assistant_epoch']
        if dispatch_epoch and end_epoch:
            total_sec = end_epoch - dispatch_epoch

        if not commits:
            notes.append('no_git_commit_in_transcript')
        if len(commits) > 1:
            notes.append(f'{len(commits)}_commit_calls')
        if not commit_sha:
            notes.append('no_main_commit_for_bead')

        rows_out.append({
            'bead_id': bead_id,
            'session_id': p['session_id'],
            'sub_agent_id': p['sub_agent_id'],
            'description': desc,
            'spin_up_seconds': round(spin_up, 1) if spin_up is not None else '',
            'task_work_seconds': round(task_work, 1) if task_work is not None else '',
            'merge_seconds': round(merge_sec, 1) if merge_sec is not None else '',
            'reviewer_seconds': round(reviewer_sec, 1) if reviewer_sec is not None else '',
            'total_seconds': round(total_sec, 1) if total_sec is not None else '',
            'jsonl_duration_ms': p['total_duration_ms'],
            'dispatch_ts_utc': dispatch_ts or '',
            'commit_sha': commit_sha,
            'commit_ts_utc': commit_iso,
            'notes': ';'.join(notes),
        })

    # Write outputs
    fieldnames = ['bead_id', 'session_id', 'sub_agent_id', 'description',
                  'spin_up_seconds', 'task_work_seconds', 'merge_seconds',
                  'reviewer_seconds', 'total_seconds', 'jsonl_duration_ms',
                  'dispatch_ts_utc', 'commit_sha', 'commit_ts_utc', 'notes']
    with open(OUT + '/kerf_beads.csv', 'w', newline='') as f:
        w = csv.DictWriter(f, fieldnames=fieldnames)
        w.writeheader()
        for r in rows_out:
            w.writerow(r)

    with open(OUT + '/kerf_unmatched.txt', 'w') as f:
        for line in unmatched:
            f.write(line + '\n')

    with open(OUT + '/kerf_wasted_effort.csv', 'w', newline='') as f:
        w = csv.writer(f)
        w.writerow(['bead_id', 'total_seconds', 'sub_agent_id', 'description'])
        for r in wasted:
            w.writerow(r)

    print(f'rows: {len(rows_out)}, unmatched: {len(unmatched)}, wasted: {len(wasted)}', file=sys.stderr)
    return rows_out


if __name__ == '__main__':
    main()
