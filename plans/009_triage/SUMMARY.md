# Plan 009 — Triage Workflow (Summary)

## What kerf can't do today that this fixes

When a project's task list changes outside of kerf — someone closes a task, adds
a new one, deletes another, or relabels a few — kerf doesn't notice. It keeps
running with its old picture of the project and quietly gives the wrong answers.
Agents trying to use kerf to figure out "what should I work on?" end up
ignoring it and going back to the raw task list, which defeats the point.

## What an agent will be able to do after this lands

An agent walks into a project and runs one command, `kerf triage`. kerf shows
exactly what changed since the last time anyone looked: which tasks are new,
which got closed externally, which aren't bucketed into any work, and which got
double-bucketed. The agent acts on each item — bucketing new tasks, splitting
ambiguous ones — and re-runs `kerf triage` until kerf reports "all clean".
Because the command has a clean/not-clean exit code, the agent can put the
whole thing in a loop and keep going until the project is tidy.

## Shape of the change

- One new command, `kerf triage`, that surfaces drift and exits clean/not-clean
  (with a third exit code that signals "stuck — break the loop").
- One small new command, `kerf pin`, as an escape hatch for unusual cases.
  Pinning a task to one work removes it from any other work — so the loop
  always converges.
- Existing commands get small additions: `kerf show` learns to list the
  tasks attached to a work; `kerf new` gets a flag for setting up a work's
  task filter at creation; `kerf work edit` gets flags to widen or narrow an
  existing work's filter (the main remediation path when a filter is too
  narrow); `kerf next` shows a one-line drift summary at the top of its
  output so agents see problems without running `triage`.
- A small cache file inside each project so kerf remembers what the task list
  looked like last time. The cache only advances when an agent explicitly
  acknowledges it — no silent rebasing.

## Rough cost

One and a half to two and a half sprint-weeks for one engineer. The bulk of
the work is the new `kerf triage` command and the drift-detection cache; the
rest is small edits across existing commands plus careful YAML editing so
kerf never silently destroys a user's comments in `spec.yaml`.

## What it doesn't do

- It only works on one project at a time. There's no cross-project rollup view.
- It never fixes anything on its own. Every drift item is surfaced for an
  agent (or a human) to decide what to do with. We deliberately avoided any
  "auto-fix" mode — silent changes are exactly the failure we're trying to
  eliminate.
- It doesn't create new tasks. The agent still adds tasks through the existing
  task tool first, then runs `kerf triage` to bucket them.
