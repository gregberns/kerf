# Independent Review — Plan 013 (Self-Diagnostics)

**Reviewer:** fresh-context, no prior critique read until after forming an initial take.
**Date:** 2026-05-19
**Verdict:** **proceed-with-changes.** The plan is real work that solves a real problem (procedural drift kerf is meant to catch), and the freshness-check correction to extend `kerf doctor` is right. But the plan as written is over-built: ship 2 detectors first, not 6, and clarify the surface story before more code lands.

The user's worry — "two tools that do similar things" / "building crap just to build it" — is partially answered by the in-thread critique (no `kerf diagnose`), but new duplication risks remain. Specifics below.

---

## 1. Feature duplication / surface sprawl

### What the in-thread critique got right

Killing `kerf diagnose` was correct. `internal/doctor/` already has `Detector`, `Finding`, `Registry`, `Report`, severity vocabulary, `--detector`/`--format`/`--strict` flags, and a "name the fix" output convention. A sibling command would have been pure duplication.

### What the in-thread critique missed

**`kerf doctor`'s current detectors are categorically different from D1–D6.** The five existing detectors are **static, snapshot, project-state** checks:

- `project-yaml` — does the file exist and parse?
- `storage-drift` — does state live in the canonical location?
- `symlink-integrity` — is a symlink intact?
- `bead-filter-coverage` — does each work resolve to ≥1 bead?
- `archive-orphans` — codename collisions?

Every one of these is **idempotent against repo state** and runs in milliseconds. The proposed D1–D6 are **historical-corpus** checks that:

- Read JSONL transcripts from outside the repo (`~/.claude/projects/...`).
- Walk `git log --all` across worktrees and indexes commit messages by bead-ID regex.
- Compute rolling-window statistics (p95, medians, phase rates).
- Take seconds to minutes to run on a real corpus.

These don't belong under the same command surface just because they share an interface type. A user running `kerf doctor` today expects sub-second feedback on "is my project state sane?" Mixing in "scan 30 days of transcripts and run statistical detectors" silently changes the contract.

**Concrete sprawl risks:**

1. **Output volume.** 5 detectors + 6 new = 11 rows minimum; on the harmonik corpus the in-source examples show 28 D1 findings, 29 D6 findings, multiple D4s. A `kerf doctor` invocation that today produces ~5 lines becomes a 60-line wall. The plan's sample output (lines 103–119 of `_plan.md`) shows this but doesn't grapple with it.
2. **Runtime contract.** `kerf doctor` is invoked in tight feedback loops (the `kerf next` footer at `commands.md:1855` even tells the user to run it). If it now takes 15s to scan transcripts, that loop breaks.
3. **`kerf next` integration is unspecified.** The storage-drift footer already mentions "run 'kerf doctor' for details." If D1–D6 ride the same channel, will every `kerf next` invocation in a session with abandoned dispatches show "12 doctor findings — run kerf doctor"? That's exactly the "alerts people ignore" failure mode the plan itself flags in Open Decision 5.

**Recommended fix — pick one:**

