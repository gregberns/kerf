package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestRun_CannedScenario verifies that `kerfsim run small-linear` produces a
// run directory containing the four policy subdirectories, each with the
// canonical artifact set.
func TestRun_CannedScenario(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "out")

	var buf bytes.Buffer
	opts := &runOpts{
		runs:   1,
		format: "text",
		outDir: outDir,
	}
	if err := runRun(&buf, "small-linear", opts); err != nil {
		t.Fatalf("runRun: %v", err)
	}

	for _, name := range []string{"kerf", "random", "fifo-bead", "fifo-work"} {
		sub := filepath.Join(outDir, name)
		for _, fname := range []string{"summary.json", "summary.txt", "events.jsonl", "scenario.yaml", "weights.yaml"} {
			p := filepath.Join(sub, fname)
			info, err := os.Stat(p)
			if err != nil {
				t.Errorf("missing %s: %v", p, err)
				continue
			}
			if info.Size() == 0 {
				t.Errorf("%s is empty", p)
			}
		}
		// summary.json must parse and carry stable keys.
		b, err := os.ReadFile(filepath.Join(sub, "summary.json"))
		if err != nil {
			t.Fatalf("read summary.json: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Errorf("summary.json for %s: parse: %v", name, err)
		}
		for _, key := range []string{"full", "warmup", "wall_ticks", "agents", "scenario_sha256", "weights_sha256"} {
			if _, ok := doc[key]; !ok {
				t.Errorf("summary.json for %s: missing key %q", name, key)
			}
		}
	}
}

