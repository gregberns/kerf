# kerf — Roadmap

kerf is a spec-driven Go CLI that helps AI coding agents manage development work — a single binary built on the "measure twice, cut once" principle. It is becoming the **process-management layer the agent reasons about**: not a tool that helps do the work, but one that helps manage the process around the work — what to pick up next, when to merge, when to defer, when something is going off the rails.

## Top-level goals

- Make kerf usable by a coding agent end-to-end without friction on a real repo.
- Move from "find one set of scheduler weights" to a process layer that adapts to the work in front of it.
- Validate scheduler behavior against realistic, contrasting workloads in simulation before shipping it to users.
- Detect process issues live, from actual session transcripts, so workflow regressions surface quickly.
- Stay spec-driven — every change lands in `specs/` first, then code.

## Active plans

- [Plan 011 — simulator validation](plans/011_sim_validation/_plan.md) — closing the learning loop on the simulator (concurrency sweeps, adversarial scenarios, weight tuning). Mostly shipped; weight tuning returned a useful null result that fed Plan 014.
- [Plan 012 — real-workload corpus](plans/012_real_corpus/_plan.md) — drives the simulator with real bead shapes and fitted per-phase durations from harmonik and kerf history. Shipped.
- [Plan 013 — self-diagnostics from Claude transcripts](plans/013_self_diagnostics/_plan.md) — built-in `kerf diagnose` that flags procedural issues (abandoned dispatches, missing reviewer phases, wasted work) from session logs. Designed with six detectors and example fixtures; not yet implemented.
- [Plan 014 — process-management reframe](plans/014_process_management_reframe/_plan.md) — reframes the scheduler from a single global weight vector to declarative inputs the agent already knows. **On the roadmap, not yet prioritized.**
- [Plan 015 — harmonik beta feedback triage](plans/015_harmonik_beta_feedback/_plan.md) — just landed; triages ~50 friction items from real harmonik beta use and routes them into a handful of follow-on plans (016–020) covering init UX, storage reconciliation, triage rework, work-filter bootstrap, and the spec-jig review gate.

## Near-term focus

The current focus is the Plan 015 follow-on plans (016–020) — friction items from real harmonik use.

## Future direction (deferred)

[Plan 014](plans/014_process_management_reframe/_plan.md) is the longer-arc reframe of how the scheduler works. Deferred until the Plan 015 follow-ons are absorbed and the surface that agents actually touch is solid.

## Where details live

Plans live under `plans/`. Specs live under `specs/`. This doc points; it does not duplicate.
