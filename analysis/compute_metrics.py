import os
import pandas as pd

CSV_PATH = "data/parsed_results.csv"

def compute_metrics():
    if not os.path.exists(CSV_PATH) or os.path.getsize(CSV_PATH) == 0:
        print("No benchmark data available yet in data/parsed_results.csv.")
        return

    try:
        df = pd.read_csv(CSV_PATH)
    except pd.errors.EmptyDataError:
        print("data/parsed_results.csv is empty or missing headers.")
        return

    if df.empty:
        print("data/parsed_results.csv contains no rows.")
        return

    print("==================================================")
    print("           WARDGATE PERFORMANCE METRICS           ")
    print("==================================================")

    k6_data = df[df["metric"] == "http_req_duration"]
    if not k6_data.empty:
        print("\n--- LATENCY PERCENTILES (ms) BY SCENARIO & MODE ---")
        latency_summary = k6_data.groupby(['scenario', 'mode'])['value'].quantile([0.50, 0.95, 0.99]).unstack()
        latency_summary.columns = ['p50', 'p95', 'p99']
        print(latency_summary.round(2))

    attack_data = df[df["metric"] == "attack_log"]
    if not attack_data.empty:
        print("\n--- ATTACK LOG COUNT BY SCENARIO & MODE ---")
        attack_summary = attack_data.groupby(['scenario', 'mode']).size().unstack(fill_value=0)
        print(attack_summary)

    print("\n==================================================")

if __name__ == "__main__":
    compute_metrics()