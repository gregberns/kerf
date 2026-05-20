# Exploratory test summary — 2026-05-18

Six parallel agents tested everything shipped this session. Per-area findings in sibling files (`init.md`, `doctor.md`, `triage.md`, `filters_next.md`, `review_preview.md`, `integration.md`).

## Headline

**Most features work as spec'd.** Counts by area:
- Plan 016 (init): 7/7 scripted scenarios pass + 1 nit
- Plan 017 (doctor): all 5 detectors verified + suppression matrix works
- Plan 018 (triage): 13/14 scenarios pass
- Plan 019 (filters/next): 11/14 scenarios pass — **near-match advisor never fires**
- Plan 020 (review/preview/show/status): 14/14 pass
- Cross-cutting integration: partially productive — several real friction points

## Triage list (severity-ordered)

### BLOCKER / RED

1. **`kerf doctor` crashes on `br` subprocess errors** instead of degrading to a RED finding. Unusable on this very repo today because `br` v0.1.45 is incompatible with the `bd`-shaped store. Detector should trap subprocess errors. *Source: doctor.md F1+F2.*

2. **`.gitignore` pattern advertised by `kerf setup` is broken.** `.kerf/` + `!.kerf/project-identifier` doesn't allow the negation. Correct form: `.kerf/*` + `!.kerf/project-identifier`. Every agent following the setup instructions hits this on `git add`. *Source: integration.md.*

3. **`kerf next` exits 0 on hard `br` subprocess failure**, while sibling commands exit 1. Scripts can't detect the failure. *Source: integration.md.*

4. **Near-match advisor (kerf-d9f) never fires in practice.** Reviewer's tests passed because the advisor was unit-tested in isolation; real-world exact case (`bead_filter: label=gama` against store with `codename:gama` beads) shows generic hint, no advisor. Marquee 019 feature broken. *Source: filters_next.md.*

### MAJOR

5. **`--created-by self` is a no-op.** Session ID isn't recorded as creator in `sessions[0].id` (stays null) — the column the filter consults is empty. *Source: filters_next.md.*

6. **Malformed `project.yaml` on rerun is silently overwritten.** Spec says "skip with informative output." Observed: kerf overwrites + emits full agent block. User-edited fields destroyed. *Source: init.md.*

7. **Corrupt `.kerf/project-identifier` not validated.** Garbage bytes pass through to `mkdir(2)`, low-level Go error leaks. *Source: init.md.*

8. **`any:` grammar asymmetry.** Bootstrap proposes `any:` unions but `kerf work edit --bead-filter-add 'any:label=foo,label=bar'` is rejected by the parser. *Source: filters_next.md.*

9. **`kerf new <codename>` does not pre-populate `bead_filter` from matching labels.** Three `codename:auth` beads + `kerf new auth` leaves slot null. Most of "attach work to beads" value-prop lost on first contact. *Source: integration.md.*

10. **`kerf config tools.tasks bd`** errors with "unknown configuration key" — agents must edit yaml manually. Conflicts with documented setup. *Source: triage.md (out-of-band).*

### MINOR

- `kerf doctor` storage-drift hint says "run `kerf localize`" in bench mode where the actual fix is to move dirs *off* `.kerf/works/`.
- JSON output of `kerf next` has no drift footer field; JSON consumers blind to storage drift.
- Drift footer is storage-only — bead-filter REDs don't trigger it.
- `--ack` re-run inside same second emits identical timestamp; could confuse fast loops.
- Tier-2 fallback always pins lex-earliest active work; could prefer "investigate manually" when no overlap.
- `--no` reports detector-silence wording instead of naming the flag as source.
- `kerf review` after `kerf new` says "no review criteria for Problem Space" — could hint `try --pass decompose`.
- `kerf preview <codename> <status>` positional vs `kerf review --pass <status>` named — inconsistent surface.
- Init block contains `kerf list` twice (Available + daily-driver).
- "not in a git repository" error message doubled (wrapper+cause concat).
- Plan 020 templates land for `spec` jig only — the `plan` jig has no pre-creation. Documentation gap if not feature gap.
- Pass 1 template (`01-problem-space.md`) not pre-created on `kerf new`, only on subsequent advances.
- `--bead-filter-remove` post-clear message claims "falls back to project filter" but observed work classifies as `unwired`.

### Process observation

The functional review gate caught spec drift well during implementation, but **didn't catch**:
- Subprocess error handling at integration boundaries (doctor crash on br failure)
- End-to-end flows that cross multiple commands (.gitignore pattern works in spec, fails in `git add`)
- Real-world inputs vs. test-fixture inputs (near-match advisor with realistic codenames)

Suggests: the next process improvement is **end-to-end integration tests** that exercise the full agent bootstrap flow against a real `br`-shaped store, not just unit-test-stub fixtures.
