import json
import os
import numpy as np
import matplotlib.pyplot as plt

RESULTS_DIR = os.path.join("benchmarks", "results")
PLOTS_DIR = os.path.join("analysis", "plots")
os.makedirs(PLOTS_DIR, exist_ok=True)

def parse_k6_json(filepath):
    durations = []
    statuses = {}
    
    with open(filepath, "r", encoding="utf-8") as f:
        for line in f:
            try:
                record = json.loads(line.strip())
                if record.get("type") == "Point" and record.get("metric") == "http_req_duration":
                    durations.append(record["data"]["value"])
                elif record.get("type") == "Point" and record.get("metric") == "http_reqs":
                    status = record["data"]["tags"].get("status")
                    if status:
                        statuses[status] = statuses.get(status, 0) + 1
            except Exception:
                continue

    if not durations:
        return None

    durations = np.array(durations)
    return {
        "count": len(durations),
        "mean": float(np.mean(durations)),
        "p50": float(np.percentile(durations, 50)),
        "p90": float(np.percentile(durations, 90)),
        "p95": float(np.percentile(durations, 95)),
        "p99": float(np.percentile(durations, 99)),
        "statuses": statuses
    }

def main():
    scenarios = {
        "S1: Steady": "s1_steady.json",
        "S2: Bursty": "s2_bursty.json",
        "S8: Volumetric": "s8_burst.json"
    }

    metrics = {}
    print(f"\n{'Scenario':<18} | {'Total Req':<10} | {'Avg (ms)':<9} | {'p50 (ms)':<9} | {'p95 (ms)':<9} | {'p99 (ms)':<9}")
    print("-" * 75)

    for label, filename in scenarios.items():
        path = os.path.join(RESULTS_DIR, filename)
        if not os.path.exists(path):
            print(f"{label:<18} | [File {filename} not found]")
            continue

        res = parse_k6_json(path)
        if not res:
            continue

        metrics[label] = res
        print(f"{label:<18} | {res['count']:<10} | {res['mean']:<9.2f} | {res['p50']:<9.2f} | {res['p95']:<9.2f} | {res['p99']:<9.2f}")

    if not metrics:
        print("\nNo metric data available to generate plots.")
        return

    labels = list(metrics.keys())
    p50_vals = [metrics[k]["p50"] for k in labels]
    p95_vals = [metrics[k]["p95"] for k in labels]
    p99_vals = [metrics[k]["p99"] for k in labels]

    x = np.arange(len(labels))
    width = 0.24

    fig, ax = plt.subplots(figsize=(8, 4.8), dpi=300)
    ax.bar(x - width, p50_vals, width, label="p50 (Median)", color="#1f77b4")
    ax.bar(x, p95_vals, width, label="p95", color="#ff7f0e")
    ax.bar(x + width, p99_vals, width, label="p99", color="#d62728")

    ax.set_title("WardGate Zero Trust Gateway Latency Profile")
    ax.set_ylabel("Latency (ms)")
    ax.set_xticks(x)
    ax.set_xticklabels(labels, fontweight="bold")
    ax.grid(axis="y", linestyle="--", alpha=0.5)
    ax.legend(frameon=True)

    for i in x:
        ax.text(i - width, p50_vals[i] + 0.1, f"{p50_vals[i]:.1f}", ha="center", fontsize=8)
        ax.text(i, p95_vals[i] + 0.1, f"{p95_vals[i]:.1f}", ha="center", fontsize=8)
        ax.text(i + width, p99_vals[i] + 0.1, f"{p99_vals[i]:.1f}", ha="center", fontsize=8)

    plt.tight_layout()
    plot_file = os.path.join(PLOTS_DIR, "latency_profile.png")
    plt.savefig(plot_file)
    print(f"\nGenerated benchmark plot: {plot_file}")

if __name__ == "__main__":
    main()
