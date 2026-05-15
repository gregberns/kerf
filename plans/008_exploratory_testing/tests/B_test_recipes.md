# Agent B test recipes — scoring, queue ordering, item feed

All recipes assume:

- `$KERF` points at the kerf binary.
- A scratch project directory exists with `git init` done.
- The bench is `~/.kerf`. Set up `~/.kerf/projects/<id>/project.yaml` per
  `kerf init`.
- `br` (beads CLI) is on PATH and a `.beads/` store is in cwd. Recipes use
  short titles and `--silent` for compactness.

## R1 — Rework dominance under default weights

```bash
$KERF new alpha --jig implementation
$KERF new beta  --jig implementation
br create "a1" -l "work:alpha" --silent
br create "a2" -l "work:alpha" --silent
br create "a3" -l "work:alpha" --silent
br create "b1" -l "work:beta"  --silent
br create "b-rework" -l "work:beta,rework:true" --silent
br create "b-finding" -l "work:beta,finding:upstream" --silent
$KERF next | head -10
```

**Expect:** `beta` ranks above `alpha` (rework 2×15 = 30 beats creation delta
≈ 0.1). Both `rework:true` and `finding:<x>` count as rework.

## R2 — Fan-out unblocks bonus

```bash
$KERF new core   --jig implementation
$KERF new leaf-a --jig implementation
$KERF new leaf-b --jig implementation
# Manually edit leaf-a and leaf-b spec.yaml to add:
# depends_on:
#   - codename: core
#     relationship: must-complete-first
$KERF next | head -20
```

**Expect:** `core` shows `unblocks 2 work(s) (+20.0)`. `leaf-a` and `leaf-b`
do NOT appear (must-complete-first dep unmet).

After `$KERF status core complete`: `core` drops out, `leaf-a` and `leaf-b`
appear.

## R3 — Creation-order tiebreaker on identical scores

```bash
$KERF new first  --jig implementation
sleep 1
$KERF new second --jig implementation
$KERF next
```

**Expect:** `first` ranks above `second`; reason `creation order (+0.1)` on
`first`, no creation-order reason on `second` (because score is 0).

## R4 — Bead matching multiple works

```bash
$KERF new w1 --jig implementation
$KERF new w2 --jig implementation
br create "shared" -l "work:w1,work:w2" --silent
$KERF next | grep completion
```

**Expect:** Both `w1` and `w2` show `completion 0/1`.

## R5 — Archived/terminal exclusion

```bash
$KERF new a --jig implementation
$KERF new b --jig implementation
$KERF archive a
$KERF status b complete   # advance to terminal
$KERF next
```

**Expect:** `No actionable works for project '<id>'.`, exit 0.

## R6 — Project-wide `bead_filter` should drive scoring

```bash
# Edit ~/.kerf/projects/<id>/project.yaml to add:
# bead_filter:
#   label: "codename:{codename}"
$KERF new claude-hook-bridge --jig implementation
# Real beads in store already labeled codename:claude-hook-bridge
$KERF next | grep claude
```

**Expect (per spec):** `completion 0/N` where N is count of matching beads.
**Today's actual:** Reason omitted; filter ignored. See finding F4.

## R7 — Case-sensitive match (spec) vs case-insensitive (today)

```bash
$KERF new mywork --jig implementation
br create "x" -l "WORK:mywork" --silent
$KERF next | grep mywork
```

**Expect (per spec):** No completion contribution; possibly a case-mismatch
warning.
**Today's actual:** `completion 0/1`; uppercase prefix matches. See F5.

## R8 — Help text contract

```bash
$KERF next --help
```

**Expect:** Six elements in order: returns / kinds / loop / flags / json /
scoring (with link). See spec commands.md §1527.
**Today:** Two-line summary; only `--area`/`--limit`/`--project`. Fails.

## R9 — Unknown kind / format errors

```bash
$KERF next --kinds bead,zzz
$KERF next --format yaml
```

**Expect (per spec error table):**
`Error: unknown item kind 'zzz'. Known kinds: bead, cleanup, warning.`
`Error: unknown format 'yaml'. Supported: text, json.`
**Today:** `Error: unknown flag: --kinds` / `--format`.

## R10 — JSON output round-trip

```bash
$KERF next --format=json | jq .
```

**Expect:** Stream of item records with snake_case fields:
`kind`, `score`, `title`, `action`, `reason`, `work_codename`, `bead_id`.
`work_codename` is literal `null` when not applicable.
**Today:** Flag missing.

## R11 — `--only=warning` empty-but-not-empty

```bash
$KERF next --only=warning
```

**Expect:** Warning header block only (no ranked items). If no warnings,
exit cleanly with the "no items" message.

## R12 — `--only` + `--include` precedence

```bash
$KERF next --only=bead --include=warning
$KERF next --only=bead --only=cleanup
$KERF next --kinds=bead,cleanup --only=bead --include=warning
```

**Expect:**
- First → beads, plus warning header.
- Second → beads and cleanup (multiple `--only` union).
- Third → kinds sets base, only intersects, include unions warning back in.

## R13 — `work_no_attached_beads` cleanup detector

```bash
$KERF new lonely --jig implementation
# Make sure no bead matches "work:lonely".
$KERF next --kinds=cleanup
```

**Expect:** A `cleanup` item with kind `work_no_attached_beads`, target
`lonely`, action pointing at editing the filter or spec.

## R14 — `work_beads_done_status_open` cleanup detector

```bash
$KERF new done-but-open --jig implementation
br create "b1" -l "work:done-but-open" --status=closed --silent
$KERF next --kinds=cleanup
```

**Expect:** A `cleanup` item — all beads closed, status non-terminal, suggested
action `kerf status done-but-open <next>` or shelve.

## R15 — Cleanup sort order

```bash
# Two works, both attached but all-done with stale status.
# work-hi has higher would-be bead score (more rework / fan-out).
# work-lo has none.
$KERF next --kinds=cleanup
```

**Expect:** `work-hi` cleanup row above `work-lo` cleanup row. Tie? sort by
`created` ascending.

## R16 — Unmatched-bead warning

```bash
# Bead store contains beads matching no work's filter.
$KERF next
```

**Expect:** Header line:
`warning: <N> beads match no work — check bead_filter in project.yaml`.

## R17 — Filter literal yields zero matches (case-mismatch hint)

```bash
# project.yaml: bead_filter: { label: "Codename:{codename}" }
# All beads carry "codename:" (lowercase).
$KERF next
```

**Expect:** A warning suggesting the user check for a case mismatch
(`Subsystem:` vs `subsystem:`).

## R18 — Malformed spec.yaml is reported, not silently dropped

```bash
echo "not: valid: yaml" > ~/.kerf/projects/<id>/somework/spec.yaml
$KERF next
$KERF list
```

**Expect:** A clear warning on stderr that `somework/spec.yaml` failed to
parse. The work is excluded but the user is told why.
**Today:** Silent drop; user must compare dir listings. See F11.

## R19 — Idempotence

```bash
$KERF next > /tmp/first
$KERF next > /tmp/second
diff /tmp/first /tmp/second
```

**Expect:** Empty diff. The spec promises idempotence ("running it ten times
with no state changes produces the same result"). PASSES today.

## R20 — `--limit` edge cases

```bash
$KERF next --limit 0
$KERF next --limit -1
$KERF next --limit 9999
```

**Expect (current):** All three behave as "no limit". Negative arguably
should error. Today: silent.
