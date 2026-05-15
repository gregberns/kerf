// Command kerfsim — `run` subcommand.
//
// Executes a scenario through the run orchestrator and writes a run
// directory per the canonical layout in specs/simulator.md §Run Output. The
// scenario argument is resolved first against the embedded canned scenarios
// (small-linear, wide-fanout, rework-heavy) and then as a path on disk.
//
// For each invocation the four Phase-1 policies (kerf + random, fifo-bead,
// fifo-work) are run from the same generated world, mutation-isolated. Each
// policy gets its own subdirectory under the run directory.
//
// Spec: specs/simulator.md §CLI, §`kerfsim run`, §Run Output.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/gberns/kerf/internal/queue"
	"github.com/gberns/kerf/internal/sim/duration"
	"github.com/gberns/kerf/internal/sim/output"
	"github.com/gberns/kerf/internal/sim/run"
	"github.com/gberns/kerf/internal/sim/scenario"
	"github.com/gberns/kerf/scenarios"
)

// runOpts is the flag-bag for the `run` subcommand. Kept small and explicit
// so the wiring stays linear.
type runOpts struct {
	weightsPath   string
	seed          int64
	seedSet       bool
	runs          int
	outDir        string
	quiet         bool
	verbose       bool
	format        string
	debugDispatch string
}

// policyNames is the canonical on-disk subdirectory name for each policy.
// Order is fixed for deterministic output.
var policyNames = []string{"kerf", "random", "fifo-bead", "fifo-work"}

