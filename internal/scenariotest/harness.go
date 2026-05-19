package scenariotest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// SkipMessage is the message used when `bd` is not on PATH.
const SkipMessage = "scenariotest: bd not found on PATH; install bd to run real-binary scenarios"

// defaultRunTimeout is the per-subprocess wall-clock timeout. Scenarios can
// override via RunOpt (future extension).
const defaultRunTimeout = 30 * time.Second

// BeadSpec describes a bead fixture to be seeded via `bd create`.
type BeadSpec struct {
	// Title is the bead title (positional arg to `bd create`).
	Title string
	// Type is the bead type (--type). Empty means bd default.
	Type string
	// Priority is the bead priority (--priority). Empty means bd default.
	Priority string
	// Labels are appended via repeated --label flags.
	Labels []string
	// Description is the bead body (--description). Empty means no body.
	Description string
	// Extra flags appended after the standard ones. Used for escape hatches.
	Extra []string
}

// buildOnce serialises the one-time kerf binary build.
var (
	buildOnce   sync.Once
	builtBinary string
	buildErr    error
)

// kerfBinary returns the path to the kerf binary, building it once per process
// if necessary. Honours KERF_TEST_BINARY for a prebuilt override.
func kerfBinary(t testing.TB) string {
	t.Helper()
	if override := os.Getenv("KERF_TEST_BINARY"); override != "" {
		return override
	}
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "kerf-scenariotest-bin-*")
		if err != nil {
			buildErr = fmt.Errorf("scenariotest: tempdir for build: %w", err)
			return
		}
		name := "kerf"
		if runtime.GOOS == "windows" {
			name = "kerf.exe"
		}
		out := filepath.Join(dir, name)
		modRoot, err := findModuleRoot()
		if err != nil {
			buildErr = fmt.Errorf("scenariotest: locate module root: %w", err)
			return
		}
		cmd := exec.Command("go", "build", "-o", out, ".")
		cmd.Dir = modRoot
		if combined, err := cmd.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("scenariotest: go build failed: %v\n%s", err, combined)
			return
		}
		builtBinary = out
	})
	if buildErr != nil {
		t.Fatalf("%v", buildErr)
	}
	return builtBinary
}

// findModuleRoot walks up from this file's location until a go.mod is found.
func findModuleRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod found walking up from " + thisFile)
		}
		dir = parent
	}
}

// RequireBd skips the test if `bd` is not on PATH.
func RequireBd(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip(SkipMessage)
	}
}

// Runner drives a single scenario: a kerf binary, a fresh project tempdir,
// a fresh HOME, a fresh bd store, and a scrubbed env.
type Runner struct {
	t           testing.TB
	binary      string
	projectRoot string
	homeDir     string
	benchDir    string
	taskTool    string   // configured tools.tasks value; defaults to "bd".
	env         []string // base env passed to every subprocess.
}

// New constructs a Runner for a scenario. It:
//   - Skips the test if `bd` is not on PATH.
//   - Builds the kerf binary if not already built.
//   - Creates fresh tempdirs for the project root and HOME.
//   - Scrubs KERF_* and BD_* env vars from the parent process.
//   - Runs `bd init` in the project root.
func New(t testing.TB) *Runner {
	t.Helper()
	RequireBd(t)

	binary := kerfBinary(t)

	projectRoot := t.TempDir()
	homeDir := t.TempDir()
	benchDir := filepath.Join(homeDir, ".kerf", "bench")

	r := &Runner{
		t:           t,
		binary:      binary,
		projectRoot: projectRoot,
		homeDir:     homeDir,
		benchDir:    benchDir,
		taskTool:    "bd",
		env:         scrubbedEnv(homeDir),
	}

	if err := r.initBdStore(); err != nil {
		t.Fatalf("scenariotest: bd init failed: %v", err)
	}
	return r
}

// UseTaskTool overrides the configured tools.tasks value that the runner
// will apply after `kerf init`. Default is "bd". Returns the receiver for
// chaining. Must be called before any `kerf init` invocation that should
// pick up the override; the runner re-applies the value automatically on
// each Run("init", ...) invocation it observes.
func (r *Runner) UseTaskTool(name string) *Runner {
	r.taskTool = name
	return r
}

