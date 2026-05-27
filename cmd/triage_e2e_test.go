package cmd

// Plan 009 / Bead 12 — E2E triage loop test (kerf-xty).
//
// Drives the full triage-driven coordination loop end-to-end:
//
//   1. Seed a project with two works (`bridge`, `gateway`) and a project-wide
//      `bead_filter` of `label: "subsystem:{codename}"`.
//   2. Seed a stub bead store with: three matching beads per work, one
//      unmatched bead, and one bead matching BOTH works.
//   3. `kerf triage` (no flags) — Untriaged section lists the unmatched
//      bead; Multi-matched section lists the dual-match bead; per-work
//      health renders.
//   4. `kerf triage --resolved` — drift exists, exit 2.
//   5. Resolve the untriaged bead by creating a new work via
//      `kerf new <codename> --bead-filter 'label=<lbl>'`.
//   6. Resolve the multi-matched bead via `kerf pin bridge <id>`.
//   7. `kerf triage --ack` — advances the baseline.
//   8. `kerf triage --resolved` — exit 0.
//   9. `kerf show bridge` renders the pinned bead with `(pinned)` marker.
//  10. Externally close a bead (re-stub the store), without `--ack`:
//      `kerf show bridge` shows the `closed externally` drift marker.
//  11. Comment survival: a head comment on `bridge/spec.yaml` survives
//      `kerf pin` and `kerf work edit --bead-filter-add`.
//
// Spec references:
//   - specs/commands.md §"kerf triage" / §"kerf pin" / §"kerf work edit" /
//     §"kerf show" attached-beads block / §"kerf new" --bead-filter.
//   - specs/coordination.md §"Drift detection" / §"Pin layer" /
//     §"Baseline advancement".
//   - plans/009_triage/_plan.md §"Triage agent workflow (canonical)".

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/kerf/internal/testutil"
)

