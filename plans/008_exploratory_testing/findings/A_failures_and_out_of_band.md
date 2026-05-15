# Exploratory Testing A — Failure Modes & Out-of-Band Bead Changes

Date: 2026-05-15
Workspace: `/tmp/kerf-explore-A` (git repo, project id `kerf-explore-a`)
Binary: `/tmp/kerf` built from local repo.
Bead store: copied from harmonik; subsequently reinitialized with `bd init --prefix kex` (dolt backend).

Headline: **kerf has almost no bead integration at runtime.** The spec describes a rich item-feed (`bead`/`cleanup`/`warning` kinds, filter resolution, unmatched/case-mismatch warnings, `--only`/`--kinds`), and an `internal/feed` package exists, but `cmd/next.go` does not import it. As a result every out-of-band scenario is silently invisible.

---

## F1 — `kerf next` ignores beads entirely (P0)

**What I did**
```
cd /tmp/kerf-explore-A
bd create -l "work:sample-work" "Foo task" ; bd create -l "work:sample-work" "Bar task" ; bd create -l "work:sample-work" "Baz task"
/tmp/kerf next
/tmp/kerf next --only bead
/tmp/kerf next --kinds bead,cleanup,warning
```

**What I expected** (per `specs/commands.md` `kerf next` section, lines 1412-1456)
A unified item feed with `bead`, `cleanup`, and `warning` items, supporting `--only` and `--kinds` flags.

**What happened**
```
Next actions for kerf-explore-a:
  1. sample-work  implementation  breakdown
...
Error: unknown flag: --only
Error: unknown flag: --kinds
```
Beads are never surfaced. `cmd/next.go` has only `--limit`, `--area`. `internal/feed/` exists (`feed.go`, `filter.go`, `item.go`, `feed_test.go`) but is never imported from `cmd/`.

**Severity**: P0 — the primary agent pull signal does not do what the spec says.
**Fix**: wire `internal/feed` into `cmd/next.go`; add `--only` and `--kinds` flags.

---

## F2 — `kerf show` bead summary uses unknown `bd` flag (P0)

**What I did** Created beads, ran `kerf show sample-work`. Expected `Beads: N total, M closed…` per `specs/commands.md` line 247.

**What happened** No bead section in the output.

**Root cause** `cmd/show.go:274-278` runs:
```go
args := []string{"list", "--json"}
if projectID != "" { args = append(args, "--project", projectID) }
out, err := exec.Command("bd", args...).Output()
```
`bd list` does not accept `--project` (confirmed: `Error: unknown flag: --project`). The exec fails, `err != nil`, function returns `""`, silently no output. The bead summary is unreachable whenever a project id is present (i.e. always in normal use).

**Severity**: P0 — guaranteed silent failure of a spec-described feature.
**Fix**: drop the `--project` arg; filter beads by the project's resolved bead_filter in Go instead.

---

## F3 — `kerf init` auto-detect never fires (P1)

**What I did** Created 6 beads labeled `work:sample-work` and 39 labeled `subsystem:newprefix*`, then deleted `~/.kerf/projects/kerf-explore-a/project.yaml` and ran `/tmp/kerf init`.

**What I expected** Per `specs/commands.md` lines 1173-1186: list label prefixes with ≥3 beads, compute a match_score, propose / write a `bead_filter`. The init report (line 1196) should include a "Bead-filter detection summary".

**What happened** No mention of bead detection in init output; `project.yaml` written with no `bead_filter`. The summary block in init output (item 4 in spec) is absent.

**Severity**: P1 — feature commit exists (`f998c0c feat(cmd): kerf init bead_filter auto-detect`) but is non-functional in this environment. Could be because `internal/beads.List()` calls `br list ...` and `br` is incompatible with the dolt-backed `bd` store (see F10), so detection silently no-ops.

**Fix**: surface "bead tool unreachable" in init output rather than silently skipping; or call `bd list --json` (which works).

---

## F4 — `kerf init` silently overwrites `project.yaml` (P1)

**What I did**
```
# add bead_filter manually
echo 'bead_filter: { label: "work:{codename}" }' >> ~/.kerf/projects/kerf-explore-a/project.yaml
/tmp/kerf init
cat ~/.kerf/projects/kerf-explore-a/project.yaml   # bead_filter is gone
```

**What I expected** Either preserve the existing config or refuse with a clear message.

**What happened** Output said "Project already initialized: kerf-explore-a" then immediately "Created project.yaml with 6 active jigs" — the file was rewritten and the user-added `bead_filter` lost. Note the contradictory message.

**Severity**: P1 — silent destructive overwrite of user config.
**Fix**: if `project.yaml` exists, do not rewrite; if a refresh is desired, merge.

---

## F5 — Out-of-band bead close not noticed (P1)

**What I did** `bd close kex-edf` (one of the work:sample-work beads). Then `kerf next`, `kerf list`, `kerf map`, `kerf show sample-work`.

**What I expected** Bead progress reflecting one closed, or at minimum a non-zero closed count somewhere.

**What happened** Identical output before and after. Nothing reflects bead state in any view.

**Severity**: P1.
**Fix**: same as F1/F2 — wire bead reads.

---

## F6 — Beads orphaned after work deletion are unmatched, never surfaced (P1)

**What I did** Created `sample-work` + 6 beads `work:sample-work`. `kerf delete sample-work --yes`. Beads remain in store. Ran `kerf next` / `kerf list`.

**What I expected** A `warning` item: 6 unmatched beads referring to a deleted/missing work. Per `specs/coordination.md` line 252 unmatched beads must surface as a project-level warning.

