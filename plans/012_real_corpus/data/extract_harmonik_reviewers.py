#!/usr/bin/env python3
"""Extract per-bead phase durations from harmonik Claude Code sessions, oldest-first,
keeping only beads with a non-null reviewer_seconds value.

Schema matches harmonik_beads.csv:
  bead_id, session_id, agent_id, reviewer_agent_id, commit_sha,
  orchestrator_dispatch_ts, first_subagent_tool_ts, commit_ts, commit_author_ts,
  reviewer_last_assistant_ts, spin_up_seconds, task_work_seconds, merge_seconds,
  reviewer_seconds, total_seconds, notes
"""
import json, glob, os, re, subprocess, csv, sys
from datetime import datetime
from collections import defaultdict

ROOT = '/Users/gb/.claude/projects/-Users-gb-github-harmonik'
REPO = '/Users/gb/github/harmonik'
OUT = '/Users/gb/github/kerf/plans/012_real_corpus/data'

MAX_SESSIONS = 80
MIN_REVIEWER_ROWS = 30

BEAD_RE = re.compile(r'hk-[a-z0-9]+(?:\.[a-z0-9]+)*', re.I)


def parse_ts(s):
    if not s:
        return None
    s = s.replace('Z', '+00:00')
    try:
        return datetime.fromisoformat(s).timestamp()
    except Exception:
        return None


def load_jsonl(path):
    out = []
    try:
        with open(path) as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    out.append(json.loads(line))
                except Exception:
                    pass
    except Exception:
        pass
    return out


def iter_tool_uses(events):
    for o in events:
        msg = o.get('message') or {}
        content = msg.get('content')
        if not isinstance(content, list):
            continue
        ts = o.get('timestamp')
        epoch = parse_ts(ts)
        for c in content:
            if isinstance(c, dict) and c.get('type') == 'tool_use':
                yield epoch, ts, c.get('name', ''), c.get('input', {}) or {}, c.get('id', ''), o


def iter_assistant_messages(events):
    for o in events:
        if o.get('type') != 'assistant':
            continue
        ts = o.get('timestamp')
        yield parse_ts(ts), ts, o


def first_bead_id(s):
    if not s:
        return None
    m = BEAD_RE.search(s)
    if not m:
        return None
    return m.group(0).lower()


def collect_git_log():
    res = subprocess.run(
        ['git', '-C', REPO, 'log', '--all', '--format=%H%x09%aI%x09%s'],
        capture_output=True, text=True)
    bead_index = defaultdict(list)
    if res.returncode != 0:
        return bead_index
    for line in res.stdout.strip().split('\n'):
        if not line:
            continue
        try:
            sha, aiso, subject = line.split('\t', 2)
        except ValueError:
            continue
        epoch = parse_ts(aiso)
        bid = first_bead_id(subject)
        if bid:
            bead_index[bid].append((sha, epoch, subject, aiso))
    return bead_index


def session_sort_key(p):
    try:
        return os.path.getmtime(p)
    except OSError:
        return 0


