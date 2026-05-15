package scenario_import

import (
	"os"
	"path/filepath"
	"testing"
)

const harmonikCorpus = "/Users/gb/github/harmonik/docs/decompose-to-tasks"

// requireHarmonikCorpus skips the test if the harmonik checkout is not
// available locally. The import package is checked-in and must build
// regardless of where the developer cloned harmonik to.
func requireHarmonikCorpus(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("harmonik corpus not available at %s: %v", path, err)
	}
}

func TestImportHarmonik_SinglePilotCP(t *testing.T) {
	path := filepath.Join(harmonikCorpus, "cp-pilot-data.yaml")
	requireHarmonikCorpus(t, path)

	res, err := ImportHarmonik(path)
	if err != nil {
		t.Fatalf("ImportHarmonik: %v", err)
	}
	if got, want := len(res.Pilots), 1; got != want {
		t.Fatalf("pilots: got %d want %d", got, want)
	}
	p := res.Pilots[0]
	if p.Mnem != "cp" {
		t.Errorf("pilot mnem: got %q want %q", p.Mnem, "cp")
	}
	if p.BeadCount < 50 {
		t.Errorf("bead count looks low: got %d (expected ~85)", p.BeadCount)
	}
	if p.EdgeCount < 50 {
		t.Errorf("edge count looks low: got %d (expected several hundred)", p.EdgeCount)
	}
	if err := res.Scenario.Validate(); err != nil {
		t.Errorf("scenario validation: %v", err)
	}
	// Single-pilot import: no cross-pilot deps possible.
	if len(res.Scenario.Works) != 1 {
		t.Fatalf("works: got %d want 1", len(res.Scenario.Works))
	}
	w := res.Scenario.Works[0]
	if w.Codename != "cp" {
		t.Errorf("work codename: got %q want %q", w.Codename, "cp")
	}
	if d := w.DepsSlice(); len(d) != 0 {
		t.Errorf("expected no deps for single-pilot import, got %v", d)
	}
	if len(w.Areas) == 0 {
		t.Errorf("expected at least one area")
	}
}

func TestImportHarmonik_DirectoryAllPilots(t *testing.T) {
	requireHarmonikCorpus(t, harmonikCorpus)

	res, err := ImportHarmonik(harmonikCorpus)
	if err != nil {
		t.Fatalf("ImportHarmonik: %v", err)
	}
	if len(res.Pilots) < 5 {
		t.Fatalf("expected at least 5 pilots, got %d", len(res.Pilots))
	}
	if err := res.Scenario.Validate(); err != nil {
		t.Errorf("scenario validation: %v", err)
	}
	// At least one work should declare a cross-pilot dep (CP edges cite
	// `ar-*`, `em-*`, etc.).
	found := false
	for _, w := range res.Scenario.Works {
		if len(w.DepsSlice()) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected at least one cross-pilot dep across the corpus")
	}
}

func TestMarshalScenarioRoundTrip(t *testing.T) {
	path := filepath.Join(harmonikCorpus, "cp-pilot-data.yaml")
	requireHarmonikCorpus(t, path)

	res, err := ImportHarmonik(path)
	if err != nil {
		t.Fatalf("ImportHarmonik: %v", err)
	}
	body, err := MarshalScenario(res.Scenario, path, res.Notes)
	if err != nil {
		t.Fatalf("MarshalScenario: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("empty body")
	}
	// The YAML must round-trip through scenario.LoadBytes.
	tmp := filepath.Join(t.TempDir(), "scenario.yaml")
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		t.Fatal(err)
	}
	// Use the package the production code uses to be sure validation runs.
	got, err := loadAndValidate(tmp)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(got.Works) != 1 {
		t.Fatalf("works after reload: got %d want 1", len(got.Works))
	}
}
