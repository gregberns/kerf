#!/usr/bin/env python3
"""Analyze sweep_results.csv to identify the winning weight combo.

Decision rule (per task brief):
  1. A weight change is adopted only if it dominates the current default
     (momentum=5, rework=15 -> label m5_r15) on >=60% of scenarios on
     goal_completion_3d (tie-break work_completed).
  2. No scenario loses by more than 5% on any of:
     goal_completion_3d, work_completed, area_collisions, rework_p95_wait.

Note: area_collisions and rework_p95_wait are LOWER-IS-BETTER, so a "loss" there
is an INCREASE. We treat >5% increase from baseline as a violating loss.

We focus the comparison on policy=kerf (the policy whose weights we're tuning).
Other policies are reported for context but not used in the decision.
"""
import csv
import statistics
from collections import defaultdict
from pathlib import Path

WT = Path("/Users/gb/github/kerf/plans/011_sim_validation/weight_tuning")
CSV = WT / "sweep_results.csv"
BASELINE = "m5_r15"

# higher-is-better metrics
HIB = {"goal_completion_3d", "work_completed"}
# lower-is-better metrics
LIB = {"area_collisions", "rework_p95_wait", "top_of_queue_churn",
       "agent_idle_pct", "priority_inversions"}

def load():
    rows = []
    with CSV.open() as f:
        for r in csv.DictReader(f):
            for k in ("work_completed","goal_completion_3d","area_collisions",
                      "rework_p95_wait","top_of_queue_churn","agent_idle_pct",
                      "priority_inversions","seed"):
                try: r[k] = float(r[k]) if r[k] not in ("",None) else None
                except: r[k] = None
            rows.append(r)
    return rows

def median_by(rows, keys, metric, policy="kerf"):
    """Return {(*keys): median(metric)} for given policy."""
    buckets = defaultdict(list)
    for r in rows:
        if r["policy"] != policy: continue
        v = r.get(metric)
        if v is None: continue
        buckets[tuple(r[k] for k in keys)].append(v)
    return {k: statistics.median(v) for k,v in buckets.items() if v}

def pct_change(new, base):
    if base == 0: return 0.0 if new == 0 else float("inf") if new>0 else float("-inf")
    return (new - base) / base * 100.0

