# Critique — Plan 008 Beads Parallelization

Scope: dependency-graph correctness and real concurrency in `beads.md`.

## Claimed vs. actual concurrency

**Phase 0 — "up to 7 workers" overstates it.** `cmd/next.go` is touched by
B3, B4, B6 (and B10-code in Phase 1). The File Ownership table itself
sequences these: B3 → B4 → B6. So Phase 0 has only **4 truly parallel
streams**:

1. `cmd/next.go` chain: B3 → B4 → B6 (3 beads, sequential)
2. `cmd/show.go` chain: B1 → B5 (B5 also touches `cmd/map.go`)
3. `cmd/square.go`: B2
4. `cmd/root.go` + testdata: B8, B7

B7 ("terminology audit across `cmd/*.go` help strings") edits multiple
`cmd/*.go` files and therefore conflicts opportunistically with any other
cmd-touching bead. Treat B7 as **serialized last** in Phase 0, not parallel.

Realistic Phase 0 max parallelism: **4 workers**, not 7. Critical path is
the next.go chain (~3 beads serial), not B1→B5 as claimed.

## B5-after-B1 sequencing

Correct. Same file (`cmd/show.go`). But B5 also owns `cmd/map.go`, which
nothing else touches — B5's map.go work could be split into a separate
B5a runnable in parallel with B1. Marginal value (~½ day saved); only
worth it if a worker is idle.

## Phase 1 spec beads

B9/B10/B11/B12/B13-spec are listed as 5 independent workers. **Conflict:**
B10-spec, B11-spec, and B13-spec all edit `specs/coordination.md`;
B9-spec, B10-spec, B13-spec all edit `specs/commands.md`. File Ownership
table acknowledges this with sequencing, contradicting the "5 workers"
claim. Real Phase 1-spec parallelism: **2 workers** (B12-spec independent;
one worker chains the commands.md/coordination.md edits).

## Investigation gate scope

B14 gates only scoring/weights work, none of which is in Phase 0/1.
Correctly **not** blocking B1–B13. Good. B14 can genuinely run in parallel
with everything.

## Reorders that save wall-clock

1. Start B14 on day 1 (longest single bead, 1–2d) in parallel with Phase 0.
2. Run B7 **last** in Phase 0; do not promise it as parallel.
3. Chain commands.md spec edits (B9→B10→B13) on one worker; B11-spec and
   B12-spec on a second. Drop "5 workers" claim.
4. Split B5's `cmd/map.go` portion if a worker is free during B1.

Net: honest Phase 0 ≈ 2 days wall-clock (not 4) at 4 workers; Phase 1
spec ≈ 1 day at 2 workers.
