# Scope + Design Critique — Plan 009

## Drift-detection hash

Hashing only kerf-consumed fields (status, sorted labels, title, deps) is the
right call — full-record hashing churns on every bd metadata write. But the
list is incomplete:

- **`id_prefix` is a filter clause type.** Renames or ID changes alter
  attachment; ID must be part of the hashed identity (it is the map key in the
  example, so this is implicit — say so).
- **`deps` belongs in the hash** (plan lists it once in Open Questions, drops
  it from the example payload on L96). Dependency edits change which beads are
  eligible in `kerf next`; not hashing them re-creates the same "silent drift"
  the plan exists to fix.
- **What about labels kerf doesn't currently consume but the user's filter
  references?** Hash scope is "fields kerf consumes" — but the project filter
  is user-defined. Recommendation: hash *all* labels (sorted), not a kerf-
  curated subset. Labels are cheap.

**Timing is underspecified.** Plan says "every kerf invocation that touches
the bead store … records deltas" but never says when the *baseline* advances.
L86 ("any successful triage resolution") + Open Question #2 contradict each
other. Pick one: baseline advances only on explicit `--ack` or on a `triage
--resolved` exit-0 run. Implicit advancement on `kerf new` is a footgun — an
agent runs `new`, drift silently rebaselines, the next `triage` shows clean
without the agent ever seeing the closed beads.

## 7 → 2+3+1+2 cut

Right call on `kerf show` (clear daily loop) and `kerf triage --resolved`
(load-bearing exit code). Drops are mostly fine, but:

- **`kerf work edit --bead-filter-add/remove` was dropped.** This is the only
  remediation path for the `work_no_attached_beads` cleanup item that the
  agent-UX critique flagged as a wrong-action-loop trigger. Without it, the
  triage canonical workflow (L124-133) has no way to *broaden* an existing
  work's filter; the agent can only create a new work or `attach`. The plan's
  own L66 "the right fix is usually to broaden the filter" assumes a command
  it deleted. Bring this back, even as a v1.1.
- `kerf map` bead counts: correctly deferred.

## `internal/drift/` package location

Right call to make it its own package, not folded into `feed/` or `beads/`:

- `internal/beads/` is the read path against `br`; mixing snapshot persistence
  there muddles "source of truth" vs "kerf's memory of it".
- `internal/feed/` consumes drift signals to emit warnings; it should depend
  on `drift`, not own it. The plan implicitly gets this right (L178 "three
  new warning detectors" in feed, separate drift package).
- One nit: the cache file lives at `.kerf/sync-cache.json` (project-local).
  The `internal/project/` package owns project paths today; `drift` will need
  to import `project` for resolution. Call that dependency out.

## `--resolved` partial-resolution

The exit table (L114-118) is binary: any non-zero count → exit 2. Real
workflow is incremental — agent resolves 3 of 5 untriaged, hits a hard case,
wants to commit progress. With binary exit, the loop `until kerf triage
--resolved; do <act>; done` never terminates on the hard case; agent has no
"made progress" signal.

Recommendations, pick one:

- Exit 2 = unresolved, but emit a `progress` line: `resolved 3 since last
  ack`. Loop scripts can detect "no progress two runs in a row" → break.
- Or accept binary and document the escape: agent calls `--ack` to skip a
  specific bead bucket (currently `--ack` is all-or-nothing — also a gap).

## 1.5 sprint-weeks honesty

Plausible but tight. File list says 6 cmd files + `internal/drift/` new + `internal/spec/` mutators + 3 warning detectors + 5 spec sections. Real risks:

- `internal/spec/` "safe in-place mutators" for `spec.yaml` (adding filters,
  pinned beads) is the hidden expensive piece. Round-trip comment preservation
  is hand-waved as "nice-to-have"; first time an agent's edit nukes a user
  comment, this becomes mandatory rework. Add 2 days if it slips into v1.
- Spec changes first (L185) is correct per CLAUDE.md, but four spec files
  touch (commands, coordination, works, cli) — that's 1–2 days alone before
  any code.

Honest range: **1.5–2.5 sprint-weeks**. Headline 1.5 is the floor.

## Plan 008 dependencies under-called

L197-201 names two (P0#3 show.go rewrite, P0#1 work_codename:null). Missing:

- **Plan 008 P0.2 (corrupt `spec.yaml` warning)** — plan 009 adds a
  `pinned_beads:` field to `spec.yaml`; without the corrupt-spec detector, a
  malformed pin list silently drops the work and triage's multi-match logic
  becomes wrong.
- **Plan 008 spec-conformance "P1.3 relabel drift hashing scope undefined"** —
  the spec-conformance critique flags that hash scope is currently
  underspecified; plan 009 must close that gap in `coordination.md` *before*
  the drift package lands, or two reviewers will read it differently.
- **Plan 008 P0.4 / P0.3 (bd→br shell removal)** — drift snapshot reads must
  go through `internal/beads.List()` for consistency; if any `bd` shell path
  remains, baseline and live reads diverge.
