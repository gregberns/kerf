// E2E determinism test for kerfsim (Plan 007 / Bead 15 — kerf-e53).
//
// Verifies the central property from specs/simulator.md §Determinism:
//
//	Same scenario + same weights + same seed → byte-identical
//	summary.json and events.jsonl.
//
// The tests drive `kerfsim run` end-to-end via the cobra command's RunE
// function (runRun) and assert byte-equality of the resulting artifacts
// across the four Phase-1 policies (kerf, random, fifo-bead, fifo-work).
//
// Coverage:
//
//  1. Byte-identical determinism across two invocations of the same
//     scenario/seed, for the canned scenarios small-linear, wide-fanout,
//     and rework-heavy. Repeated under --runs 3 (the aggregate must also be
//     byte-identical).
//  2. Seed sensitivity: seeds 42 and 43 produce at least one differing
//     events.jsonl per scenario.
//  3. Cross-scenario diff smoke: `kerfsim diff` runs cleanly against
//     directories produced by two different scenarios at the same seed.
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// policyDirs is the canonical set of policy subdirectories written by
// `kerfsim run`. Kept in sync with runOpts.policyNames.
var policyDirs = []string{"kerf", "random", "fifo-bead", "fifo-work"}

// runOnce invokes runRun for a single seed and returns the output directory.
func runOnce(t *testing.T, scenario string, seed int64, outDir string) {
	t.Helper()
	opts := &runOpts{
		runs:    1,
		format:  "text",
		outDir:  outDir,
		seed:    seed,
		seedSet: true,
		quiet:   true,
	}
	if err := runRun(&bytes.Buffer{}, scenario, opts); err != nil {
		t.Fatalf("runRun(%s seed=%d): %v", scenario, seed, err)
	}
}

// runMulti invokes runRun with --runs 3 and returns the output directory.
func runMulti(t *testing.T, scenario string, seed int64, runs int, outDir string) {
	t.Helper()
	opts := &runOpts{
		runs:    runs,
		format:  "text",
		outDir:  outDir,
		seed:    seed,
		seedSet: true,
		quiet:   true,
	}
	if err := runRun(&bytes.Buffer{}, scenario, opts); err != nil {
		t.Fatalf("runRun(%s seed=%d runs=%d): %v", scenario, seed, runs, err)
	}
}

// assertByteIdentical asserts that the two given files have identical bytes.
// Reports the path and lengths on mismatch to aid debugging.
func assertByteIdentical(t *testing.T, label, pathA, pathB string) {
	t.Helper()
	a, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatalf("%s: read %s: %v", label, pathA, err)
	}
	b, err := os.ReadFile(pathB)
	if err != nil {
		t.Fatalf("%s: read %s: %v", label, pathB, err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("%s: %s vs %s not byte-identical (len %d vs %d)", label, pathA, pathB, len(a), len(b))
	}
}

// TestE2E_Determinism_SameSeed_ByteIdentical is the load-bearing determinism
// contract: two runs of the same scenario with the same seed must produce
// byte-identical summary.json and events.jsonl for every policy.
func TestE2E_Determinism_SameSeed_ByteIdentical(t *testing.T) {
	scenarios := []string{"small-linear", "wide-fanout", "rework-heavy"}
	for _, sc := range scenarios {
		sc := sc
		t.Run(sc, func(t *testing.T) {
			tmp := t.TempDir()
			dirA := filepath.Join(tmp, "a")
			dirB := filepath.Join(tmp, "b")
			runOnce(t, sc, 42, dirA)
			runOnce(t, sc, 42, dirB)

			for _, p := range policyDirs {
				assertByteIdentical(t,
					sc+"/"+p+"/summary.json",
					filepath.Join(dirA, p, "summary.json"),
					filepath.Join(dirB, p, "summary.json"))
				assertByteIdentical(t,
					sc+"/"+p+"/events.jsonl",
					filepath.Join(dirA, p, "events.jsonl"),
					filepath.Join(dirB, p, "events.jsonl"))
			}
		})
	}
}

