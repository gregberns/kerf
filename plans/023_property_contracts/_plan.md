# Plan 023 — Cross-Command Property Contracts

> **Status: baked.** Companion to plan 022 (real-binary scenario harness) and plan 024 (CI). Implements recommendation R2 from `plans/_dogfood/testing_strategy_audit.md`: encode cross-command invariants as property-based tests over the cobra tree, so whole classes of asymmetry between sibling commands cannot regress unnoticed.

## Intent

The dogfood run on 2026-05-18 surfaced three bugs that share a shape: two surfaces in kerf claim to honour the same rule, but only one actually does. `kerf next` returns exit 0 on subprocess failure while every sibling returns 1 (BLOCKER #3). `bootstrap-filters` writes `any:` clauses that `work edit --bead-filter-add` will not parse back (MAJOR #8). `kerf config tools.tasks bd` is documented but rejected at runtime (MAJOR #10). Each was easy to ship because each parser, exit-code path, and config schema is tested in isolation. The fix is to assert the contract once, run it across every command, and let a single failure pin the asymmetry.

## Background

The current property layer (`internal/codename`, `internal/config`, `internal/spec`, `internal/snapshot`) covers serialization invariants inside one package. It does not cross the cobra-command boundary. Four packages get property coverage today; thirty-odd commands get none of the cross-cutting kind. The audit identifies a set of properties that have been **claimed in spec or implied by symmetry** but never tested:

- *Subprocess exit symmetry.* Every kerf command that shells out exits non-zero when the subprocess exits non-zero. `kerf next` violates this today (BLOCKER #3); the test that would have caught it is one property assertion run across every cobra leaf, not thirty per-command unit tests.
- *Filter-clause parser symmetry.* Anything `internal/labelsample` proposes is accepted by `internal/beads.ParseFilterClause` and by the `work edit --bead-filter-add` mutator. The `any:` grammar (MAJOR #8) is the live counter-example: the proposer writes a shape the mutator rejects.
- *`kerf config` key round-trip.* For every key documented in `specs/commands.md`, `kerf config K V` followed by `kerf config K` returns `V`. `tools.tasks` (MAJOR #10) fails this today.
- *`kerf show` / `kerf work show` field agreement.* Fields rendered by both commands carry identical text for the same underlying record. No live bug, but the audit calls it out as a future-bug surface; cheap to lock down now.
- *bead_filter slot invariant (kerf-3ac).* A present-but-empty `bead_filter` resolves identically to an absent one. Already unit-tested inside one package; the cross-command framing asserts that every command consuming the slot agrees.

None of these properties live in any one package; they describe how the cobra tree hangs together. Plan 023 picks a property-testing library, picks generators, and lands one test file per contract that walks the tree by reflection rather than by hand.

## Scope

In scope:
- A new package (likely `internal/contracttest` or `cmd/contract_test.go`) hosting the cross-command property tests.
- Reflective enumeration of the cobra command tree, with an opt-out tag for commands that legitimately exit 0 on subprocess failure (none expected; the audit asserts this set is empty).
- Generators for filter clauses, documented config keys, project IDs, and codenames, drawing from existing fixtures where possible.
- One property per item in the §Background list, plus the kerf-3ac slot invariant promoted from its existing unit-level home.
- Wiring into `go test ./...` so the contracts run on every commit (the CI piece itself is plan 024's job).

Out of scope:
- Real-binary execution (plan 022 owns that). Properties here run against in-process cobra `RunE` and `stubBr`-shaped stubs.
- New product behaviour. If a property exposes a bug, the bug gets a separate fix bead; this plan ships the test surface only.
- Promoting the existing four serialization property suites; they stay where they are.

## Design notes

**Library.** Default to stdlib `testing/quick`, matching the in-repo convention. Add `gopter` only if a property's generator becomes awkward in `quick` (likely candidates: filter-clause grammar, config-key enumeration). The cost of mixing libraries is small; the cost of forcing every contract through one is not. Either way, no new top-level dependency beyond what the audit already contemplates.

**Cobra enumeration.** Walk `rootCmd.Commands()` recursively, filter by a small interface (`Runnable()` plus non-hidden). Each property test takes the enumerated list and asserts the invariant per command. A command opts out by tag (struct field, annotation, or a centralised allowlist) — opting out requires a one-line rationale in the same file, so the asymmetry is at least documented.

**Generators.**
- *Filter clauses.* Reuse `labelsample`'s candidate-shape enumerator as the producer; feed each output to `beads.ParseFilterClause` and assert acceptance. For the reverse direction, generate clauses from a small grammar (`label=V`, `status=V`, `any:LEAF,LEAF`) and assert `labelsample` (or its mutator) accepts the same shapes its proposer emits.
- *Config keys.* Parse `specs/commands.md` (or wherever the canonical key list lives) at test time; for each, drive `kerf config K V` then `kerf config K`, assert the read echoes the write. Open question on the source-of-truth file below.
- *Project IDs / codenames.* Reuse the existing `codename` generators; no new work.
- *Subprocess failure.* Inject via the existing `stubBr` PATH shim with an `exit 1` script; assert kerf's own exit is non-zero.

**Invariant encoding.** Each contract is one `TestContract_<Name>` function. Failures report the offending command or input plus the contract id (so review can grep for the contract by name in `specs/testing.md`). No table-driven hand-rolled lists where a reflective walk works — the whole point is that adding a new cobra command auto-extends the contract.

**Shape of a contract test.** Concretely: the exit-symmetry contract walks `rootCmd`, skips opt-outs, and for each leaf invokes `RunE` with `stubBr` set to `exit 1` plus a minimal arg fixture. The assertion is `kerf-exit != 0`. The config round-trip walks the documented-key list, sets a generator-produced value, reads back, and asserts string equality. The filter-clause contract loops 1000× over `labelsample`-produced clauses, feeds each through both `ParseFilterClause` and the `--bead-filter-add` mutator, asserts both accept. The point of the framing is that a new command or a new clause grammar is one line of registration, not a new test file.

**What does not change.** No product code in this plan. No new top-level dependency unless `gopter` is adopted (open question 3). The existing serialization property suites stay where they are. The fix beads for BLOCKER #3, MAJOR #8, MAJOR #10 are separate work; this plan ships the tests that make them stay fixed.

**Interaction with plan 022.** Property tests here run in-process against `RunE`, fast, on every commit. The scenario harness in 022 runs the same shape of assertion against a real subprocess, slower, on PRs. Same contract, two enforcement points; the property version catches refactors, the harness version catches integration drift.

## Spec changes proposed

`specs/testing.md` — add a short subsection under "Property-Based Tests" listing the cross-command contracts as a recognised category. Names the five contracts above so future commands inherit the obligation by default. No change to the existing six-layer table.

Possibly a one-line cross-reference from `specs/commands.md` pointing to the config-key round-trip contract, so the documented-key list is understood to be load-bearing.

No new specs file. No edits to `specs/_index.md` beyond the existing testing.md entry.

## Beads outline

Rough decomposition (sequencing left to plan-implementation; sizes are estimates):

- **B1** — `internal/contracttest` skeleton + cobra walker + opt-out registry. One trivial smoke contract to validate the harness.
- **B2** — Contract: subprocess exit symmetry. Reuses `stubBr` with `exit 1`. Catches BLOCKER #3.
- **B3** — Contract: filter clause round-trip. Producer side (`labelsample` → parser) and a constrained reverse generator. Catches MAJOR #8.
- **B4** — Contract: documented config-key round-trip. Includes deciding the documented-key source of truth (open question 1). Catches MAJOR #10.
- **B5** — Contract: `show` / `work show` shared-field agreement. Lower priority; no live bug, but the audit names this as a likely future bug surface.
- **B6** — Contract: bead_filter slot invariant (present-empty vs absent). Promote/wrap the existing kerf-3ac unit-level check into the cross-command framing.
- **B7** — Spec edit + reviewer pass; closes the plan.

B1 blocks the rest. B2–B6 are mutually independent and parallelisable. B7 waits on whichever contracts land.

## Open questions

1. **Documented-config-key source of truth.** `specs/commands.md` enumerates them in prose; there is no machine-readable list. Options: (a) parse the markdown at test time, (b) add a `config.Documented()` exported slice and treat the spec as derived, (c) embed a YAML keys file. Option (b) is the least magical and the easiest to keep in sync, but it moves the source of truth from spec to code, which inverts the prime directive. Probably (b) with a `specs/testing.md` note that this list is the documented set; flag during implementation.
2. **Opt-out registry shape.** Cobra-annotation, struct-field, or a literal map in `contracttest`? The map is the simplest; the annotation keeps the rationale next to the command. Pick during B1.
3. **`gopter` adoption.** Worth the extra dependency for filter-clause generators alone? If the stdlib `quick` version is under ~50 lines, stay with stdlib; if generators sprawl, switch. Decide during B3.
4. **Reverse direction of filter symmetry.** "Anything `labelsample` produces, the parser accepts" is well-defined. The reverse — "anything the parser accepts, `labelsample` can produce" — is almost certainly false (the parser accepts a wider grammar than the sampler proposes). Probably skip the reverse and document the asymmetry as intentional.
5. **`show` / `work show` agreement scope.** What counts as a "shared field"? The audit names this loosely. Likely defer to whatever the shared rendering helper exposes; if no shared helper exists, B5 may also introduce one — pushing it past pure-test scope. Reconsider when B5 is picked up.

---

Translations glossary:
- `stubBr` — the PATH-shim helper in `cmd/init_bead_filter_test.go` that echoes canned JSON for `br`.
- `kerf-3ac` — the existing bead/work-filter slot invariant: a present-but-empty `bead_filter` resolves identically to an absent one.
- BLOCKER #3 / MAJOR #8 / MAJOR #10 — severity tags from `plans/_dogfood/test_2026-05-18/SUMMARY.md`.
- R2 — recommendation 2 in `plans/_dogfood/testing_strategy_audit.md`, "cross-command contracts as property tests."