- **Option A (lightest):** Keep `kerf doctor` but **gate the diagnostic detectors behind a category flag**: `kerf doctor --diagnostic` (or `--category=diagnostic` vs `--category=structural`). The default `kerf doctor` stays fast. CI / wave-cleanup operators opt into the heavy run.
- **Option B:** Two sub-commands sharing the registry: `kerf doctor` (current detectors, fast) and `kerf doctor diagnose` (the new family). The shared registry already accepts ID-prefixed filtering; this is one branch in `cmd/doctor.go`.
- **Option C (most aligned with user's instinct):** Surface the highest-signal findings (abandoned dispatch, reviewer-absent) as **`kerf next` warnings**, not doctor findings. `kerf next` is the agent's pull signal that runs every cycle; doctor is run manually. Procedural drift that "no one was actively monitoring" needs the always-on surface, not the on-demand one. The current footer convention already exists for this.

The plan picks none of these explicitly. It picks "extend doctor" without confronting that doctor's existing role is different.

### Does `kerf review` belong in this conversation?

`specs/commands.md:529` shows `kerf review` emits the canonical reviewer prompt — it's the **prescriptive** side of the review gate. D6 (reviewer-absent) is the **detective** side. These should cross-reference. If `kerf review` was used, D6 would have evidence. The plan should commit a sentence in `specs/diagnostics.md` that D6's "reviewer dispatch" definition aligns with whatever `kerf review` produces, otherwise the definitions drift.

---

## 2. Transcript parser duplication (Python vs Go)

Plan 012 has a Python parser at `plans/012_real_corpus/data/extract.py`. Plan 013 ports it to Go as bead B2. The in-thread critique calls this out as "real new work" but accepts it without questioning whether the duplication itself is the problem.

**The duplication is the problem.** Kerf's stated pain point (from MEMORY.md `feedback_integrated_tests.md` and `feedback_worktrees.md`) is that "two implementations diverge." Two transcript parsers will diverge:

- Plan 012's parser is being actively iterated for simulator-corpus extraction; it lives in `plans/`, not `internal/`.
- Plan 013's Go parser will be the one running in production diagnostics.
- When Claude's transcript JSONL schema changes (it has changed multiple times historically), one parser gets updated and the other rots.

**Options:**

1. **Keep Python authoritative; Go shells out.** `kerf doctor --detector d1-abandoned` runs `python3 .kerf/extract.py --format json` and consumes the output. Pro: one parser. Con: Python dependency for kerf users, packaging headache.
2. **Port to Go, retire Python.** Plan 012's simulator-corpus extraction also uses the Go parser. One canonical implementation. Pro: clean. Con: more upfront work, and Plan 012 has to be re-routed.
3. **Port to Go, accept duplication.** What the plan currently says. Pro: ships fast. Con: known divergence risk.

The plan picked option 3 without naming options 1 or 2 as alternatives. Given the user's worry, the plan should at least **acknowledge** that two parsers is a known kerf failure mode and commit to a contract test (a fixture transcript that both parsers must agree on) so divergence is caught.

**Recommended:** Option 2 if scope permits — port to Go, route Plan 012 through it. Otherwise option 3 with a mandatory cross-parser contract test bead.

---

## 3. UX of the detectors — when does a user see them?

The plan's silence here is the loudest gap. `kerf doctor` is run manually. Nobody runs it after every wave today, and nothing in the plan changes that.

**Failure mode to prevent:** Plan 013 ships, the harmonik reviewer-absent regression is now detectable, but the operator only runs `kerf doctor` weekly. The detector finds 30 reviewer-absent commits. The operator dismisses them as "old news." The procedural drift kerf was meant to catch is technically surfaced and practically ignored.

**The plan's own quote (`_plan.md:26`):** *"this isn't great — that's why I want harmonik so the system doesn't have a choice whether it performs actions or not — they are procedural/enforced through harmonik's workflows."* — Enforcement requires the signal to interrupt the loop, not sit in a doctor report.

**Where the signal should appear:**

| Detector | Best surface | Why |
|----------|-------------|-----|
| D1 abandoned dispatch | `kerf next` warning | High-frequency, needs visibility every cycle |
| D2 phase regression | `kerf doctor` (manual) | Rolling-window stat, doesn't change every cycle |
| D3 stalled conflict | `kerf next` warning | Real-time signal, blocking |
| D4 outlier duration | `kerf doctor` (manual) | Post-hoc analysis, fine to batch |
| D5 silent retry | `kerf next` warning | Real-time |
| D6 reviewer-absent | `kerf next` footer | Per-bead, agent should see immediately |

The plan's commitment to one surface (`kerf doctor`) for all six detectors flattens this distinction. **The user's worry is exactly about this:** building features that aren't well thought out for actual use.

**Recommended fix:** Add a section to `specs/diagnostics.md` mapping each detector to its primary surface and consumption pattern. At minimum, name which findings should ride the `kerf next` footer channel.

---

## 4. Threshold defaults — defensibility

The plan commits: 60s dispatch floor, 30-bead baseline, 50%/10% phase cutoffs, 600s outlier floor, 2× median, 30-min conflict window, 20-bead D4 dormancy.

**Honest assessment of each:**

| Threshold | Source | Defensible? |
|-----------|--------|-------------|
| 60s dispatch floor (D1) | "suggest 60s default" in plan body, no derivation | **Weak.** Why not 30s or 120s? Sub-agents in `source/detector_examples.md` ran 9s apart in one example. The floor exists only to suppress noise from instant dispatches; pick by inspecting the corpus duration distribution and naming the percentile (e.g., "60s = ~10th percentile of dispatch durations in the kerf+harmonik corpus") |
| 30-bead baseline (D2, D4) | Convention | **OK** as starting point, but the source examples show kerf has only 52 beads total. A 30-bead window covers ~half kerf's history; for D2 that's reasonable, for D4 the "20-bead dormancy threshold" partially compensates |
| 50% historical / 10% current (D2) | "Example: 78% → 0%" in plan body | **Weak.** The single grounding example is 100%→0% (harmonik reviewers). 50%/10% is a guess at the boundary. Should the historical bar be 80% (clearer signal) and the current bar be 20% (cleaner regression)? No analysis presented |
| 600s outlier floor + 2× median (D4) | Plan body acknowledges the small-sample issue | **Good.** This is the only threshold with explicit reasoning. The compound predicate (floor AND ratio) is correct |
| 30-min conflict window (D3) | Convention | **Weak.** Source examples show conflicts unresolved for 19.8h and 35.4h — way past 30 min. So 30 min will fire ~immediately on any real conflict. Is that desired (sensitive) or noisy (most conflicts resolve in <30 min naturally)? The corpus has the data to answer this; plan didn't analyze it |
| 20-bead D4 dormancy | New, plausible | **OK.** Self-suppressing on small samples is the right move |

**False-positive risk audit (against `source/detector_examples.md`):**

- D1: source explicitly says ~75–80% FP rate from naive implementation. The indexer (B3) is supposed to fix this. Until B3 is proven against the harmonik corpus, **D1 will be the loudest detector with the lowest signal**. Reordering to ship D6 first is correct.
- D6: source examples show 29 of 30 recent harmonik beads are reviewer-absent. D6 will fire 29 times immediately. Is that one finding ("D2 fires; D6 suppresses") or 29 findings? The suppression rule exists but `_plan.md:138` says D6 suppresses *when* D2 fires — meaning until D2 ships (Wave 5), D6 floods. This sequencing is wrong if D6 ships first.

**Recommended fix:** Add a "calibration" step before B7 ships: run the proposed thresholds against the kerf and harmonik corpora, record the finding counts, adjust if any detector fires >5 times per project on the existing baseline. Document the rationale in `specs/diagnostics.md` next to each number.

**Specific concern:** the plan moves "drafting numeric thresholds" to bead B1 (spec draft) but doesn't commit B1 to running a calibration pass. B1 as currently scoped will commit thresholds by feel.

---

## 5. Are all six detectors necessary in one plan?

**No.** This is the strongest critique I have. The plan ships 6 detectors + parser + indexer + fixtures + spec + integration test = 12 beads, 6 waves, ~12 bead-units of work, before any user sees any finding.

**The MVP question the plan should have asked:** which 2 detectors, shipped tomorrow, would have caught the issues that motivated this plan?

- The plan's motivating examples (`_plan.md:17–19`): 123 abandoned dispatches (D1), reviewer phase missing from 100% of harmonik beads (D2/D6), wasted-effort signals (D1 again, D3).
- **D1 alone** catches abandoned dispatches.
- **D6 alone** catches the reviewer regression (it doesn't require D2's rolling baseline — D2 just makes the alert smarter).

**Two-detector MVP (D1 + D6):**

- Forces the parser and indexer to be production-grade.
- Surfaces the actual incidents that motivated the plan.
- Validates the `kerf doctor` extension surface choice (or surfaces that `kerf next` is the right channel).
- Defers D2 (rolling-window), D3 (event-pattern), D4 (statistics), D5 (D1-dependent) until we've learned whether the surface and threshold defaults survive contact with the corpus.

**Order of operations:**

1. **Plan 013a (this plan, scoped down):** parser, indexer, fixtures, registry, D1, D6. ~6–8 beads, 3 waves.
2. **Plan 013b (sibling, scheduled):** D2, D3, D4, D5 + threshold calibration based on what 013a found.

This sequencing also helps Plan 014's adaptive layer: 014 needs **some** diagnostic feed, not all six. D1 + D6 is sufficient to prove the feed exists; the rest can land as 014 demands them.

**Counter-argument the plan could make:** the parser and indexer are the long-pole work; once they're built, each additional detector is small. So ship all 6 together to amortize. — This is reasonable but assumes thresholds and surface choices won't change after D6 ships. They will.

**Recommended:** Cut beads B8 (D3), B9 (D4), B10b (D5), B11 (D2) from the initial wave plan. Keep their spec sentences in B1 (so `specs/diagnostics.md` ships complete) but defer implementation to a follow-on plan after D1+D6 land and are run against real corpora for a week.

---

## Other findings

### Beads.md vs Plan body — naming inconsistency

The beads.md says "12 beads across 5 waves" but the overview shows 6 waves (Wave 1 through Wave 6 / B12). The "5 waves" headline is wrong. Minor, but the plan's own arithmetic doesn't agree with itself.

### Plan body still describes obsolete architecture

`_plan.md` lines 84–119 sketch the `kerf diagnose` command, sample output, and `watch` mode. The 2026-05-19 reconciliation (lines 122–162) says these are superseded, but the obsolete content remains in-line. Implementer agents reading this plan during dispatch will get confused. Either delete the obsolete sections or move them to an "ORIGINAL DRAFT (superseded)" appendix.

### Transcript discovery rule still TBD

`source/detector_examples.md` uses `/Users/gb/.claude/projects/-Users-gb-github-harmonik/`. That path is user-, hostname-, and OS-specific. The discovery rule must be in B1's spec — and it must be tested cross-OS or explicitly scoped to "macOS Claude Code only, v1." This isn't called out as a risk.

### B7 first-ship suppression rule has a temporal bug

B7 (D6) ships in Wave 3. B11 (D2) ships in Wave 5. The plan says "D6 suppresses when D2 fires" — but during Waves 3–4 D2 doesn't exist, so D6 fires unsuppressed. The first-ship validation (B7 alone, before Wave 3) will show 29 D6 findings on harmonik. Is that the validation? Then B7's validation goal is "fires loudly on the known incident" — but that's not what the plan claims as the validation milestone.

---

## Summary of recommended changes

1. **Scope down to 2 detectors (D1, D6) for the initial plan.** Defer D2/D3/D4/D5 to a follow-on plan after D1+D6 run against real corpora.
2. **Confront the surface question.** `kerf doctor` is not the right channel for high-frequency findings (D1, D3, D5, D6). Add a section to `specs/diagnostics.md` mapping each detector to its surface, including `kerf next` warnings.
3. **Address parser duplication explicitly.** Either port to Go and retire Python, or accept duplication and commit to a cross-parser contract test.
4. **Calibrate thresholds against the existing corpus before B1 commits numbers.** A one-bead calibration pass before spec lockdown.
5. **Clean the plan body.** Delete or appendix the superseded `kerf diagnose` content. Fix the "5 vs 6 waves" discrepancy.
6. **Specify the transcript discovery rule and bead-ID regex source in B1.** These are load-bearing and currently TBD.

---

## Verdict

**proceed-with-changes.**

Plan 013 is the right idea (kerf should detect its own procedural drift) and the freshness-check correction (extend `kerf doctor` not new command) is right. But the plan is over-scoped for a v1, papers over the surface question (`kerf doctor` vs `kerf next`), and commits thresholds without calibration. The user's worry is well-founded for this plan as currently written.

The fix is small: cut to D1+D6, name the surface per detector, calibrate thresholds, clean the body. With those changes, this is a high-value plan.
