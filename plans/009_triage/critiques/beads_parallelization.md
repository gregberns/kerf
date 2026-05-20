# Plan 009 — Parallelization Critique

## Dependency graph: correct, with slack

No cycles. Per-bead deps match the import map. B3 is correctly flagged as file-disjoint from B1 and could equally sit at L0. B6 depends only on B2 and could sit at L1.

## Peak concurrency: writer's "5" is right, but late

Writer claims 5 workers at L3 (B8/B9/B10/B11a/B11b). True — each owns a distinct `cmd/*.go`. However, the layering understates parallelism by forcing B9/B10/B11a to wait for L2 (B6, B7) when they only need B1+B3.

Reorder for wall-clock savings:

| Promoted layer | Beads | Rationale |
|---|---|---|
| L0' | B1, B2, B3 | B3 is a pure SpecYAML field add — leaf. |
| L1' | B4, B5, B6, B9, B10, B11a | B6 only needs B2. B9/B10/B11a only need B1/B3. |
| L2' | B7, B8, B11b | B7 needs B5; B8/B11b need B4+B5+B6. |
| L3' | B12 | unchanged. |

Peak concurrency rises to **6** at L1' (B4, B5, B6, B9, B10, B11a). Critical path shrinks from 5 hops (writer) to 4: B2 → B6 → B8 → B12 (or B2 → B5 → B8 → B12). Writer's "5 hops" counts nodes, not edges; either way the promoted layering saves one wall-clock layer.

## Hidden dep check: B8 vs B4/B6 sequencing

B8 reads detectors from B4 and cache I/O from B6 — both must land before B8 starts. In the promoted plan, B4+B6 finish in L1', B8 starts in L2'. Correct. B11b has identical dep set to B8 and is correctly co-scheduled.

## Real serialization risk

`cmd/root.go` registration: writer notes B8 may absorb the AddCommand lines for B9/B10/B11a/B11b. If the repo uses centralized `AddCommand` (not `init()`), all five L3 beads serialize on `root.go`. Verify the existing pattern up front and pin to `init()`-attach to preserve fan-out.

B7's external dep on plan-008 B1 is a real gate — confirm landed before kicking L1'.