def parse_subagent(meta_path):
    try:
        meta = json.load(open(meta_path))
    except Exception:
        return None
    desc = meta.get('description', '') or ''
    bead_id = first_bead_id(desc)
    agent_jsonl = meta_path.replace('.meta.json', '.jsonl')
    sess_dir = os.path.basename(os.path.dirname(os.path.dirname(meta_path)))
    sub_agent_id = os.path.basename(agent_jsonl).replace('agent-', '').replace('.jsonl', '')
    if not os.path.exists(agent_jsonl):
        return None
    ev = load_jsonl(agent_jsonl)
    if not ev:
        return None

    first_tool_epoch = None
    first_tool_ts = None
    commits = []  # (epoch, ts_str, cmd, tool_use_id)
    last_assistant_epoch = None
    last_assistant_ts = None

    for epoch, ts, name, inp, tid, o in iter_tool_uses(ev):
        if first_tool_epoch is None and epoch is not None:
            first_tool_epoch = epoch
            first_tool_ts = ts
        if name == 'Bash':
            cmd = inp.get('command', '') or ''
            if re.search(r'\bgit\s+commit\b', cmd):
                commits.append((epoch, ts, cmd, tid))
    for epoch, ts, _o in iter_assistant_messages(ev):
        if epoch is not None:
            last_assistant_epoch = epoch
            last_assistant_ts = ts

    # Find commit SHA from tool_result text in the sub-agent's own transcript
    commit_sha = None
    if commits:
        commit_tids = {c[3] for c in commits}
        for o in ev:
            msg = o.get('message') or {}
            content = msg.get('content')
            if not isinstance(content, list):
                continue
            for c in content:
                if not isinstance(c, dict):
                    continue
                if c.get('type') == 'tool_result' and c.get('tool_use_id') in commit_tids:
                    text = c.get('content', '')
                    if isinstance(text, list):
                        text = ' '.join(
                            (t.get('text', '') if isinstance(t, dict) else str(t))
                            for t in text)
                    m = re.search(r'\[[^\]]+\s([0-9a-f]{7,40})\]', text or '')
                    if m:
                        commit_sha = m.group(1)
                        break
            if commit_sha:
                break

    return {
        'meta_path': meta_path,
        'agent_jsonl': agent_jsonl,
        'sub_agent_id': sub_agent_id,
        'session_id': sess_dir,
        'description': desc,
        'bead_id': bead_id,
        'first_tool_epoch': first_tool_epoch,
        'first_tool_ts': first_tool_ts,
        'commits': commits,
        'commit_sha': commit_sha,
        'last_assistant_epoch': last_assistant_epoch,
        'last_assistant_ts': last_assistant_ts,
    }


def index_orchestrator_dispatch(session_id):
    """For a single orchestrator transcript, return list of dispatches:
    [(epoch, ts, description, tool_use_id), ...] in order.
    Also map: tool_use_id -> agent_id (parsed from tool_result text).
    """
    op = os.path.join(ROOT, session_id + '.jsonl')
    ev = load_jsonl(op)
    dispatches = []
    for epoch, ts, name, inp, tid, _o in iter_tool_uses(ev):
        if name not in ('Task', 'TaskCreate', 'Agent'):
            continue
        desc = inp.get('description') or inp.get('subject') or ''
        if not desc:
            continue
        dispatches.append((epoch, ts, desc, tid))

    # tool_use_id -> agent_id from tool_result content
    tid_to_agent = {}
    for o in ev:
        msg = o.get('message') or {}
        content = msg.get('content')
        if not isinstance(content, list):
            continue
        for c in content:
            if not isinstance(c, dict):
                continue
            if c.get('type') == 'tool_result':
                tid = c.get('tool_use_id')
                text = c.get('content', '')
                if isinstance(text, list):
                    text = ' '.join(
                        (t.get('text', '') if isinstance(t, dict) else str(t))
                        for t in text)
                m = re.search(r'agentId[:\s"]+([a-f0-9]{10,})', text or '')
                if m and tid:
                    tid_to_agent[tid] = m.group(1)
    return dispatches, tid_to_agent


def fmt_ts(epoch_or_str):
    if not epoch_or_str:
        return ''
    return epoch_or_str


