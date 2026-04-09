# kerf

Spec-writing CLI for AI agents. Single binary. Go.

*Measure twice, cut once.*

## Quick Start

```bash
# Install
go install github.com/gberns/kerf@latest

# Open your AI coding agent in your project, then paste:
```

> Install kerf (`go install github.com/gberns/kerf@latest`), run `kerf init` in this project, and follow its setup instructions.

That's it. The agent bootstraps itself — creates config files, updates gitignore, and learns how to use kerf. It works with any AI coding agent (Claude Code, Cursor, Windsurf, Codex, etc.).

## What kerf does

kerf manages structured planning workflows for AI agents. Instead of jumping straight to code, the agent walks through a defined process: understand the problem, decompose it, research options, write a detailed spec, then break it into implementation tasks.

Every step produces files on disk. Work is resumable across sessions. Multiple works can be in flight simultaneously.

## Two workflows

**Plan-first** (`kerf config default_jig plan`) — for existing projects. The agent writes a change plan before touching code. The codebase remains the source of truth.

**Spec-first** (`kerf config default_jig spec`) — for new projects or teams maintaining living specs. Specs define what the system does. Code that doesn't match the spec is wrong.

## Commands

```
kerf new <codename>              Create a new work
kerf show <codename>             Current state + what to do next
kerf status <codename> [status]  Read or advance status
kerf shelve <codename>           Save progress, end session
kerf resume <codename>           Pick up where you left off
kerf list                        Show all works
kerf square <codename>           Verify work is complete
kerf finalize <codename>         Package into a git branch
kerf snapshot <codename>         Save a snapshot
kerf history <codename>          View snapshot history
kerf restore <codename> <snap>   Restore a previous snapshot
kerf archive <codename>          Archive a completed work
kerf delete <codename>           Remove a work
kerf config [key] [value]        View or set configuration
kerf jig list|show|save|load     Manage process templates
kerf init                        Bootstrap kerf in a project
```

## How it works

kerf stores works on a **bench** (`~/.kerf/`) outside your git repo. Each work has a `spec.yaml` tracking its state and a set of artifact files produced during each pass of the workflow.

**Jigs** are process templates — they define the passes an agent walks through (problem space, decomposition, research, spec writing, etc.) and include detailed instructions the agent follows at each step.

Works enter your git repo only at **finalization**, when `kerf finalize` creates a branch and commits the artifacts.

## Project layout

```
specs/              # Normative specifications (source of truth for spec-first projects)
plans/              # Change proposals with rationale and task breakdowns
  {name}/
    _plan.md        # Intent, design, spec changes
    beads.md        # Implementation task breakdown
```

## License

MIT
