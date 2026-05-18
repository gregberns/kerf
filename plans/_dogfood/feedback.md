# Kerf Self-Dogfood Feedback Log

Bootstrap of kerf on its own repo, May 2026.
Tester: Claude (Opus 4.7).
Goal: use `kerf next` to drive implementation of Plans 016-020 (46 beads). Capture every friction point — this is direct input for the same plans being implemented.

Severity legend: BLOCKER (cannot proceed) / MAJOR (workflow stalls or confuses) / MINOR (small UX gap) / NIT (cosmetic).

This file is the live log. Sub-agents append entries; the orchestrator reviews periodically and routes items to:
- existing Plans 016/017/018/019/020 (add as bead or annotate existing bead)
- a new plan if the gap is broader than the 016-020 scope
- workarounds if it's a one-time bootstrap quirk

---

## Setup / environment

**2026-05-18 14:20 — pre-init repo state**

- Branch `main` clean at 17bb0e9 (Plan 011 weight-tuning sweep).
- `.beads/` contains 46 open beads created across Plans 016-020. Label conventions in this store: `plan:NNN`, `codename:<name>`, `kind:work`, `spec:<area>`.
- `kerf` and `kerfsim` are installed at `/Users/gb/go/bin/` (rebuilt this session with `go install ./...`, build was clean — no warnings).
- No `project.yaml` in repo, no `.kerf/` artifacts beyond what kerf later creates (project lives on the bench at `~/.kerf/projects/gregberns-kerf/`).
- Bench output of plain `kerf` says "Total active works: 28" — but those are from *other* projects on the same bench. `kerf list` for this project shows zero works. The 28 figure is misleading at the top-level invocation when in a fresh project root.
  - Severity: **MINOR**. Top-level `kerf` summary leaks bench-global state to a per-project context.
  - Routing: **Plan 017 (storage-recon)** — add doctor detector / clarify bench vs project storage.

**Bead-tool name mismatch (the first real blocker)**

- This repo's `.beads/` is a `bd` 0.62 (dev) store. Installed `br` is 0.1.45.
- `br list --format json --all --limit 0` (the exact argv kerf uses) errors:
  ```
  {"error":{"code":"JSON_ERROR","message":"JSON error: missing field `jsonl_export` at line 7 column 1",...}}
  ```
- `kerf` hardcodes `DefaultToolName = "br"` in `internal/beads/beads.go`. `next.go` calls `beads.List()` (not the `ListNamed` variant), so the configurable `tools.tasks` field in `project.yaml` is **not consulted** for the `kerf next` path.
- Effect: pre-init `kerf next` shows zero beads; post-init it would still show zero beads, with no diagnostic explaining *why*. The store has 46 beads, kerf says "no items."
- Workaround used: PATH-shadowed `br` with a shim that execs `bd` (`plans/_dogfood/bin/br` in this commit). With the shim on PATH, `kerf next` works end-to-end.
- Severity: **BLOCKER**. Any kerf user whose beads tool is `bd` (most current installs) cannot get `kerf next` to return anything without either (a) installing/downgrading `br`, or (b) shimming PATH.
- Routing: **new plan** — "Plan 021 (bead-tool-resolution)?" — `next.go` and friends should honor `project.yaml` `tools.tasks` like other code paths claim to. Sketch: thread `ListNamed` through every kerf entry point that loads beads. Also surface a clear error when the configured tool fails (currently `beads.List()` swallows the error and returns nil).

**Pre-init command behavior**

- `kerf next` (pre-init) →
  ```
  warning: No project.yaml for 'gregberns-kerf' — kerf init
           Project 'gregberns-kerf' has no project.yaml. Run 'kerf init' to create one before using 'kerf next'.
  Error: no project.yaml — run 'kerf init'.
  ```
  Then dumps the `--help` block. Help spam on a normal error is noisy.
  - Severity: **MINOR**. The error is good, the help-on-error is not.
  - Routing: **Plan 016 (init-ux)** or **Plan 017** — suppress cobra default help on fatal errors.

- `kerf triage` (pre-init) → terse `Error: project not initialized. Run 'kerf init' first.`, then full `--help`. Same help-spam issue. Inconsistent with `kerf next` (which printed a friendlier warning block first).
  - Severity: **MINOR**. Inconsistency between `next` and `triage` on the same precondition.
  - Routing: **Plan 018 (triage-rework)** — mirror `next`'s warning-before-fatal pattern.

---

## kerf init

**2026-05-18 14:25 — `printf 'n\n' | kerf init`**

Observed behavior (first run, piping `n` to be safe):

