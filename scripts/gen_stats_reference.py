#!/usr/bin/env python3
"""
Generate statistical reference values for perfguard/internal/stats tests.
Uses scipy.stats and numpy for independent verification.
Output is written to testdata/stats/reference.json.
"""

import json
import math
import os
import numpy as np
import scipy.stats as st

def compute_case(name, base, head):
    base_arr = np.array(base, dtype=np.float64)
    head_arr = np.array(head, dtype=np.float64)

    base_med = float(np.median(base_arr))
    head_med = float(np.median(head_arr))

    base_mean = float(np.mean(base_arr))
    head_mean = float(np.mean(head_arr))

    base_std = float(np.std(base_arr, ddof=1)) if len(base) > 1 else 0.0
    head_std = float(np.std(head_arr, ddof=1)) if len(head) > 1 else 0.0

    base_cv = float((base_std / base_mean) * 100.0) if base_mean != 0 else 0.0
    head_cv = float((head_std / head_mean) * 100.0) if head_mean != 0 else 0.0

    delta_pct = float(((head_med - base_med) / base_med) * 100.0) if base_med != 0 else 0.0

    # Mann-Whitney U test (two-sided)
    mwu = st.mannwhitneyu(base_arr, head_arr, alternative='two-sided')

    return {
        "name": name,
        "base": base,
        "head": head,
        "base_median": round(base_med, 8),
        "head_median": round(head_med, 8),
        "base_mean": round(base_mean, 8),
        "head_mean": round(head_mean, 8),
        "base_stddev": round(base_std, 8),
        "head_stddev": round(head_std, 8),
        "base_cv": round(base_cv, 6),
        "head_cv": round(head_cv, 6),
        "delta_pct": round(delta_pct, 6),
        "u_statistic": float(mwu.statistic),
        "p_value": float(mwu.pvalue),
    }

def main():
    cases = []

    # 1. Clear regression (+10% latency shift)
    base1 = [100.0, 102.0, 99.0, 101.0, 100.5, 98.5, 101.5, 100.0, 99.5, 101.0]
    head1 = [110.0, 112.5, 109.0, 111.0, 110.5, 108.5, 111.5, 110.0, 109.5, 112.0]
    cases.append(compute_case("clear_regression", base1, head1))

    # 2. No difference (A/A test)
    base2 = [50.1, 49.8, 50.3, 50.0, 49.9, 50.2, 50.0, 49.7, 50.4, 50.1]
    head2 = [50.0, 50.2, 49.9, 50.1, 50.0, 49.8, 50.3, 50.2, 49.9, 50.0]
    cases.append(compute_case("no_difference", base2, head2))

    # 3. With ties in values
    base3 = [10.0, 10.0, 12.0, 12.0, 14.0, 14.0, 16.0, 16.0]
    head3 = [12.0, 12.0, 14.0, 14.0, 16.0, 16.0, 18.0, 18.0]
    cases.append(compute_case("with_ties", base3, head3))

    # 4. Asymmetric sample sizes (N1 != N2)
    base4 = [20.0, 21.0, 19.5, 20.5, 20.0, 21.5]
    head4 = [22.0, 23.0, 22.5, 21.5, 23.5, 22.0, 24.0, 22.5, 23.0, 21.0]
    cases.append(compute_case("asymmetric_sizes", base4, head4))

    # 5. Improvement (-15% latency)
    base5 = [200.0, 205.0, 198.0, 202.0, 201.0, 199.0, 203.0, 200.0, 204.0, 197.0]
    head5 = [170.0, 172.0, 168.0, 171.0, 169.0, 173.0, 170.0, 167.0, 171.0, 169.0]
    cases.append(compute_case("clear_improvement", base5, head5))

    # 6. High variance / noise
    base6 = [10.0, 25.0, 12.0, 30.0, 15.0, 28.0, 11.0, 22.0, 14.0, 26.0]
    head6 = [12.0, 27.0, 14.0, 32.0, 17.0, 30.0, 13.0, 24.0, 16.0, 28.0]
    cases.append(compute_case("high_variance", base6, head6))

    script_dir = os.path.dirname(os.path.abspath(__file__))
    repo_root = os.path.dirname(script_dir)
    out_dir = os.path.join(repo_root, "testdata", "stats")
    os.makedirs(out_dir, exist_ok=True)

    out_file = os.path.join(out_dir, "reference.json")
    with open(out_file, "w", encoding="utf-8") as f:
        json.dump(cases, f, indent=2)

    print(f"Generated {len(cases)} reference cases to {out_file}")

if __name__ == "__main__":
    main()
