#!/usr/bin/env python3
"""Deeper inspection of the sweep — secondary metrics across weights."""
import csv, statistics
from collections import defaultdict
from pathlib import Path

WT = Path("/Users/gb/github/kerf/plans/011_sim_validation/weight_tuning")
CSV = WT / "sweep_results.csv"
BASELINE = "m5_r15"

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

def med(rows, scn, w, pol, metric):
    vs = [r[metric] for r in rows if r["scenario"]==scn and r["weights_label"]==w
          and r["policy"]==pol and r[metric] is not None]
    return statistics.median(vs) if vs else None

def main():
    rows = load()
    scenarios = sorted({r["scenario"] for r in rows})
    weights = ["m0_r5","m0_r10","m0_r15","m2_r5","m2_r10","m2_r15","m5_r5","m5_r10","m5_r15"]

    # Focused look: scenarios where rankings actually moved.
    interesting = ["adv-area-collisions","adv-momentum-lock","adv-rework-swamp",
                   "s1_rework_storm_sat","s4_area_collisions_sat","s5_asymmetric_sizes_sat",
                   "s7_diamond_layers_sat"]
    metrics = ["goal_completion_3d","work_completed","area_collisions","rework_p95_wait",
               "top_of_queue_churn","priority_inversions"]
    pol = "kerf"

    for scn in interesting:
        print(f"\n=== {scn} (policy=kerf, median over 3 seeds) ===")
        hdr = f"{'w':<8} " + " ".join(f"{m[:10]:>10}" for m in metrics)
        print(hdr)
        for w in weights:
            vals = [med(rows, scn, w, pol, m) for m in metrics]
            line = f"{w:<8} " + " ".join(f"{v if v is not None else 'NA':>10}" if isinstance(v,(int,float)) else f"{str(v):>10}" for v in vals)
            print(line.replace("None","   NA"))

    # Total work_completed across all scenarios per weight (sanity check throughput).
    print("\n=== Total work_completed across all scenarios (sum of medians), policy=kerf ===")
    for w in weights:
        total = sum(med(rows, s, w, pol, "work_completed") or 0 for s in scenarios)
        marker = "  <-- baseline" if w == BASELINE else ""
        print(f"  {w}: {total:.0f}{marker}")
    print("\n=== Total area_collisions ===")
    for w in weights:
        total = sum(med(rows, s, w, pol, "area_collisions") or 0 for s in scenarios)
        marker = "  <-- baseline" if w == BASELINE else ""
        print(f"  {w}: {total:.0f}{marker}")
    print("\n=== Total rework_p95_wait ===")
    for w in weights:
        total = sum(med(rows, s, w, pol, "rework_p95_wait") or 0 for s in scenarios)
        marker = "  <-- baseline" if w == BASELINE else ""
        print(f"  {w}: {total:.0f}{marker}")
    print("\n=== Total top_of_queue_churn ===")
    for w in weights:
        total = sum(med(rows, s, w, pol, "top_of_queue_churn") or 0 for s in scenarios)
        marker = "  <-- baseline" if w == BASELINE else ""
        print(f"  {w}: {total:.3f}{marker}")

if __name__ == "__main__":
    main()
