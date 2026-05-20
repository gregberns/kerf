# Beads — Architecture Critique

## Import cycles

No cycles. `internal/drift` is genuinely new (verified: nothing under
`internal/` or `cmd/` references it today). The map is clean:
`drift → beads` only; `feed → drift, beads, spec`; `spec → (yaml only)`;
`beads` stays a leaf. The judgment call to put `ParseFilterClause` in
`internal/spec` rather than `internal/beads` is correct — it preserves
`beads` as a leaf — but the parser logically belongs with the matcher in
`internal/beads/filter.go`. Consider exposing `beads.ParseFilterClause` and
having `spec` call it; otherwise `spec` ends up reimplementing clause syntax
that lives next to `Filter.Match`.

## File-ownership collisions

B1 vs B3 in `internal/spec`: disjoint (`mutate.go` new vs `spec.go` modify). Clean.
B4 vs B5 in `internal/feed`: disjoint (`warning.go` vs `feed.go`). Clean.
But B4 silently renames the existing `unmatchedBeadsDetector` (line 53 of
`warning.go`) to `untriagedBeadsDetector` and rewires the registration in
`NewWarningDetectors` (line 42). That is non-trivial and the bead spec
buries it as a one-line "rename rather than coexist" — it should be its own
deliverable with a callers-audit.

## YAML Node API requirement

Confirmed required. `gopkg.in/yaml.v3` is already in `go.mod` (currently
`indirect`; will become direct). `spec.go` uses the high-level
`yaml.Marshal/Unmarshal` path which destroys comments. Comment-preserving
round-trip mandates `yaml.Node` decode + targeted mutation + encode.
Non-trivial: idempotent re-encode with byte-equal output (B1 test asserts
this) requires careful handling of style flags (`FlowStyle`, indent,
head/foot/line comment slots) and an empty `pinned_beads: []` rendered as
flow `[]` rather than block. Budget more than the plan's "land cheaply"
assumption.

## Pin-layer wiring (real architectural risk)

**The plan misplaces the pin layer.** `feed.Assemble` operates on
*already-classified* `Item` slices; filter resolution happens earlier when
the caller (`cmd/next.go:245`) constructs `BeadToWork`. `feed.BeadSource`
emits one item per `(bead, matched-work)` pair from `BeadToWork`. The pin
override must mutate `BeadToWork` *before* `BeadSource` runs, or live
inside `BeadSource`. Putting it in `Assemble` is too late — items are
already emitted. B5 must move the pin step into `BeadSource` (or a new
`ResolvePins` helper called by `cmd/next.go` / `cmd/triage.go` after the
filter join). Tests in B5 will pass against a misplaced implementation
only if they bypass `BeadSource`.

## cmd/root.go registration

Existing pattern is per-file `init()` + `rootCmd.AddCommand` (see
`cmd/new.go:66-71`, `cmd/next.go:81-86`). B8's note that it "owns the
registration line in `cmd/root.go`" is wrong — `root.go` is not touched at
all. Each L3 bead self-registers via `init()`. Drop the special-case
sequencing in the parallelization plan.

## Module rearrangement

- Move `ParseFilterClause` to `internal/beads` (keeps clause syntax beside
  `Filter.Match`); `internal/spec/mutate.go` calls it.
- Fold B6 into B2 — `cache.go` is 4 small functions; splitting only adds an
  inter-bead dep on the critical path.