def main():
    rows = load()
    # Build per (scenario, weights_label) median for each metric, policy=kerf.
    scenarios = sorted({r["scenario"] for r in rows})
    weights = sorted({r["weights_label"] for r in rows})
    print(f"Loaded {len(rows)} rows. scenarios={len(scenarios)} weight-labels={len(weights)}")
    print(f"Weight labels: {weights}")
    print(f"Baseline: {BASELINE}")
    print()

    metrics = ["goal_completion_3d","work_completed","area_collisions","rework_p95_wait"]
    med = {m: median_by(rows, ["scenario","weights_label"], m) for m in metrics}

    # Per-scenario winner on goal_completion_3d (tie-break work_completed).
    print("=== Per-scenario winner on goal_completion_3d (kerf policy, median across 3 seeds) ===")
    print(f"{'scenario':<28} {'best':<8} {'g3d':>6} {'wc':>6}  | baseline g3d={'g3d_b':>5} wc_b={'wc_b':>5}")
    wins_count = defaultdict(int)
    for s in scenarios:
        best = None
        for w in weights:
            g = med["goal_completion_3d"].get((s,w))
            wc = med["work_completed"].get((s,w))
            if g is None: continue
            key = (g, wc if wc is not None else 0)
            if best is None or key > best[0]:
                best = (key, w)
        gb = med["goal_completion_3d"].get((s,BASELINE))
        wcb = med["work_completed"].get((s,BASELINE))
        if best:
            wins_count[best[1]] += 1
            print(f"{s:<28} {best[1]:<8} {best[0][0]:>6.0f} {best[0][1]:>6.0f}  |        {gb if gb is not None else 'NA':>5} {wcb if wcb is not None else 'NA':>5}")
    print()
    print("Wins-per-weight-combo on g3d:", dict(wins_count))
    print()

    # Domination test: for each non-baseline weight combo, count scenarios where
    # (g3d strictly greater than baseline) OR (equal g3d AND wc >= baseline wc).
    print("=== Domination over baseline (m5_r15) per weight combo, policy=kerf ===")
    print(f"{'weights':<8} {'dom%':>6} {'ties':>5} {'losses':>7} {'violations':>11}")
    rule1_pass = []
    for w in weights:
        if w == BASELINE: continue
        dom = ties = loss = 0
        viol = 0
        viol_detail = []
        for s in scenarios:
            g_new = med["goal_completion_3d"].get((s,w))
            g_base = med["goal_completion_3d"].get((s,BASELINE))
            wc_new = med["work_completed"].get((s,w))
            wc_base = med["work_completed"].get((s,BASELINE))
            if g_new is None or g_base is None: continue
            if g_new > g_base: dom += 1
            elif g_new == g_base:
                if (wc_new or 0) > (wc_base or 0): dom += 1
                elif (wc_new or 0) == (wc_base or 0): ties += 1
                else: loss += 1
            else: loss += 1
            # >5% loss check on the four metrics
            for m in metrics:
                new = med[m].get((s,w))
                base = med[m].get((s,BASELINE))
                if new is None or base is None: continue
                if m in HIB:
                    pc = pct_change(new, base)
                    if pc < -5.0:
                        viol += 1
                        viol_detail.append((s,m,base,new,pc))
                else:  # LIB
                    pc = pct_change(new, base)
                    if pc > 5.0:
                        viol += 1
                        viol_detail.append((s,m,base,new,pc))
        total_scn = dom + ties + loss
        dom_pct = 100.0 * dom / total_scn if total_scn else 0
        print(f"{w:<8} {dom_pct:>5.1f}% {ties:>5} {loss:>7} {viol:>11}")
        if dom_pct >= 60.0 and viol == 0:
            rule1_pass.append((w, dom_pct, viol_detail))
        elif dom_pct >= 60.0:
            print(f"    (>=60% dom but {viol} violations: {viol_detail[:5]})")
    print()
    print("=== Combos meeting both rules ===")
    if not rule1_pass:
        print("  NONE. Default m5_r15 retained.")
    else:
        # pick largest median improvement in g3d
        best = None
        for w, dom_pct, _ in rule1_pass:
            diffs = []
            for s in scenarios:
                g_new = med["goal_completion_3d"].get((s,w))
                g_base = med["goal_completion_3d"].get((s,BASELINE))
                if g_new is not None and g_base is not None:
                    diffs.append(g_new - g_base)
            mi = statistics.median(diffs) if diffs else 0
            print(f"  {w}: dom={dom_pct:.1f}%, median g3d improvement = {mi:+.1f}")
            if best is None or mi > best[1]:
                best = (w, mi)
        print(f"\n  WINNER: {best[0]} (median +{best[1]:.1f} on goal_completion_3d)")

    # Bonus: also show overall median across scenarios for each combo.
    print("\n=== Overall (median across scenarios) per weight combo, policy=kerf ===")
    print(f"{'weights':<8} {'g3d':>8} {'wc':>8} {'col':>8} {'r_p95':>8}")
    for w in weights:
        g = statistics.median([v for (s,ww),v in med["goal_completion_3d"].items() if ww==w])
        wc = statistics.median([v for (s,ww),v in med["work_completed"].items() if ww==w])
        col = statistics.median([v for (s,ww),v in med["area_collisions"].items() if ww==w])
        rp = statistics.median([v for (s,ww),v in med["rework_p95_wait"].items() if ww==w])
        marker = "  <-- baseline" if w == BASELINE else ""
        print(f"{w:<8} {g:>8.1f} {wc:>8.1f} {col:>8.1f} {rp:>8.1f}{marker}")

if __name__ == "__main__":
    main()
