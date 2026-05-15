# Agent B findings — scoring, queue ordering, item feed

Workspace: `/tmp/kerf-explore-B/`. Binary: `/tmp/kerf` (built from current main).
Project: `kerf-explore-b`. Bead store: harmonik `.beads` (1298 issues), accessed via `br`.

Specs read: `specs/coordination.md` (priority/ordering, bead attachment),
`specs/commands.md` §`kerf next` (item kinds, flags, behavior, help-text contract).

Severity legend: **S1** = spec contract broken, agent contract at risk; **S2** =
behavior diverges from spec but tolerated; **S3** = polish/UX.

---

## F1 — `kerf next` is not the item feed described in the spec (S1)

**Did:** Ran `/tmp/kerf next` with multiple works, beads, rework labels.

**Expected (specs/commands.md §1408–1521):** A ranked feed mixing `bead`,
`cleanup`, and `warning` items. Each line carries `kind`, `target id`, `title`,
`work codename`. Cleanup items follow beads. Warning items render as a header
block. `--format=json` emits per-item records.

**Got:** A ranked list of **work items** (not beads/cleanup/warnings):

```
1. imrest              implementation  breakdown
   completion 0/5 (+0.0)
   rework 2x (+30.0)
2. claude-hook-bridge  implementation  breakdown
   ...
   Suggested: kerf resume imrest  (continue breakdown pass)
```

There is no item-kind column, no bead IDs, no cleanup rows, no warning header,
no `--format=json` flag at all. The output is closer to `kerf list` ranked by
the queue scorer than to the item feed described in commands.md.

**Severity:** S1 — `kerf next` is the agent's primary pull signal. The spec
calls this its "agent contract". The implementation does not satisfy it.

**Suggested fix:** Either (a) re-scope the spec so `kerf next` v1 is documented
as a ranked work feed (current behavior) with the bead/cleanup/warning surface
deferred, or (b) implement the item-feed (Plan 005 phases?) before the spec is
treated as binding. Right now the spec promises behavior the binary does not
have.

---

## F2 — `--only`, `--include`, `--kinds`, `--format` are absent (S1)

**Did:** `kerf next --only=bead`, `--include=warning`, `--kinds=bead,cleanup`,
`--format=json`.

**Expected:** Per spec §Flags: each accepted; precedence rules apply; JSON
output documented.

**Got:** `Error: unknown flag: --only` (likewise for the others).
`--limit` and `--area` are accepted instead; neither appears in the spec for
`kerf next`.

**Severity:** S1 — the spec's flag precedence corner cases (scenario 6) cannot
even be probed because the flags don't exist. The "JSON output contract"
(scenario 7) is also moot.

**Suggested fix:** Either implement the flags, or delete the flag section from
commands.md and add `--limit`/`--area` to the spec.

---

## F3 — `kerf next --help` violates the six-element help contract (S1)

**Did:** `/tmp/kerf next --help`.

**Expected (commands.md §1525–1536):** Six elements **in this fixed order**:
(1) what it returns, (2) the item kinds, (3) the default action loop,
(4) the filter flags with concrete examples, (5) machine output via
`--format=json`, (6) scoring in one sentence with a pointer to
coordination.md.

**Got:**

```
Computed ordering of actionable work items. Filters out blocked and shelved
works. Orders by dependency depth and completion momentum.
```

Then examples that mention only `--area`, `--limit`, and `--project`. None of
the six required elements are present. The spec explicitly says "Changes to
this help text require a spec change."

**Severity:** S1. A fresh agent running `--help` does not learn the loop.

**Suggested fix:** Rewrite the help text to follow the six bullets verbatim, or
revise the spec to describe the actual surface.

---

## F4 — Project-wide `bead_filter` is ignored by `kerf next` (S1)

**Did:** Set `bead_filter: { label: "codename:{codename}" }` in
`~/.kerf/projects/kerf-explore-b/project.yaml`. Created works whose codenames
matched existing `codename:claude-hook-bridge` (37 beads) and `codename:imrest`
(6 beads) labels. Re-ran `kerf next`.

