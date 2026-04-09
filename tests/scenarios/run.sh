#!/usr/bin/env bash
# Scenario test harness for kerf
# Usage: ./run.sh <scenario-script>
#
# Sets up an isolated environment (temp HOME for bench, temp git repo)
# then runs the scenario script inside it.

set -euo pipefail

SCENARIO="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
test -f "$SCENARIO" || { echo "Error: scenario not found: $1"; exit 1; }
KERF_BIN="${KERF_BIN:-$(cd "$(dirname "$0")/../.." && go build -o /tmp/kerf-test-bin . && echo /tmp/kerf-test-bin)}"

# Create isolated environment
export TEST_HOME=$(mktemp -d)
export TEST_REPO=$(mktemp -d)
export HOME="$TEST_HOME"
# Ensure 'kerf' is in PATH (symlink the built binary)
KERF_BINDIR=$(mktemp -d)
ln -s "$KERF_BIN" "$KERF_BINDIR/kerf"
export PATH="$KERF_BINDIR:$PATH"

# Initialize a git repo for the scenario
git init "$TEST_REPO" >/dev/null 2>&1
cd "$TEST_REPO"
git commit --allow-empty -m "init" >/dev/null 2>&1

echo "=== Scenario: $(basename "$SCENARIO" .sh) ==="
echo "    HOME=$TEST_HOME"
echo "    REPO=$TEST_REPO"
echo "    KERF=$KERF_BIN"
echo ""

# Run the scenario
PASS=0
FAIL=0
ERRORS=""

assert_pass() {
  local desc="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    PASS=$((PASS + 1))
    echo "  ✓ $desc"
  else
    FAIL=$((FAIL + 1))
    ERRORS="${ERRORS}\n  ✗ $desc: command failed: $*"
    echo "  ✗ $desc"
  fi
}

assert_fail() {
  local desc="$1"
  shift
  if ! "$@" >/dev/null 2>&1; then
    PASS=$((PASS + 1))
    echo "  ✓ $desc"
  else
    FAIL=$((FAIL + 1))
    ERRORS="${ERRORS}\n  ✗ $desc: expected failure but succeeded: $*"
    echo "  ✗ $desc"
  fi
}

assert_file_exists() {
  local desc="$1"
  local path="$2"
  if [ -e "$path" ]; then
    PASS=$((PASS + 1))
    echo "  ✓ $desc"
  else
    FAIL=$((FAIL + 1))
    ERRORS="${ERRORS}\n  ✗ $desc: file not found: $path"
    echo "  ✗ $desc"
  fi
}

assert_file_contains() {
  local desc="$1"
  local path="$2"
  local pattern="$3"
  if grep -q "$pattern" "$path" 2>/dev/null; then
    PASS=$((PASS + 1))
    echo "  ✓ $desc"
  else
    FAIL=$((FAIL + 1))
    ERRORS="${ERRORS}\n  ✗ $desc: '$pattern' not found in $path"
    echo "  ✗ $desc"
  fi
}

assert_output_contains() {
  local desc="$1"
  local pattern="$2"
  shift 2
  local output
  output=$("$@" 2>&1) || true
  if echo "$output" | grep -q "$pattern"; then
    PASS=$((PASS + 1))
    echo "  ✓ $desc"
  else
    FAIL=$((FAIL + 1))
    ERRORS="${ERRORS}\n  ✗ $desc: '$pattern' not in output of: $*"
    echo "  ✗ $desc"
  fi
}

# Export helpers for scenario scripts
export -f assert_pass assert_fail assert_file_exists assert_file_contains assert_output_contains
export KERF_BIN TEST_HOME TEST_REPO PASS FAIL ERRORS

# Source the scenario (runs in this shell to access helpers)
source "$SCENARIO"

# Report
echo ""
echo "--- Results: $PASS passed, $FAIL failed ---"
if [ $FAIL -gt 0 ]; then
  echo -e "$ERRORS"
  exit 1
fi

# Cleanup
rm -rf "$TEST_HOME" "$TEST_REPO"