func newRunCmd() *cobra.Command {
	opts := &runOpts{runs: 1, format: "text"}
	cmd := &cobra.Command{
		Use:   "run <scenario>",
		Short: "Run a kerfsim scenario and write a run directory.",
		Long: `Run a scenario through the simulator.

<scenario> is either the name of a built-in canned scenario (small-linear,
wide-fanout, rework-heavy) or a path to a scenario YAML file on disk.

For each invocation the four Phase-1 policies (kerf + random, fifo-bead,
fifo-work) run from the same generated world; each policy's output is
written under its own subdirectory.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Track whether --seed was explicitly set; cobra does not surface
			// "flag set vs default" cleanly without this check.
			opts.seedSet = cmd.Flags().Changed("seed")
			return runRun(cmd.OutOrStdout(), args[0], opts)
		},
	}
	cmd.Flags().StringVar(&opts.weightsPath, "weights", "", "Path to a weights YAML file (defaults to queue.DefaultWeights)")
	cmd.Flags().Int64Var(&opts.seed, "seed", 0, "Override the scenario seed")
	cmd.Flags().IntVar(&opts.runs, "runs", 1, "Run N times with seeds [seed, seed+N-1]")
	cmd.Flags().StringVar(&opts.outDir, "out", "", "Output directory (default: runs/<timestamp>-<scenario>-<weights-hash>/)")
	cmd.Flags().BoolVar(&opts.quiet, "quiet", false, "Suppress all stdout (exit code only)")
	cmd.Flags().BoolVar(&opts.verbose, "verbose", false, "Stream events to stdout after the run")
	cmd.Flags().StringVar(&opts.format, "format", "text", "Output format: text or json")
	cmd.Flags().StringVar(&opts.debugDispatch, "debug-dispatch", "", "Write per-arrival and per-dispatch JSONL for the kerf policy to this path (B14 diagnostic). Only valid with --runs=1.")
	return cmd
}

// runRun is the entry point of the run subcommand. Separated from the cobra
// glue so it is unit-testable.
func runRun(stdout io.Writer, scenarioArg string, opts *runOpts) error {
	if opts.runs < 1 {
		return fmt.Errorf("--runs must be >= 1 (got %d)", opts.runs)
	}
	if opts.format != "text" && opts.format != "json" {
		return fmt.Errorf("unknown --format %q (want \"text\" or \"json\")", opts.format)
	}

	// Resolve scenario: canned name first, then disk path.
	scBytes, scLabel, err := resolveScenario(scenarioArg)
	if err != nil {
		return err
	}
	sc, err := scenario.LoadBytes(scBytes)
	if err != nil {
		return fmt.Errorf("scenario %s: %w", scLabel, err)
	}

	// Load the fitted-distribution registry. A missing file is non-fatal;
	// scenarios that use kind=from_distribution will surface a clearer
	// "distribution not found" error downstream.
	reg, err := duration.LoadRegistry(duration.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("load fitted distributions: %w", err)
	}

	// Resolve weights: file if --weights given, defaults otherwise.
	weights := queue.DefaultWeights()
	if opts.weightsPath != "" {
		wb, err := os.ReadFile(opts.weightsPath)
		if err != nil {
			return fmt.Errorf("read --weights: %w", err)
		}
		weights, err = parseWeights(wb)
		if err != nil {
			return fmt.Errorf("parse --weights %s: %w", opts.weightsPath, err)
		}
	}

	// Resolve seed override.
	if opts.seedSet {
		sc.Seed = opts.seed
	}
	baseSeed := sc.Seed

	// Resolve output directory.
	outRoot := opts.outDir
	if outRoot == "" {
		outRoot = defaultOutDir(scLabel, weights)
	}

	if opts.runs == 1 {
		// Single-run: write the four policy dirs directly under outRoot.
		if err := runOneSeed(stdout, sc, weights, baseSeed, outRoot, opts, reg); err != nil {
			return err
		}
		if !opts.quiet {
			emitFinishLine(stdout, scLabel, baseSeed, outRoot, opts)
		}
		return nil
	}

	// Multi-run: create subdirectory per seed.
	for i := 0; i < opts.runs; i++ {
		seedN := baseSeed + int64(i)
		// Mutate the in-memory scenario's seed for this iteration. Re-loading
		// from bytes for each seed is unnecessary; the orchestrator validates
		// and uses the value as-is.
		sc.Seed = seedN
		seedDir := filepath.Join(outRoot, fmt.Sprintf("seed_%d", seedN))
		if err := runOneSeed(stdout, sc, weights, seedN, seedDir, opts, reg); err != nil {
			return fmt.Errorf("seed %d: %w", seedN, err)
		}
	}
	if !opts.quiet {
		emitFinishLine(stdout, scLabel, baseSeed, outRoot, opts)
	}
	return nil
}

// runOneSeed drives the orchestrator once and writes its four policy outputs
// under dir/{policy}/.
func runOneSeed(stdout io.Writer, sc *scenario.Scenario, weights queue.Weights, seed int64, dir string, opts *runOpts, reg *duration.Registry) error {
	if !opts.quiet && opts.format == "text" {
		// Single-line progress: emit a "starting" line. We do not currently
		// have per-tick progress callbacks from run.Run, so the line is
		// emitted at the boundary; the carriage-return keeps the line in
		// place for the eventual finish line (see emitFinishLine).
		fmt.Fprintf(stdout, "scenario=%s  seed=%d  ticks=%d  agents=%d  running...\r",
			truncate(filepath.Base(dir), 24), seed, sc.Ticks, sc.Agents)
	}

	var sink *jsonlDebugSink
	if opts.debugDispatch != "" {
		s, err := newJSONLDebugSink(opts.debugDispatch)
		if err != nil {
			return fmt.Errorf("--debug-dispatch: %w", err)
		}
		defer s.Close()
		sink = s
	}

	var result *run.Result
	var err error
	runOpts := run.Options{Registry: reg}
	if sink != nil {
		runOpts.KerfDebug = sink
	}
	result, err = run.RunWithOptions(sc, weights, sc.Agents, runOpts)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	// Write all four policies' outputs into per-policy subdirectories.
	perPolicy := map[string]output.Result{
		"kerf":      result.Kerf,
		"random":    result.Random,
		"fifo-bead": result.FIFOBead,
		"fifo-work": result.FIFOWork,
	}
	for _, name := range policyNames {
		sub := filepath.Join(dir, name)
		if err := output.WriteRun(sub, perPolicy[name]); err != nil {
			return fmt.Errorf("write %s: %w", sub, err)
		}
	}

	// --format=json: also stream the kerf policy's summary.json to stdout.
	if opts.format == "json" && !opts.quiet {
		b, err := output.SummaryJSON(result.Kerf)
		if err != nil {
			return fmt.Errorf("encode summary.json: %w", err)
		}
		if _, err := stdout.Write(b); err != nil {
			return err
		}
	}

	// --verbose: dump events.jsonl from the kerf policy directory to stdout
	// (post-run) so the user can inspect what happened.
	if opts.verbose && !opts.quiet {
		eventsPath := filepath.Join(dir, "kerf", "events.jsonl")
		b, err := os.ReadFile(eventsPath)
		if err != nil {
			return fmt.Errorf("read events.jsonl: %w", err)
		}
		if _, err := stdout.Write(b); err != nil {
			return err
		}
	}

	return nil
}

// emitFinishLine prints a one-line summary pointing at the run directory.
// Suppressed under --quiet.
func emitFinishLine(stdout io.Writer, scLabel string, seed int64, outRoot string, opts *runOpts) {
	if opts.format == "json" {
		// --format=json already streamed the summary; emit a path-only line
		// to stderr-style output is undesirable, so print path on its own.
		fmt.Fprintln(stdout, outRoot)
		return
	}
	// Clear the in-place progress line then print the finish summary.
	fmt.Fprintf(stdout, "scenario=%s  seed=%d  out=%s\n", scLabel, seed, outRoot)
}

// resolveScenario returns the raw YAML bytes and a label for the scenario
// argument. It tries the embedded canned scenarios first (so `kerfsim run
// small-linear` works without a file path), then falls back to a disk read.
func resolveScenario(arg string) ([]byte, string, error) {
	// Canned: arg is a bare name like "small-linear".
	if !strings.ContainsAny(arg, "/.\\") {
		fs := scenarios.FS()
		path := arg + ".yaml"
		if b, err := fs.ReadFile(path); err == nil {
			return b, arg, nil
		}
	}
	b, err := os.ReadFile(arg)
	if err != nil {
		return nil, "", fmt.Errorf("scenario %s: not a canned name and not readable: %w", arg, err)
	}
	// Use the file's base name without extension as the label.
	base := filepath.Base(arg)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return b, base, nil
}

// weightsYAML is the on-disk shape of a weights file. Mirrors the four keys
// shown in specs/simulator.md §Weights File.
type weightsYAML struct {
	FanOut   *float64 `yaml:"fan_out"`
	Momentum *float64 `yaml:"momentum"`
	Creation *float64 `yaml:"creation"`
	Rework   *float64 `yaml:"rework"`
}

// parseWeights overlays a weights file onto queue.DefaultWeights. Missing
// fields fall through to the default values so partial files are usable.
func parseWeights(b []byte) (queue.Weights, error) {
	var raw weightsYAML
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return queue.Weights{}, err
	}
	w := queue.DefaultWeights()
	if raw.FanOut != nil {
		w.FanOut = *raw.FanOut
	}
	if raw.Momentum != nil {
		w.Momentum = *raw.Momentum
	}
	if raw.Creation != nil {
		w.Creation = *raw.Creation
	}
	if raw.Rework != nil {
		w.Rework = *raw.Rework
	}
	return w, nil
}

// defaultOutDir returns the timestamped directory path used when --out is not
// supplied. Shape: runs/<UTC-timestamp>-<scenario>-<weights-hash>/. The
// weights hash is the first 8 hex chars of the canonical weights YAML so two
// runs with the same weights collide on directory name only by timestamp.
func defaultOutDir(scLabel string, w queue.Weights) string {
	stamp := time.Now().UTC().Format("20060102-150405")
	wh := weightsHashShort(w)
	return filepath.Join("runs", fmt.Sprintf("%s-%s-%s", stamp, scLabel, wh))
}

// weightsHashShort returns the first 8 hex chars of a stable fingerprint of
// w. The fingerprint is derived from the same canonical byte form the run
// orchestrator uses for `weights_sha256`, mirrored here to avoid a circular
// dep. Only the first 8 chars are used so the directory name stays short.
func weightsHashShort(w queue.Weights) string {
	// Render the same canonical bytes the orchestrator embeds; hashing is
	// purposeful here (collision-resistant fingerprint for the dir name),
	// not security-sensitive.
	bs := []byte(fmt.Sprintf("fan_out: %g\nmomentum: %g\ncreation: %g\nrework: %g\n",
		w.FanOut, w.Momentum, w.Creation, w.Rework))
	return shortHex(bs)
}

// shortHex returns the first 8 hex chars of the SHA-256 of b. Used to give
// run directories a short, stable fingerprint of the effective weights.
func shortHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:4])
}

// truncate clamps s to n characters with a trailing ellipsis when truncated.
// Used by the in-place progress line so very long scenario names don't blow
// out the terminal width.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
