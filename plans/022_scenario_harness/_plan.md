# Plan 022 — Real-Binary Scenario Test Harness

> **Status: baked.** First of three follow-ups to the 2026-05-19 testing-strategy audit (`plans/_dogfood/testing_strategy_audit.md`). Sibling Plans 023 (cross-command property contracts) and 024 (CI + lint + coverage gates) cover the audit's R2 and R3.

## Intent

Build a Go-test-driven scenario harness that compiles the real `kerf` binary once per run, provisions a real `bd`-shaped bead store in a tempdir, and drives the full agent lifecycle — `init → setup → bootstrap-filters → new → next → status → review → preview → finalize` — as subprocesses. The existing `*_e2e_test.go` files invoke cobra `RunE` in-process and stub `br` with a static-JSON PATH shim (`stubBr` in `cmd/init_bead_filter_test.go`); every dogfood-class bug at the kerf↔bd integration boundary slips that net. The harness closes the gap with the smallest viable real-world surface.

## Background

The 2026-05-18 dogfood run (`plans/_dogfood/test_2026-05-18/SUMMARY.md`) surfaced 4 BLOCKERs and 6 MAJORs. The audit's gap-analysis table attributes 7 of those 10 directly to layers the suite stubs or skips:

- **BLOCKER #1** — `kerf doctor` crashes when `br` exits non-zero against an incompatible store. The doctor tests inject beads via a Go loader and never cross the subprocess boundary.
- **BLOCKER #2** — `.gitignore` pattern from `kerf setup` doesn't allow `!.kerf/project-identifier`. `setup_test.go` string-matches stdout but never runs `git add` against the file it just wrote.
- **BLOCKER #3** — `kerf next` exits 0 on `br` failure where siblings exit 1. No test runs a failing `br` and asserts exit code.
- **BLOCKER #4** — Near-match advisor (the marquee Plan 019 feature) never fires for realistic input. Unit tests used label `foo` against store labels `foo`; real beads carry `codename:foo` and the advisor's prefix logic never gets exercised.
- **MAJORs #5, #9, #10** — `--created-by self` no-op, `kerf new <codename>` not pre-populating from labels, `kerf config tools.tasks bd` rejected. All require a populated real store + subprocess call.

The audit names this work as **R1, highest ROI**. Plans 023 and 024 complement but don't replace it.

## Scope

In scope:

- A `internal/scenariotest/` package (Go) that builds `kerf` once per `go test` invocation, exposes a `Harness` value with `Run(args...)` returning stdout/stderr/exit code, and seeds a fresh `bd` store + tempdir bench per scenario.
- A `scenarios_test.go` file (location TBD — likely `cmd/scenarios_test.go` or a top-level package) housing the scenario functions themselves.
- Three opening scenarios chosen for coverage breadth: (a) happy-path bootstrap-to-finalize, (b) doctor-degrade against a deliberately incompatible store, (c) `bootstrap-filters` proposal round-tripped through `work edit --bead-filter-add`.
- Skip-with-message behaviour when `bd` is not on PATH, so contributors without `bd` installed can still run `go test ./...`.
- Per-scenario isolation: each scenario gets its own bench tempdir, its own `bd` store, its own HOME, its own PATH ordering. Cleanup via `t.TempDir` and `t.Cleanup`.

Out of scope:

- CI provisioning of the `bd` binary — Plan 024.
- Cross-command property contracts (audit R2) — Plan 023.
- Negative-input integration tests (audit R4) — separate plan when prioritised.
- Replacing existing in-process integration tests; this harness adds a layer, it doesn't substitute.

## Design notes

**One build per test run.** A `TestMain` (or `sync.Once` inside the harness package) runs `go build -o $TMPDIR/kerf .` (root package) once. Subsequent scenarios reuse the binary path. Build is skipped if `KERF_TEST_BINARY` env var points at a prebuilt binary, so CI and local dev can avoid double builds.

**Real `bd` store.** Each scenario calls `bd init` (or the equivalent setup command) in a tempdir. If `bd` is absent from PATH, the harness calls `t.Skip("bd not on PATH — install bd to run scenario tests")`. We do not vendor or re-implement `bd` for v1 — it's a real external dependency, same as `git` already is for the existing E2E tests. The Go re-implementation alternative (audit open-question #1) stays open for later.

**Subprocess invocation.** Harness builds `exec.Cmd` rooted at the kerf binary, with `HOME`, `PATH`, and the scenario's working directory pinned. Stdout/stderr captured into buffers, exit code surfaced verbatim. Helper assertions live on the harness (`AssertExitCode`, `AssertStdoutContains`, `AssertStderrMatches`). Timeouts come from a per-call default (5s) overridable per scenario; a hung subprocess fails loud rather than blocking the suite.

**Environment hygiene.** The harness scrubs `KERF_*` and `BD_*` env vars from the parent process before each scenario, then sets only what the scenario declares. Without this, a developer's `KERF_BENCH` or `BD_STORE` will silently steer tests into the wrong store and either pass spuriously or smear state across scenarios.

**Fixture seeding.** Scenario authors can stage beads, write `.kerf/config.yaml`, plant a malformed `project.yaml`, or pre-populate work directories before invoking `kerf`. The harness exposes thin helpers (`SeedBeads([]beadFixture)`, `WriteFile(rel, contents)`) rather than baking fixture DSLs. `SeedBeads` shells out to `bd` rather than poking the SQLite file directly — keeps the test contract aligned with what real agents do, and means a schema bump in `bd` doesn't quietly invalidate the fixtures.

