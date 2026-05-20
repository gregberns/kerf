# Plan 019 exploratory test — filters + `kerf next`

Date: 2026-05-18. Binary: `/Users/gb/go/bin/kerf`. Bead tool: `br 0.1.45` at `/Users/gb/.local/bin/br` (kerf hard-defaults to `br`, not `bd`). Sandboxes: `/tmp/kerf-019-Zr0f` (primary), `/tmp/kerf-019b-j9r5` (empty-store probe).

## Scenario pass count

11 of 14 scenarios pass cleanly. 3 issues, 1 minor inconsistency.

| # | Scenario | Result |
|---|---|---|
| 1 | `kerf new` emits `bead_filter:` slot (empty value) | pass |
| 2 | `kerf show` renders `bead_filter: (none)` empty / literal populated | pass |
| 3 | `kerf work show` field-by-field flat dump, includes `bead_filter` slot | pass |
| 4 | `kerf show --compact` 4-line shape, `bead_filter` slot preserved | pass |
| 5 | `kerf show` "Pass N: name → Output: file" rendering | pass |
| 6 | Rank labels `unwired` (no filter) and `empty` (zero matches) on `kerf next` | pass |
| 7 | `kerf next` payload-first: beads → cleanups → drift footer/warnings | pass |
| 8 | Near-match advisor for prefix-swap empties | **fail** |
| 9 | No spurious near-match suggestion when none available | n/a (advisor doesn't fire at all) |
| 10 | `bootstrap-filters` dry-run / `--yes` apply / idempotent re-run | pass |
| 11 | Edit confirmation `Now matches: N (M open / K closed). Previously: ...` | pass |
| 12 | Remove last clause leaves `bead_filter:` key with null value | pass |
| 13 | First edit on fresh-`new` work (null filter → populated) | pass |
| 14 | `--created-by self|all`, `(you)` / `(by anon)` markers | **partial fail** |

Pokes: bootstrap on empty bead store reports cleanly ("no proposal — no label resembles 'foo' in the bead store"); `--created-by bogus` returns a clean usage error; `--bead-filter-add 'any:label=…,label=…'` is **rejected** by `work edit` parser even though bootstrap can emit `any:` unions per its docstring.

## Issues

### I1 — Near-match advisor does not fire (kerf-d9f, plan §"Near-match advisor")

Setup that should trigger advisor: work `gamma` with `bead_filter: label=codename:gamma`; bead store contains 2 open beads labeled `codename:gama` (one-character-swap, "heavily-populated" candidate). Also work `gama` with `bead_filter: label=gama` against the same store (prefix-swap of `codename:gama`).

`kerf next` output for both:

```
5. empty   gamma   resolved bead_filter matches zero beads in the store
          edit spec.yaml bead_filter or check the project filter
6. empty   gama   resolved bead_filter matches zero beads in the store
          edit spec.yaml bead_filter or check the project filter
```

Generic advice only. Spec requires inline `— try: kerf work edit <codename> --bead-filter '<proposed>'` when "exactly one alternate clause would lift the work out of `empty`." Neither work surfaced a `try:` line.

Severity: medium. The whole motivation for the advisor was the harmonik prefix-swap miss; the data path is exactly the case the spec describes.

### I2 — `--created-by self` filters nothing; attribution markers don't follow `KERF_SESSION_ID` (kerf-c33, plan §scope item 5.5)

Setup: created works `alpha…gama` under default session, then `KERF_SESSION_ID=other-agent kerf new epsilon`.

Expected behavior: `--created-by self` shows only works the current session created; markers `(you)` vs. `(by <id>)` distinguish authors.

Actual:

- `kerf list` (default — which `--help` documents as `--created-by all`) showed **all six works** with `(you)` on every row regardless of which session created them.
- `KERF_SESSION_ID=other-agent kerf list` showed every row as `(by anon)` — including epsilon, which `other-agent` itself created.
- `kerf list --created-by self` returned **the same six rows as `--created-by all`**, with markers stripped. Filtering doesn't actually filter.

Looks like creation isn't recording the session ID at all (sessions list on `spec.yaml` shows `id: null` per S3), so every row reads as "you" from inside any session and "anon" from outside. The `--created-by self` flag is wired (no error), but the filter clause is a no-op because the input column is empty.

Severity: medium. The flag is the only multi-agent safety surface in this plan; in its current shape it silently passes everything.

### I3 — `any:` union grammar not accepted by `work edit` despite being the bootstrap output for multi-convention works (plan §scope, §design "multi-clause fallback")

```
$ kerf work edit alpha --bead-filter-add 'any:label=codename:alpha,label=alpha'
Error: clause "any:label=codename:alpha,label=alpha" does not parse.
Expected 'label=<value>' or 'id_prefix=<value>'
```

Bootstrap is documented to propose `any:` unions for works whose labels split between prefixed and bare conventions; an agent following the dry-run output by hand cannot reproduce that proposal through `work edit`. Either bootstrap should never emit `any:` (it didn't in this corpus, so the path may be untested), or `work edit --bead-filter-add` should accept the same grammar bootstrap writes.

Severity: low for this plan (Out-of-scope notes that grammar extensions belong to a separate surface), but the user-facing inconsistency is real — `work edit --help` documents only the two clause shapes.

## Minor

- `work edit --bead-filter-remove` of the last clause prints `bead_filter cleared; this work now falls back to the project filter or built-in default.` Useful, but the spec.yaml shows the key with **null value** (`bead_filter:` with nothing after it), and `kerf next` then classifies the work as `unwired` (no_bead_filter), not `empty`. Wording suggests "fallback to project filter" which is not what subsequent commands observe. Suggest: "bead_filter cleared (key remains with empty value; work is now rank-label `unwired`)."

## Top issues (ranked)

1. **Near-match advisor never fires** (I1). The marquee feature of the warning block in this plan is missing in practice.
2. **`--created-by self` is a no-op filter; attribution markers don't reflect creator session** (I2). The multi-agent safety net is open.
3. **`any:` grammar asymmetry** between bootstrap output and `work edit` input (I3). Cosmetic in this plan, but a real usability stub for the multi-convention case.

## Confirmed strengths

- Payload-first reordering on `kerf next` works cleanly: beads → cleanups → single-line warnings stanza.
- `unwired` vs. `empty` distinction visible and accurate; `(none)` rendering everywhere it ought to be.
- `bootstrap-filters` dry-run / `--yes` / re-run idempotency all match spec, including a clean per-work failure mode (`no label resembles 'foo'`) when the store is empty.
- Edit confirmation `Now matches: N (M open / K closed). Previously: P (Q open / R closed).` matches kerf-90d exactly.
- First-edit-on-null-filter (kerf-o7x) works without ceremony.
