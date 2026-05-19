# Testing-strategy audit — 2026-05-19

> Audit motivated by `plans/_dogfood/test_2026-05-18/SUMMARY.md`: review caught spec drift but missed 4 BLOCKERs + 6 MAJORs at integration boundaries. This document characterises the current testing surface and proposes a layered fix.

## 1. Current state

### 1.1 By layer

| Layer | Convention | Volume | Coverage | Quality / notes |
|-------|-----------|--------|----------|-----------------|
| **Unit** | `*_test.go` co-located in package; table-driven where applicable | 88 `*_test.go` files; ~70 are unit-flavoured | `internal/` packages: 49–100% (median ~86%); `cmd/` 77.9% | Generally solid. Most `internal/*` packages hit ≥85%. Weak spots: `internal/cmdutil` 29%, `internal/storage` 49%, `internal/doctor` 63%, `internal/sim/scenario` 40%. |
| **Property-based** | `*_property_test.go` using `testing/quick` | 4 files: `codename`, `config`, `spec`, `snapshot` | Limited to serialization & validators | Library is stdlib `quick`. No `gopter`/`rapid`. Spec calls out broader property coverage (concurrent FS ops, config merging) — not implemented. |
| **Fuzz** | `*_fuzz_test.go` using `testing.F` | 3 files: `codename`, `config`, `spec` | YAML parse + validator inputs | Tiny corpus. Fuzz is not part of any documented run loop. |
| **Integration** | `*_test.go` in `cmd/` invoking cobra `RunE` against `t.TempDir()` | `integration_test.go` (569 LOC), `plan020_integration_test.go` (332 LOC) | Lifecycle, status, square, multi-work, config layering | Exercise the command graph in-process — **no subprocess**, no real `br`/`bd`. Stubs use shell-script PATH shims that echo canned JSON (`stubBr` in `init_bead_filter_test.go`). |
| **End-to-end** | `*_e2e_test.go` in `cmd/` | `e2e_test.go` (467), `coordination_e2e_test.go` (139), `triage_e2e_test.go` (363), `plan006_e2e_test.go` (454) | Real git via `exec.Command("git", …)`; lifecycle with finalize; multi-project; jig-from-file; triage drift | E2E for **git**, not for **beads/bd/br**. No test exercises the real `br` or `bd` binary; every bead-tool surface is stubbed via PATH shim emitting a static JSON blob. |
| **Scenario / agent-flow** | None | 0 | — | `scenarios/` is unrelated (simulator corpus YAML). No test simulates "init → bootstrap-filters → new → next → status → review → finalize" as one continuous flow with a real bead store. |
| **Coverage** | `go test -cover` (manual) | — | Aggregate run today: see §1.2 | No `.codecov.yaml`, no Makefile, no script. Numbers are not tracked over time. |
| **Linting** | None enforced | — | — | No `.golangci.yml`, no `staticcheck` config. `go vet` runs implicitly with `go test` and that's it. |
| **CI** | **None.** | — | — | `.github/` does not exist. No workflows. Nothing prevents a broken `main`. |

### 1.2 Coverage snapshot (`go test -cover ./...`)

```
cmd                                 77.9%
cmd/kerfsim                         63.3%
internal/areas                      88.1%
internal/beads                      90.2%
internal/bench                      63.8%
internal/cmdutil                    29.2%   ← low
internal/codename                  100.0%
internal/config                     90.2%
internal/dep                       100.0%
internal/doctor                     62.9%   ← low; this is where doctor BLOCKER lives
internal/drift                      90.1%
internal/feed                       86.9%
internal/jig                        76.8%
internal/labelsample                85.2%
internal/project                    88.2%
internal/queue                     100.0%
internal/session                    88.9%
internal/sim/baselines              93.2%
internal/sim/duration               74.4%
internal/sim/event                  96.8%
internal/sim/generator              74.9%
internal/sim/loop                   87.5%
internal/sim/metrics                82.6%
internal/sim/output                 67.3%
internal/sim/policy                 [no tests]
internal/sim/run                    85.6%
internal/sim/scenario               39.9%   ← low
internal/sim/scenario_import        77.0%
internal/sim/seed                  100.0%
internal/sim/store                  68.0%
internal/snapshot                   79.7%
internal/spec                       82.7%
internal/storage                    49.1%   ← low
internal/testutil                   86.8%
scenarios                            0.0%   (only embed FS asserts)
github.com/gberns/kerf (main.go)     0.0%
```

### 1.3 Subprocess realism (critical for the bugs that slipped)

