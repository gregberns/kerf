#!/usr/bin/env python3
"""Fit candidate distributions to per-phase bead duration data.

Outputs:
- fitted_distributions.yaml
- distribution_fit_report.md
"""
from __future__ import annotations

import math
import os
from pathlib import Path

import numpy as np
import pandas as pd
import yaml
from scipy import stats

DATA_DIR = Path(__file__).resolve().parent
KERF = DATA_DIR / "kerf_beads.csv"
HARM = DATA_DIR / "harmonik_beads.csv"
CONFLICT = DATA_DIR / "conflict_incidents.csv"


# ---------------------------------------------------------------- fitting ---


CANDIDATES = {
    # name -> (scipy dist, fit kwargs forcing loc=0)
    "lognormal": (stats.lognorm, dict(floc=0)),
    "gamma": (stats.gamma, dict(floc=0)),
    "weibull": (stats.weibull_min, dict(floc=0)),
    "exponential": (stats.expon, dict(floc=0)),
}


def fit_one(name, data):
    dist, kw = CANDIDATES[name]
    params = dist.fit(data, **kw)
    # log-likelihood -> AIC
    ll = np.sum(dist.logpdf(data, *params))
    k = len([p for p in params if p != 0])  # rough
    aic = 2 * k - 2 * ll
    ks_stat, ks_p = stats.kstest(data, dist.cdf, args=params)
    return {"family": name, "params": params, "ks_p": float(ks_p), "ks_stat": float(ks_stat), "aic": float(aic)}


def best_fit(data, families=None):
    """Fit candidates; pick best by KS p-value, tiebreak lowest AIC."""
    families = families or list(CANDIDATES.keys())
    data = np.asarray(data, dtype=float)
    data = data[np.isfinite(data)]
    # need strictly positive for most candidates; treat zeros separately later
    pos = data[data > 0]
    results = []
    for fam in families:
        try:
            results.append(fit_one(fam, pos))
        except Exception as e:
            results.append({"family": fam, "error": str(e)})
    ok = [r for r in results if "error" not in r]
    ok.sort(key=lambda r: (-r["ks_p"], r["aic"]))
    return ok[0], ok


def params_to_yaml(family, params):
    if family == "lognormal":
        s, loc, scale = params
        return {"mu": float(np.log(scale)), "sigma": float(s)}
    if family == "gamma":
        a, loc, scale = params
        return {"shape": float(a), "scale": float(scale)}
    if family == "weibull":
        c, loc, scale = params
        return {"shape": float(c), "scale": float(scale)}
    if family == "exponential":
        loc, scale = params
        return {"rate": float(1.0 / scale)}
    return {"raw": list(map(float, params))}


def describe(data):
    a = np.asarray(data, dtype=float)
    a = a[np.isfinite(a)]
    if len(a) == 0:
        return {"n": 0}
    return {
        "n": int(len(a)),
        "mean": float(np.mean(a)),
        "median": float(np.median(a)),
        "p95": float(np.percentile(a, 95)),
        "max": float(np.max(a)),
        "zeros": int(np.sum(a == 0)),
    }


# -------------------------------------------------------------- mixtures ---


