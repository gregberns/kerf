# Distribution Fit Report

## Descriptive stats (seconds)

| Phase | n | mean | median | p95 | max |
|---|---:|---:|---:|---:|---:|
| spin_up (combined) | 202 | 3.4 | 3.4 | 5.0 | 7.4 |
| task_work (combined) | 202 | 423.9 | 235.6 | 1085.1 | 6559.3 |
| task_work (kerf) | 52 | 248.8 | 216.6 | 536.4 | 696.4 |
| task_work (harmonik) | 150 | 484.6 | 260.9 | 1204.9 | 6559.3 |
| merge (combined, clamped >=0) | 120 | 9.3 | 0.0 | 1.6 | 362.5 |
| reviewer (kerf only) | 24 | 97.7 | 93.4 | 139.1 | 146.1 |
| conflict_resolution (patterns 1+2) | 31 | 8048.8 | 159.0 | 55562.0 | 127610.0 |

## Best-fit families

| Phase | Family | Params | KS p |
|---|---|---|---:|
| spin_up (combined) | gamma | shape=14.671, scale=0.235 | 0.595 |
| task_work (combined) | lognormal | mu=5.561, sigma=0.877 | 0.473 |
| task_work (kerf) | lognormal | mu=5.332, sigma=0.626 | 0.943 |
| task_work (harmonik) | lognormal | mu=5.641, sigma=0.935 | 0.523 |
| merge tail (weight=0.05) | weibull | shape=1.423, scale=202.946 | 0.949 |
| reviewer (kerf) | lognormal | mu=4.547, sigma=0.268 | 0.724 |
| conflict_resolution (mixture) | 2-lognormal | w1=0.91,mu1=4.53,s1=1.98; w2=0.09,mu2=11.18,s2=0.47 | 0.989 |

## Merge mixture

- Point mass at 0: weight = 0.95 (n=114 with merge<1s).
- Tail (merge >= 1s): weight = 0.05, n=6, best-fit weibull.

## Observations

- **task_work is workflow-dependent.** kerf median = 217s vs harmonik median = 261s (1.2x). Combined fit smooths this; use the variants when modelling a specific workflow.
- **conflict_resolution is clearly bimodal.** Fast component median ~92s (weight 0.91); slow component median ~71835s (weight 0.09). Matches expected fast re-push vs rebase split.
- **merge** is dominated by a sub-second point mass (95% of merges < 1s); the tail captures lock contention / hook latency.
- **reviewer** sample is tiny (n=24, kerf only); fitted lognormal but treat as low-confidence. Harmonik recent sessions skip reviewer entirely.
- **spin_up** is tight and lognormal-ish (median 3.4s, p95 5.0s); dropped 0 rows with spin_up=0 for the fit.
