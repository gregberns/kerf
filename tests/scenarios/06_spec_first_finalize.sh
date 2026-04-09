# Scenario 6: Spec-first finalization
# Tests: spec jig, spec_path config, spec drafts copied to spec_path on finalize

echo "--- Setup: Configure spec jig and spec_path ---"

assert_pass "set default_jig spec" kerf config default_jig spec
assert_pass "set spec_path" kerf config spec_path specs-test/

echo ""
echo "--- Phase 1: Create spec-first work ---"

assert_pass "create work with spec jig" kerf new spec-work --jig spec --title "Spec First Test"

# Commit the project-identifier file so working tree is clean for finalize
(cd "$TEST_REPO" && git add .kerf/project-identifier && git commit -m "add project-identifier" >/dev/null 2>&1)

PROJECT_ID=$(cat "$TEST_REPO/.kerf/project-identifier" 2>/dev/null)
WORK_DIR="$TEST_HOME/.kerf/projects/$PROJECT_ID/spec-work"

# Verify spec.yaml has jig: spec
assert_file_contains "spec.yaml has jig spec" "$WORK_DIR/spec.yaml" "jig: spec"

echo ""
echo "--- Phase 2: Create required artifacts ---"

# Create all required artifact files for the spec jig
echo "# Problem Space" > "$WORK_DIR/01-problem-space.md"
echo "# Components" > "$WORK_DIR/02-components.md"
echo "# Integration" > "$WORK_DIR/06-integration.md"
echo "# Tasks" > "$WORK_DIR/07-tasks.md"
echo "# Changelog" > "$WORK_DIR/05-changelog.md"
echo "# Session" > "$WORK_DIR/SESSION.md"

# Create spec drafts directory with a test spec
mkdir -p "$WORK_DIR/05-spec-drafts"
echo "# Test Spec" > "$WORK_DIR/05-spec-drafts/test-spec.md"
echo "" >> "$WORK_DIR/05-spec-drafts/test-spec.md"
echo "This is a test specification document." >> "$WORK_DIR/05-spec-drafts/test-spec.md"

echo ""
echo "--- Phase 3: Advance to ready ---"

# Advance through spec jig statuses
assert_pass "advance to decompose" kerf status spec-work decompose
assert_pass "advance to research" kerf status spec-work research
assert_pass "advance to change-design" kerf status spec-work change-design
assert_pass "advance to spec-draft" kerf status spec-work spec-draft
assert_pass "advance to integration" kerf status spec-work integration
assert_pass "advance to tasks" kerf status spec-work tasks
assert_pass "advance to ready" kerf status spec-work ready

echo ""
echo "--- Phase 4: Finalize ---"

assert_pass "finalize to branch" kerf finalize spec-work --branch kerf/spec-work

# Verify we're on the new branch
BRANCH=$(cd "$TEST_REPO" && git branch --show-current)
if [ "$BRANCH" = "kerf/spec-work" ]; then
  echo "  ✓ on branch kerf/spec-work"
  PASS=$((PASS + 1))
else
  echo "  ✗ expected branch kerf/spec-work, got: $BRANCH"
  FAIL=$((FAIL + 1))
fi

echo ""
echo "--- Phase 5: Verify spec-first finalization ---"

# Verify: spec drafts were copied to specs-test/ (spec_path)
assert_file_exists "spec draft in spec_path" "$TEST_REPO/specs-test/test-spec.md"

# Verify: finalize commit message contains "finalize"
COMMIT_MSG=$(cd "$TEST_REPO" && git log -1 --format=%s)
assert_output_contains "finalize commit message" "finalize" echo "$COMMIT_MSG"

# Verify: process artifacts were copied to repo_spec_path (.kerf/{codename}/)
assert_file_exists "artifacts in repo" "$TEST_REPO/.kerf/spec-work/01-problem-space.md"

# Verify: 05-spec-drafts/ was NOT copied to repo_spec_path (excluded for spec-first)
if [ -d "$TEST_REPO/.kerf/spec-work/05-spec-drafts" ]; then
  echo "  ✗ 05-spec-drafts/ should not be in repo_spec_path for spec-first works"
  FAIL=$((FAIL + 1))
else
  echo "  ✓ 05-spec-drafts/ excluded from repo_spec_path"
  PASS=$((PASS + 1))
fi
