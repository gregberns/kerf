# kerftranscript testdata

Fixture transcripts and golden findings for the D1 (abandoned dispatch) and
D6 (reviewer-absent commit) detectors. Each fixture is derived from a real
incident catalogued in `plans/013_self_diagnostics/source/detector_examples.md`
and trimmed to the minimum events needed to exercise the parser, indexer, and
detector under test.

## Layout

```
testdata/
  d1_abandon_a.jsonl                + d1_abandon_a.golden.json
  d1_abandon_b.jsonl                + d1_abandon_b.golden.json
  d6_reviewer_absent_a.jsonl        + d6_reviewer_absent_a.golden.json
  d6_reviewer_absent_b.jsonl        + d6_reviewer_absent_b.golden.json
  d6_reviewer_absent_c.jsonl        + d6_reviewer_absent_c.golden.json
```

The `.jsonl` is the transcript input. The `.golden.json` is the detector
output the integration tests assert against.

## JSONL event schema (v0, fixture-only)

One JSON object per line. Required field: `kind`. Other fields are
kind-specific. Timestamps are RFC3339 UTC.

| field         | type    | meaning                                                    |
|---------------|---------|------------------------------------------------------------|
| `timestamp`   | string  | RFC3339 UTC                                                |
| `kind`        | string  | one of: `dispatch`, `tool_result`, `commit_ref`, `bead_close` |
| `session_id`  | string  | Claude session UUID                                        |
| `sub_agent_id`| string  | sub-agent id, present on `dispatch` and `tool_result`      |
| `bead_id`     | string  | bead the event is about, when known                        |
| `text`        | string  | optional free-form payload (e.g. commit message body)      |
| `commit_sha`  | string  | present on `commit_ref` / `bead_close`                     |
| `is_error`    | bool    | present on `tool_result` for error-tagged results          |
| `role`        | string  | optional sub-agent role tag, e.g. `implementer`, `reviewer` |

This is a **fixture schema**, not the parser's final input shape. The real
parser (bead B2) reads Claude Code session JSONL; these fixtures are a
distilled projection of that format with just enough fields to drive the
detectors. The parser bead is expected to calibrate the on-disk schema to
match (or to add a thin adapter), keeping these fixtures stable.

## Event kinds

- `dispatch` — orchestrator launched a sub-agent against a bead. The
  detector treats this as the start of the dispatch interval.
- `tool_result` — a tool invocation returned to the sub-agent. Used to
  observe sub-agent activity / errors.
- `commit_ref` — a commit landed referencing one or more bead IDs. The
  indexer keys on these to decide whether a dispatched bead produced code.
- `bead_close` — a bead was closed (possibly as SUBSUMED). Does not by
  itself indicate code landed for the bead.

## Finding schema (v0, fixture-only)

```jsonc
{
  "findings": [
    {
      "kind": "abandoned_dispatch" | "reviewer_absent",
      "bead_id": "...",
      "detail": { /* detector-specific evidence */ }
    }
  ]
}
```

D1 (`abandoned_dispatch`) `detail` fields:

- `session_id`
- `sub_agent_id`
- `dispatched_at` (RFC3339)
- `last_activity_at` (RFC3339)
- `reason_category` — one of `appears_completed_no_commit`,
  `errored_mid_task`, `orphaned`, `tool_linkage_broken`. See
  `plans/013_self_diagnostics/_plan.md` §"Reason categories".
- `close_commit` (optional) — commit SHA that retired the bead without
  implementing it, e.g. a SUBSUMED close.

D6 (`reviewer_absent`) `detail` fields:

- `session_id`
- `commit_sha`
- `committed_at` (RFC3339)
- `implementer_sub_agent_id` (optional)

## Source mapping

Every fixture corresponds 1:1 to an incident in
`plans/013_self_diagnostics/source/detector_examples.md`:

| Fixture                        | Source incident                                              |
|--------------------------------|--------------------------------------------------------------|
| `d1_abandon_a`                 | session `fed61a3d-…`, bead `hk-qo08q.15`, SUBSUMED via `4a3c217` |
| `d1_abandon_b`                 | session `fed61a3d-…`, bead `hk-2ubs8`, same SUBSUMED close   |
| `d6_reviewer_absent_a`         | bead `hk-iuaed.6`, commit `dcd7f7e…`, session `801120b5-…`   |
| `d6_reviewer_absent_b`         | bead `hk-zixbp`, commit `cc3da5c…`, session `4c818416-…`     |
| `d6_reviewer_absent_c`         | bead `hk-qo08q`, commit `76a55be…`, same session as `_b`     |

SHAs, bead IDs, session UUIDs, and sub-agent IDs are preserved verbatim so
that diffs against the corpus stay obvious.
