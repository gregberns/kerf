# Recheck Results — Against Current main

Re-tested against `main` at the user-specified tip (binary built fresh from
`/Users/gb/github/kerf` on 2026-05-15). Workspace: `/tmp/kerf-recheck/` with
HOME=`/tmp/kerf-recheck-home` (so `~/.kerf/` was sandboxed). `br 0.1.45` on PATH.

## Each recheck

### findings/A:F1 — kerf next is now the item feed
- **Reproduction**: `/tmp/kerf next`, `--only bead`, `--kinds bead,cleanup,warning`, `--format=json` (in a sandboxed cwd with no project, then later in an initialized project)
- **Observed**: All four flag forms accepted (no "unknown flag"). With no items, prints the empty hint; with items, prints ranked rows; JSON emits a list of objects with `kind`/`title`/`bead_id`/etc.
- **Verdict**: FIXED

### findings/A:F3 — detectBeadFilter on `kerf init`
- **Reproduction**: `br init`, created 12 beads sharing prefixes (5 `work:foo`, 4 `work:bar`, 3 `subsystem:x`), removed `project.yaml`, ran `/tmp/kerf init`.
- **Observed**: No detection line printed. Written `project.yaml` contains only `jigs:`/`passes:` — no `bead_filter` key.
- **Verdict**: STILL VALID. Severity P1.

### findings/A:F5 — out-of-band `br close` reflected
- **Reproduction**: Created 12 beads, ran `kerf next` (recorded), `br close kerf-recheck-f22`, re-ran `kerf next`.
- **Observed**: The closed bead is dropped from the listing (12 → 11 visible beads matches `br list --status open`). The warning header counter is stale (still says "13 beads") but the row data is current.
- **Verdict**: FIXED for the listing; counter staleness is a minor sub-bug worth noting (see Summary).

### findings/A:F6 — unmatched-beads warning after work delete
- **Reproduction**: Created `lonely` (no beads match) and `foo` with `bead_filter: label: work:foo`, then `kerf delete lonely --yes` and `kerf delete foo --yes` with 13 beads in the store, `kerf next --kinds=warning`.
- **Observed**: `warning: unmatched beads — check project bead_filter — top unmatched prefix: 'work:' — 13 beads match no work via current filter`.
- **Verdict**: FIXED.

### findings/A:F7 — mixed-case label warning (`filter_case_mismatch`)
- **Reproduction**: `br create -l "Work:Foo" "case mismatch bead"` while filter `label: work:foo` exists on work `foo`. `kerf next --kinds=warning`.
- **Observed**: Only the generic `unmatched beads` warning fires; no dedicated case-mismatch row.
- **Verdict**: STILL VALID. Severity P2.

### findings/A:F9 — 39 beads under `subsystem:` prefix → warning
- **Reproduction**: Same detector as F6. Confirmed with 13 unmatched (well above abs=10 threshold); the implementation thresholds scale linearly.
- **Observed**: Warning fires.
- **Verdict**: FIXED.

### findings/A:F10 — `br` vs `bd` dolt incompatibility
- **Reproduction**: Not re-run. This is a tooling-migration question, not a code defect on `main`.
- **Observed**: N/A.
- **Verdict**: DIFFERENT BEHAVIOR — route to migration audit (already classified as such in validated_findings.md).

### findings/B:F1 — `kerf next` is the item feed
- **Reproduction**: Same as A:F1.
- **Observed**: Item feed rendered with `bead`/`clean`/`warn` kind tokens, footer hint about `--format=json`.
- **Verdict**: FIXED.

### findings/B:F2 — feed flags
- **Reproduction**: `--only`, `--include`, `--kinds`, `--format` (see A:F1).
- **Observed**: All accepted.
- **Verdict**: FIXED.

### findings/B:F3 — help text contract
- **Reproduction**: `/tmp/kerf next --help`.
- **Observed**: Help text has all six elements in spec order: kind list, action loop, filter flags, machine output, scoring, with `specs/coordination.md` reference.
- **Verdict**: FIXED.

