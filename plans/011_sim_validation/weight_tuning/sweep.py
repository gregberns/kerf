#!/usr/bin/env python3
"""Plan 011 / D weight-tuning sweep driver.

Runs the 9 (momentum, rework) weight combos across 16 scenarios x 3 seeds.
Each kerfsim invocation already covers all 4 policies and 3 seeds.

Parallelism: scenarios x weights = 144 jobs; run with a small pool.
"""
import csv
import json
import os
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

ROOT = Path("/Users/gb/github/kerf")
WT   = ROOT / "plans/011_sim_validation/weight_tuning"
RUNS = WT / "runs"
WEIGHTS_DIR = WT / "weights"

SCENARIOS = []
# Adversarial (5)
for n in ["adv-area-collisions","adv-cascade-chain","adv-fanout-trap","adv-momentum-lock","adv-rework-swamp"]:
    SCENARIOS.append((n, ROOT / f"plans/011_sim_validation/scenarios/adversarial/{n}.yaml"))
# Exploratory saturated (4: s1, s4, s5, s7)
for n, fn in [
    ("s1_rework_storm_sat",     "s1_rework_storm_sat.yaml"),
    ("s4_area_collisions_sat",  "s4_area_collisions_sat.yaml"),
    ("s5_asymmetric_sizes_sat", "s5_asymmetric_sizes_sat.yaml"),
    ("s7_diamond_layers_sat",   "s7_diamond_layers_sat.yaml"),
]:
    SCENARIOS.append((n, ROOT / f"plans/011_sim_validation/scenarios/exploratory/{fn}"))
# Real-corpus per-pilot (7; skip all_pilots)
for n in ["cp","hc","on","pl","rc","sh","wm"]:
    SCENARIOS.append((n, ROOT / f"plans/012_real_corpus/scenarios/{n}.yaml"))

WEIGHT_COMBOS = [(m, r) for m in (0,2,5) for r in (5,10,15)]
POLICIES = ["kerf","random","fifo-bead","fifo-work"]
SEEDS_PER_RUN = 3

def label(m,r): return f"m{m}_r{r}"

def run_one(scn_name, scn_path, m, r):
    lbl = label(m,r)
    out = RUNS / f"{scn_name}__{lbl}"
    wpath = WEIGHTS_DIR / f"weights_{lbl}.yaml"
    if out.exists():
        # idempotent skip
        return (scn_name, lbl, "skip-exists", None)
    cmd = ["kerfsim","run",str(scn_path),"--weights",str(wpath),
           "--runs",str(SEEDS_PER_RUN),"--out",str(out),"--quiet"]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=900, cwd=str(ROOT))
        if proc.returncode != 0:
            return (scn_name, lbl, "fail", proc.stderr[-400:])
        return (scn_name, lbl, "ok", None)
    except subprocess.TimeoutExpired:
        return (scn_name, lbl, "timeout", None)
    except Exception as e:
        return (scn_name, lbl, "exc", str(e))

def collect():
    """Walk runs/, read each policy/summary.json, build CSV rows."""
    rows = []
    for scn_name, _ in SCENARIOS:
        for m,r in WEIGHT_COMBOS:
            lbl = label(m,r)
            base = RUNS / f"{scn_name}__{lbl}"
            if not base.is_dir(): continue
            # seed_N subdirs
            for seed_dir in sorted(base.glob("seed_*")):
                seed = int(seed_dir.name.split("_")[1])
                for pol in POLICIES:
                    sj = seed_dir / pol / "summary.json"
                    if not sj.exists(): continue
                    try:
                        d = json.loads(sj.read_text())
                        full = d.get("full", {})
                        rows.append({
                            "scenario": scn_name,
                            "weights_label": lbl,
                            "seed": seed,
                            "policy": pol,
                            "work_completed": full.get("work_completed"),
                            "goal_completion_3d": full.get("goal_completion_3d"),
                            "area_collisions": full.get("area_collisions"),
                            "rework_p95_wait": full.get("rework_p95_wait"),
                            "top_of_queue_churn": full.get("top_of_queue_churn"),
                            "agent_idle_pct": full.get("agent_idle_pct"),
                            "priority_inversions": full.get("priority_inversions"),
                        })
                    except Exception as e:
                        print(f"WARN: parse {sj}: {e}", file=sys.stderr)
    return rows

def main():
    RUNS.mkdir(parents=True, exist_ok=True)
    jobs = [(s,p,m,r) for s,p in SCENARIOS for m,r in WEIGHT_COMBOS]
    print(f"Total jobs: {len(jobs)} (scenarios={len(SCENARIOS)}, weight-combos={len(WEIGHT_COMBOS)})")
    results = []
    with ThreadPoolExecutor(max_workers=8) as ex:
        futs = {ex.submit(run_one, *j): j for j in jobs}
        done = 0
        for f in as_completed(futs):
            r = f.result()
            results.append(r)
            done += 1
            status = r[2]
            if status not in ("ok","skip-exists"):
                print(f"[{done}/{len(jobs)}] {r[0]} {r[1]} {status} {r[3] or ''}")
            elif done % 10 == 0:
                print(f"[{done}/{len(jobs)}] progress... last={r[0]} {r[1]} {status}")
    fails = [r for r in results if r[2] not in ("ok","skip-exists")]
    print(f"Done. ok={sum(1 for r in results if r[2]=='ok')} skip={sum(1 for r in results if r[2]=='skip-exists')} fail={len(fails)}")
    for f in fails:
        print(" FAIL:", f)

    # Collate CSV
    rows = collect()
    csv_path = WT / "sweep_results.csv"
    with csv_path.open("w", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=[
            "scenario","weights_label","seed","policy",
            "work_completed","goal_completion_3d","area_collisions",
            "rework_p95_wait","top_of_queue_churn","agent_idle_pct",
            "priority_inversions",
        ])
        w.writeheader()
        for r in rows: w.writerow(r)
    print(f"Wrote {csv_path} with {len(rows)} rows.")

if __name__ == "__main__":
    main()
