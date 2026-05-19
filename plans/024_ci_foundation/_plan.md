# Plan 024 — CI Foundation (test workflow + lint + coverage gates)

> **Status: baked.** Closes the third recommendation from `plans/_dogfood/testing_strategy_audit.md` (R3). Sibling plans: 022 (real-binary scenario harness), 023 (cross-command property contracts). 024 gives both of those somewhere to run automatically.

## Intent

Stand up GitHub Actions CI for kerf so that `main` is protected by an objective signal — not just a human review gate. Three workflows: a per-PR test+vet+race+cover run, a per-PR lint run, and a nightly fuzz run. Add a minimal `.golangci.yml`, a coverage ratchet checked in CI, and a README badge.

## Background

The repo has no `.github/` directory today. `go vet` runs implicitly with `go test`; nothing else does. There is no Makefile, no `.golangci.yml`, no `.codecov.yaml`, and no script that wraps the test invocation. Coverage numbers exist (audit §1.2) but only because someone ran `go test -cover` by hand.

This is the highest-leverage process gap surfaced by the 2026-05-18 dogfood:

- **Bug #2 (.gitignore pattern).** A scenario test would catch it — but only if CI runs scenario tests. No CI means the test would have been written, run once locally, then rotted.
- **Bug #3 (`kerf next` exits 0 on `br` failure).** A cross-command property contract would catch it — but a property test that nobody runs is no better than no test.
- **Bug #1 (doctor crash on `br` error).** Same shape: the fix is a test, the meta-fix is making the test run on every PR.

Every recommendation in the audit assumes a CI loop exists. None of them are durable without it. Plans 022 and 023 land tests; 024 makes those tests gate-keep `main`.

The review gate caught spec drift this session — but spec drift is the kind of defect humans can see. Integration-boundary regressions are exactly what a green-or-red CI signal is for.

**Dogfood bugs CI alone would have flagged.** Independent of 022/023:

- Any of the bugs whose fix lands with a regression test — once that test exists, CI keeps it green. Without CI, the test runs once on the author's laptop and rots.
- A future regression in any of the 70+ unit tests we already have. Today a careless commit can land on `main` with `go test` red and nobody notices until the next person runs it locally.
- Lint-grade defects: unused imports, unchecked errors, format drift. `errcheck` would have flagged at least one of the "function returns error, caller ignores it" patterns the audit's low-coverage packages exhibit.

CI does not by itself catch the integration-boundary bugs — that's what 022 and 023 are for. CI's job is to make those tests *durable*.

## Scope

**In.**
- A `test.yml` workflow: `go test -race -cover ./...` on push to `main` and on PR, with `bd` installed.
- A `lint.yml` workflow: `go vet ./...` + `golangci-lint run` on PR.
- A `fuzz.yml` workflow: nightly schedule, runs each `Fuzz*` target for a short budget.
- A `.golangci.yml` config — minimal, opinionated.
- A coverage ratchet: per-package thresholds, fail PR if any package drops below its current floor.
- A README badge linking to the test workflow.
- A short note in `specs/testing.md` § "CI Strategy" pointing at the workflow files as the implementation of that section's table.

**Out.**
- Release pipelines, binary builds, version tagging, `goreleaser`.
- Cross-OS or cross-arch matrices. Single `ubuntu-latest`, single Go version.
- Codecov.io or any third-party coverage SaaS. Local-only ratchet, no upload.
- Deploy, container build, anything that produces an artifact.
- Agentic/exploratory automation. That stays human-driven until someone designs a budget for it.

## Design notes

### Lint level

Start permissive. The `.golangci.yml` enables: `govet`, `staticcheck`, `errcheck`, `ineffassign`, `unused`, `gofmt`, `goimports`. No style-only linters (`revive`, `stylecheck`, `gocritic`) on day one — they generate noise the team has to triage. PR-blocking from the first PR; if the baseline isn't green, fix it before merging the workflow, don't merge the workflow with a long ignore list. The audit's R3 calls out exactly this preset and the working-style file's "permissive first, tighten later" maps cleanly.

### Coverage tool

`go test -cover -coverprofile=coverage.out ./...` plus a tiny `scripts/coverage-ratchet.go` that parses the profile, compares each package to a checked-in `coverage.floor.json`, and exits non-zero if any package regressed. No Codecov, no HTML upload in v1. The floor file lives in `.github/` and gets bumped manually when a PR raises coverage. Open question below covers whether `cmd/` and `internal/beads/` get tighter floors than the rest.