**Git substrate.** The happy-path scenario needs a real git repo (kerf derives project ID from the remote). The harness reuses `testutil.SetupGitRepo` semantics: `git init`, an empty initial commit, and a fake remote URL. No network.

**Stdin and interactive prompts.** `kerf bootstrap-filters` and a few others read from stdin. The harness exposes `RunWithStdin(input, args...)` for those cases and defaults non-interactive commands to a closed stdin so a missing `--yes`-style flag fails fast rather than blocking on the read.

**Parallelism.** Scenarios call `t.Parallel()` by default; isolation is per-tempdir so they don't contend. The single `go build` is shared safely (one-time init).

**Cleanup.** All state is under `t.TempDir`, which Go cleans automatically. The kerf binary lives in `TestMain`'s tempdir and dies with the test process.

**Speed.** Each scenario costs roughly: build amortised + 1 `bd init` (~100ms) + 4–8 kerf subprocess calls (~50ms each). Three scenarios should land under 5 seconds wall-clock — comparable to the existing `*_e2e_test.go` set.

## Spec changes

`specs/testing.md` already names an "End-to-end" layer with the right shape ("E2E tests use a Go test harness... that sets up real git repos, runs the CLI binary, and verifies outcomes"), but its current coverage targets list git-only flows. Update needed:

- Add a coverage bullet under the E2E section: "Full agent bootstrap flow against a real `bd` store: `init → setup → bootstrap-filters → new → next → status → review → finalize`."
- Note the `bd`-on-PATH skip convention so future contributors don't reintroduce in-process stubs for this layer.
- No new layer; no normative MUST language.

The spec change is small and lands inside this plan's first bead, before the harness code.

## Beads outline

Rough work units (codenames assigned at implementation time — leaving the namespace open for Plans 023 and 024):

1. **Spec touch-up.** Update `specs/testing.md` E2E section per above. Independent, mergeable alone.
2. **Harness scaffold.** New `internal/scenariotest/` package: `Harness` struct, `TestMain`-driven build, subprocess runner, assertion helpers, `bd`-on-PATH skip. Includes its own unit tests for the harness primitives (build cache, env scrubbing, capture semantics).
3. **Scenario A — happy path.** `init → setup → bootstrap-filters → new → next → status → review → finalize` with a seeded multi-bead store. Asserts each command's exit code, key stdout fragments, and the `.gitignore` + `project-identifier` interaction (catches BLOCKER #2).
4. **Scenario B — doctor-degrade.** Reproduce the BLOCKER #1 case by either (a) installing a deliberately-old `br` shim alongside a fresh `bd` store, or (b) corrupting the store to force `bd` to exit non-zero. Either way: `kerf doctor` must exit 0 with a RED finding, not panic. Bead spec should pick the cleaner reproduction once both are tried.
5. **Scenario C — bootstrap round-trip.** Stage beads, run `kerf bootstrap-filters`, capture the proposed filter, feed it back through `kerf work edit --bead-filter-add`, assert acceptance (catches MAJOR #8). The near-match advisor case (BLOCKER #4) needs its own scenario — split if it grows beyond one assertion.
6. **Failure-mode shakedown.** A single scenario that deliberately kills `bd` mid-flow and asserts every kerf command's exit code is non-zero (catches BLOCKER #3 without waiting for Plan 023's full property test).

Beads 3–6 are independent and parallelisable once bead 2 lands. Beads 1 and 2 can run concurrently.

## Open questions

1. **`bd` version pinning.** Do we pin a specific `bd` release for scenario tests, or test against whatever's on PATH? Pinning gives reproducibility but risks divergence from what users actually run.
2. **Skip vs. fail when `bd` missing.** Skip is contributor-friendly; fail is CI-honest. Plan 024 will resolve this for CI specifically — local default stays skip.
3. **Test-binary caching across runs.** Worth a `go test`-aware build cache (e.g., hash of source tree) to skip rebuilds on unchanged source? Probably premature.
4. **Scenario corpus governance.** Where do new scenarios live as the suite grows — one file per scenario, or grouped by surface? Defer until we have >10.
5. **Overlap with the simulator's subprocess driver** (audit open-question #3). The simulator already shells out to `kerf` in some paths; want to avoid two parallel harnesses. Worth a brief survey of `internal/sim/*` before bead 2 starts.
6. **Negative-input scenarios.** The audit's R4 names a separate `negative_inputs_test.go` family (malformed `project.yaml`, garbage in `project-identifier`, etc.). These could be additional scenarios in this harness rather than a sibling plan. Decide once Scenarios A–C are landed and we know whether the harness scales to that shape cleanly.
7. **Flake budget.** Subprocess tests historically flake on slow CI. Track flake rate per scenario; if any scenario flakes >1% over 100 runs, fix or quarantine. Mechanism for the quarantine register is deferred to Plan 024.

## Translations glossary

- `bd` / `br` — the beads task-tracker CLIs kerf shells out to. `bd` is the current dev branch (SQLite-backed); `br` is the older release. Plan 021 (`bead-tool-resolution`) introduced configurable selection.
- `dogfood test` — the 2026-05-18 exploratory run that produced the 10-bug triage (`plans/_dogfood/test_2026-05-18/SUMMARY.md`).
- Audit — `plans/_dogfood/testing_strategy_audit.md`, dated 2026-05-19.
- Scenario test — an end-to-end test that drives the real `kerf` binary as a subprocess against a real `bd` store, asserting on stdout / stderr / exit code.
- BLOCKER / MAJOR — severity tags carried over from the dogfood summary.
