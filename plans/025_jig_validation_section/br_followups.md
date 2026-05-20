# br / beads_rust follow-ups (OUT OF KERF SCOPE)

Three items from `/Users/gb/github/harmonik/docs/kerf-feedback/2026-05-19.md` that target the `br` (beads_rust) CLI, not kerf. Recorded here so the orchestrator can file them upstream to `gastownhall/beads` (or wherever beads_rust lives). Do not scope into kerf work.

## Item 1 — BEADS-RUST-UPSTREAM: WAL auto-checkpoint (hk-5dewt, wave 1 filing)

**Status:** filer-corrected later (see Item 2). The original hypothesis was that `br` does not auto-checkpoint its SQLite WAL, causing write latency to grow from 0.35s → 19s as the WAL grows past ~12 MB. Harmonik applied a host-side `PRAGMA wal_checkpoint(TRUNCATE)` pre-flight at daemon startup. Suggested upstream fix: PASSIVE checkpoint after every terminal-transition write, or `wal_autocheckpoint=1000` pragma.

**Why it's still worth filing:** even though the WAL was not the actual root cause of the 19s latency (Item 3 is), WAL bloat is still a defense-in-depth concern that the harmonik daemon now mitigates host-side. The upstream fix would remove the harmonik workaround.

## Item 2 — CORRECTION: `.br_history/` snapshot bloat, NOT WAL (hk-5dewt, wave 2)

**Component:** beads_rust history-snapshot mechanism. Each `br` write appends a ~1.2 MB snapshot to `.beads/.br_history/`. At 200+ entries (~226 MB) every write scans the directory in ~19.5 s. Harmonik applied a rotation pre-flight that archives oldest entries beyond a keepLatest=20 cap. Suggested upstream fix: auto-rotate `.br_history/` after a configurable entry cap (e.g. 50), or add `br history prune` subcommand.

## Item 3 — Root-cause investigation: `.write.lock` fcntl contention, no lock-acquisition timeout

**Component:** `br` write lock. `br close` acquires `fcntl LOCK_EX` on `.beads/.write.lock` with no timeout; under harmonik's retry-storm (UnavailableRetryMax=10, 10 s timeout each, ~5 s SIGTERM grace), parallel `br close` invocations sequentially block on the lock for up to 19 s. Suggested upstream fix: expose `.write.lock` acquisition wait as a configurable timeout (e.g. `--lock-timeout 15000`). Also reported: `idx_dependencies_blocking` corruption flagged by `br doctor` — separate issue, unrelated to the latency.

---

All three of the above are upstream to beads_rust. Orchestrator: file as issues against the beads_rust repository (or whichever upstream repo owns `br`). Do not include in plan 025.

---

## Update 2026-05-19 — `.github/br-version` bumped 0.1.45 → 0.2.10

Kerf's pinned `br` version was bumped from 0.1.45 (released 2026-04-20) to 0.2.10 (released 2026-05-14). The three items above were originally observed against 0.1.45 in the harmonik dogfood session. They should be re-evaluated against 0.2.10 before any upstream filing — several of the surfaces named below have moved on in the 0.2.x line.

## Research notes (no issues filed)

Investigation of the three items against the `Dicklesworthstone/beads_rust` issue tracker. **No issues or PRs were filed.** Decisions about filing are deferred pending re-test on 0.2.10.

### Item 1 — WAL auto-checkpoint: do not file

- Self-corrected later in the same harmonik feedback doc (Item 2 supersedes Item 1 as the actual root cause, then Item 3 supersedes Item 2).
- Item 3's own measurements ("WAL up to 41 MB → 0.44 s, fsqlite auto-checkpoints on every write — `frames_to_backfill=81 progress=Complete`") confirm br already checkpoints.
- Historical issues already closed upstream: **#108** "WAL never checkpoints to main db file on Linux — unbounded growth leads to corruption" (closed 2026-02-28); **#270** "WAL wedging under concurrent multi-agent SQLite access" (closed 2026-05-04).
- Conclusion: not a real upstream bug; the harmonik WAL pre-flight is defense-in-depth at best.

### Item 2 — `.br_history/` bloat: partially already addressed upstream

- `br history prune` **already exists** as a subcommand in 0.1.45 (`br history --help` lists `list / diff / restore / prune`). Harmonik's suggested "add a `br history prune` subcommand" is already implemented.
- Issue **#293** "Add a config flag to disable `.br_history/`" (closed 2026-05-14, lands in next release after 0.2.10 per maintainer comments) added `sync.history_enabled: false` config plus follow-up fix `b0ff7572` covering the `--no-db` and MCP auto-flush paths.
- The remaining unaddressed ask is **automatic rotation after a configurable entry cap** (e.g. auto-prune when count > 50). `prune` exists but must be invoked manually; `history_enabled: false` disables entirely. There is no auto-rotation policy.
- Conclusion: if filed, scope it narrowly to "auto-rotate by entry-count / size cap" and acknowledge `prune` + `history_enabled` already exist. Re-verify behavior on 0.2.10 first.

### Item 3 — `.write.lock` fcntl acquisition wait: likely unaddressed, needs re-verification

- `br --lock-timeout <ms>` flag **already exists** but the `br history --help` text describes it as "**`SQLite` busy timeout in ms**" — i.e. the SQLite-level busy timeout, not the fcntl `.write.lock` acquisition wait that Item 3 is about.
- No clearly matching open or closed issue surfaced under searches for `lock acquisition`, `--lock-timeout`, `fcntl LOCK_EX`, `write.lock`.
- Item 3's repro recipe (hold `.write.lock` externally, time a `br close`) is clean and reproducible — would make a good issue body if still valid on 0.2.10.
- Conclusion: most likely the only genuinely unaddressed of the three. Re-run the repro on 0.2.10; if still reproducing, this is the one worth filing — but only with user approval.

### Disposition

- File nothing right now.
- Re-test on 0.2.10 in harmonik. If Item 2 (auto-rotation) and/or Item 3 (fcntl wait timeout) still reproduce, surface them to the user before filing.
- Item 1 is closed out: not a real bug.
