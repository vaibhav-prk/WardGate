import os
from parse_results import parse_k6_json
from plot_figures import generate_comparison_plot

RESULTS_DIR = os.path.join("benchmarks", "results")
PLOTS_DIR = os.path.join("analysis", "plots")
os.makedirs(PLOTS_DIR, exist_ok=True)

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

    generate_comparison_plot(scenarios, labels, metrics, PLOTS_DIR)

if __name__ == "__main__":
    main()