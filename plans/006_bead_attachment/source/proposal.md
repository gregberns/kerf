# Investigations — 2026-05-14

Two design questions surfaced during smoke testing. Not plans yet — design proposals for review. Live under repo root rather than `plans/` until you pick a direction; promote to a plan when ready.

---

## A — Flexible bead-to-work attachment

**Problem.** Kerf attaches beads to works via the hardcoded label `work:<codename>`. A real project like harmonik uses `subsystem:*` and `hk-*` ID prefixes — pointing kerf at it gives zero signal. The convention is a kerf opinion forced on the bead store.

**Recommendation: a `bead_filter` config that defaults project-wide, with per-work override.**

Default-default (no config) keeps `work:{codename}` — full back-compat.

Project-wide override:

```yaml
# project.yaml
bead_filter:
  label: "subsystem:{codename}"   # {codename} substitutes per work
```

Per-work override for messy cases:

```yaml
# works/bridge/spec.yaml
codename: bridge
bead_filter:
  any:
    - label: "subsystem:bridge"
    - label: "codename:claude-hook-bridge"
    - id_prefix: "hk-cb"
```

`any:` is union. No `all:` / intersection in v1. One template variable: `{codename}`.

**Implementation shape.** `internal/beads/beads.go` gains a `Filter` struct + `Match(bead, codename)`. `ForWork` becomes a wrapper around it. `cmd/next.go` loads the project default once, checks per-work overrides, runs `Match`. No other call sites change.

**Onboarding.** `kerf init` (or new `kerf adopt`) scans existing beads, tallies label prefixes, picks the one with highest per-codename coverage, proposes it: *"Detected: most beads use `subsystem:*`. Set `bead_filter.label: subsystem:{codename}`? [Y/n]"*. Falls through to a prompt with top 5 prefixes if nothing fits.

**Multiple / zero matches.** A bead matching multiple works counts for each (today's implicit behavior — keep it, document it). Beads matching no work get a `kerf doctor` line surfacing top unmatched prefixes — visible misconfiguration, not fatal.

**Alternatives rejected.** Using `epic` field (harmonik doesn't populate it). Walking `parent`/dependency chains (indirect, expensive). Full predicate DSL (premature).

**Open questions for you.**
1. Template syntax: `{codename}` vs `${codename}` vs Go `{{.Codename}}`. Agent picked `{codename}`.
2. Auto-detect at `init` vs new `kerf adopt` — does init currently assume empty project?
3. `kerf doctor` warning on unmatched beads — useful, or noise for projects with intentional non-work beads?
4. Case sensitivity (current is `EqualFold`) — keep?
5. Ship per-work override in v1 or wait for someone to need it? (Agent recommends ship — harmonik already needs it.)

---

## B — Work-status vs bead-status coupling

**Problem.** Kerf gates downstream works on **kerf work status**, not bead reality. A work can have all 43 beads closed but if its status is still `problem-space`, dependents stay blocked. User must walk 7 jig statuses manually before the gate releases. Your framing: "if the beads are done, why does the work item's status matter?"

**Recommendation: separate "done for gating" from "done with the jig" via a derived predicate.**

Add one read-only signal — **effectively-complete** — defined as:

> A work is effectively-complete if either (a) its status is terminal/past, OR (b) it has at least one attached bead and every attached bead is closed.

**Code change is small.** Three callsites consult the new predicate instead of strict `IsComplete`:
- `internal/queue/queue.go:isTerminal` (drops effectively-complete works from `kerf next`)
- `internal/queue/queue.go:hasUnmetDeps` (releases dep gates on effectively-complete upstreams)
- `kerf list` status badge (display only)

**`kerf square` / `kerf finalize` keep the strict check.** Those are the moments where the jig's required artifacts matter; bead completion is not a substitute for the thinking process. So:
- Queue and gating: bead reality drives.
- Finalization: status walk required.

The discipline that protects artifacts stays exactly where it should — at packaging time, not at every `kerf next` call.

### Concrete before / after

**Today** (43/43 beads closed, status `problem-space`):
- `kerf next` skips dependents.
- User runs `kerf status <work> <next-stage>` 7 times.
- Only then does downstream surface.

**With change:**
- `kerf next` sees all beads closed → marks effectively-complete → downstream surfaces immediately.
- `kerf list` shows the work badged `problem-space (beads done)` so the user knows the jig walk is still owed if they ever `finalize`.
- `kerf finalize` refuses until real statuses are walked. Artifacts still protected.

### Why not auto-advance status?

Tempting ("first in_progress bead → set status to `analyze`") but ambushes the user. Spike case is real — someone may want a work stuck in `problem-space` while exploratory beads churn. Auto-mutation across two source-of-truth systems is the kind of magic that gets blamed for everything later. A derived predicate is read-only and easy to explain.

### Alternative rejected

Adding a second relationship type `must-empty-bead-queue` alongside `must-complete-first`. Doubles the vocabulary, forces users to predict which type of completion matters per dependency. The two coincide in practice.

### Jig differences

Universal predicate works for `plan`, `bug`, `spike` — same meaning everywhere. If some jig later wants strict status gating, add opt-in `gate_requires_status: true` to the jig definition. Don't build until needed.

**Open questions for you.**
1. What's the canonical "attached beads" join — the new flexible filter from proposal A, or current `work:<codename>`?
2. Should `kerf list` separate "done in beads, jig not walked" into its own section to nudge eventual finalization?
3. Should `kerf next` ever say "you have idle works that are effectively-complete — walk them or shelve them"?
4. Zero-bead works: proposal leaves them gated on status only. Confirm.

---

## How the two interact

These compose cleanly. Proposal A defines *which* beads attach to a work (flexible filter). Proposal B defines *what bead completion does to gating* (the predicate). A is a prerequisite for B in the sense that "all attached beads closed" only means something useful if attachment isn't accidentally empty. If you ship B first with the old `work:<codename>` rule, harmonik-style users still see no benefit; ship A first or together.

**Recommended sequencing.** A first (or together with B). One plan can cover both — they're a coherent "kerf adapts to existing bead stores and respects their reality" theme.


---

## Source

Both proposals came from background investigation agents. Originals are in the agent transcripts; this file is the durable record. When you pick a direction on either, create a real `plans/00N_<name>/` and move the design in.
