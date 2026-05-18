#!/usr/bin/env python3
"""Analyze sweep_v2_results.csv: 27-combo (momentum x rework x fan_out)
grid on the saturated all_pilots_sat scenario.

Because there is only one scenario, the v1 "domination on >=60% of
scenarios" rule degenerates to a per-seed test. We treat each of the 3
seeds as an independent observation; a candidate must beat the
default (m5_r15_f10) on >=60% of seeds on goal_completion_3d (tie-
break: work_completed) with no >5% loss on any guardrail metric
(goal_completion_3d, work_completed, area_collisions, rework_p95_wait).

This keeps the v1 decision rule's spirit while acknowledging that
v2's signal is a single (saturated) scenario rather than 16.
"""
import csv
import statistics
from collections import defaultdict
from pathlib import Path

WT  = Path("/Users/gb/github/kerf/plans/011_sim_validation/weight_tuning")
CSV = WT / "sweep_v2_results.csv"
BASELINE = "m5_r15_f10"

HIB = {"goal_completion_3d", "work_completed", "goal_completion_7d"}
LIB = {"area_collisions", "rework_p95_wait", "top_of_queue_churn",
       "agent_idle_pct", "priority_inversions"}
GUARDRAILS = ["goal_completion_3d", "work_completed",
              "area_collisions", "rework_p95_wait"]

def load():
    rows = []
    with CSV.open() as fh:
        for r in csv.DictReader(fh):
            for k in ("work_completed", "goal_completion_3d", "goal_completion_7d",
                      "area_collisions", "rework_p95_wait", "top_of_queue_churn",
                      "agent_idle_pct", "priority_inversions",
                      "seed", "momentum", "rework", "fan_out"):
                v = r.get(k)
                try: r[k] = float(v) if v not in ("", None) else None
                except: r[k] = None
            rows.append(r)
    return rows

def pct_change(new, base):
    if base == 0: return 0.0 if new == 0 else float("inf")
    return (new - base) / base * 100.0

def by_seed(rows, policy="kerf"):
    """Return {weights_label: {seed: {metric: value}}}."""
    out = defaultdict(dict)
    for r in rows:
        if r["policy"] != policy: continue
        out[r["weights_label"]][int(r["seed"])] = r
    return out

def main():
    rows = load()
    print(f"Loaded {len(rows)} rows.")
    weights = sorted({r["weights_label"] for r in rows})
    print(f"Weight combos: {len(weights)}")
    print(f"Baseline: {BASELINE}")
    print()

    if BASELINE not in weights:
        print(f"FATAL: baseline {BASELINE} missing from results.")
        return

    grid = by_seed(rows, policy="kerf")
    seeds = sorted(grid[BASELINE].keys())
    print(f"Seeds: {seeds}")
    print()

    # Per-combo: median across seeds for each metric.
    medians = {}
    for w in weights:
        cells = grid[w]
        medians[w] = {}
        for m in HIB | LIB:
            vals = [cells[s].get(m) for s in seeds if cells.get(s) and cells[s].get(m) is not None]
            medians[w][m] = statistics.median(vals) if vals else None

    base_med = medians[BASELINE]

    # Domination test: count seeds where w beats baseline on g3d.
    print("=== Per-combo per-seed domination on goal_completion_3d (kerf) ===")
    print(f"{'combo':<14} {'g3d':>10} {'wc':>10} {'col':>8} {'rwk_p95':>9} {'idle':>6}  vs base  wins/seeds  viol")
    print(f"{'':<14} {'(med)':>10}")
    contenders = []
    for w in weights:
        g_med  = medians[w]["goal_completion_3d"]
        wc_med = medians[w]["work_completed"]
        col_med= medians[w]["area_collisions"]
        rwk_med= medians[w]["rework_p95_wait"]
        idle_med= medians[w]["agent_idle_pct"]
        wins = 0
        comparable = 0
        for s in seeds:
            r_new = grid[w].get(s); r_base = grid[BASELINE].get(s)
            if not r_new or not r_base: continue
            comparable += 1
            g_new, g_base = r_new["goal_completion_3d"], r_base["goal_completion_3d"]
            wc_new, wc_base = r_new["work_completed"], r_base["work_completed"]
            if g_new is None or g_base is None: continue
            if g_new > g_base: wins += 1
            elif g_new == g_base and (wc_new or 0) > (wc_base or 0): wins += 1
        # Guardrail violations using medians.
        viol = []
        for m in GUARDRAILS:
            new = medians[w][m]; base = base_med[m]
            if new is None or base is None: continue
            pc = pct_change(new, base)
            if m in HIB and pc < -5.0: viol.append(f"{m}:{pc:+.1f}%")
            if m in LIB and pc > 5.0:  viol.append(f"{m}:{pc:+.1f}%")
        marker = " <-- baseline" if w == BASELINE else ""
        wpc = 100.0 * wins / comparable if comparable else 0
        print(f"{w:<14} {g_med:>10} {wc_med:>10} {col_med:>8.0f} {rwk_med:>9.0f} {idle_med:>6.3f}  {wins}/{comparable} ({wpc:.0f}%) {'; '.join(viol) if viol else '-'}{marker}")
        if w != BASELINE and wpc >= 60.0 and not viol:
            contenders.append((w, wpc, medians[w]))

    print()
    print("=== Combos passing >=60% seed wins + no >5% guardrail loss ===")
    if not contenders:
        print("  NONE. Default m5_r15_f10 retained.")
    else:
        # Pick the one with the largest median g3d improvement.
        best = None
        for w, wpc, med in contenders:
            d = med["goal_completion_3d"] - base_med["goal_completion_3d"]
            print(f"  {w}: wins={wpc:.0f}% g3d +{d:.1f}, wc +{(med['work_completed'] or 0)-(base_med['work_completed'] or 0):.1f}")
            if best is None or d > best[1]:
                best = (w, d)
        print(f"\n  WINNER: {best[0]} (median +{best[1]:.1f} on g3d)")

    # Marginal-effect sanity: hold two of three dims, vary the third.
    print()
    print("=== Marginal effect of each dimension at baseline midpoint (medians, kerf) ===")
    base_m, base_r, base_f = 5.0, 15.0, 10.0
    def lbl(m, r, f):
        def s(x): return str(int(x))
        return f"m{s(m)}_r{s(r)}_f{s(f)}"
    print("vary momentum (rework=15, fan_out=10):")
    for m in (2.0, 5.0, 10.0):
        w = lbl(m, base_r, base_f)
        med = medians.get(w, {})
        print(f"  m={m:<4} g3d={med.get('goal_completion_3d')} wc={med.get('work_completed')} col={med.get('area_collisions')} rwk={med.get('rework_p95_wait')}")
    print("vary rework (momentum=5, fan_out=10):")
    for r in (5.0, 15.0, 30.0):
        w = lbl(base_m, r, base_f)
        med = medians.get(w, {})
        print(f"  r={r:<4} g3d={med.get('goal_completion_3d')} wc={med.get('work_completed')} col={med.get('area_collisions')} rwk={med.get('rework_p95_wait')}")
    print("vary fan_out (momentum=5, rework=15):")
    for f in (5.0, 10.0, 20.0):
        w = lbl(base_m, base_r, f)
        med = medians.get(w, {})
        print(f"  f={f:<4} g3d={med.get('goal_completion_3d')} wc={med.get('work_completed')} col={med.get('area_collisions')} rwk={med.get('rework_p95_wait')}")

if __name__ == "__main__":
    main()