**What happened** "No actionable works for project". No warning anywhere. Beads are now invisible to kerf.

**Severity**: P1 — beads become silent garbage.

---

## F7 — Mixed-case / typo'd labels never flagged (P1)

**What I did** Created beads with labels `Work:Sample-Work` (case mismatch) and `subsytem:foo` (typo of `subsystem:`).

**What I expected** Per `specs/coordination.md` line 232, kerf must surface a `warning` for likely case-mismatched filters; typos in labels with no work match should appear as unmatched.

**What happened** Silent. No warning.

**Severity**: P1.

---

## F8 — Broken dependency chain after `bd delete` not flagged (P2)

**What I did** Created two beads with a dep edge `kex-gvo --blocks kex-jgc`, then `bd delete kex-gvo --force`. `bd` itself reported `Removed 1 dependency link(s)`. kerf views unchanged.

**What I expected** A warning that a dangling dependency reference exists or that the surviving bead lost its blocker (depending on what bd does). bd silently severs the edge, so kerf may need no action — but a project-health view that says "bead kex-jgc had its only blocker deleted yesterday" would help.

**Severity**: P2.

---

## F9 — 39 beads under unrecognized prefix never flagged (P1)

**What I did** `for i in $(seq 1 30); do bd create -l "subsystem:newprefix$i" "Auto $i"; done` plus earlier ad-hoc creates → 39 unmatched beads.

**What I expected** A warning item like "39 beads with prefix `subsystem:` do not match any work's filter — consider setting `bead_filter`".

**What happened** Nothing.

**Severity**: P1 — the cheapest possible signal that the project drifted into a new labeling convention is invisible.

---

## F10 — `br` is incompatible with bd's dolt backend (P1, infra)

**What I did** `br list --format json --all --limit 0` (the exact command kerf's `internal/beads.List()` runs).

**What happened**
```
{"error":{"code":"JSON_ERROR","message":"JSON error: missing field `jsonl_export` at line 7 column 1"...
```
`br` (v0.1.45, sqlite+JSONL) cannot read the metadata.json that bd (v0.62.0, dolt) writes. So kerf's bead pipeline cannot fetch beads in this configuration.

**Severity**: P1 — undocumented infra requirement. Kerf appears to assume `br` and the sqlite/JSONL backend; users on `bd` with dolt see silent zero-bead results everywhere.
**Fix**: detect bd/br compatibility at init and warn; or shell to `bd list --json` directly.

---

## F11 — Custom (non-jig) status removes a work from `kerf next` (P1)

**What I did**
```
/tmp/kerf status other-work weird-status-not-in-jig   # kerf warned but accepted
/tmp/kerf next
```

**What I expected** Per invariant 5 (`specs/_index.md`: "Status is an open string. Jigs recommend values; the CLI warns on unrecognized values but does not enforce."), the work should still be actionable.

**What happened** `No actionable works for project 'kerf-explore-a'`. The work is filtered out silently. Once an agent sets a custom status, the work effectively disappears from the next-action signal.

**Severity**: P1 — contradicts a stated invariant.
**Fix**: include works with unknown statuses in `kerf next`, possibly with a `warning` item.

---

## F12 — Corrupt `spec.yaml` makes a work disappear silently (P0)

**What I did**
```
echo "not-valid-yaml: [unclosed" > ~/.kerf/projects/kerf-explore-a/other-work/spec.yaml
/tmp/kerf list      # work missing
/tmp/kerf next      # work missing
/tmp/kerf show other-work   # "Error: work 'other-work' not found"
/tmp/kerf map       # work missing
```

**What I expected** A clear "spec.yaml is malformed" error, not silent erasure.

**What happened** Work is gone from all views; `show` reports "not found" — which is misleading. Snapshots (in `.history/`) still exist on disk.

**Severity**: P0 — silent data loss surface. An agent could conclude work was deleted and create a new one, losing context.
**Fix**: when listing works, on yaml parse error emit a project-level warning item or a list row tagged `[corrupt]`.

---

## F13 — Deleted spec.yaml + orphan dir blocks `kerf new` with same codename, list says it doesn't exist (P1)

**What I did**
```
rm ~/.kerf/projects/kerf-explore-a/other-work/spec.yaml
/tmp/kerf list                 # work absent
/tmp/kerf new other-work --jig spec   # "Error: work 'other-work' already exists"
```

**What I expected** Either (a) treat orphan dir as nonexistent and recreate, or (b) surface the orphan in `list`.

**What happened** Inconsistent: list says it doesn't exist, new says it does.

**Severity**: P1.

---

## F14 — `--area` accepts undefined areas with no warning (P2)

**What I did** `kerf next --area no-such-area` (only `api` is defined).

**What happened** "No actionable works." No warning that the area is undefined.

**Severity**: P2 — minor UX/typo trap.

---

## F15 — `kerf delete` has no `--force`/`--yes` matching its own prompt-skipping conventions (P2)

**What I did** `kerf delete sample-work --force` → "unknown flag: --force". Correct flag is `--yes`.

**Severity**: P2 — naming inconsistency with `bd`/`br` and with common convention. Not breaking, but agent will guess wrong.

---

## Summary of severities

- P0 (silent data loss / spec-stated feature absent): F1, F2, F12 — 3
- P1 (misleading or missing signal): F3, F4, F5, F6, F7, F9, F10, F11, F13 — 9
- P2 (nit): F8, F14, F15 — 3

The dominant pattern: **kerf accepts out-of-band state without surfacing it.** Any failure mode that should produce a `warning` or `cleanup` item in `kerf next` is silent today, because `kerf next` was never wired to the feed package even though feed/filter/item code (and tests) ship in the binary.