1. `Created .kerf/project-identifier: gregberns-kerf` — true, file exists.
2. `Created project.yaml with 6 active jigs: bug, implementation, plan, retrofit, spec, spike` — **lie**. The file `project.yaml` is **not** in the cwd; it's in `~/.kerf/projects/gregberns-kerf/project.yaml`. The agent who reads this line will `cat project.yaml` and fail.
   - Severity: **MAJOR**. The path claim is wrong for bench-storage projects (the default mode). Should say "Created bench project.yaml at ~/.kerf/projects/<id>/project.yaml" or equivalent.
   - Routing: **Plan 016 (init-ux)** — already has bead kerf-c2j ("Update kerf setup instruction text: add missing commands, exact gitignore, bench-location stub"). This is exactly that bead's scope; reinforces it.
3. No `[Y/n]` prompt was printed at all on this run (despite the brief saying we'd hit one). Tracked to `detectBeadFilter` in `cmd/init.go`: when there are zero works in the project, the codename-correlation fails to find a winner, so it falls through to `promptFallbackPrefix` only when interactive — non-interactive stdin returns `priorFilter` (nil) silently. So the well-known "init prompt" issue **only triggers when a tty is attached AND the bead store has labels that can be mapped to existing works**. Both Plan 016's bead kerf-pjs ("drop y/N prompt") and the brief assume the prompt fires; with the kerf-on-kerf bootstrap, it doesn't.
   - Severity: **MINOR**. The detector is silent when it should perhaps say "no confident prefix detected; bead_filter left unset (run `kerf bootstrap-filters` later)".
   - Routing: **Plan 019 (filter-bootstrap)** — bead kerf-yxl (confidence threshold) plus a new hint line. Also informs **Plan 016** bead kerf-yl1 (state-change summary block in init output).
4. Two back-to-back "AGENT SETUP INSTRUCTIONS" blocks emitted — same MAJOR friction harmonik logged. Confirmed reproduces in kerf-on-kerf.
   - Severity: **MAJOR**.
   - Routing: **Plan 016 (init-ux)** — already covered by bead kerf-6jw.
5. The first instruction block mentions `kerf new / show / status / shelve / resume / square / finalize`. It does **not** mention `kerf next`, `kerf triage`, `kerf pin`, `kerf map`, `kerf areas`, `kerf work edit`, `kerf list`, `kerf delete`, `kerf archive`. Same stale-text bug harmonik flagged.
   - Severity: **MAJOR**.
   - Routing: **Plan 016 (init-ux)** — bead kerf-c2j.
6. The verification step says `kerf new test-setup --title "Verify kerf setup" && kerf show test-setup && kerf delete test-setup --yes`. The `kerf delete` command in current help does **not** advertise a `--yes` flag (it might support one — not checked). Either way, telling agents to *run* a delete command as part of init verification is risky; a softer "kerf list / kerf show" would do.
   - Severity: **MINOR**.
   - Routing: **Plan 016 (init-ux)** — bead kerf-c2j.
7. `default_jig` was not requested via `--jig`; the second instruction block lists the six jigs in alphabetical order with no "default" marker. The earlier-emitted bench summary later shows `default_jig: spec`. So the global config has a default but init never told the agent which one is in effect.
   - Severity: **MINOR**.
   - Routing: **Plan 016 (init-ux)** — bead kerf-q5l ("persist default_jig in project.yaml or strip the claim").

**2026-05-18 14:30 — `printf 'y\n' | kerf init --force`**

Same output as above (5+ pages, two AGENT SETUP blocks). No "detected prefix" line printed → confirms the detector silently no-ops when no works exist. The `y` input was simply discarded.

---

## Filter bootstrap (hand-wired)

Goal: get `kerf next` to surface the 46 beads.

**Decision.** Beads are labeled `codename:init-ux`, `codename:storage-recon`, `codename:triage-rework`, `codename:filter-bootstrap`, `codename:review-gate` (one codename per plan, 7-14 beads each). The natural mapping is one **work** per codename, each with `bead_filter: label=codename:<codename>`. This is also exactly what Plan 019's `kerf bootstrap-filters` is supposed to produce automatically.

**Commands run:**
```
for cn in init-ux storage-recon triage-rework filter-bootstrap review-gate; do
  kerf new "$cn" --jig implementation \
    --bead-filter "label=codename:$cn" \
    --title "Plan: $cn"
done
```

All five succeeded (`--bead-filter` flag works exactly as documented).

**First `kerf next` after creating works (no shim):**
```
1. clean  filter-bootstrap   resolved bead_filter matches zero beads in the store
          edit spec.yaml bead_filter or check the project filter
... (one per work)
```

This is the symptom of the **br vs bd** mismatch. The diagnostic says "edit spec.yaml bead_filter or check the project filter" — but the filter is correct; the problem is the bead-tool subprocess silently returned empty (because `br` errored on the store and `beads.List` swallowed the error).

- Severity: **MAJOR**. The cleanup hint points at the wrong diagnosis. The agent will spend time twiddling filter clauses when the actual fix is "your bead-tool subprocess is failing silently."
- Routing: **Plan 017 (storage-recon)** — `kerf doctor` should include a "bead-tool reachable + returns parseable JSON" detector. **Plan 019 (filter-bootstrap)** — bead kerf-mgx (rank-label vocabulary) needs an "unreachable" state in addition to "empty/unwired/broken".

**After `PATH=/tmp/kerf-shim:$PATH` (shim makes `br` exec `bd`):**

```
$ kerf next | head
1. bead   kerf-a7t  "Plan 019 / B5 — kerf bootstrap-filters command surface"  work: filter-bootstrap
2. bead   kerf-iak  "Plan 019 / B4 — label-convention sampler (standalone)"   work: filter-bootstrap
3. bead   kerf-mgx  "Plan 019 / B2 — rank-label vocabulary (empty/unwired/broken)" work: filter-bootstrap
...
46. bead   kerf-ytt  "Suggester: tier-1 vs tier-2 prefix routing"  work: triage-rework
```

All 46 open beads attached, distributed correctly across works (filter-bootstrap=8, init-ux=8, review-gate=7, storage-recon=14, triage-rework=9). Count matches `bd list --status open` exactly.

`kerf triage` confirms the same:
```
Per-work bead health:
  filter-bootstrap  filter: label=codename:filter-bootstrap  beads: 8 open / 0 closed
  init-ux           filter: label=codename:init-ux           beads: 8 open / 0 closed
  review-gate       filter: label=codename:review-gate       beads: 7 open / 0 closed
  storage-recon     filter: label=codename:storage-recon     beads: 14 open / 0 closed
  triage-rework     filter: label=codename:triage-rework     beads: 9 open / 0 closed
```

Success criteria reached.

---

## Other commands tried (cheap pokes)

- `kerf triage` (post-bootstrap, with shim) — clean, no warnings; shows the per-work health table. Good.
- `kerf list` (post-bootstrap) — five works, all in `breakdown` status. Output is plain, readable.
- `kerf next --format=json` — 46 items, valid JSON. Good.
- `kerf config` — global config readout; does not include the just-initialized project. Probably fine, but the agent looking for "my project's config" gets bench-global config back, not project. Minor confusion vector.
  - Severity: **NIT**.
  - Routing: **Plan 017 (storage-recon)** if it grows a "where state lives" cheat-sheet (bead kerf-enr).

---

## Routing summary (entries above, deduplicated)

| Entry | Severity | Routing |
|-------|----------|---------|
| Top-level `kerf` summary shows global "28 active works" in a fresh project | MINOR | 017 |
| `br` vs `bd` bead-tool subprocess fails silently, kerf hardcodes `br` in `next.go` | **BLOCKER** | **new plan (bead-tool-resolution)** |
| `kerf next` / `kerf triage` dump full help on a precondition error | MINOR | 016 / 018 |
| Init claims `Created project.yaml` but file is on the bench, not cwd | MAJOR | 016 (kerf-c2j) |
| Detector silent when no works exist; no "filter left unset" hint | MINOR | 019 (kerf-yxl) + 016 (kerf-yl1) |
| Two AGENT SETUP INSTRUCTIONS blocks emitted back-to-back | MAJOR | 016 (kerf-6jw) |
| Init instructions omit `kerf next/triage/pin/map/areas/work edit/list/delete/archive` | MAJOR | 016 (kerf-c2j) |
| Verification step recommends `kerf delete --yes` (risky for an init guide) | MINOR | 016 (kerf-c2j) |
| `default_jig` not surfaced in init output | MINOR | 016 (kerf-q5l) |
| "Zero beads match filter" cleanup hint mis-routes the diagnosis when subprocess fails | MAJOR | 017 + 019 (kerf-mgx) |
| `kerf config` shows bench-global state, not the just-initialized project | NIT | 017 (kerf-enr) |

---

## Potential new plan

**Plan 021 — bead-tool resolution.** `kerf next`, `kerf triage`, and friends must (a) read `tools.tasks` from project.yaml via `beads.ListNamed`, not hardcode `br`, (b) surface a clear diagnostic when the subprocess errors (currently swallowed) instead of silently returning zero beads. Without this, kerf is unusable for any user whose beads store is `bd`-shaped. Sketch: thread tool-name through `next.go`/`triage.go`/anywhere else that calls `beads.List()`; add a one-line warning to the `kerf next` warnings header when the subprocess fails or returns invalid JSON.

---

## Reproduction recipe

```sh
cd /Users/gb/github/kerf
go install ./...
# Shim br → bd so kerf can see the .beads store
export PATH=$PWD/plans/_dogfood/bin:$PATH
kerf init --force                                       # creates bench project.yaml
for cn in init-ux storage-recon triage-rework filter-bootstrap review-gate; do
  kerf new "$cn" --jig implementation \
    --bead-filter "label=codename:$cn" --title "Plan: $cn"
done
kerf next                                                # 46 ranked beads
kerf triage                                              # clean, per-work counts
```

End state on bench: `~/.kerf/projects/gregberns-kerf/project.yaml` + five work directories (one per codename). No files in the repo besides `.kerf/project-identifier` (and the shim under `plans/_dogfood/bin/br`).
