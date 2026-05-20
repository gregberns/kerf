# Plan 016 — `kerf init` Exploratory Test

- **Binary:** `/Users/gb/go/bin/kerf` (built from main HEAD `b96cf54`)
- **Date:** 2026-05-18
- **Tester:** Opus 4.7 (1M)
- **Method:** Each scenario run in a fresh `mktemp -d` with disposable project IDs (`tmp-*`); no touches to `gregberns-kerf`.

Spec references throughout cite `specs/commands.md` and `specs/cli.md`.

## Summary

- **Scenarios run:** 7 scripted + 4 exploratory pokes = 11
- **Scripted pass count:** 7/7 (with 1 NIT inside scenario 2)
- **Exploratory pass count:** 2/4 (P3 OK, P4 OK; P1 MAJOR, P2 MAJOR)

## Top 3 issues

1. **MAJOR — Malformed `project.yaml` on rerun silently overwritten (P2).** Spec line 1402 says the default rerun path is "skip with informative output" and preserve any user-edited fields. Observed: when `project.yaml` is unparseable, `kerf init` (no `--force`) prints `Project already initialized: <id>` *and then re-runs the full init flow* (writes a fresh `project.yaml`, emits full agent block). The malformed file is destroyed without warning. Should either skip with a parse-error notice, or refuse and require `--force`.
2. **MAJOR — Corrupt `.kerf/project-identifier` not validated (P1).** Garbage bytes (NUL + embedded newlines) are read raw, printed in the output, and passed unfiltered to `mkdir(2)`. Final error message leaks low-level Go formatting. Init should validate the identifier file shape (single line, allowed charset) and surface a clean "corrupt project-identifier, remove `.kerf/project-identifier` and re-run init" error.
3. **NIT — `--no` reports detector-silence reason instead of source (scenario 2e).** When the user passes `--no`, the state-change summary still says `bead_filter unchanged (detector returned no confident suggestion; …)`. Per spec line 1377, the bead-filter summary should name the source (`--bead-filter` / `--yes` / `--no` / default). Should read something like `bead_filter unchanged (detector skipped by --no)`.

## Scenarios

### 1. Fresh init, no flags — OK

- **Command:** `git init && kerf init`
- **Expected:** no interactive prompt (spec 1310), single AGENT INSTRUCTIONS block (spec 1514), state-change fence with header inside.
- **Observed:** No prompt. Single `--- START AGENT INSTRUCTIONS ---` / `--- END AGENT INSTRUCTIONS ---` (1 each). State-change block fenced as expected. All four artifact rows present (`project-identifier`, `project.yaml`, `bead_filter`; `default_jig` omitted because none set on either layer — matches spec 1390).
- **Verdict:** OK
- **Reproducer:** `T=$(mktemp -d); cd "$T" && git init -q && kerf init`

### 2. `--yes` / `--no` / `--bead-filter` — OK with NIT

- 2a `--yes --no` → `Error: --yes and --no are mutually exclusive` (exit 1) — matches spec line 1400. **OK**
- 2b `--bead-filter garbage-no-equals` → `Error: --bead-filter expects 'label=<value>' or 'id_prefix=<value>', got "garbage-no-equals"` (exit 1) — matches spec 1401. **OK**
- 2c `--bead-filter label=subsystem:auth` → `bead_filter created (label=subsystem:auth)` in state summary; `project.yaml` carries the value. **OK**
- 2d `--bead-filter "any: [label=a:b, label=c:d]"` → Error (any-union literals not accepted at init flag — composition expected to land via `kerf work edit --bead-filter-add` per spec 2229). **OK** (matches the documented one-shot rule).
- 2e `--no` → Init succeeds, `bead_filter unchanged` row but with the detector-silence reason rather than `--no`. **NIT** (see top-3).
- **Reproducer:** `T=$(mktemp -d); cd "$T" && git init -q && kerf init --no`

### 3. `--jig spec` persists + inheritance — OK

- `kerf init --jig spec` writes `default_jig: spec` to `project.yaml` (top of file) and reports `default_jig created (set to 'spec')` in the state-change summary.
- `kerf new mywork` (same project, no `--jig`) → creates work with `Jig: spec (v1)`. Inherited.
- `kerf new other --jig bug` → creates with `Jig: bug (v2)`. Override works.
- **Verdict:** OK
- **Reproducer:** `T=$(mktemp -d); cd "$T" && git init -q && kerf init --jig spec && kerf new w1`

### 4. `--force` rewrite vs idempotent — OK

- First `kerf init --bead-filter label=foo:bar` → `bead_filter created (label=foo:bar)`.
- Second `kerf init` (no flags) → `project.yaml already exists at … — skipping re-initialisation.` Summary reports all four rows as `unchanged`; preserves filter. Exit 0.
- Third `kerf init --force` (no `--bead-filter`) → `project.yaml updated`, `bead_filter unchanged (label=foo:bar)` — filter preserved per spec 1362. **OK.**
- Fourth `kerf init --force --bead-filter label=other:zzz` → `bead_filter updated (label=other:zzz)`. **OK.**
- **Verdict:** OK
- **Reproducer:** `kerf init --bead-filter label=foo:bar; kerf init; kerf init --force`

### 5. State-change summary shape — OK

