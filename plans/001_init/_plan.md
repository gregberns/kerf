# Plan 001: Initial Spec Generation

## Intent

Bootstrap the kerf project's spec-driven workflow by converting the design documents (produced during the exploration phase) into normative, structured specifications.

## What Changes

This plan creates the entire `specs/` directory from scratch. There are no prior specs — this is the founding plan.

## Source Material

The `source/` directory contains the original design documents (formerly `docs/`):

| Source File | Content | Feeds Into |
|---|---|---|
| 00-index.md | Decision summary, project overview | specs/_index.md |
| 01-problem-statement.md | Why kerf exists | specs/_index.md (overview) |
| 02-proposed-solution.md | Design principles, user journey | specs/architecture.md, specs/cli.md |
| 03-core-concepts.md | Works, jigs, sessions, status, deps, bench | specs/works.md, specs/sessions.md, specs/dependencies.md, specs/jig-system.md, specs/verification.md |
| 04-cli-design.md | All commands, output philosophy | specs/cli.md, specs/commands.md, specs/finalization.md |
| 05-data-model.md | Schemas, directory layout, file formats | specs/architecture.md, specs/works.md, specs/sessions.md, specs/snapshots.md |
| 06-default-jigs.md | Feature and bug jig definitions | specs/jig-feature.md, specs/jig-bug.md |
| 07-testing-strategy.md | Testing layers, CI strategy | specs/testing.md |
| 08-future-work.md | Out of scope items | specs/future.md |
| 09-naming.md | Name, vocabulary, tagline | specs/_index.md (glossary) |
| 10-open-questions.md | Resolved/deferred decisions | Distributed across relevant specs |

## Spec Files Created

See `specs/_index.md` for the full map. 15 files total:
_index.md, architecture.md, works.md, sessions.md, snapshots.md, dependencies.md,
jig-system.md, jig-feature.md, jig-bug.md, cli.md, commands.md, finalization.md,
verification.md, testing.md, future.md

## Spec Format

All specs follow this structure:
- Normative language: "the system does X", not "we chose X because Y"
- Organized by domain with consistent heading structure
- Code blocks for schemas, directory layouts, examples
- Cross-references via relative links to other spec files
- No YAML frontmatter — pure markdown

## Post-Plan

After specs are generated and reviewed, `docs/` is removed. The source material is preserved here in `plans/001_init/source/` as historical record.
