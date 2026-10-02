import os
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

CSV_PATH = "data/parsed_results.csv"
OUTPUT_DIR = "../docs/figures"

def generate_plots():
    if not os.path.exists(CSV_PATH) or os.path.getsize(CSV_PATH) == 0:
        print("No benchmark data available to plot yet.")
        return

    try:
        df = pd.read_csv(CSV_PATH)
    except pd.errors.EmptyDataError:
        print("data/parsed_results.csv is empty or missing headers.")
        return

    if df.empty:
        print("data/parsed_results.csv contains no rows.")
        return

    os.makedirs(OUTPUT_DIR, exist_ok=True)
    sns.set_theme(style="whitegrid")

    k6_data = df[df["metric"] == "http_req_duration"]
    if not k6_data.empty:
        plt.figure(figsize=(9, 5))
        sns.barplot(
            data=k6_data,
            x="scenario",
            y="value",
            hue="mode",
            palette="Blues_d",
            errorbar=None
        )
        plt.title("Wardgate: Latency Overhead (Static vs. Adaptive)", fontsize=14, fontweight="bold")
        plt.xlabel("Scenario", fontsize=12)
        plt.ylabel("Latency (ms)", fontsize=12)
        plt.tight_layout()
        
        output_path = os.path.join(OUTPUT_DIR, "latency_comparison.png")
        plt.savefig(output_path, dpi=300)
        plt.close()
        print(f"Saved figure -> {output_path}")

if __name__ == "__main__":
    generate_plots()