- `internal/beads/beads.go` shells out to the configured beads tool (`br` by default, configurable via `tools.tasks`).
- Every test that touches this path uses one of two patterns:
  - `isolatePATH(t)` — points PATH at empty tempdir, asserts the "tool missing" degrade.
  - `stubBr(t, json)` — writes a `#!/bin/sh\ncat <<EOF` script to a tempdir on PATH, emitting a fixed JSON document.
- **There is no test that runs a real `br` or `bd` binary against a real store.** Plan-021 (`BEADS_TOOL_ERROR`) explicitly went through review; the dogfood found that the actual bug (doctor crashes on `br` v0.1.45 against a `bd` store) lives precisely in the gap between the stub and reality.

### 1.4 What the testing spec said vs. what we built

`specs/testing.md` calls for six layers (unit, property, integration, E2E, agentic/exploratory, fuzz) and a CI matrix per trigger. **None of the CI matrix is implemented.** Agentic/exploratory currently means "a human runs six sub-agents by hand once per session" — i.e. the dogfood test that surfaced the BLOCKERs.

## 2. Gap analysis — which layer would have caught each dogfood bug

| # | Severity | Bug | Layer that would have caught it | Why current layers missed it |
|---|----------|-----|---------------------------------|-------------------------------|
| 1 | BLOCKER | `kerf doctor` crashes on `br` subprocess error | **E2E with real `br`/`bd` binary** against an incompatible store; or contract test that simulates `br` exiting non-zero with malformed JSON | doctor tests inject beads via a Go loader (`beadFilterCoverageLoader`), bypassing the subprocess code path entirely |
| 2 | BLOCKER | `.gitignore` pattern `.kerf/` + `!.kerf/project-identifier` is broken | **Scenario test** that runs `kerf setup`, then real `git add .` + `git status` and asserts `project-identifier` is staged | `setup_test.go` only string-matches stdout; it never executes `git add` against the written `.gitignore` |
| 3 | BLOCKER | `kerf next` exits 0 on `br` failure (other commands exit 1) | **Cross-command consistency property test**: for every command, when `br` returns non-zero, exit code is non-zero | No such property exists. Per-command unit tests use the stubbed-JSON happy path |
| 4 | BLOCKER | Near-match advisor never fires for realistic input (`label=gama` vs `codename:gama`) | **Scenario / agentic E2E** with realistic codenames; or property test asserting "advisor fires whenever ≥1 bead has a label whose colon-suffix equals the filter value" | Unit test used label `foo`, store had label `foo` — the test never crossed the namespace-prefix gap that real beads have |
| 5 | MAJOR | `--created-by self` no-op (session ID not recorded) | **Integration test**: create work, query `--created-by self`, assert ≥1 row | No test ever round-trips the filter through a populated `sessions[0].id` |
| 6 | MAJOR | Malformed `project.yaml` overwritten silently on rerun | **Integration test**: hand-write malformed yaml, run `kerf init`, assert file unchanged | `init_test.go` only tests clean-slate init |
| 7 | MAJOR | Corrupt `.kerf/project-identifier` leaks raw Go error | **Fuzz / property test** on project-identifier bytes; or integration test with garbage in the file | `internal/project` tests assume well-formed input |
| 8 | MAJOR | `any:` parser asymmetry (bootstrap writes it, `work edit` rejects it) | **Round-trip property test**: anything bootstrap proposes, `work edit --bead-filter-add` accepts | Two parsers, two test suites, no shared corpus |
| 9 | MAJOR | `kerf new <codename>` doesn't pre-populate `bead_filter` from matching labels | **Scenario test**: stage store with `codename:auth` beads, run `kerf new auth`, assert `bead_filter.label == "codename:auth"` | `new_test.go` doesn't stage beads; this code path is dead in tests |
| 10 | MAJOR | `kerf config tools.tasks bd` errors with "unknown key" | **Integration test**: round-trip every documented config key | `config_test.go` covers a fixed set, not the documented schema |

