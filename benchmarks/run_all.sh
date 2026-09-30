#!/bin/bash
set -e

RESULTS_DIR="benchmarks/results"
mkdir -p "$RESULTS_DIR"

echo "=== WARDGATE BENCHMARK SUITE ==="

for MODE in static adaptive; do
  echo ">>> Starting Gateway in MODE=$MODE..."
  LIMITER_MODE=$MODE docker compose -f deploy/docker-compose.yml up -d --build
  sleep 5

  echo "Running S1: Legit Steady..."
  k6 run --out json="$RESULTS_DIR/s1_${MODE}.json" benchmarks/scenarios/s1_legit_steady.js || true

  echo "Running S2: Legit Bursty (False Positive Check)..."
  k6 run --out json="$RESULTS_DIR/s2_${MODE}.json" benchmarks/scenarios/s2_legit_bursty.js || true

  echo "Running S3-S7: Attack Scenarios..."
  for SCENARIO in s3 s4 s5 s6 s7; do
    echo "  -> Executing $SCENARIO ($MODE)..."
    go run benchmarks/attackgen/main.go -scenario "$SCENARIO" -count 15 > "$RESULTS_DIR/${SCENARIO}_${MODE}.log" 2>&1 || true
  done

  echo "Running S8: Volumetric Overload..."
  k6 run --out json="$RESULTS_DIR/s8_${MODE}.json" benchmarks/scenarios/s8_volumetric_burst.js || true

  docker compose -f deploy/docker-compose.yml down
done

echo "=== BENCHMARKS COMPLETE. Output saved in $RESULTS_DIR ==="