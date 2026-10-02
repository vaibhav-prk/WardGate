import os
import json
import glob
import pandas as pd

RESULTS_DIR = "../benchmarks/results"

def parse_k6_json(filepath):
    """Parses k6 metric JSON output."""
    records = []
    if not os.path.exists(filepath):
        return records
        
    with open(filepath, 'r') as f:
        for line in f:
            try:
                data = json.loads(line)
                # k6 json exports contain HTTP request metrics under 'http_req_duration'
                if data.get("type") == "Point" and data.get("metric") == "http_req_duration":
                    records.append({
                        "metric": "http_req_duration",
                        "value": data["data"]["value"],  # latency in ms
                        "timestamp": data["data"]["time"]
                    })
            except json.JSONDecodeError:
                continue
    return records

def parse_all_results():
    parsed_data = []
    
    # 1. Parse k6 JSON results (S1, S2, S8)
    for mode in ["static", "adaptive"]:
        for scenario in ["s1", "s2", "s8"]:
            filename = f"{scenario}_{mode}.json"
            filepath = os.path.join(RESULTS_DIR, filename)
            
            records = parse_k6_json(filepath)
            for r in records:
                r.update({"scenario": scenario, "mode": mode})
                parsed_data.append(r)
                
    # 2. Parse Attack Log results (S3 - S7)
    for mode in ["static", "adaptive"]:
        for scenario in ["s3", "s4", "s5", "s6", "s7"]:
            filename = f"{scenario}_{mode}.log"
            filepath = os.path.join(RESULTS_DIR, filename)
            
            if os.path.exists(filepath):
                with open(filepath, "r") as f:
                    for line in f:
                        parsed_data.append({
                            "scenario": scenario,
                            "mode": mode,
                            "metric": "attack_log",
                            "raw_log": line.strip()
                        })

    df = pd.DataFrame(parsed_data)
    os.makedirs("data", exist_ok=True)
    df.to_csv("data/parsed_results.csv", index=False)
    print(f"Parsed {len(df)} records into analysis/data/parsed_results.csv")
    return df

if __name__ == "__main__":
    parse_all_results()