// TestE2E_Determinism_MultiRun_ByteIdentical verifies that --runs N is also
// deterministic: invoking the simulator twice with --runs 3 from the same
// base seed produces byte-identical per-seed subdirectories.
func TestE2E_Determinism_MultiRun_ByteIdentical(t *testing.T) {
	scenarios := []string{"small-linear", "wide-fanout", "rework-heavy"}
	for _, sc := range scenarios {
		sc := sc
		t.Run(sc, func(t *testing.T) {
			tmp := t.TempDir()
			dirA := filepath.Join(tmp, "a")
			dirB := filepath.Join(tmp, "b")
			runMulti(t, sc, 42, 3, dirA)
			runMulti(t, sc, 42, 3, dirB)

			for _, seedSub := range []string{"seed_42", "seed_43", "seed_44"} {
				for _, p := range policyDirs {
					assertByteIdentical(t,
						sc+"/"+seedSub+"/"+p+"/summary.json",
						filepath.Join(dirA, seedSub, p, "summary.json"),
						filepath.Join(dirB, seedSub, p, "summary.json"))
					assertByteIdentical(t,
						sc+"/"+seedSub+"/"+p+"/events.jsonl",
						filepath.Join(dirA, seedSub, p, "events.jsonl"),
						filepath.Join(dirB, seedSub, p, "events.jsonl"))
				}
			}
		})
	}
}

// TestE2E_Determinism_SeedSensitivity verifies that different seeds produce
// different output. A run at seed 42 and a run at seed 43 must differ in at
// least one events.jsonl per scenario (across the four policies).
func TestE2E_Determinism_SeedSensitivity(t *testing.T) {
	scenarios := []string{"small-linear", "wide-fanout", "rework-heavy"}
	for _, sc := range scenarios {
		sc := sc
		t.Run(sc, func(t *testing.T) {
			tmp := t.TempDir()
			dir42 := filepath.Join(tmp, "s42")
			dir43 := filepath.Join(tmp, "s43")
			runOnce(t, sc, 42, dir42)
			runOnce(t, sc, 43, dir43)

			anyDiff := false
			for _, p := range policyDirs {
				a, err := os.ReadFile(filepath.Join(dir42, p, "events.jsonl"))
				if err != nil {
					t.Fatalf("read seed42 %s: %v", p, err)
				}
				b, err := os.ReadFile(filepath.Join(dir43, p, "events.jsonl"))
				if err != nil {
					t.Fatalf("read seed43 %s: %v", p, err)
				}
				if !bytes.Equal(a, b) {
					anyDiff = true
				}
			}
			if !anyDiff {
				t.Errorf("scenario %s: seeds 42 and 43 produced identical events.jsonl across all policies — seed has no effect", sc)
			}
		})
	}
}

// TestE2E_CrossScenarioDiff is a light smoke test of the full
// run -> diff workflow: produce two run directories from different
// scenarios at the same seed, then exercise `kerfsim diff` across the
// kerf policy subdirectories. We only assert that the diff command exits
// without error and writes non-empty output; the exact metric values are
// covered by diff_test.go.
func TestE2E_CrossScenarioDiff(t *testing.T) {
	tmp := t.TempDir()
	dirA := filepath.Join(tmp, "A")
	dirB := filepath.Join(tmp, "B")
	runOnce(t, "small-linear", 42, dirA)
	runOnce(t, "wide-fanout", 42, dirB)

	var buf bytes.Buffer
	if err := runDiff(&buf, filepath.Join(dirA, "kerf"), filepath.Join(dirB, "kerf"), "text"); err != nil {
		t.Fatalf("runDiff: %v", err)
	}
	if buf.Len() == 0 {
		t.Errorf("runDiff produced no output")
	}
}
