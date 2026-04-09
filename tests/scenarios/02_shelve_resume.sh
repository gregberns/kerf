# Scenario 2: Shelve and resume workflow
# Tests: session tracking, shelve, resume, status preservation

echo "--- Setup: Create work and advance ---"

assert_pass "set default_jig" kerf config default_jig plan
assert_pass "create work" kerf new shelve-test --title "Shelve Test"
assert_pass "advance to analyze" kerf status shelve-test analyze
assert_pass "advance to decompose" kerf status shelve-test decompose

echo ""
echo "--- Phase 1: Shelve ---"

assert_pass "kerf shelve succeeds" kerf shelve shelve-test

# Work should still appear in list
assert_output_contains "shelved work in list" "shelve-test" kerf list

echo ""
echo "--- Phase 2: Resume ---"

assert_pass "kerf resume succeeds" kerf resume shelve-test

# Show should work after resume
assert_output_contains "show works after resume" "shelve-test" kerf show shelve-test

# Status should still be decompose (preserved across shelve/resume)
assert_output_contains "status preserved as decompose" "decompose" kerf status shelve-test

echo ""
echo "--- Phase 3: Continue advancing ---"

assert_pass "advance to research" kerf status shelve-test research
assert_pass "advance to change-spec" kerf status shelve-test change-spec
assert_pass "advance to integration" kerf status shelve-test integration
assert_pass "advance to tasks" kerf status shelve-test tasks
assert_pass "advance to ready" kerf status shelve-test ready