**Expected (coordination.md §234–244 + commands.md §1451):** Beads are filtered
per work via the **resolved `bead_filter`**, with fallback to the built-in
`label: "work:{codename}"`.

**Got:** The bead counts in the queue reasons stayed at zero. After creating
fresh test beads with `work:claude-hook-bridge` labels, counts appeared
immediately. Tracing the code: `cmd/next.go:89` calls
`beads.ForWork(allBeads, w.Codename)`, which hard-codes
`label == "work:<codename>"`. `ForWorkWithFilter` exists in
`internal/beads/beads.go` but is never invoked from `next`.

**Severity:** S1 — the spec's first/second resolution rules
(per-work, then project-wide) are non-operational for `kerf next`. Any project
using a real-world labeling scheme (subsystem prefix, id-prefix, etc.) gets
zero bead scoring contribution.

**Suggested fix:** Wire `cmd/next.go` through the filter resolver and use
`ForWorkWithFilter` with `Resolve(workFilter, projectFilter)`. Same fix likely
needed in `cmd/map.go` and `cmd/show.go`.

---

## F5 — `ForWork` matching is case-insensitive; spec says case-sensitive (S2)

**Did:** Created a bead with label `WORK:imrest` (uppercase prefix). Re-ran
`kerf next`.

**Expected (coordination.md §232 "Matching is case sensitive"):** The
uppercase-prefix bead should not match `work:imrest`. A project-wide filter
with a different prefix case should surface the "case-mismatch" warning.

