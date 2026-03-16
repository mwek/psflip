#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

OUTPUT_FILE="tests/output"
CONFIG="tests/test_backend.yaml"

cleanup() {
  if [[ -n "${HUP_LOOP_PID:-}" ]]; then
    kill "$HUP_LOOP_PID" 2>/dev/null || true
  fi
  killall psflip 2>/dev/null || true
}
trap cleanup EXIT

# 1. Compile psflip and backend
echo "Compiling psflip and backend..."
make psflip
go build -o tests/mock_backend ./tests/backend

# 2. Spin up psflip in the background
./psflip -c "$CONFIG" &

# 3. Wait 5s and verify with curl
sleep 5
HTTP_CODE="$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/ || true)"
if [[ "$HTTP_CODE" != "200" ]]; then
  echo "Service not responding: got HTTP $HTTP_CODE (expected 200)"
  exit 1
fi
echo "Service is up (HTTP 200)."

# 4. HUP loop in background
(
  while true; do
    kill -HUP "$(cat tests/test_backend.pid)" 2>/dev/null || true
    sleep 1
  done
) &
HUP_LOOP_PID=$!

# 5. Run oha: 30s, 100 qps, 20 connections, write results to output
echo "Starting load test (60s, 1000 qps, 20 connections)..."
oha -z 60s -q 1000 -c 20 --no-tui -o "$OUTPUT_FILE" http://127.0.0.1:8000/

# 6. Parse output and verify 100% requests returned 200
if [[ ! -f "$OUTPUT_FILE" ]]; then
  echo "oha did not write output to $OUTPUT_FILE"
  exit 1
fi
if ! grep -qE 'Success rate:.*100\.?0?%|100\.00%' "$OUTPUT_FILE"; then
  echo "Expected 100% success rate. oha output:"
  cat "$OUTPUT_FILE"
  exit 1
fi
echo "All requests returned 200 (100% success)."

# 7. Cleanup is done by trap (kill HUP loop, killall psflip)
echo "Load test passed."
