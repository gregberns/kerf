# Scenario 4: Snapshot and restore
# Tests: named snapshots, restore, session preservation, history ordering

echo "--- Setup ---"

assert_pass "create work" kerf new snap-test --title "Snapshot Test"
assert_pass "advance to decomposition" kerf status snap-test decomposition

echo ""
echo "--- Phase 1: Take snapshots ---"

assert_pass "snapshot: first" kerf snapshot snap-test --name first-snap

# Advance and take another
assert_pass "advance to research" kerf status snap-test research
assert_pass "snapshot: second" kerf snapshot snap-test --name second-snap

echo ""
echo "--- Phase 2: History ---"

assert_output_contains "history shows first" "first-snap" kerf history snap-test
assert_output_contains "history shows second" "second-snap" kerf history snap-test

echo ""
echo "--- Phase 3: Restore ---"

# Get the full snapshot name (includes timestamp prefix)
FIRST_SNAP=$(kerf history snap-test 2>&1 | grep "first-snap" | awk '{print $1}')
echo "  ℹ Full snapshot name: $FIRST_SNAP"

if [ -n "$FIRST_SNAP" ]; then
  # Restore using the full snapshot name
  assert_pass "restore to first" kerf restore snap-test "$FIRST_SNAP"

  # Status should be back to decomposition
  assert_output_contains "status restored to decomposition" "decomposition" kerf status snap-test

  # History should now have a pre-restore snapshot
  assert_output_contains "pre-restore snapshot exists" "pre-restore" kerf history snap-test
else
  echo "  ✗ could not find first-snap in history"
  FAIL=$((FAIL + 3))
fi
