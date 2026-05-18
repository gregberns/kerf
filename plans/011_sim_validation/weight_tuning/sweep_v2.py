#!/usr/bin/env python3
"""Plan 011 / D follow-up: weight-tuning sweep on saturated all_pilots.

v1 (sweep.py) swept 9 (momentum, rework) combos x 16 scenarios x 3 seeds
and found no winner because the corpus was undersaturated. v2 narrows
to the single saturated scenario (all_pilots_sat) added in this round
and widens the grid to include fan_out.

Grid:
  momentum ∈ {2, 5, 10}
  rework   ∈ {5, 15, 30}
  fan_out  ∈ {5, 10, 20}
  creation = 0.1 (held)

27 combos x 3 seeds x 4 policies = 324 policy/seed datapoints, in one
kerfsim invocation per combo (kerfsim --runs sweeps seeds, and each
invocation runs all 4 policies on the same world). Wall time ~5 min.

Outputs:
  weights/v2/weights_m{M}_r{R}_f{F}.yaml   - generated weight files
  runs_v2/all_pilots_sat__m{M}_r{R}_f{F}/  - per-combo run dirs
  sweep_v2_results.csv                     - collated metrics
"""
import csv
import json
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

ROOT = Path("/Users/gb/github/kerf")
WT   = ROOT / "plans/011_sim_validation/weight_tuning"
RUNS = WT / "runs_v2"
WEIGHTS_DIR = WT / "weights" / "v2"

SCENARIO_NAME = "all_pilots_sat"
SCENARIO_PATH = ROOT / "plans/012_real_corpus/scenarios/all_pilots_sat.yaml"

MOMENTUMS = [2.0, 5.0, 10.0]
REWORKS   = [5.0, 15.0, 30.0]
FAN_OUTS  = [5.0, 10.0, 20.0]
CREATION  = 0.1

POLICIES = ["kerf", "random", "fifo-bead", "fifo-work"]
SEEDS_PER_RUN = 3

def label(m, r, f):
    def s(x): return str(int(x)) if x == int(x) else str(x).replace(".", "p")
    return f"m{s(m)}_r{s(r)}_f{s(f)}"

def write_weight_files():
    WEIGHTS_DIR.mkdir(parents=True, exist_ok=True)
    written = []
    for m in MOMENTUMS:
        for r in REWORKS:
            for f in FAN_OUTS:
                lbl = label(m, r, f)
                path = WEIGHTS_DIR / f"weights_{lbl}.yaml"
                content = (
                    f"momentum: {m}\n"
                    f"rework: {r}\n"
                    f"fan_out: {f}\n"
                    f"creation: {CREATION}\n"
                )
                path.write_text(content)
                written.append((lbl, path))
    return written

def run_one(lbl, wpath):
    out = RUNS / f"{SCENARIO_NAME}__{lbl}"
    if out.exists():
        return (lbl, "skip-exists", None)
    cmd = [
        "kerfsim", "run", str(SCENARIO_PATH),
        "--weights", str(wpath),
        "--runs", str(SEEDS_PER_RUN),
        "--out", str(out),
        "--quiet",
    ]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=900, cwd=str(ROOT))
        if proc.returncode != 0:
            return (lbl, "fail", proc.stderr[-400:])
        return (lbl, "ok", None)
    except subprocess.TimeoutExpired:
        return (lbl, "timeout", None)
    except Exception as e:
        return (lbl, "exc", str(e))

def collect():
    rows = []
    for m in MOMENTUMS:
        for r in REWORKS:
            for f in FAN_OUTS:
                lbl = label(m, r, f)
                base = RUNS / f"{SCENARIO_NAME}__{lbl}"
                if not base.is_dir():
                    continue
                for seed_dir in sorted(base.glob("seed_*")):
                    seed = int(seed_dir.name.split("_")[1])
                    for pol in POLICIES:
                        sj = seed_dir / pol / "summary.json"
                        if not sj.exists():
                            continue
                        try:
                            d = json.loads(sj.read_text())
                            full = d.get("full", {})
                            rows.append({
                                "scenario": SCENARIO_NAME,
                                "weights_label": lbl,
                                "momentum": m, "rework": r, "fan_out": f,
                                "seed": seed, "policy": pol,
                                "work_completed": full.get("work_completed"),
                                "goal_completion_3d": full.get("goal_completion_3d"),
                                "goal_completion_7d": full.get("goal_completion_7d"),
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
    combos = write_weight_files()
    print(f"Wrote {len(combos)} weight files under {WEIGHTS_DIR}")
    print(f"Total kerfsim invocations: {len(combos)} (each runs {SEEDS_PER_RUN} seeds x {len(POLICIES)} policies)")
    results = []
    with ThreadPoolExecutor(max_workers=8) as ex:
        futs = {ex.submit(run_one, lbl, wp): lbl for lbl, wp in combos}
        done = 0
        for fut in as_completed(futs):
            r = fut.result()
            results.append(r)
            done += 1
            if r[1] not in ("ok", "skip-exists"):
                print(f"[{done}/{len(combos)}] FAIL {r[0]}: {r[2]}")
            elif done % 5 == 0:
                print(f"[{done}/{len(combos)}] progress... last={r[0]} {r[1]}")
    ok = sum(1 for r in results if r[1] == "ok")
    skip = sum(1 for r in results if r[1] == "skip-exists")
    fail = [r for r in results if r[1] not in ("ok", "skip-exists")]
    print(f"Done. ok={ok} skip={skip} fail={len(fail)}")
    for f in fail:
        print(" FAIL:", f)

    rows = collect()
    csv_path = WT / "sweep_v2_results.csv"
    with csv_path.open("w", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=[
            "scenario", "weights_label", "momentum", "rework", "fan_out",
            "seed", "policy",
            "work_completed", "goal_completion_3d", "goal_completion_7d",
            "area_collisions", "rework_p95_wait", "top_of_queue_churn",
            "agent_idle_pct", "priority_inversions",
        ])
        w.writeheader()
        for r in rows:
            w.writerow(r)
    print(f"Wrote {csv_path} with {len(rows)} rows.")

if __name__ == "__main__":
    main()