Rationale: the audit (§1.2) shows a long tail of low-coverage packages (`internal/cmdutil` 29%, `internal/storage` 49%). Hard-failing at current numbers prevents *further* erosion without forcing a coverage rewrite first. The ratchet is the lightest tool that delivers that signal.

### Go version

Single version: 1.26.1, matching `go.mod`. No matrix. Adding 1.25 buys nothing — kerf has no library consumers and the binary ships built against the toolchain in `go.mod`. Revisit if/when kerf becomes an importable library.

### `bd` provisioning

`bd` is required by some integration tests (and by 022's scenario harness once it lands). The test workflow installs `bd` via `go install` against a pinned version, written to the runner's PATH before `go test`. Pinned version lives in a single CI variable so updates are one-line PRs. If `bd` install fails, the workflow fails — kerf's tests assume `bd` exists.

### Fuzz cadence

Nightly, `-fuzztime=2m` per target (3 targets today → ~6 minutes total). Findings stored as artefacts; on failure the workflow opens a `bd` issue via the `kerf` CLI itself (stretch — not v1).

## Spec changes

`specs/testing.md` already declares a "CI Strategy" table (per-trigger layers). Add a single sentence at the top of that section: *"Workflow files in `.github/workflows/` implement this table; see plan 024."* No normative change — the section already specifies the desired behaviour. Nothing else in `specs/` is about build infrastructure.

CI itself is not normative behaviour of the tool, so no other spec is touched.

## Beads outline

1. **`workflow-scaffold`** — `.github/workflows/test.yml`. `go test -race -cover ./...` on push + PR, install `bd`, single Go version. Green on `main` before merge.
2. **`golangci-config`** — `.github/workflows/lint.yml` + `.golangci.yml` with the preset above. Baseline must be green or the PR fixes the noise.
3. **`coverage-ratchet`** — `scripts/coverage-ratchet.go` + `.github/coverage.floor.json` seeded from current numbers. Wired into `test.yml` as a post-step.
4. **`fuzz-nightly`** — `.github/workflows/fuzz.yml` on `schedule: cron`. Iterates over `Fuzz*` targets via `go test -list`.
5. **`readme-badge`** — README badge linking the test workflow. Updates `specs/testing.md` § "CI Strategy" with the one-sentence pointer above.

Dependencies: 2, 3, 4 all depend on 1 (need a workflows directory and a known-good baseline). 5 depends on 1 only.

Each bead is mergeable independently. None of them require Plans 022 or 023 to have landed — but once 022/023 do land, their tests automatically pick up CI coverage.

## Open questions

1. **PR-blocking lint vs advisory.** Default in the design above is blocking. The argument for advisory: noisy linter findings in early PRs train people to dismiss the signal. The argument for blocking (which I'd take): if we're going to add a linter, the only thing worse than no linter is a linter people learn to ignore.
2. **Coverage threshold policy — hard fail vs warn.** Default above is hard-fail on regression. Same logic as lint. Alternative: warn-only for the first month while the ratchet stabilises. Flagging because it's the question with the highest risk of bikeshedding.
3. **Per-package floor vs aggregate.** Per-package catches regressions in the long-tail low-coverage packages; aggregate is one number that's easier to reason about. Default: per-package.
4. **Should `cmd/` and `internal/beads/` get higher floors than the rest?** They're the integration surface where the dogfood bugs landed. Maybe — but tightening floors is a follow-up PR, not a v1 decision.
5. **`bd` version pinning strategy.** Currently `bd` is moving fast (the dogfood ran against v0.1.45 with known incompatibilities). Pin to a known-good commit, or track a release tag? Default: pin to a commit, document it in `.github/bd-version`.
6. **Cache.** Go module cache + build cache via `actions/setup-go@v5`'s built-in caching. Cheap; no question, just flagging it'll be in the workflow.
7. **Branch protection.** Once the workflow is green for a week, turn on "require status checks to pass before merging" on `main`. Out of this plan's scope (it's a GitHub UI setting, not code) but worth queueing as a follow-up.

## Translations glossary

- `bd` / `br` — the beads task-tracker CLIs kerf shells out to; `bd` is the current dev branch.
- `dogfood test` — `plans/_dogfood/test_2026-05-18/`, the half-day exploratory run that surfaced the 10 bugs the audit catalogues.
- `coverage ratchet` — a small script that compares current per-package coverage against a checked-in floor file and fails CI on regression.
- Plans 022 / 023 — sibling plans landing the real-binary scenario harness and cross-command property contracts; both consume 024's CI loop.
