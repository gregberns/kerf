# Scenario 3: Config management + jig commands + alias resolution + multiple works
# Tests: config get/set, jig list/show, alias resolution, multiple works, archive, delete

echo "--- Phase 1: Config ---"

# Set default_jig first (required since default_jig is now unset)
assert_pass "set default_jig plan" kerf config default_jig plan

# Config with no args should show all keys
assert_output_contains "config shows keys" "default_jig" kerf config

# Set and get a config value
assert_pass "set snapshots.enabled" kerf config snapshots.enabled true
assert_output_contains "get snapshots.enabled" "true" kerf config snapshots.enabled

echo ""
echo "--- Phase 2: Jig commands ---"

# jig list should show available jigs (plan replaces feature, with alias)
assert_output_contains "jig list shows plan" "plan" kerf jig list
assert_output_contains "jig list shows bug" "bug" kerf jig list

# jig show should display a jig definition
assert_output_contains "jig show plan" "plan" kerf jig show plan
assert_output_contains "jig show bug" "bug" kerf jig show bug

echo ""
echo "--- Phase 2b: Alias resolution ---"

# Create work using alias "feature" — should resolve to canonical "plan"
assert_pass "create alias-test with --jig feature" kerf new alias-test --jig feature --title "Alias Test"

# spec.yaml should record canonical name "plan", not alias "feature"
PROJECT_ID=$(cat "$TEST_REPO/.kerf/project-identifier" 2>/dev/null)
ALIAS_WORK_DIR="$TEST_HOME/.kerf/projects/$PROJECT_ID/alias-test"
assert_file_contains "spec.yaml has canonical jig plan" "$ALIAS_WORK_DIR/spec.yaml" "jig: plan"

echo ""
echo "--- Phase 3: Multiple works ---"

assert_pass "create work A" kerf new work-alpha --title "Work Alpha"
assert_pass "create work B" kerf new work-beta --title "Work Beta"

# Both should appear in list
assert_output_contains "list shows alpha" "work-alpha" kerf list
assert_output_contains "list shows beta" "work-beta" kerf list

echo ""
echo "--- Phase 4: Archive and delete ---"

assert_pass "archive work-alpha" kerf archive work-alpha
# Archived work should NOT appear in regular list
LIST_OUTPUT=$(kerf list 2>&1)
if echo "$LIST_OUTPUT" | grep -q "work-alpha"; then
  echo "  ✗ archived work still in list"
  FAIL=$((FAIL + 1))
else
  echo "  ✓ archived work hidden from list"
  PASS=$((PASS + 1))
fi

# Delete work-beta
assert_pass "delete work-beta" kerf delete work-beta --yes