**Got:** Bead was matched and counted. `internal/beads/beads.go:167` uses
`strings.EqualFold(label, target)`. The code itself notes this divergence as a
TODO ("confirm the case-sensitivity divergence … is acceptable until callers
are migrated in later beads").

**Severity:** S2 — explicit known gap, but the case-mismatch warning
(commands.md §1459) cannot fire while matching is case-insensitive. The two
specs are now coupled: fixing one breaks the other.

**Suggested fix:** Migrate callers to `ForWorkWithFilter`, then make
`ForWork`'s case-sensitivity match the spec.

---

## F6 — `work_no_attached_beads` cleanup never fires (S1, follows from F1)

**Did:** Created `ghost-work` with codename that matches **no** beads in the
store.

**Expected:** A `cleanup` item of type `work_no_attached_beads` for ghost-work
in the feed.

**Got:** ghost-work appears as a plain entry with `creation order (+0.1)` and
no indication that it has no beads attached. No cleanup detector runs.

**Severity:** S1 (subsumed by F1).

---

## F7 — Unmatched-bead warning never fires (S1, follows from F1)

**Did:** Out of 1298 beads in the store, only ~5 have `work:*` labels. The
other ~1293 are unmatched (mostly `codename:*`, `subsystem:*`).

**Expected:** A warning header block like
`warning: 1293 beads match no work — check bead_filter in project.yaml`.

**Got:** No warning surfaces in `kerf next`. The unmatched-bead detector is not
running. The spec does not mention a numeric threshold (scenario 5's "9 vs 10"
hypothesis was incorrect: the spec says "any beads" → one warning). So today
the behavior is "always silent" regardless of count.

**Severity:** S1 (subsumed by F1).

---

## F8 — Score tiebreak: `creation` order, oldest wins. Matches spec. (PASS)

**Did:** Created two works back-to-back with no beads and no deps.

**Expected (coordination.md §169):** `creation` multiplier favors older works.

**Got:** Older work scored `(+0.1)`, newer `(+0.0)`; older listed first. Works
correctly. Stable across repeated invocations.

---

## F9 — Rework dominance is correct under default weights (PASS)

**Did:** `imrest`: 5 attached beads, 2 with rework labels (`rework:true` and
`finding:work-a`). `claude-hook-bridge`: 5 beads, no rework. No deps either
side.

**Expected:** Rework should outrank new work. Default `rework=15.0` × 2 = 30,
beats momentum/creation deltas.

**Got:** `imrest` first with `rework 2x (+30.0)`. Both `rework:true` and
`finding:<origin>` tags counted. Matches spec.

---

## F10 — Fan-out from `must-complete-first` deps works (PASS)

**Did:** Made `work-tied` and `work-tied2` declare
`depends_on: ghost-work / must-complete-first`. `ghost-work` then shows
`unblocks 2 work(s) (+20.0)`. After advancing `ghost-work` to `complete`,
`work-tied` and `work-tied2` enter the queue and `ghost-work` exits (terminal).

**Severity:** PASS. Note: rework still beat fan-out (30 vs 20.1) in this setup,
which is reasonable but worth noting as a design choice.

---

## F11 — Malformed `spec.yaml` silently drops a work from the queue (S2)

**Did:** Wrote depends_on via `python3 yaml.safe_dump`, which serialised the
`created` timestamp as `2026-05-15 16:14:30+00:00` (space separator, no `Z`).

**Expected:** Either kerf parses tolerant ISO timestamps, or it errors loudly
on parse failure.

**Got:** The work silently disappears from `kerf next` and `kerf show <name>`
returns `work 'work-tied' not found`. `kerf list` also omits it. No warning is
emitted anywhere. The only way to discover the problem is to compare directory
contents to `kerf list`.

**Severity:** S2 — silent data loss in views. An agent has no signal that
something is wrong.

**Suggested fix:** Emit a warning (stderr) when a `spec.yaml` exists but fails
to parse, naming the file. Optional: accept space-separated RFC3339 too.

---

## F12 — `--limit` accepts negative and zero values silently (S3)

**Did:** `kerf next --limit -1`, `--limit 0`, `--limit 9999`.

**Got:** All three behave like "no limit" (show every entry). No error, no
warning.

**Severity:** S3 — cosmetic. Probably fine, but negative numbers should
arguably error.

---

## F13 — `kerf next --area=''` (empty string) silently treated as "no filter" (S3)

**Did:** `kerf next --area=''`.

**Got:** Same output as `kerf next`. No error.

**Severity:** S3.

---

## F14 — `kerf next --area=foo` where no work matches: empty-feed message (S3)

**Did:** Filtered to an area no work touches.

**Got:** `No actionable works for project 'kerf-explore-b'.` — same as the
"everything archived" case. The message doesn't distinguish "filter excluded
all works" from "no works exist."

**Severity:** S3 — small but bad for an agent debugging why the queue is
empty.

**Suggested fix:** When `--area` is set, say
`No actionable works in area 'foo' for project 'X'.` and (optionally) list
known areas.

---

## F15 — Bead matching multiple works counts for each (PASS)

**Did:** Created one bead with both `work:imrest` and `work:claude-hook-bridge`
labels.

**Expected (coordination.md §247–248):** Many-to-many; bead counts for each.

**Got:** Both works' `completion x/N` incremented by 1. Confirmed.

---

## F16 — `kerf delete --force` rejected (S3, surface mismatch)

**Did:** `/tmp/kerf delete <work> --force`.

**Got:** `Error: unknown flag: --force`. The real flag is `--yes`.

**Severity:** S3 — `--force` is a near-universal alias. Either accept it as an
alias or document loudly.

---

## F17 — `kerf next` with all works archived: exit 0, plain text (PASS)

**Did:** Archived every active work.

**Got:** `No actionable works for project 'kerf-explore-b'.`, exit 0. Matches
spec ("If no items exist, the output says so").

---

## Summary

| Severity | Count |
|----------|------:|
| **S1**   | 5     |
| **S2**   | 3     |
| **S3**   | 4     |
| **PASS** | 5     |

The biggest finding by a wide margin is **F1/F2/F3 together**: `kerf next` as
specified does not exist in the implementation. The binary ships a ranked
**work** feed; the spec describes a ranked **item** feed of beads, cleanup,
and warnings, with `--only/--include/--kinds/--format` flags and a six-bullet
help-text contract. Until that gap is reconciled (either build it or rewrite
the spec to v1-current), most of the prompt's scoring scenarios (cleanup
ordering, threshold warnings, JSON contract, flag precedence) cannot be tested
against the binary.