def main():
    git_bead_index = collect_git_log()
    print(f'git: {len(git_bead_index)} bead-tagged commits', file=sys.stderr)

    all_sessions = sorted(glob.glob(ROOT + '/*.jsonl'), key=session_sort_key)  # oldest first
    print(f'sessions available: {len(all_sessions)}', file=sys.stderr)

    rows_out = []
    sessions_processed = 0
    oldest_mtime = None
    newest_mtime = None

    for sess_path in all_sessions:
        if sessions_processed >= MAX_SESSIONS:
            break
        if len(rows_out) >= MIN_REVIEWER_ROWS and sessions_processed >= 1:
            # we have enough, stop
            break
        session_id = os.path.basename(sess_path).replace('.jsonl', '')
        sess_dir = os.path.join(ROOT, session_id)
        if not os.path.isdir(sess_dir):
            sessions_processed += 1
            continue
        meta_paths = sorted(glob.glob(os.path.join(sess_dir, 'subagents', 'agent-*.meta.json')))
        if not meta_paths:
            sessions_processed += 1
            continue

        mt = os.path.getmtime(sess_path)
        if oldest_mtime is None or mt < oldest_mtime:
            oldest_mtime = mt
        if newest_mtime is None or mt > newest_mtime:
            newest_mtime = mt
        sessions_processed += 1

        # Parse sub-agents in this session
        subs = []
        for mp in meta_paths:
            p = parse_subagent(mp)
            if p:
                subs.append(p)

        dispatches, tid_to_agent = index_orchestrator_dispatch(session_id)
        # Map description -> dispatch list (in order)
        desc_to_disp = defaultdict(list)
        for d in dispatches:
            desc_to_disp[d[2]].append(d)

        # Group by bead
        by_bead = defaultdict(list)
        for p in subs:
            if p['bead_id']:
                by_bead[p['bead_id']].append(p)

        for bead_id, plist in by_bead.items():
            implementers = [p for p in plist if not re.search(r'\breview', p['description'], re.I)
                            and p['commits']]
            reviewers = [p for p in plist if re.search(r'\breview', p['description'], re.I)]
            if not implementers or not reviewers:
                continue

            for impl in implementers:
                # find dispatch matching impl description
                disp_list = desc_to_disp.get(impl['description'], [])
                # pick the one closest to (just before) impl's first tool ts
                impl_disp = None
                if impl['first_tool_epoch']:
                    cand = [d for d in disp_list if d[0] and d[0] <= impl['first_tool_epoch'] + 5]
                    if cand:
                        impl_disp = min(cand, key=lambda d: impl['first_tool_epoch'] - d[0])
                if impl_disp is None and disp_list:
                    impl_disp = disp_list[0]

                # Commit landmarks
                # Use the LAST git commit Bash call (final commit) as commit ts
                commit_call_epoch, commit_call_ts, _cmd, commit_tid = impl['commits'][-1]
                commit_sha = impl['commit_sha'] or ''

                # Find author-date from git log if we matched a sha; else use bead_index closest-after
                commit_author_ts = ''
                commit_author_epoch = None
                git_rows = git_bead_index.get(bead_id, [])
                if commit_sha:
                    for sha, ep, _subj, aiso in git_rows:
                        if sha.startswith(commit_sha) or commit_sha.startswith(sha):
                            commit_author_ts = aiso
                            commit_author_epoch = ep
                            break
                if not commit_author_ts and git_rows and commit_call_epoch:
                    after = [c for c in git_rows if c[1] and c[1] >= commit_call_epoch - 5]
                    pick = None
                    if after:
                        pick = min(after, key=lambda c: c[1] - commit_call_epoch)
                    else:
                        pick = min(git_rows, key=lambda c: abs(c[1] - commit_call_epoch))
                    commit_sha = commit_sha or pick[0]
                    commit_author_ts = pick[3]
                    commit_author_epoch = pick[1]

                # Pick the reviewer dispatched after this implementer's commit
                rev_pick = None
                rev_disp = None
                for r in reviewers:
                    rdisp_list = desc_to_disp.get(r['description'], [])
                    # Want rdisp.epoch >= commit_call_epoch - 60
                    for rd in rdisp_list:
                        if rd[0] and commit_call_epoch and rd[0] >= commit_call_epoch - 60:
                            if rev_pick is None or rd[0] < rev_disp[0]:
                                rev_pick = r
                                rev_disp = rd
                if rev_pick is None:
                    # fallback: any reviewer for this bead with any dispatch
                    for r in reviewers:
                        rdisp_list = desc_to_disp.get(r['description'], [])
                        if rdisp_list:
                            rev_pick = r
                            rev_disp = rdisp_list[0]
                            break
                if rev_pick is None:
                    continue

                # reviewer_seconds = commit_call_epoch -> reviewer last assistant
                if not (commit_call_epoch and rev_pick['last_assistant_epoch']):
                    continue
                reviewer_sec = rev_pick['last_assistant_epoch'] - commit_call_epoch
                if reviewer_sec <= 0:
                    # implausible — skip
                    continue

                # Phase computations
                dispatch_epoch = impl_disp[0] if impl_disp else None
                dispatch_ts = impl_disp[1] if impl_disp else ''

                spin_up = None
                if impl['first_tool_epoch'] and dispatch_epoch:
                    spin_up = impl['first_tool_epoch'] - dispatch_epoch

                task_work = None
                if impl['first_tool_epoch'] and commit_call_epoch:
                    task_work = commit_call_epoch - impl['first_tool_epoch']

                merge_sec = None
                if commit_call_epoch and commit_author_epoch:
                    merge_sec = commit_author_epoch - commit_call_epoch

                total_sec = None
                end_epoch = rev_pick['last_assistant_epoch']
                if dispatch_epoch and end_epoch:
                    total_sec = end_epoch - dispatch_epoch

                # Resolve agent_id from orchestrator's tool_result if available, else fall back to sub_agent_id
                impl_agent_id = ''
                if impl_disp:
                    impl_agent_id = tid_to_agent.get(impl_disp[3], '') or impl['sub_agent_id']
                rev_agent_id = ''
                if rev_disp:
                    rev_agent_id = tid_to_agent.get(rev_disp[3], '') or rev_pick['sub_agent_id']

                notes = []
                if len(impl['commits']) > 1:
                    notes.append(f"{len(impl['commits'])}_commit_calls")
                if not commit_sha:
                    notes.append('no_commit_sha')

                rows_out.append({
                    'bead_id': bead_id,
                    'session_id': session_id,
                    'agent_id': impl_agent_id,
                    'reviewer_agent_id': rev_agent_id,
                    'commit_sha': commit_sha,
                    'orchestrator_dispatch_ts': dispatch_ts or '',
                    'first_subagent_tool_ts': impl['first_tool_ts'] or '',
                    'commit_ts': commit_call_ts or '',
                    'commit_author_ts': commit_author_ts or '',
                    'reviewer_last_assistant_ts': rev_pick['last_assistant_ts'] or '',
                    'spin_up_seconds': round(spin_up, 1) if spin_up is not None else '',
                    'task_work_seconds': round(task_work, 1) if task_work is not None else '',
                    'merge_seconds': round(merge_sec, 1) if merge_sec is not None else '',
                    'reviewer_seconds': round(reviewer_sec, 1),
                    'total_seconds': round(total_sec, 1) if total_sec is not None else '',
                    'notes': ';'.join(notes),
                })

        print(f'  [{sessions_processed}] {session_id[:8]}.. rows so far: {len(rows_out)}', file=sys.stderr)

    print(f'sessions_processed={sessions_processed} reviewer_rows={len(rows_out)}', file=sys.stderr)

    fieldnames = [
        'bead_id', 'session_id', 'agent_id', 'reviewer_agent_id', 'commit_sha',
        'orchestrator_dispatch_ts', 'first_subagent_tool_ts', 'commit_ts',
        'commit_author_ts', 'reviewer_last_assistant_ts',
        'spin_up_seconds', 'task_work_seconds', 'merge_seconds',
        'reviewer_seconds', 'total_seconds', 'notes',
    ]
    out_path = os.path.join(OUT, 'harmonik_reviewer_beads.csv')
    with open(out_path, 'w', newline='') as f:
        w = csv.DictWriter(f, fieldnames=fieldnames)
        w.writeheader()
        for r in rows_out:
            w.writerow(r)
    print(f'wrote {out_path}', file=sys.stderr)

    # Stats
    revs = [r['reviewer_seconds'] for r in rows_out if r['reviewer_seconds'] != '']
    if revs:
        revs_sorted = sorted(revs)
        n = len(revs_sorted)
        def pct(p):
            i = max(0, min(n - 1, int(round(p * (n - 1)))))
            return revs_sorted[i]
        stats = dict(
            n=n,
            min=revs_sorted[0],
            median=pct(0.5),
            p95=pct(0.95),
            max=revs_sorted[-1],
            oldest_mtime=oldest_mtime,
            newest_mtime=newest_mtime,
            sessions_processed=sessions_processed,
        )
        print(json.dumps(stats, indent=2), file=sys.stderr)


if __name__ == '__main__':
    main()