// PrependPath prepends dir to the PATH entry in the runner's env. Returns
// the receiver for chaining. If no PATH is currently set, dir becomes PATH.
func (r *Runner) PrependPath(dir string) *Runner {
	r.t.Helper()
	for i, kv := range r.env {
		if strings.HasPrefix(kv, "PATH=") {
			r.env[i] = "PATH=" + dir + string(os.PathListSeparator) + kv[len("PATH="):]
			return r
		}
	}
	r.env = append(r.env, "PATH="+dir)
	return r
}

// AttachFakeRemote configures a fake `origin` git remote on the scenario's
// project root. Used by scenarios that need kerf's project-ID derivation to
// hit the remote branch rather than the directory-name fallback. The remote
// URL string is opaque to the runner; callers pick something deterministic.
func (r *Runner) AttachFakeRemote(remoteURL string) {
	r.t.Helper()
	cmd := exec.Command("git", "remote", "add", "origin", remoteURL)
	cmd.Dir = r.projectRoot
	cmd.Env = r.env
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("scenariotest: git remote add origin %s: %v\n%s", remoteURL, err, out)
	}
}

// Env returns a copy of the runner's scrubbed env slice. Read-only — mutate
// via SetEnv / PrependPath.
func (r *Runner) Env() []string {
	out := make([]string, len(r.env))
	copy(out, r.env)
	return out
}

// applyTaskTool runs `kerf config tools.tasks <r.taskTool>` so scenarios
// don't have to repeat the boilerplate after `kerf init`.
func (r *Runner) applyTaskTool() {
	r.t.Helper()
	if r.taskTool == "" {
		return
	}
	stdout, stderr, code, err := r.run(nil, "config", "tools.tasks", r.taskTool)
	if err != nil {
		r.t.Fatalf("scenariotest: kerf config tools.tasks %s: %v\nstdout: %s\nstderr: %s",
			r.taskTool, err, stdout, stderr)
	}
	if code != 0 {
		r.t.Fatalf("scenariotest: kerf config tools.tasks %s exit=%d\nstdout: %s\nstderr: %s",
			r.taskTool, code, stdout, stderr)
	}
}

// scrubbedEnv returns a copy of os.Environ with KERF_* and BD_* keys removed,
// and HOME pinned to the scenario's homeDir.
func scrubbedEnv(homeDir string) []string {
	out := make([]string, 0, len(os.Environ())+2)
	for _, kv := range os.Environ() {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		key := kv[:eq]
		if strings.HasPrefix(key, "KERF_") || strings.HasPrefix(key, "BD_") {
			continue
		}
		if key == "HOME" {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "HOME="+homeDir)
	return out
}

// initBdStore runs `bd init` inside the project root. Non-interactive defaults
// are used; if `bd` requires interactivity here, the scenario can drive it via
// Run/RunWithStdin separately.
func (r *Runner) initBdStore() error {
	cmd := exec.Command("bd", "init", "--agents-profile", "minimal")
	cmd.Dir = r.projectRoot
	cmd.Env = r.env
	// bd init may produce noise on stderr even on success; only surface on
	// failure.
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd init: %v\n%s", err, out)
	}
	return nil
}

// ProjectRoot returns the absolute path to the scenario's project root.
func (r *Runner) ProjectRoot() string { return r.projectRoot }

// HomeDir returns the absolute path to the scenario's HOME.
func (r *Runner) HomeDir() string { return r.homeDir }

// BenchDir returns the absolute path to the scenario's bench dir (under HOME).
// Note: the directory may not exist until kerf creates it.
func (r *Runner) BenchDir() string { return r.benchDir }

// Binary returns the absolute path to the compiled kerf binary.
func (r *Runner) Binary() string { return r.binary }

// SetEnv appends or overrides a single env var for subsequent Run calls.
// Repeated keys are appended; the last value wins per usual exec semantics.
func (r *Runner) SetEnv(key, value string) {
	r.env = append(r.env, key+"="+value)
}