def fit_2comp_lognormal_em(data, n_iter=200, seed=0):
    """EM for a 2-component lognormal mixture."""
    rng = np.random.default_rng(seed)
    x = np.asarray(data, dtype=float)
    x = x[(x > 0) & np.isfinite(x)]
    lx = np.log(x)
    # init: split by median
    med = np.median(lx)
    mu = np.array([np.mean(lx[lx <= med]), np.mean(lx[lx > med])])
    sig = np.array([np.std(lx[lx <= med]) + 1e-3, np.std(lx[lx > med]) + 1e-3])
    w = np.array([0.5, 0.5])
    for _ in range(n_iter):
        # E-step
        p1 = w[0] * stats.norm.pdf(lx, mu[0], sig[0])
        p2 = w[1] * stats.norm.pdf(lx, mu[1], sig[1])
        denom = p1 + p2 + 1e-300
        r1 = p1 / denom
        r2 = p2 / denom
        # M-step
        n1 = r1.sum()
        n2 = r2.sum()
        w = np.array([n1 / len(lx), n2 / len(lx)])
        mu = np.array([(r1 * lx).sum() / n1, (r2 * lx).sum() / n2])
        sig = np.array([
            math.sqrt((r1 * (lx - mu[0]) ** 2).sum() / n1) + 1e-6,
            math.sqrt((r2 * (lx - mu[1]) ** 2).sum() / n2) + 1e-6,
        ])
    # KS vs mixture CDF
    def mix_cdf(v):
        return w[0] * stats.lognorm.cdf(v, s=sig[0], scale=math.exp(mu[0])) + \
               w[1] * stats.lognorm.cdf(v, s=sig[1], scale=math.exp(mu[1]))
    ks_stat, ks_p = stats.kstest(x, mix_cdf)
    # order components by location
    order = np.argsort(mu)
    return {
        "weights": [float(w[i]) for i in order],
        "mus": [float(mu[i]) for i in order],
        "sigmas": [float(sig[i]) for i in order],
        "ks_p": float(ks_p),
        "ks_stat": float(ks_stat),
        "n": int(len(x)),
    }


# ------------------------------------------------------------------ main ---