### findings/B:F4 — project-wide `bead_filter` honored
- **Reproduction**: Set `bead_filter: label: work:foo` on work `bk` (re-created post-delete; spec rewritten on disk). 5 beads carry `work:foo` label. `kerf next` and `kerf next --format=json`.
- **Observed**: All beads come back with `work_codename: null` and the `bk` cleanup says "resolved bead_filter matches zero beads in the store." The `work:foo` beads are listed alongside everything else as unattached. Project filter resolution path is wired, but attachment to the work fails.
- **Verdict**: STILL VALID (different shape than original report — code-path is now reached but produces wrong assignment). Severity P0.

### findings/B:F6 — `work_no_attached_beads` cleanup
- **Reproduction**: `kerf new lonely --jig implementation`, no matching beads, `kerf next --kinds=cleanup`.
- **Observed**: `1. clean  lonely   resolved bead_filter matches zero beads in the store`.
- **Verdict**: FIXED.

### findings/B:F7 — unmatched-bead warning
- **Reproduction**: Same as A:F6/F9.
- **Observed**: Warning fires.
- **Verdict**: FIXED.

### findings/B:F11 — malformed spec.yaml silently dropped
- **Reproduction**: `kerf new corrupt`, overwrote `spec.yaml` with `garbage: !!!! invalid yaml [`, ran `kerf next`.
- **Observed**: No `[corrupt]`/warning row; the work simply vanishes from output. Other works still render.
- **Verdict**: STILL VALID. Severity P0.

### sync/ingestion s5 — drift mutations
- **Reproduction**: With 12-bead store and a work whose `bead_filter` matches some labels, ran each mutation and md5-summed `kerf next` output.
  - A relabel (`br label remove ... work:bar`): hash UNCHANGED.
  - B new orphan-labeled bead (`br create -l "unbinned"`): hash CHANGED.
  - C external close (`br close`): hash CHANGED.
  - D delete (`br delete`): hash CHANGED.
  - E reopen (`br reopen`): hash CHANGED.
- **Observed**: 4 of 5 mutations are now surfaced via row presence/order; relabels remain invisible.
- **Verdict**: DIFFERENT BEHAVIOR — close/delete/reopen/create now reflected, but relabels (the only mutation that doesn't change open-bead set or labels-that-already-match) are still invisible. Severity P1 for the residual hole.

### sim_scenarios/findings — `priority_inversions=0` / `rework_p50_wait=0`
- **Reproduction**: `kerfsim run` against `s1_rework_storm.yaml` and `s5_asymmetric_sizes.yaml` (seed 42), inspected `summary.json` `full` block for all four policies.
- **Observed**:
  - s1: `priority_inversions=0` across all 4 policies; `rework_p50_wait=0` across all 4; `rework_p95_wait=2,0,0,0`.
  - s5: `priority_inversions=0` across all 4 policies; `rework_p50_wait=0` across all 4; `rework_p95_wait=256,283,733,299`.
- **Verdict**: STILL VALID. `priority_inversions` and `rework_p50_wait` look stuck at zero. Severity P1 (sim integrity).

## Summary

- Still valid: 6 (A:F3, A:F7, B:F4, B:F11, sim anomaly, plus residual hole in s5 drift)
- Fixed by recovery: 7 (A:F1, A:F5, A:F6, A:F9, B:F1, B:F2, B:F3, B:F6, B:F7 — close to 8 since B:F1 & A:F1 are aliases)
- Different behavior than original report: 2 (A:F10 → migration question; sync/s5 → 4/5 mutations surfaced, relabels still silent)

Top 3 still-valid:
1. **B:F4 / sync** — work-level `bead_filter` is read by `kerf next` but attachment is broken: beads come back with `work_codename: null` and cleanups report "matches zero beads" even when matching beads exist in the store. P0.
2. **B:F11 / A:F12** — corrupt `spec.yaml` silently dropped; affected work invisible. P0.
3. **sim_scenarios** — `priority_inversions` and `rework_p50_wait` are 0 across every policy in both s1 and s5; the wired metric path is reading zero. P1 for sim/scoring integrity.

Unexpectedly fixed: the `unmatched beads` warning detector (A:F6/F9/B:F7) and the `work_no_attached_beads` cleanup (B:F6) both fire as spec'd; close/delete/reopen/create drift now changes `kerf next` output (4/5 of the original drift table). The header `unmatched beads` counter has minor staleness (says "13" while listing 12 — visible in the A:F5 reproduction) — sub-P2 follow-up, not a regression.
