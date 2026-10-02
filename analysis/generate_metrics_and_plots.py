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
    scenarios = ["s1", "s2", "s8"]
    labels = {"s1": "S1: Steady", "s2": "S2: Bursty", "s8": "S8: Volumetric"}
    modes = ["static", "adaptive"]

    metrics = {}
    print(f"\n{'Scenario':<22} | {'Mode':<9} | {'Total':<7} | {'Avg(ms)':<8} | {'p50(ms)':<8} | {'p95(ms)':<8} | {'p99(ms)':<8}")
    print("-" * 90)

    for s in scenarios:
        for mode in modes:
            filename = f"{s}_{mode}.json"
            path = os.path.join(RESULTS_DIR, filename)
            if not os.path.exists(path):
                print(f"{labels[s]:<22} | {mode:<9} | [File {filename} not found]")
                continue

            res = parse_k6_json(path)
            if not res:
                continue

            key = f"{labels[s]} ({mode})"
            metrics[key] = res
            print(f"{labels[s]:<22} | {mode:<9} | {res['count']:<7} | {res['mean']:<8.2f} | {res['p50']:<8.2f} | {res['p95']:<8.2f} | {res['p99']:<8.2f}")

    if not metrics:
        print("\nNo metric data available to generate plots.")
        return

    # Plot: grouped bars comparing static vs adaptive for each scenario
    fig, ax = plt.subplots(figsize=(10, 5.5), dpi=300)

    group_labels = []
    static_p50, static_p95, static_p99 = [], [], []
    adaptive_p50, adaptive_p95, adaptive_p99 = [], [], []

    for s in scenarios:
        label = labels[s]
        sk = f"{label} (static)"
        ak = f"{label} (adaptive)"
        if sk in metrics and ak in metrics:
            group_labels.append(label)
            static_p50.append(metrics[sk]["p50"])
            static_p95.append(metrics[sk]["p95"])
            static_p99.append(metrics[sk]["p99"])
            adaptive_p50.append(metrics[ak]["p50"])
            adaptive_p95.append(metrics[ak]["p95"])
            adaptive_p99.append(metrics[ak]["p99"])

    if not group_labels:
        print("\nNeed both static and adaptive results to generate comparison plot.")
        return

    x = np.arange(len(group_labels))
    width = 0.13

    ax.bar(x - 2.5*width, static_p50, width, label="Static p50", color="#1f77b4")
    ax.bar(x - 1.5*width, static_p95, width, label="Static p95", color="#ff7f0e")
    ax.bar(x - 0.5*width, static_p99, width, label="Static p99", color="#d62728")
    ax.bar(x + 0.5*width, adaptive_p50, width, label="Adaptive p50", color="#1f77b4", alpha=0.5)
    ax.bar(x + 1.5*width, adaptive_p95, width, label="Adaptive p95", color="#ff7f0e", alpha=0.5)
    ax.bar(x + 2.5*width, adaptive_p99, width, label="Adaptive p99", color="#d62728", alpha=0.5)

    ax.set_title("WardGate: Static vs Adaptive Latency Profile")
    ax.set_ylabel("Latency (ms)")
    ax.set_xticks(x)
    ax.set_xticklabels(group_labels, fontweight="bold")
    ax.grid(axis="y", linestyle="--", alpha=0.5)
    ax.legend(frameon=True, fontsize=8, ncol=2)

    plt.tight_layout()
    plot_file = os.path.join(PLOTS_DIR, "latency_profile.png")
    plt.savefig(plot_file)
    print(f"\nGenerated comparison plot: {plot_file}")

if __name__ == "__main__":
    main()

