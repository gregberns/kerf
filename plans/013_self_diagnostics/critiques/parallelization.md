# Critique — Parallelization Opportunities

## Headline

Plan 013 sketches a three-layer L0/L1/L2 sequence in the plan body, but the
real critical path is **transcript parser → bead-ID indexer → detectors**.
Once those two foundations land, the six detectors are largely independent
and can run in parallel (4–6 implementers concurrently). The plan's current
sequencing under-states parallelism.

## True dependency graph

```
L0  parser       — internal/transcript: JSONL → events, sessions, sub-agent dispatches
L0  indexer      — internal/transcript: commit-message bead-ID index (parent/child rollup,
                   worktree refs, close-commit detection, alias resolution)
L0  spec doc     — specs/diagnostics.md: thresholds, severity, output shape
L0  fixtures     — internal/diagnose/testdata: promote source/detector_examples.md to
                   committed fixtures (transcripts + expected findings)

L1  detector skel — internal/diagnose/registry.go (plugs into doctor.Registry)
                    Depends on: L0 spec doc

L2  D1 abandoned dispatch   [needs parser + indexer + skel + fixtures]
L2  D6 reviewer-absent      [needs parser + skel + fixtures]
L2  D3 stalled conflict     [needs parser + skel + fixtures]
L2  D5 silent retry         [needs parser + skel + fixtures]
L2  D2 phase regression     [needs parser + indexer + skel + fixtures + baseline window]
L2  D4 outlier duration     [needs parser + skel + fixtures + p95 from project history]
       (D4 also pulls in Plan 012's duration distributions; the current
        Go binary has no duration-distribution loader yet, so D4 has an
        extra prereq — either port the Plan 012 fit data, or compute p95
        on-the-fly from the corpus.)

L3  CLI surface — register all detectors in cmd/doctor.go; integration test
                   one end-to-end run against a fixture project; documentation
```

L2 has six detector beads, all independent of each other. Spawning 5+ workers
in parallel after L0+L1 is the right shape.

## Parallelization gotchas (from past kerf incidents)

- **Newfunc-name collisions in test helpers.** The existing review-gate notes
  mention three parallel doctor-detector worktrees each defined
  `newTestContext` and the merge failed. The bead specs must each declare a
  distinct helper name (e.g., `newAbandonedCtx`, `newReviewerCtx`) or all use
  a shared helper that lands as part of L1.
- **Shared `internal/diagnose` package directory.** Six detectors landing in
  the same package directory is the textbook scenario for the integrated-state
  test failure described in `~/.claude/projects/-Users-gb-github-kerf/memory/MEMORY.md`.
  Mitigation: each detector in its own file (`d1_abandoned.go`, …), and the
  orchestrator runs `go test ./internal/diagnose/...` after each merge, not
  just the worktree-local test.
- **Registry-entry merge contention.** All six detectors `Register` in
  `internal/diagnose/init.go` (or similar). Concurrent edits to one file by
  six worktrees will conflict. Two options:
  - Land L1 with all six `Register` calls present but each detector returns
    a TODO finding; L2 worktrees only edit their own detector file.
  - Or have each detector self-register in its own file's `init()`.
  The second is cleaner and avoids the merge.

## Detectors that can be sequenced cheaper than parallelized

- **D2 and D6.** Source examples flag they fire on the same population. Land
  one bead that builds both side-by-side and decides the suppression rule in
  code, not two parallel worktrees that have to be reconciled at merge.
- **D1 and D5.** D5 ("silent retry") is "two D1-flagged dispatches for the
  same bead, second produced a commit." Cheaper to land D1, then D5 as a
  small follow-up that consumes D1's findings, than to write the dispatch
  detection logic twice in parallel.

## Recommended sequencing

```
Wave 1 (parallel, ~4 workers):
  B1 parser
  B2 indexer
  B3 specs/diagnostics.md
  B4 fixtures from source/detector_examples.md

Wave 2 (after Wave 1 lands; 1 worker):
  B5 internal/diagnose skeleton + doctor.Registry integration

Wave 3 (parallel, ~4 workers):
  B6  D1 abandoned dispatch
  B7  D3 stalled conflict
  B8  D4 outlier duration (with floor + effect-size compound predicate)
  B9  D2+D6 phase regression and reviewer-absent (single bead, two
       sub-findings, shared baseline-window helper)

Wave 4 (1 worker; depends on B6):
  B10 D5 silent retry (consumes B6's dispatch index)

Wave 5 (1 worker):
  B11 cmd/doctor.go wiring + integration test + commands.md detector
      table update
```

Total: ~11 beads, three serial waves with parallel fans in waves 1 and 3.
End-to-end with 4-worker parallelism: ~3 wave durations vs. the plan's
implicit ~6 (L0/L1/L2 each running serially).

## Time-to-first-value

The plan says "ship one detector first to validate the surface." With this
sequencing, D6 (reviewer-absent) is the cheapest first ship: no indexer, no
distributions, just "parse the transcript, count sub-agent dispatches per
bead, report beads with zero reviewer dispatches." D6 could ship after
Wave 2 as a minimal proof, before the rest of Wave 3 starts.

## Recommendation

- Restructure the plan's "Sequencing" section into the wave model above.
- Promote the parser, indexer, and fixture-promotion to first-class beads.
- Merge D2 and D6 into one bead by default; split only if the implementer
  finds a structural reason.
- Add an explicit "ship D6 alone after Wave 2" milestone for early validation.