All four verbs observed across the matrix:
- `created`: `.kerf/project-identifier`, `project.yaml`, `default_jig`, `bead_filter` (in scenarios 1, 3, 2c).
- `updated`: `project.yaml` (force path), `bead_filter` (force + new value), `default_jig` (when changed — not observed in this run; consider follow-up).
- `unchanged`: all rows on idempotent rerun.

Fence shape: single triple-backtick block; `State changes:` header is INSIDE the fence (matches kerf-oca convention). **OK.**

### 6. Setup block check — OK

From scenario 1 transcript:
- `START AGENT INSTRUCTIONS` count: 1
- `END AGENT INSTRUCTIONS` count: 1
- Daily-driver commands all mentioned exactly once each: `kerf next`, `kerf triage`, `kerf pin`, `kerf map`, `kerf areas`, `kerf work edit`, `kerf list` (the latter twice — once in `Available commands` and once in the daily-driver listing; arguably the spec intended a single mention — minor).
- `.gitignore` block is exactly `.kerf/` followed by `!.kerf/project-identifier` inside a triple-backtick block. **OK** (spec 1522).
- `### Bench location` section present with `Bench path for this project: ~/.kerf/projects/<id>/` and a `<!-- placeholder: plan 017 expands -->` comment. **OK** (spec 1523).

**Verdict:** OK

### 7. Detector silence on tiny corpus — OK

- Empty `.beads/issues.jsonl` → `bead_filter unchanged (detector returned no confident suggestion; ...)`. Silent (no suggestion line). **OK.**
- Single bead `{"id":"foo-1","labels":["kerf:something"]}` → same behavior, silent. **OK** (matches plan 016 open question resolution: silent on too-small corpus).
- **Reproducer:** `mkdir .beads && touch .beads/issues.jsonl && kerf init`

## Exploratory pokes

### P1 — Corrupt `.kerf/project-identifier` — MAJOR

- **Command:** `printf '\x00\x01garbage\nwith\nnewlines' > .kerf/project-identifier && kerf init`
- **Observed:** Output prints `Project already initialized:  garbage` (newline-broken across three lines, NUL not visible), then attempts `mkdir /Users/gb/.kerf/projects/ garbage\nwith\nnewlines` and bails with raw Go error: `Error: creating project.yaml: creating project config directory: mkdir … invalid argument`. Exit 1 (good), but no validation step on the identifier content and the user is left to figure out what went wrong.
- **Verdict:** MAJOR
- **Reproducer:** `T=$(mktemp -d); cd "$T" && git init -q && mkdir -p .kerf && printf '\x00x\ny' > .kerf/project-identifier && kerf init`

### P2 — Malformed `project.yaml` on rerun — MAJOR

- **Command:** `echo "garbage: :: not yaml" > ~/.kerf/projects/<id>/project.yaml; kerf init`
- **Expected (spec 1402):** "skipping re-initialisation. Use 'kerf init --force' to overwrite..." with exit 0.
- **Observed:** kerf prints `Project already initialized: tmp-...`, then proceeds to write a fresh `project.yaml` over the malformed one AND emit the full agent-setup block. No warning, no skip. Exit 0.
- **Verdict:** MAJOR — silently overwrites user state; contradicts the documented idempotent default. Possibly because the "existing project.yaml" detection requires successful parse first; if parse fails the code falls through to the create branch.
- **Reproducer:** `kerf init >/dev/null; PID=$(cat .kerf/project-identifier); echo bad > ~/.kerf/projects/$PID/project.yaml; kerf init`

### P3 — Non-git dir — OK

- **Command:** `cd "$(mktemp -d)" && kerf init`
- **Observed:** `Error: not in a git repository. kerf requires a git repo: not in a git repository (searched from /var/folders/...)`. Exit 1. Matches spec 1398, though the error string is doubled ("not in a git repository" appears twice — once from kerf wrapper, once from the underlying error). **NIT** on doubled phrasing.

### P4 — Init from subdir of repo — OK

- **Command:** From `<repo>/sub/deeper`, run `kerf init`.
- **Observed:** `project-identifier` lands at the repo root (`<repo>/.kerf/project-identifier`), not in the subdir. Correct.

### P5 — `any:` union literal via `--bead-filter` — OK (rejected)

- See scenario 2d. Init rejects with the standard `expects 'label=<value>' or 'id_prefix=<value>'` error. Composition via `kerf work edit --bead-filter-add` is the documented path.

## Surprises

- The state-change summary for the `--force` re-run preserves a previously-set `bead_filter` and reports it as `unchanged`. Implementation matches spec exactly — pleasantly correct, given how easy it would have been to clobber.
- `kerf init` from a subdir Just Works. No spec requirement found for this, but it's the friendly behavior an agent would expect.
- The detector currently reports `unchanged` for `bead_filter` even when there was no prior value and no new value. Arguably that artifact "did not exist before and does not exist now" is closer to `unchanged` than to any other verb, but the wording "detector returned no confident suggestion" doubles as a hint for what to do. Acceptable as-is.

## Suggested follow-ups (for triage, not this test)

- Add a unit test: malformed `project.yaml` rerun without `--force` → expect skip behavior, no overwrite.
- Add a unit test: corrupt `.kerf/project-identifier` (non-printable, multiline) → expect clean error.
- Add a state-summary case where the source is `--no` and verify the parenthetical names the source.
- Consider whether the "`not in a git repository`" message is doubled by mistake (wrapper + cause both name the condition).