// TestRun_SeedOverride verifies that --seed changes the produced result. Two
// runs with different seeds must produce different scenario_sha256 (because
// the orchestrator re-hashes the seed-bearing scenario bytes) — or at the
// very least different events.jsonl content. Compare events.jsonl bytes.
func TestRun_SeedOverride(t *testing.T) {
	tmp := t.TempDir()
	dirA := filepath.Join(tmp, "a")
	dirB := filepath.Join(tmp, "b")

	// First run uses the scenario's own seed.
	if err := runRun(&bytes.Buffer{}, "small-linear", &runOpts{
		runs: 1, format: "text", outDir: dirA,
	}); err != nil {
		t.Fatalf("runRun A: %v", err)
	}

	// Second run overrides the seed; explicitly mark seedSet.
	if err := runRun(&bytes.Buffer{}, "small-linear", &runOpts{
		runs: 1, format: "text", outDir: dirB,
		seed: 9999, seedSet: true,
	}); err != nil {
		t.Fatalf("runRun B: %v", err)
	}

	// events.jsonl from the kerf policy should differ.
	a, err := os.ReadFile(filepath.Join(dirA, "kerf", "events.jsonl"))
	if err != nil {
		t.Fatalf("read A events: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dirB, "kerf", "events.jsonl"))
	if err != nil {
		t.Fatalf("read B events: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatalf("seed override produced identical events.jsonl — expected differences")
	}
}

// TestRun_MultiRunSubdirs verifies --runs 3 produces three seed_<n>/
// subdirectories under the output directory.
func TestRun_MultiRunSubdirs(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "agg")

	if err := runRun(&bytes.Buffer{}, "small-linear", &runOpts{
		runs: 3, format: "text", outDir: outDir,
		// Pin the seed so test is deterministic against canned scenario edits.
		seed: 100, seedSet: true,
	}); err != nil {
		t.Fatalf("runRun: %v", err)
	}

	for _, sn := range []string{"seed_100", "seed_101", "seed_102"} {
		sub := filepath.Join(outDir, sn)
		if info, err := os.Stat(sub); err != nil || !info.IsDir() {
			t.Errorf("missing seed subdir %s: err=%v", sub, err)
			continue
		}
		// And each should contain the four policy dirs.
		for _, p := range []string{"kerf", "random", "fifo-bead", "fifo-work"} {
			if _, err := os.Stat(filepath.Join(sub, p, "summary.json")); err != nil {
				t.Errorf("missing %s/%s/summary.json: %v", sn, p, err)
			}
		}
	}
}

// TestRun_OutHonored verifies that --out is used verbatim as the output
// directory (no timestamp injected).
func TestRun_OutHonored(t *testing.T) {
	tmp := t.TempDir()
	custom := filepath.Join(tmp, "my-custom-name")
	if err := runRun(&bytes.Buffer{}, "small-linear", &runOpts{
		runs: 1, format: "text", outDir: custom,
	}); err != nil {
		t.Fatalf("runRun: %v", err)
	}
	if _, err := os.Stat(filepath.Join(custom, "kerf", "summary.json")); err != nil {
		t.Errorf("custom out dir not used: %v", err)
	}
}

// TestRun_QuietSuppressesOutput verifies that --quiet writes nothing to the
// supplied stdout writer.
func TestRun_QuietSuppressesOutput(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "out")
	var buf bytes.Buffer
	if err := runRun(&buf, "small-linear", &runOpts{
		runs: 1, format: "text", outDir: outDir, quiet: true,
	}); err != nil {
		t.Fatalf("runRun: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("--quiet produced stdout: %q", buf.String())
	}
}

// TestRun_ScenarioFromPath verifies that a path argument is used when not a
// canned name. Uses a copy of the canned small-linear written to a temp file.
func TestRun_ScenarioFromPath(t *testing.T) {
	tmp := t.TempDir()

	// Load the canned bytes via the resolver, then write them to disk.
	canBytes, _, err := resolveScenario("small-linear")
	if err != nil {
		t.Fatalf("resolveScenario: %v", err)
	}
	path := filepath.Join(tmp, "custom.yaml")
	if err := os.WriteFile(path, canBytes, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	outDir := filepath.Join(tmp, "out")
	if err := runRun(&bytes.Buffer{}, path, &runOpts{
		runs: 1, format: "text", outDir: outDir,
	}); err != nil {
		t.Fatalf("runRun: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "kerf", "summary.json")); err != nil {
		t.Errorf("file-path scenario did not produce output: %v", err)
	}
}

// TestRun_WeightsFile verifies that a --weights file is parsed without error
// and produces a run directory. We do not assert on metric values — just that
// the override path is wired.
func TestRun_WeightsFile(t *testing.T) {
	tmp := t.TempDir()
	wpath := filepath.Join(tmp, "w.yaml")
	if err := os.WriteFile(wpath, []byte("fan_out: 20.0\nrework: 5.0\n"), 0o644); err != nil {
		t.Fatalf("write weights: %v", err)
	}
	outDir := filepath.Join(tmp, "out")
	if err := runRun(&bytes.Buffer{}, "small-linear", &runOpts{
		runs: 1, format: "text", outDir: outDir, weightsPath: wpath,
	}); err != nil {
		t.Fatalf("runRun: %v", err)
	}
	// Check weights.yaml in the kerf subdir reflects the orchestrator's
	// canonical render. The exact bytes are not specified; just confirm the
	// file exists and is non-empty.
	b, err := os.ReadFile(filepath.Join(outDir, "kerf", "weights.yaml"))
	if err != nil {
		t.Fatalf("read weights.yaml: %v", err)
	}
	if !strings.Contains(string(b), "fan_out:") {
		t.Errorf("weights.yaml missing fan_out: %s", string(b))
	}
}

// TestRun_FormatJSONStreamsSummary verifies --format=json writes summary.json
// bytes (kerf policy) to stdout in addition to the run directory.
func TestRun_FormatJSONStreamsSummary(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "out")
	var buf bytes.Buffer
	if err := runRun(&buf, "small-linear", &runOpts{
		runs: 1, format: "json", outDir: outDir,
	}); err != nil {
		t.Fatalf("runRun: %v", err)
	}
	// Should contain a parseable JSON object with "full" key.
	out := buf.String()
	idx := strings.Index(out, "{")
	if idx < 0 {
		t.Fatalf("--format=json produced no JSON on stdout: %q", out)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out[idx:strings.LastIndex(out, "}")+1]), &doc); err != nil {
		t.Fatalf("stdout JSON did not parse: %v\nstdout=%q", err, out)
	}
	if _, ok := doc["full"]; !ok {
		t.Errorf("stdout JSON missing \"full\" key; got keys: %v", keys(doc))
	}
}

// TestRun_AgentsSweep verifies that --agents-sweep produces per-agent-count
// output directories under <out>/<scenario>/seed_<n>/agents_<k>/<policy>/ and
// writes a sweep_summary.csv with one row per (agent_count, policy, seed).
func TestRun_AgentsSweep(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "sweep")

	if err := runRun(&bytes.Buffer{}, "small-linear", &runOpts{
		runs: 1, format: "text", outDir: outDir,
		seed: 42, seedSet: true,
		agentsSweep: "1,2",
	}); err != nil {
		t.Fatalf("runRun: %v", err)
	}

	scDir := filepath.Join(outDir, "small-linear")

	// Both agent-count subdirectories must exist with the four policy dirs.
	for _, k := range []int{1, 2} {
		base := filepath.Join(scDir, "seed_42", "agents_"+strconv.Itoa(k))
		for _, p := range policyNames {
			if _, err := os.Stat(filepath.Join(base, p, "summary.json")); err != nil {
				t.Errorf("missing %s/%s/summary.json: %v", base, p, err)
			}
		}
	}

	// sweep_summary.csv must exist and contain a row for each
	// (agent_count, policy, seed) combination: 2 counts × 4 policies × 1 seed = 8 rows.
	csvPath := filepath.Join(scDir, "sweep_summary.csv")
	b, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("read sweep_summary.csv: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) < 1 {
		t.Fatalf("sweep_summary.csv is empty")
	}
	header := lines[0]
	for _, col := range []string{"agent_count", "policy", "seed", "work_completed", "agent_idle_pct", "top_of_queue_churn", "area_collisions", "goal_completion_3d", "rework_p95_wait", "priority_inversions"} {
		if !strings.Contains(header, col) {
			t.Errorf("sweep_summary.csv header missing %q: %s", col, header)
		}
	}
	if got := len(lines) - 1; got != 8 {
		t.Errorf("sweep_summary.csv: got %d data rows, want 8 (2 counts × 4 policies × 1 seed)", got)
	}
	// Spot-check that both agent counts appear in column 1.
	have1, have2 := false, false
	for _, ln := range lines[1:] {
		switch {
		case strings.HasPrefix(ln, "1,"):
			have1 = true
		case strings.HasPrefix(ln, "2,"):
			have2 = true
		}
	}
	if !have1 || !have2 {
		t.Errorf("sweep_summary.csv missing rows for agent_count=1 (%v) or 2 (%v)", have1, have2)
	}
}

// TestParseAgentsSweep covers the flag-parser edge cases enumerated in the
// plan: empty value, zero/negative reject, dedupe, ordering.
func TestParseAgentsSweep(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    []int
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"whitespace only", "   ", nil, false},
		{"single", "3", []int{3}, false},
		{"basic", "1,2,3", []int{1, 2, 3}, false},
		{"unsorted gets sorted", "5,1,3", []int{1, 3, 5}, false},
		{"dedupe", "1,1,2", []int{1, 2}, false},
		{"with spaces", "1, 2 , 3", []int{1, 2, 3}, false},
		{"zero rejected", "0", nil, true},
		{"negative rejected", "-1,2", nil, true},
		{"non-integer rejected", "1,foo,2", nil, true},
		{"zero in middle", "1,0,2", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseAgentsSweep(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseAgentsSweep(%q): err=%v wantErr=%v", tc.in, err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseAgentsSweep(%q) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("parseAgentsSweep(%q) = %v, want %v", tc.in, got, tc.want)
				}
			}
		})
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