def main():
    kerf = pd.read_csv(KERF)
    harm = pd.read_csv(HARM)
    conflict = pd.read_csv(CONFLICT)

    phases = {}

    # 1) spin_up combined --------------------------------------------------
    spin = pd.concat([kerf["spin_up_seconds"], harm["spin_up_seconds"]]).dropna()
    # spin_up=0 is meaningful (dispatched-then-acted-immediately) but lognormal
    # needs positive. Drop exact zeros, document drop count.
    spin_pos = spin[spin > 0]
    spin_drops = int((spin == 0).sum())
    best, _ = best_fit(spin_pos)
    phases["spin_up"] = {
        "describe": describe(spin),
        "drops_zero_for_fit": spin_drops,
        "best": best,
    }

    # 2) task_work combined + variants ------------------------------------
    tw_all = pd.concat([kerf["task_work_seconds"], harm["task_work_seconds"]]).dropna()
    tw_all = tw_all[tw_all > 0]
    best_tw, _ = best_fit(tw_all)
    best_tw_kerf, _ = best_fit(kerf["task_work_seconds"].dropna()[kerf["task_work_seconds"].dropna() > 0])
    best_tw_harm, _ = best_fit(harm["task_work_seconds"].dropna()[harm["task_work_seconds"].dropna() > 0])
    phases["task_work"] = {
        "describe": describe(tw_all),
        "describe_kerf": describe(kerf["task_work_seconds"]),
        "describe_harmonik": describe(harm["task_work_seconds"]),
        "best": best_tw,
        "best_kerf": best_tw_kerf,
        "best_harmonik": best_tw_harm,
    }

    # 3) merge combined ---------------------------------------------------
    # Some merge values are negative (clock-skew clamping). Treat negatives as 0.
    merge_all = pd.concat([kerf["merge_seconds"], harm["merge_seconds"]]).dropna()
    merge_clamped = merge_all.clip(lower=0)
    # Bimodal: point mass at ~0 + lognormal on tail. Threshold: 1s.
    near_zero = merge_clamped[merge_clamped < 1.0]
    tail = merge_clamped[merge_clamped >= 1.0]
    w0 = len(near_zero) / len(merge_clamped)
    merge_desc = describe(merge_clamped)
    merge_single, _ = best_fit(merge_clamped[merge_clamped > 0])
    tail_fit, _ = best_fit(tail) if len(tail) >= 5 else (None, None)
    phases["merge"] = {
        "describe": merge_desc,
        "point_mass_weight": float(w0),
        "tail_n": int(len(tail)),
        "tail_fit": tail_fit,
        "single_fit": merge_single,
    }

    # 4) reviewer (kerf only) --------------------------------------------
    rev = kerf["reviewer_seconds"].dropna()
    rev_pos = rev[rev > 0]
    best_rev, _ = best_fit(rev_pos)
    phases["reviewer"] = {
        "describe": describe(rev),
        "best": best_rev,
        "note": "kerf only; harmonik recent sessions have no reviewer phase",
    }

    # 5) conflict resolution ---------------------------------------------
    cf = conflict[conflict["pattern"].isin([1, 2])]["duration_seconds"].dropna()
    cf_pos = cf[cf > 0]
    cf_desc = describe(cf)
    # Drop session-spanning outliers (>2h) -- these are upper-bound estimates
    # from "long session with conflict markers" detection, not true resolution
    # times. Document the count.
    CF_CAP = 7200.0
    cf_filtered = cf_pos[cf_pos <= CF_CAP]
    cf_dropped = int((cf_pos > CF_CAP).sum())
    cf_single, _ = best_fit(cf_filtered)
    cf_mix = fit_2comp_lognormal_em(cf_filtered)
    cf_mix["dropped_session_spanning"] = cf_dropped
    cf_mix["cap_seconds"] = CF_CAP
    phases["conflict_resolution"] = {
        "describe": cf_desc,
        "single_fit": cf_single,
        "mixture": cf_mix,
    }

    # ---------- emit YAML --------------------------------------------------
    out = {}

    out["spin_up"] = {
        "family": phases["spin_up"]["best"]["family"],
        "params": params_to_yaml(phases["spin_up"]["best"]["family"], phases["spin_up"]["best"]["params"]),
        "ks_p": phases["spin_up"]["best"]["ks_p"],
        "n": phases["spin_up"]["describe"]["n"],
        "source": "kerf+harmonik",
        "notes": f"dropped {phases['spin_up']['drops_zero_for_fit']} rows with spin_up=0 for fit; treat as point mass if needed",
    }

    out["task_work"] = {
        "family": phases["task_work"]["best"]["family"],
        "params": params_to_yaml(phases["task_work"]["best"]["family"], phases["task_work"]["best"]["params"]),
        "ks_p": phases["task_work"]["best"]["ks_p"],
        "n": phases["task_work"]["describe"]["n"],
        "source": "kerf+harmonik",
        "variants": {
            "kerf_only": {
                "family": phases["task_work"]["best_kerf"]["family"],
                "params": params_to_yaml(phases["task_work"]["best_kerf"]["family"], phases["task_work"]["best_kerf"]["params"]),
                "ks_p": phases["task_work"]["best_kerf"]["ks_p"],
                "n": phases["task_work"]["describe_kerf"]["n"],
            },
            "harmonik_only": {
                "family": phases["task_work"]["best_harmonik"]["family"],
                "params": params_to_yaml(phases["task_work"]["best_harmonik"]["family"], phases["task_work"]["best_harmonik"]["params"]),
                "ks_p": phases["task_work"]["best_harmonik"]["ks_p"],
                "n": phases["task_work"]["describe_harmonik"]["n"],
            },
        },
    }

    merge_components = [
        {"weight": phases["merge"]["point_mass_weight"], "family": "point_mass", "params": {"value": 0.0}},
    ]
    if phases["merge"]["tail_fit"] is not None:
        tail_best = phases["merge"]["tail_fit"]
        merge_components.append({
            "weight": 1.0 - phases["merge"]["point_mass_weight"],
            "family": tail_best["family"],
            "params": params_to_yaml(tail_best["family"], tail_best["params"]),
            "tail_ks_p": tail_best["ks_p"],
        })
    out["merge"] = {
        "family": "mixture",
        "components": merge_components,
        "n": phases["merge"]["describe"]["n"],
        "source": "kerf+harmonik (negatives clamped to 0)",
        "notes": "bimodal: most merges sub-second (FF); tail is lock/hook cluster ~10-300s",
    }

    out["reviewer"] = {
        "family": phases["reviewer"]["best"]["family"],
        "params": params_to_yaml(phases["reviewer"]["best"]["family"], phases["reviewer"]["best"]["params"]),
        "ks_p": phases["reviewer"]["best"]["ks_p"],
        "n": phases["reviewer"]["describe"]["n"],
        "notes": phases["reviewer"]["note"],
    }

    cm = phases["conflict_resolution"]["mixture"]
    out["conflict_resolution"] = {
        "family": "mixture",
        "components": [
            {
                "weight": cm["weights"][0],
                "family": "lognormal",
                "params": {"mu": cm["mus"][0], "sigma": cm["sigmas"][0]},
            },
            {
                "weight": cm["weights"][1],
                "family": "lognormal",
                "params": {"mu": cm["mus"][1], "sigma": cm["sigmas"][1]},
            },
        ],
        "ks_p": cm["ks_p"],
        "n": cm["n"],
        "source": "conflict_incidents.csv patterns 1+2",
        "notes": "two-component lognormal via EM; fast re-push cluster vs slow rebase cluster",
    }

    with open(DATA_DIR / "fitted_distributions.yaml", "w") as f:
        yaml.safe_dump(out, f, sort_keys=False)

    # ---------- emit report -----------------------------------------------
    def fmt(d):
        return f"n={d['n']}, mean={d['mean']:.1f}, median={d['median']:.1f}, p95={d['p95']:.1f}, max={d['max']:.1f}"

    sp = phases["spin_up"]
    tw = phases["task_work"]
    mg = phases["merge"]
    rv = phases["reviewer"]
    cf_ = phases["conflict_resolution"]

    lines = []
    lines.append("# Distribution Fit Report")
    lines.append("")
    lines.append("## Descriptive stats (seconds)")
    lines.append("")
    lines.append("| Phase | n | mean | median | p95 | max |")
    lines.append("|---|---:|---:|---:|---:|---:|")
    for label, d in [
        ("spin_up (combined)", sp["describe"]),
        ("task_work (combined)", tw["describe"]),
        ("task_work (kerf)", tw["describe_kerf"]),
        ("task_work (harmonik)", tw["describe_harmonik"]),
        ("merge (combined, clamped >=0)", mg["describe"]),
        ("reviewer (kerf only)", rv["describe"]),
        ("conflict_resolution (patterns 1+2)", cf_["describe"]),
    ]:
        lines.append(f"| {label} | {d['n']} | {d['mean']:.1f} | {d['median']:.1f} | {d['p95']:.1f} | {d['max']:.1f} |")
    lines.append("")
    lines.append("## Best-fit families")
    lines.append("")
    lines.append("| Phase | Family | Params | KS p |")
    lines.append("|---|---|---|---:|")

    def row(label, fam, params, ksp):
        p = params_to_yaml(fam, params)
        pstr = ", ".join(f"{k}={v:.3f}" for k, v in p.items())
        return f"| {label} | {fam} | {pstr} | {ksp:.3f} |"

    lines.append(row("spin_up (combined)", sp["best"]["family"], sp["best"]["params"], sp["best"]["ks_p"]))
    lines.append(row("task_work (combined)", tw["best"]["family"], tw["best"]["params"], tw["best"]["ks_p"]))
    lines.append(row("task_work (kerf)", tw["best_kerf"]["family"], tw["best_kerf"]["params"], tw["best_kerf"]["ks_p"]))
    lines.append(row("task_work (harmonik)", tw["best_harmonik"]["family"], tw["best_harmonik"]["params"], tw["best_harmonik"]["ks_p"]))
    if mg["tail_fit"]:
        lines.append(row(
            f"merge tail (weight={1-mg['point_mass_weight']:.2f})",
            mg["tail_fit"]["family"], mg["tail_fit"]["params"], mg["tail_fit"]["ks_p"],
        ))
    lines.append(row("reviewer (kerf)", rv["best"]["family"], rv["best"]["params"], rv["best"]["ks_p"]))

    cm = cf_["mixture"]
    lines.append(
        f"| conflict_resolution (mixture) | 2-lognormal | "
        f"w1={cm['weights'][0]:.2f},mu1={cm['mus'][0]:.2f},s1={cm['sigmas'][0]:.2f}; "
        f"w2={cm['weights'][1]:.2f},mu2={cm['mus'][1]:.2f},s2={cm['sigmas'][1]:.2f} | "
        f"{cm['ks_p']:.3f} |"
    )

    lines.append("")
    lines.append("## Merge mixture")
    lines.append("")
    lines.append(
        f"- Point mass at 0: weight = {mg['point_mass_weight']:.2f} "
        f"(n={mg['describe']['n']-mg['tail_n']} with merge<1s)."
    )
    lines.append(
        f"- Tail (merge >= 1s): weight = {1-mg['point_mass_weight']:.2f}, "
        f"n={mg['tail_n']}, best-fit {mg['tail_fit']['family'] if mg['tail_fit'] else 'n/a'}."
    )

    lines.append("")
    lines.append("## Observations")
    lines.append("")
    tw_kerf_med = tw["describe_kerf"]["median"]
    tw_harm_med = tw["describe_harmonik"]["median"]
    lines.append(
        f"- **task_work is workflow-dependent.** kerf median = {tw_kerf_med:.0f}s vs "
        f"harmonik median = {tw_harm_med:.0f}s "
        f"({tw_harm_med/tw_kerf_med:.1f}x). Combined fit smooths this; use the variants "
        f"when modelling a specific workflow."
    )
    fast_med = math.exp(cm["mus"][0])
    slow_med = math.exp(cm["mus"][1])
    lines.append(
        f"- **conflict_resolution is clearly bimodal.** Fast component median ~{fast_med:.0f}s "
        f"(weight {cm['weights'][0]:.2f}); slow component median ~{slow_med:.0f}s "
        f"(weight {cm['weights'][1]:.2f}). Matches expected fast re-push vs rebase split."
    )
    lines.append(
        f"- **merge** is dominated by a sub-second point mass ({mg['point_mass_weight']*100:.0f}% of merges < 1s); "
        f"the tail captures lock contention / hook latency."
    )
    lines.append(
        f"- **reviewer** sample is tiny (n={rv['describe']['n']}, kerf only); fitted lognormal "
        "but treat as low-confidence. Harmonik recent sessions skip reviewer entirely."
    )
    lines.append(
        f"- **spin_up** is tight and lognormal-ish (median {sp['describe']['median']:.1f}s, p95 {sp['describe']['p95']:.1f}s); "
        f"dropped {sp['drops_zero_for_fit']} rows with spin_up=0 for the fit."
    )

    with open(DATA_DIR / "distribution_fit_report.md", "w") as f:
        f.write("\n".join(lines) + "\n")

    # also print key numbers
    print("Wrote fitted_distributions.yaml and distribution_fit_report.md")
    print(f"spin_up best: {sp['best']['family']} ks_p={sp['best']['ks_p']:.3f}")
    print(f"task_work best: {tw['best']['family']} ks_p={tw['best']['ks_p']:.3f}")
    print(f"task_work kerf: {tw['best_kerf']['family']} ks_p={tw['best_kerf']['ks_p']:.3f}")
    print(f"task_work harmonik: {tw['best_harmonik']['family']} ks_p={tw['best_harmonik']['ks_p']:.3f}")
    print(f"reviewer best: {rv['best']['family']} ks_p={rv['best']['ks_p']:.3f}")
    print(f"merge tail: {mg['tail_fit']['family'] if mg['tail_fit'] else 'n/a'}")
    print(f"conflict mixture ks_p: {cm['ks_p']:.3f}")


if __name__ == "__main__":
    main()
