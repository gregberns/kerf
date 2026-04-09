#!/usr/bin/env bash
# Run all scenario tests
set -uo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
KERF_BIN="${KERF_BIN:-$(cd "$DIR/../.." && go build -o /tmp/kerf-test-bin . && echo /tmp/kerf-test-bin)}"
export KERF_BIN

TOTAL_PASS=0
TOTAL_FAIL=0
FAILED_SCENARIOS=""

for scenario in "$DIR"/[0-9]*.sh; do
  echo ""
  if "$DIR/run.sh" "$scenario" 2>&1; then
    : # pass
  else
    FAILED_SCENARIOS="$FAILED_SCENARIOS $(basename "$scenario")"
  fi
  echo ""
done

echo "========================================"
if [ -z "$FAILED_SCENARIOS" ]; then
  echo "ALL SCENARIOS PASSED"
else
  echo "FAILED SCENARIOS:$FAILED_SCENARIOS"
  exit 1
fi