// TestE2E_Plan009_TriageLoop is the canonical Plan 009 / B12 scenario.
// Sub-tests gate each stage; the test fails fast on setup so later
// assertions don't run against a partially-populated project.
func TestE2E_Plan009_TriageLoop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	repo := testutil.SetupGitRepo(t)
	t.Chdir(repo)
	projectID := "triage-e2e-proj"

	// --- Bench-mode project skeleton -------------------------------------
	projDir := filepath.Join(tmp, ".kerf", "projects", projectID)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	// project.yaml carries the templated subsystem:{codename} filter so
	// each work matches its own subsystem-labeled beads without per-work
	// override (specs/coordination.md §"Filter resolution").
	projectYAML := "jigs: []\nbead_filter:\n  label: \"subsystem:{codename}\"\n"
	if err := os.WriteFile(filepath.Join(projDir, "project.yaml"), []byte(projectYAML), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}
	// Tie this git repo to the project so drift.CachePath resolves and
	// --ack can write the snapshot.
	if err := os.MkdirAll(filepath.Join(repo, ".kerf"), 0o755); err != nil {
		t.Fatalf("mkdir repo .kerf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kerf", "project-identifier"), []byte(projectID), 0o644); err != nil {
		t.Fatalf("write project-identifier: %v", err)
	}

	// --- Seed works bridge and gateway (no per-work bead_filter) ---------
	// writeSpecWithStatus from cmd/plan006_e2e_test.go writes a research
	// stage and emits an empty bead_filter, so each work inherits the
	// project-wide `subsystem:{codename}` template.
	writeSpecWithStatus(t, filepath.Join(projDir, "bridge", "spec.yaml"),
		"bridge", projectID, "research")
	writeSpecWithStatus(t, filepath.Join(projDir, "gateway", "spec.yaml"),
		"gateway", projectID, "research")

	// Prepend a head comment to bridge's spec.yaml so we can verify it
	// survives mutators (kerf pin, kerf work edit --bead-filter-add).
	bridgeSpec := filepath.Join(projDir, "bridge", "spec.yaml")
	const headComment = "# bridge: handles inbound webhook delivery\n"
	if err := prependHeadComment(bridgeSpec, headComment); err != nil {
		t.Fatalf("prepend head comment: %v", err)
	}

	// --- Seed bead store --------------------------------------------------
	// 3 beads per work via subsystem labels, 1 unmatched (legacy:cleanup),
	// 1 dual-match (both subsystem:bridge AND subsystem:gateway).
	seedStore := `[
		{"id":"br-1","title":"bridge one","status":"open","labels":["subsystem:bridge"]},
		{"id":"br-2","title":"bridge two","status":"open","labels":["subsystem:bridge"]},
		{"id":"br-3","title":"bridge three","status":"open","labels":["subsystem:bridge"]},
		{"id":"gw-1","title":"gateway one","status":"open","labels":["subsystem:gateway"]},
		{"id":"gw-2","title":"gateway two","status":"open","labels":["subsystem:gateway"]},
		{"id":"gw-3","title":"gateway three","status":"open","labels":["subsystem:gateway"]},
		{"id":"sh-1","title":"shared concern","status":"open","labels":["subsystem:bridge","subsystem:gateway"]},
		{"id":"un-1","title":"untriaged orphan","status":"open","labels":["subsystem:metrics"]}
	]`
	stubBr(t, seedStore)

	prevProject := projectFlag
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = prevProject })

	// --- Stage 1: kerf triage surfaces untriaged + multi-matched ---------
	t.Run("triage_initial_report", func(t *testing.T) {
		resetTriageFlags()
		t.Cleanup(resetTriageFlags)
		out, err := runTriageCapturing(t)
		if err != nil {
			t.Fatalf("kerf triage: %v", err)
		}
		// Untriaged section lists the orphan bead.
		testutil.AssertStringContains(t, out, "Untriaged beads (1):")
		testutil.AssertStringContains(t, out, "un-1")
		// Multi-matched section lists the dual-match bead.
		testutil.AssertStringContains(t, out, "Multi-matched beads (1):")
		testutil.AssertStringContains(t, out, "sh-1")
		testutil.AssertStringContains(t, out, "matches: bridge, gateway")
		// Per-work health renders for both works.
		testutil.AssertStringContains(t, out, "Per-work bead health:")
		testutil.AssertStringContains(t, out, "bridge")
		testutil.AssertStringContains(t, out, "gateway")
		// First run with no baseline: External-changes section is absent
		// (the empty-baseline first-run rule — beads.md §B12 step 3).
		if strings.Contains(out, "External changes since last triage") {
			t.Errorf("first-run triage should not render External changes; got:\n%s", out)
		}
	})

	// --- Stage 2: kerf triage --resolved with drift → exit 2 -------------
	t.Run("triage_resolved_drift_exists", func(t *testing.T) {
		resetTriageFlags()
		triageResolved = true
		t.Cleanup(resetTriageFlags)
		got := withNoExitHook(t)
		_, err := runTriageCapturing(t)
		if err == nil {
			t.Fatal("expected non-nil error path under --resolved with drift")
		}
		if *got != 2 {
			t.Errorf("first --resolved with drift: exit code = %d, want 2", *got)
		}
	})

	// --- Stage 3: kerf new metrics --bead-filter 'label=subsystem:metrics'
	t.Run("new_with_bead_filter", func(t *testing.T) {
		out := captureOutput(t, func() {
			newJigFlag = "plan"
			newTitle = "Metrics bucket"
			newBeadFilter = "label=subsystem:metrics"
			defer func() {
				newJigFlag = ""
				newTitle = ""
				newBeadFilter = ""
			}()
			if err := newCmd.RunE(newCmd, []string{"metrics"}); err != nil {
				t.Fatalf("kerf new metrics: %v", err)
			}
		})
		testutil.AssertStringContains(t, out, "Work created: metrics")
	})

	// --- Stage 4: kerf pin bridge sh-1 resolves the multi-match ----------
	t.Run("pin_multi_match", func(t *testing.T) {
		out := captureOutput(t, func() {
			if err := pinCmd.RunE(pinCmd, []string{"bridge", "sh-1"}); err != nil {
				t.Fatalf("kerf pin bridge sh-1: %v", err)
			}
		})
		// Output should mention the bead and the work.
		if !strings.Contains(out, "sh-1") || !strings.Contains(out, "bridge") {
			t.Errorf("kerf pin output should mention bead and codename; got:\n%s", out)
		}
	})

	// --- Stage 5: kerf triage (post-resolution, pre-ack) -----------------
	// All drift items should now be empty (untriaged → matched by metrics,
	// multi-match → resolved by pin). But the baseline hasn't been
	// advanced yet, so --resolved will still need --ack to clear.
	t.Run("triage_post_resolution_clean", func(t *testing.T) {
		resetTriageFlags()
		t.Cleanup(resetTriageFlags)
		out, err := runTriageCapturing(t)
		if err != nil {
			t.Fatalf("kerf triage: %v", err)
		}
		if strings.Contains(out, "Untriaged beads (") {
			t.Errorf("post-resolution: untriaged section should be empty\n%s", out)
		}
		if strings.Contains(out, "Multi-matched beads (") {
			t.Errorf("post-resolution: multi_matched section should be empty\n%s", out)
		}
	})

	// --- Stage 6: kerf triage --ack advances the baseline ----------------
	t.Run("triage_ack_advances_baseline", func(t *testing.T) {
		resetTriageFlags()
		triageAck = true
		t.Cleanup(resetTriageFlags)
		if _, err := runTriageCapturing(t); err != nil {
			t.Fatalf("kerf triage --ack: %v", err)
		}
		cachePath := filepath.Join(repo, ".kerf", "sync-cache.json")
		if _, err := os.Stat(cachePath); err != nil {
			t.Fatalf("expected %s after --ack: %v", cachePath, err)
		}
	})

	// --- Stage 7: kerf triage --resolved → exit 0 (convergence) ----------
	t.Run("triage_resolved_clean_after_ack", func(t *testing.T) {
		resetTriageFlags()
		triageResolved = true
		t.Cleanup(resetTriageFlags)
		got := withNoExitHook(t)
		if _, err := runTriageCapturing(t); err != nil {
			t.Fatalf("kerf triage --resolved (clean): %v", err)
		}
		if *got != 0 {
			t.Errorf("clean --resolved exit code = %d, want 0", *got)
		}
	})

	// --- Stage 8: kerf show bridge renders (pinned) marker ---------------
	t.Run("show_bridge_pinned_marker", func(t *testing.T) {
		out := captureOutput(t, func() {
			if err := showCmd.RunE(showCmd, []string{"bridge"}); err != nil {
				t.Fatalf("kerf show bridge: %v", err)
			}
		})
		testutil.AssertStringContains(t, out, "Work: bridge")
		testutil.AssertStringContains(t, out, "Attached beads")
		testutil.AssertStringContains(t, out, "sh-1")
		// Locate the row for sh-1 and confirm the (pinned) marker.
		// Rows are line-delimited; look for "sh-1" followed by "(pinned)"
		// on the same line.
		var pinnedRow string
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "sh-1") {
				pinnedRow = line
				break
			}
		}
		if !strings.Contains(pinnedRow, "(pinned)") {
			t.Errorf("expected `(pinned)` marker on sh-1 row; got: %q\nfull:\n%s", pinnedRow, out)
		}
	})

	// --- Stage 9: drift marker — externally close a bead, no --ack -------
	// Re-stub the store with br-1 flipped to closed; baseline still has
	// br-1 open, so `kerf show bridge` should annotate it `closed externally`.
	t.Run("show_drift_marker_closed_externally", func(t *testing.T) {
		driftedStore := `[
			{"id":"br-1","title":"bridge one","status":"closed","labels":["subsystem:bridge"]},
			{"id":"br-2","title":"bridge two","status":"open","labels":["subsystem:bridge"]},
			{"id":"br-3","title":"bridge three","status":"open","labels":["subsystem:bridge"]},
			{"id":"gw-1","title":"gateway one","status":"open","labels":["subsystem:gateway"]},
			{"id":"gw-2","title":"gateway two","status":"open","labels":["subsystem:gateway"]},
			{"id":"gw-3","title":"gateway three","status":"open","labels":["subsystem:gateway"]},
			{"id":"sh-1","title":"shared concern","status":"open","labels":["subsystem:bridge","subsystem:gateway"]},
			{"id":"un-1","title":"untriaged orphan","status":"open","labels":["subsystem:metrics"]}
		]`
		stubBr(t, driftedStore)

		out := captureOutput(t, func() {
			if err := showCmd.RunE(showCmd, []string{"bridge"}); err != nil {
				t.Fatalf("kerf show bridge (drifted): %v", err)
			}
		})
		var driftRow string
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "br-1") {
				driftRow = line
				break
			}
		}
		if !strings.Contains(driftRow, "closed externally") {
			t.Errorf("expected `closed externally` drift marker on br-1 row; got: %q\nfull:\n%s",
				driftRow, out)
		}
	})

	// --- Stage 10: loop-convergence smoke — a bounded loop terminates ----
	// We don't shell-out to `until ...; do; done` (the test runs in-process
	// and shells would deadlock on the cobra root). Instead we model the
	// loop manually: run --resolved, if non-zero apply the documented
	// remediation, retry, must converge within a small bound.
	t.Run("triage_loop_terminates", func(t *testing.T) {
		// Restore the clean (pre-drift) store so the loop has something to
		// converge against.
		stubBr(t, seedStore)
		const bound = 4
		converged := false
		for i := 0; i < bound; i++ {
			resetTriageFlags()
			triageResolved = true
			got := withNoExitHook(t)
			if _, err := runTriageCapturing(t); err != nil && *got == 0 {
				// surfaced non-zero exit code error path — keep going.
				_ = err
			}
			resetTriageFlags()
			if *got == 0 {
				converged = true
				break
			}
			// Acknowledge the new baseline (the only remediation needed at
			// this point — everything has already been resolved upstream).
			resetTriageFlags()
			triageAck = true
			if _, err := runTriageCapturing(t); err != nil {
				t.Fatalf("loop iteration %d: kerf triage --ack: %v", i, err)
			}
			resetTriageFlags()
		}
		if !converged {
			t.Errorf("triage --resolved did not converge within %d iterations", bound)
		}
	})

	// --- Stage 11: head comment survives kerf pin + kerf work edit -------
	t.Run("comment_survival", func(t *testing.T) {
		// Add a second pin (different bead) — exercises the pinned_beads
		// mutator on a spec that already has the head comment.
		resetTriageFlags()
		if err := pinCmd.RunE(pinCmd, []string{"bridge", "br-2"}); err != nil {
			t.Fatalf("kerf pin bridge br-2: %v", err)
		}
		// Then add a clause to bridge's bead_filter — exercises the
		// bead_filter mutator on the same file.
		workEditBeadFilterAdd = []string{"label=tier:critical"}
		defer func() { workEditBeadFilterAdd = nil }()
		if err := workEditCmd.RunE(workEditCmd, []string{"bridge"}); err != nil {
			t.Fatalf("kerf work edit bridge --bead-filter-add: %v", err)
		}

		// The head comment must still be at the top of the file.
		data, err := os.ReadFile(bridgeSpec)
		if err != nil {
			t.Fatalf("read bridge spec.yaml: %v", err)
		}
		if !bytes.HasPrefix(data, []byte(headComment)) {
			t.Errorf("head comment was lost after pin + work edit\nfile:\n%s", string(data))
		}
	})
}

// prependHeadComment inserts the given comment text (must terminate with a
// newline) at the very top of the named file. Used to seed a head comment
// before exercising the comment-preserving spec mutators.
func prependHeadComment(path, comment string) error {
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	combined := append([]byte(comment), original...)
	return os.WriteFile(path, combined, 0o644)
}
