package areas

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "areas.yaml")

	content := `areas:
  auth:
    description: "Authentication and identity"
  api:
    description: "HTTP interface"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	af, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(af.Areas) != 2 {
		t.Fatalf("expected 2 areas, got %d", len(af.Areas))
	}

	if af.Areas["auth"].Description != "Authentication and identity" {
		t.Errorf("unexpected auth description: %s", af.Areas["auth"].Description)
	}

	if af.Areas["api"].Description != "HTTP interface" {
		t.Errorf("unexpected api description: %s", af.Areas["api"].Description)
	}
}

func TestLoadMissingFile(t *testing.T) {
	af, err := Load("/nonexistent/path/areas.yaml")
	if err != nil {
		t.Fatalf("Load returned error for missing file: %v", err)
	}

	if af.Areas == nil {
		t.Fatal("expected non-nil Areas map")
	}

	if len(af.Areas) != 0 {
		t.Fatalf("expected empty Areas map, got %d entries", len(af.Areas))
	}
}

func TestAddValid(t *testing.T) {
	af := &AreasFile{Areas: make(map[string]Area)}

	if err := Add(af, "auth", "Authentication"); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	if len(af.Areas) != 1 {
		t.Fatalf("expected 1 area, got %d", len(af.Areas))
	}

	if af.Areas["auth"].Description != "Authentication" {
		t.Errorf("unexpected description: %s", af.Areas["auth"].Description)
	}
}

func TestAddHyphenatedName(t *testing.T) {
	af := &AreasFile{Areas: make(map[string]Area)}

	if err := Add(af, "data-layer", "Data persistence"); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	if _, ok := af.Areas["data-layer"]; !ok {
		t.Error("expected data-layer area to exist")
	}
}

func TestAddDuplicate(t *testing.T) {
	af := &AreasFile{Areas: map[string]Area{
		"auth": {Description: "Authentication"},
	}}

	err := Add(af, "auth", "Duplicate")
	if err == nil {
		t.Fatal("expected error for duplicate area")
	}
}

func TestAddInvalidName(t *testing.T) {
	af := &AreasFile{Areas: make(map[string]Area)}

	cases := []string{
		"Auth",       // uppercase
		"my_area",    // underscore
		"",           // empty
		"-leading",   // leading hyphen
		"trailing-",  // trailing hyphen
		"double--hyp", // double hyphen
		"has space",  // space
	}

	for _, name := range cases {
		if err := Add(af, name, "desc"); err == nil {
			t.Errorf("expected error for invalid name %q", name)
		}
	}
}

func TestRemoveExisting(t *testing.T) {
	af := &AreasFile{Areas: map[string]Area{
		"auth": {Description: "Authentication"},
		"api":  {Description: "HTTP interface"},
	}}

	if err := Remove(af, "auth"); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	if len(af.Areas) != 1 {
		t.Fatalf("expected 1 area, got %d", len(af.Areas))
	}

	if _, ok := af.Areas["auth"]; ok {
		t.Error("auth should have been removed")
	}
}

func TestRemoveNonexistent(t *testing.T) {
	af := &AreasFile{Areas: make(map[string]Area)}

	err := Remove(af, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent area")
	}
}

func TestValidateAllValid(t *testing.T) {
	af := &AreasFile{Areas: map[string]Area{
		"auth": {Description: "Authentication"},
		"api":  {Description: "HTTP interface"},
	}}

	invalid := Validate(af, []string{"auth", "api"})
	if len(invalid) != 0 {
		t.Fatalf("expected no invalid names, got %v", invalid)
	}
}

func TestValidateSomeInvalid(t *testing.T) {
	af := &AreasFile{Areas: map[string]Area{
		"auth": {Description: "Authentication"},
	}}

	invalid := Validate(af, []string{"auth", "bogus", "nope"})
	if len(invalid) != 2 {
		t.Fatalf("expected 2 invalid names, got %v", invalid)
	}

	expected := map[string]bool{"bogus": true, "nope": true}
	for _, name := range invalid {
		if !expected[name] {
			t.Errorf("unexpected invalid name: %s", name)
		}
	}
}

func TestValidateEmptyNames(t *testing.T) {
	af := &AreasFile{Areas: map[string]Area{
		"auth": {Description: "Authentication"},
	}}

	invalid := Validate(af, nil)
	if len(invalid) != 0 {
		t.Fatalf("expected no invalid names for nil input, got %v", invalid)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "areas.yaml")

	af := &AreasFile{Areas: map[string]Area{
		"auth":     {Description: "Authentication"},
		"database": {Description: "Persistence layer"},
	}}

	if err := Save(path, af); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(loaded.Areas) != 2 {
		t.Fatalf("expected 2 areas after round-trip, got %d", len(loaded.Areas))
	}

	if loaded.Areas["auth"].Description != "Authentication" {
		t.Errorf("unexpected auth description after round-trip: %s", loaded.Areas["auth"].Description)
	}

	if loaded.Areas["database"].Description != "Persistence layer" {
		t.Errorf("unexpected database description after round-trip: %s", loaded.Areas["database"].Description)
	}
}

func TestAreasPath(t *testing.T) {
	got := AreasPath("/home/user/.kerf", "my-project")
	want := filepath.Join("/home/user/.kerf", "projects", "my-project", "areas.yaml")
	if got != want {
		t.Errorf("AreasPath = %q, want %q", got, want)
	}
}

func TestNames(t *testing.T) {
	af := &AreasFile{Areas: map[string]Area{
		"database": {Description: "DB"},
		"auth":     {Description: "Auth"},
		"api":      {Description: "API"},
	}}

	names := Names(af)
	expected := []string{"api", "auth", "database"}

	if len(names) != len(expected) {
		t.Fatalf("expected %d names, got %d", len(expected), len(names))
	}

	for i, name := range names {
		if name != expected[i] {
			t.Errorf("names[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestNamesEmpty(t *testing.T) {
	af := &AreasFile{Areas: make(map[string]Area)}

	names := Names(af)
	if len(names) != 0 {
		t.Fatalf("expected empty names, got %v", names)
	}
}
