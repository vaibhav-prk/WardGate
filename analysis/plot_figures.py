import os
import numpy as np
import matplotlib.pyplot as plt

def generate_comparison_plot(scenarios, labels, metrics, plots_dir):
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

    fig, ax = plt.subplots(figsize=(10, 5.5), dpi=300)

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
    plot_file = os.path.join(plots_dir, "latency_profile.png")
    plt.savefig(plot_file)
    print(f"\nGenerated comparison plot: {plot_file}")