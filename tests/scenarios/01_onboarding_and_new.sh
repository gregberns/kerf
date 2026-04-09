# Scenario 1: Onboarding flow and creating a work
# Tests: onboarding error, kerf new, list, show, status, snapshot, history

echo "--- Phase 0: Onboarding check ---"

# kerf new without config or --jig should fail with onboarding message
ONBOARD_OUTPUT=$(kerf new onboard-fail 2>&1) || true
if echo "$ONBOARD_OUTPUT" | grep -q "No default workflow configured"; then
  echo "  ✓ onboarding error shown without config"
  PASS=$((PASS + 1))
else
  echo "  ✗ expected onboarding error, got: $(echo "$ONBOARD_OUTPUT" | head -1)"
  FAIL=$((FAIL + 1))
fi

# Set default jig to plan
assert_pass "set default_jig plan" kerf config default_jig plan

echo ""
echo "--- Phase 1: Create a work ---"

# kerf new should work now that default_jig is set
assert_pass "kerf new succeeds" kerf new test-work --title "Test Work"

# Verify bench was created
assert_file_exists "bench dir exists" "$TEST_HOME/.kerf"

# Verify project-identifier was written
assert_file_exists "project-identifier exists" "$TEST_REPO/.kerf/project-identifier"
PROJECT_ID=$(cat "$TEST_REPO/.kerf/project-identifier" 2>/dev/null || echo "")

if [ -n "$PROJECT_ID" ]; then
  WORK_DIR="$TEST_HOME/.kerf/projects/$PROJECT_ID/test-work"
  assert_file_exists "spec.yaml exists" "$WORK_DIR/spec.yaml"
  assert_file_contains "spec.yaml has codename" "$WORK_DIR/spec.yaml" "codename: test-work"
  assert_file_contains "spec.yaml has jig" "$WORK_DIR/spec.yaml" "jig:"
  assert_file_contains "spec.yaml has status" "$WORK_DIR/spec.yaml" "status:"
else
  echo "  ✗ no project-identifier found"
  FAIL=$((FAIL + 1))
fi

echo ""
echo "--- Phase 2: List and show ---"

assert_output_contains "kerf list shows work" "test-work" kerf list
assert_output_contains "kerf show displays codename" "test-work" kerf show test-work

echo ""
echo "--- Phase 3: Status ---"

# Read current status
CURRENT_STATUS=$(kerf status test-work 2>&1 | head -5)
echo "  ℹ Current status output: $(echo "$CURRENT_STATUS" | head -1)"

# Advance status (plan jig: problem-space → analyze)
assert_pass "kerf status advance" kerf status test-work analyze

echo ""
echo "--- Phase 4: Snapshot ---"

assert_pass "kerf snapshot" kerf snapshot test-work --name baseline
assert_output_contains "kerf history shows snapshot" "baseline" kerf history test-work