// Run executes `kerf <args...>` as a subprocess with closed stdin.
// Returns captured stdout, stderr, exit code, and a non-nil err only for
// non-exit failures (start failure, timeout, etc.). A non-zero exit code is
// reported via exitCode with err == nil.
//
// Side effect: when args names a successful `kerf init`, the runner
// automatically applies tools.tasks=<r.taskTool> (default "bd") so scenarios
// don't have to repeat the boilerplate. Override via UseTaskTool("") to
// disable, or UseTaskTool("br") to pick a different tool.
func (r *Runner) Run(args ...string) (stdout, stderr string, exitCode int, err error) {
	stdout, stderr, exitCode, err = r.run(nil, args...)
	r.maybeApplyTaskTool(args, exitCode, err)
	return stdout, stderr, exitCode, err
}

// RunWithStdin is like Run but feeds the given string into the subprocess's
// stdin.
func (r *Runner) RunWithStdin(stdin string, args ...string) (stdout, stderr string, exitCode int, err error) {
	stdout, stderr, exitCode, err = r.run(strings.NewReader(stdin), args...)
	r.maybeApplyTaskTool(args, exitCode, err)
	return stdout, stderr, exitCode, err
}

// maybeApplyTaskTool fires applyTaskTool after a successful `kerf init`.
// Distinguishes init from "kerf config tools.tasks" et al. by checking the
// first positional (post-flag) argument.
func (r *Runner) maybeApplyTaskTool(args []string, exitCode int, err error) {
	if err != nil || exitCode != 0 {
		return
	}
	if r.taskTool == "" {
		return
	}
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if a == "init" {
			r.applyTaskTool()
		}
		return
	}
}

func (r *Runner) run(stdin io.Reader, args ...string) (string, string, int, error) {
	r.t.Helper()
	cmd := exec.Command(r.binary, args...)
	cmd.Dir = r.projectRoot
	cmd.Env = r.env
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if stdin != nil {
		cmd.Stdin = stdin
	}

	if err := cmd.Start(); err != nil {
		return "", "", -1, fmt.Errorf("scenariotest: start kerf %v: %w", args, err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		exitCode := 0
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				exitCode = ee.ExitCode()
				return outBuf.String(), errBuf.String(), exitCode, nil
			}
			return outBuf.String(), errBuf.String(), -1, err
		}
		return outBuf.String(), errBuf.String(), exitCode, nil
	case <-time.After(defaultRunTimeout):
		_ = cmd.Process.Kill()
		<-done
		return outBuf.String(), errBuf.String(), -1, fmt.Errorf("scenariotest: kerf %v timed out after %s", args, defaultRunTimeout)
	}
}

// SeedBeads bulk-creates beads in the project's bd store by shelling out to
// `bd create`. It fails the test on the first error.
func (r *Runner) SeedBeads(beads []BeadSpec) {
	r.t.Helper()
	for _, b := range beads {
		if b.Title == "" {
			r.t.Fatalf("scenariotest: SeedBeads: empty Title")
		}
		args := []string{"create", b.Title}
		if b.Type != "" {
			args = append(args, "--type", b.Type)
		}
		if b.Priority != "" {
			args = append(args, "--priority", b.Priority)
		}
		for _, l := range b.Labels {
			args = append(args, "--label", l)
		}
		if b.Description != "" {
			args = append(args, "--description", b.Description)
		}
		args = append(args, b.Extra...)
		cmd := exec.Command("bd", args...)
		cmd.Dir = r.projectRoot
		cmd.Env = r.env
		out, err := cmd.CombinedOutput()
		if err != nil {
			r.t.Fatalf("scenariotest: bd %v: %v\n%s", args, err, out)
		}
	}
}

// WriteFile writes content to a path relative to the project root, creating
// parent directories as needed.
func (r *Runner) WriteFile(relpath, content string) {
	r.t.Helper()
	full := filepath.Join(r.projectRoot, relpath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		r.t.Fatalf("scenariotest: mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		r.t.Fatalf("scenariotest: write %s: %v", full, err)
	}
}

// ReadFile reads a file relative to the project root and returns its contents
// as a string. Fails the test on error.
func (r *Runner) ReadFile(relpath string) string {
	r.t.Helper()
	full := filepath.Join(r.projectRoot, relpath)
	data, err := os.ReadFile(full)
	if err != nil {
		r.t.Fatalf("scenariotest: read %s: %v", full, err)
	}
	return string(data)
}
