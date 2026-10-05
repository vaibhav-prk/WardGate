import json
import numpy as np

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