**Pattern.** 9 of 10 bugs are at integration boundaries the test suite either stubs out (#1, #3, #4, #5, #8) or never exercises end-to-end (#2, #6, #7, #9, #10). Coverage % does not capture this: `internal/beads` is at 90.2% and still shipped #3.

## 3. Recommendations (priority order)

### R1 — Real-binary scenario harness (highest ROI)

**Build.** A `scenarios_test.go` (or `internal/scenariotest/`) package that:
- Builds the `kerf` binary once per test run.
- Provisions a `bd`-shaped bead store via the real `bd` binary in a tempdir (skip cleanly if `bd` not on PATH; mark CI as requiring `bd`).
- Runs `kerf` commands as subprocesses, asserts on stdout/stderr/exit code.
- Covers the canonical agent flow: `init → setup → bootstrap-filters → new → next → status → review → preview → finalize`.

**Would cover.** BLOCKERs #1, #2, #3, #4, MAJORs #5, #9, #10 directly. Each future dogfood-class bug becomes a regression test.

**Effort.** ~2 days for the harness; ~1 day per flow scenario. Start with 3 scenarios (happy path, doctor-degrade, bootstrap+next).

### R2 — Cross-command contracts as property tests

**Build.** A single test file that enumerates every cobra subcommand and asserts shared invariants:
- "If `br` exits non-zero, kerf exits non-zero" (catches #3).
- "Every config key reachable via `kerf config <k> <v>` is also documented in `commands.md`" (#10).
- "Anything `bootstrap-filters` writes is accepted by `work edit --bead-filter-add`" (#8) — round-trip the proposal corpus.

**Would cover.** BLOCKER #3, MAJORs #8, #10, and prevents whole classes of asymmetry.

**Effort.** ~1 day. Reflection over the cobra tree gets you the enumeration cheap.

### R3 — CI that actually runs

**Build.** `.github/workflows/test.yml` that:
- Installs `bd` (vendored binary or `go install`).
- Runs `go test -race -cover ./...` on push + PR.
- Uploads coverage; fails if `cmd/` or `internal/beads/` drops below thresholds (start at current numbers, ratchet up).
- Runs `go vet` and `golangci-lint run` (with a minimal `.golangci.yml`: `govet`, `staticcheck`, `errcheck`, `ineffassign`, `unused`).
- A nightly job runs fuzz with `-fuzz=. -fuzztime=5m` per fuzz target.

**Would cover.** Closes the loop on R1+R2 — gives the review gate something to actually check before merge. Catches "I forgot to run the tests."

**Effort.** ~0.5 day.

### R4 — Negative-input integration tests for first-touch surfaces

**Build.** A targeted `negative_inputs_test.go` per first-touch command (`init`, `setup`, `new`):
- Garbage bytes in `.kerf/project-identifier` (#7).
- Pre-existing malformed `project.yaml` on `kerf init` (#6).
- Pre-existing valid-but-mismatched `project.yaml`.
- Bench dir owned by another user / read-only / nonexistent.

**Would cover.** MAJORs #6, #7. Pattern is reusable for future commands.

**Effort.** ~1 day.

### R5 — Promote property/fuzz coverage to the gaps the spec already names

`specs/testing.md` already lists targets we have not implemented:
- YAML round-tripping for `spec.yaml` + `config.yaml` (we have partial).
- Snapshot integrity (snapshot then restore == identical bytes).
- Concurrent file operations.
- Config merging (bench + project + work).

Pick the two highest-leverage (round-trip + snapshot integrity) and write `gopter` or stdlib-`quick` tests.

**Effort.** ~1 day for both.

## 4. Open questions for the user

1. **`bd` as test dependency.** Are we OK requiring a `bd` binary in CI? Alternative is a Go re-implementation of `bd`'s JSON contract used only in tests — more work, fewer external moving parts.
2. **Coverage threshold policy.** Hard fail at current numbers, or warn-only initially? (Hard fail risks blocking unrelated work; warn-only risks getting ignored.)
3. **Scope of `scenario` tests.** Where does the agent simulator (`internal/sim/*`) end and the integration scenario harness begin? Both could in principle drive `kerf` subprocesses; want to avoid two parallel harnesses.
4. **Lint strictness.** Start permissive (`govet`, `staticcheck` only) and tighten, or go straight to a substantial preset? Permissive is faster to merge but risks landing a noisy baseline that nobody clears.
5. **Agentic/exploratory cadence.** The dogfood found these bugs in one half-day run with six parallel sub-agents. Schedule it (e.g. before each tagged release) and codify it in a checklist? Or invest in R1+R2 first and treat agentic tests as the long-tail catcher rather than the front line?

---

Translations glossary (this document):
- `bd` / `br` — the beads task-tracker CLIs kerf shells out to (`bd` is the current dev branch, `br` is an older release).
- `dogfood test` — `plans/_dogfood/test_2026-05-18/` exploratory run on 2026-05-18 that surfaced the 10 bugs catalogued in §2.
- BLOCKER / MAJOR — severity tags from `plans/_dogfood/test_2026-05-18/SUMMARY.md`.
- Plan 021 — `bead-tool-resolution`, the existing plan that introduced `BEADS_TOOL_ERROR` surfacing.
