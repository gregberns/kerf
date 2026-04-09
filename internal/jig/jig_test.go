package jig

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePlanJig(t *testing.T) {
	data, err := builtinFS.ReadFile("builtin/plan.md")
	if err != nil {
		t.Fatalf("failed to read built-in plan jig: %v", err)
	}

	jig, err := Parse(data)
	if err != nil {
		t.Fatalf("failed to parse plan jig: %v", err)
	}

	if jig.Name != "plan" {
		t.Errorf("Name = %q, want %q", jig.Name, "plan")
	}
	if jig.Version != 1 {
		t.Errorf("Version = %d, want %d", jig.Version, 1)
	}
	if len(jig.StatusValues) != 8 {
		t.Errorf("StatusValues count = %d, want %d", len(jig.StatusValues), 8)
	}
	if len(jig.Passes) != 8 {
		t.Errorf("Passes count = %d, want %d", len(jig.Passes), 8)
	}
	if jig.Body == "" {
		t.Error("Body is empty, expected markdown content")
	}

	// Verify aliases
	if len(jig.Aliases) != 1 || jig.Aliases[0] != "feature" {
		t.Errorf("Aliases = %v, want [feature]", jig.Aliases)
	}

	// Verify pass-status mapping
	if jig.Passes[0].Status != "problem-space" {
		t.Errorf("Pass[0].Status = %q, want %q", jig.Passes[0].Status, "problem-space")
	}
	if jig.Passes[3].Status != "research" {
		t.Errorf("Pass[3].Status = %q, want %q", jig.Passes[3].Status, "research")
	}

	// Verify file structure includes component placeholders
	hasComponentPlaceholder := false
	for _, f := range jig.FileStructure {
		if f == "04-research/{component}/findings.md" {
			hasComponentPlaceholder = true
			break
		}
	}
	if !hasComponentPlaceholder {
		t.Error("FileStructure missing component placeholder entry")
	}
}

func TestParseSpecJig(t *testing.T) {
	data, err := builtinFS.ReadFile("builtin/spec.md")
	if err != nil {
		t.Fatalf("failed to read built-in spec jig: %v", err)
	}

	jig, err := Parse(data)
	if err != nil {
		t.Fatalf("failed to parse spec jig: %v", err)
	}

	if jig.Name != "spec" {
		t.Errorf("Name = %q, want %q", jig.Name, "spec")
	}
	if jig.Version != 1 {
		t.Errorf("Version = %d, want %d", jig.Version, 1)
	}
	if len(jig.StatusValues) != 8 {
		t.Errorf("StatusValues count = %d, want %d", len(jig.StatusValues), 8)
	}
	if len(jig.Passes) != 8 {
		t.Errorf("Passes count = %d, want %d", len(jig.Passes), 8)
	}
	if len(jig.Aliases) != 0 {
		t.Errorf("Aliases = %v, want empty", jig.Aliases)
	}
}

func TestParseBugJig(t *testing.T) {
	data, err := builtinFS.ReadFile("builtin/bug.md")
	if err != nil {
		t.Fatalf("failed to read built-in bug jig: %v", err)
	}

	jig, err := Parse(data)
	if err != nil {
		t.Fatalf("failed to parse bug jig: %v", err)
	}

	if jig.Name != "bug" {
		t.Errorf("Name = %q, want %q", jig.Name, "bug")
	}
	if jig.Version != 2 {
		t.Errorf("Version = %d, want %d", jig.Version, 2)
	}
	if len(jig.StatusValues) != 6 {
		t.Errorf("StatusValues count = %d, want %d", len(jig.StatusValues), 6)
	}
	if len(jig.Passes) != 6 {
		t.Errorf("Passes count = %d, want %d", len(jig.Passes), 6)
	}

	// Fix Spec pass (index 4) has one output
	fixSpecPass := jig.Passes[4]
	if fixSpecPass.Status != "fix-spec" {
		t.Errorf("Pass[4].Status = %q, want %q", fixSpecPass.Status, "fix-spec")
	}
	if len(fixSpecPass.Output) != 1 {
		t.Errorf("Fix Spec pass output count = %d, want %d", len(fixSpecPass.Output), 1)
	}

	// Ready pass (last) has no output
	readyPass := jig.Passes[5]
	if len(readyPass.Output) != 0 {
		t.Errorf("Ready pass output count = %d, want %d", len(readyPass.Output), 0)
	}
}

func TestParseMalformedInput(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"no frontmatter", "# Just markdown\nNo frontmatter here."},
		{"missing closing delimiter", "---\nname: test\nstatus_values:\n  - foo\n"},
		{"missing name", "---\ndescription: no name\nstatus_values:\n  - x\npasses:\n  - name: p\n    status: x\n    output: [f.md]\n---\n"},
		{"missing status_values", "---\nname: test\npasses:\n  - name: p\n    status: x\n    output: [f.md]\n---\n"},
		{"missing passes", "---\nname: test\nstatus_values:\n  - x\n---\n"},
		{"invalid yaml", "---\nname: [invalid\n---\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.content))
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestPassForStatus(t *testing.T) {
	jig := &JigDefinition{
		Passes: []Pass{
			{Name: "First", Status: "alpha"},
			{Name: "Second", Status: "beta"},
			{Name: "Third", Status: "gamma"},
		},
	}

	p := jig.PassForStatus("beta")
	if p == nil {
		t.Fatal("expected pass, got nil")
	}
	if p.Name != "Second" {
		t.Errorf("Name = %q, want %q", p.Name, "Second")
	}

	if jig.PassForStatus("nonexistent") != nil {
		t.Error("expected nil for nonexistent status")
	}
}

func TestTerminalStatus(t *testing.T) {
	jig := &JigDefinition{
		StatusValues: []string{"a", "b", "c"},
	}
	if got := jig.TerminalStatus(); got != "c" {
		t.Errorf("TerminalStatus() = %q, want %q", got, "c")
	}

	empty := &JigDefinition{}
	if got := empty.TerminalStatus(); got != "" {
		t.Errorf("TerminalStatus() = %q, want %q", got, "")
	}
}

func TestIsAtOrPastTerminal(t *testing.T) {
	jig := &JigDefinition{
		StatusValues: []string{"a", "b", "c"},
	}

	tests := []struct {
		status string
		want   bool
	}{
		{"a", false},
		{"b", false},
		{"c", true},             // terminal
		{"implementing", true},  // past terminal (not in list)
		{"done", true},          // past terminal (not in list)
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := jig.IsAtOrPastTerminal(tt.status); got != tt.want {
				t.Errorf("IsAtOrPastTerminal(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}

	empty := &JigDefinition{}
	if empty.IsAtOrPastTerminal("x") {
		t.Error("expected false for empty status_values")
	}
}

func TestExpandComponents(t *testing.T) {
	fs := []string{
		"spec.yaml",
		"SESSION.md",
		"03-research/{component}/findings.md",
		"04-plans/{component}-spec.md",
		"SPEC.md",
	}
	components := []string{"parser", "resolver", "emitter"}

	got := ExpandComponents(fs, components)
	want := []string{
		"spec.yaml",
		"SESSION.md",
		"03-research/parser/findings.md",
		"03-research/resolver/findings.md",
		"03-research/emitter/findings.md",
		"04-plans/parser-spec.md",
		"04-plans/resolver-spec.md",
		"04-plans/emitter-spec.md",
		"SPEC.md",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandComponents:\ngot  %v\nwant %v", got, want)
	}
}

func TestExpandComponentsNoPlaceholders(t *testing.T) {
	fs := []string{"spec.yaml", "SESSION.md", "01-triage.md"}
	got := ExpandComponents(fs, []string{"anything"})
	if !reflect.DeepEqual(got, fs) {
		t.Errorf("expected no change without placeholders, got %v", got)
	}
}

func TestExpandComponentsEmpty(t *testing.T) {
	fs := []string{"03-research/{component}/findings.md"}
	got := ExpandComponents(fs, nil)
	// No components means placeholder entries are dropped
	if len(got) != 0 {
		t.Errorf("expected empty result when no components, got %v", got)
	}
}

func TestInstructionsForPass(t *testing.T) {
	data, err := builtinFS.ReadFile("builtin/plan.md")
	if err != nil {
		t.Fatalf("failed to read built-in plan jig: %v", err)
	}
	jig, err := Parse(data)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	instructions := jig.InstructionsForPass("Problem Space")
	if instructions == "" {
		t.Fatal("expected instructions for Problem Space pass, got empty")
	}
	if !contains(instructions, "problem-space") {
		t.Error("instructions should contain 'problem-space'")
	}
	if !contains(instructions, "01-problem-space.md") {
		t.Error("instructions should reference output file")
	}

	// Nonexistent pass
	if jig.InstructionsForPass("Nonexistent") != "" {
		t.Error("expected empty for nonexistent pass")
	}
}

func TestVersionMismatch(t *testing.T) {
	jig := &JigDefinition{Version: 2}

	if jig.VersionMismatch(2) {
		t.Error("same version should not be a mismatch")
	}
	if !jig.VersionMismatch(1) {
		t.Error("different version should be a mismatch")
	}
	if !jig.VersionMismatch(3) {
		t.Error("different version should be a mismatch")
	}
}

func TestResolveBuiltin(t *testing.T) {
	jig, source, err := Resolve("plan", "")
	if err != nil {
		t.Fatalf("Resolve(plan) error: %v", err)
	}
	if source != "built-in" {
		t.Errorf("source = %q, want %q", source, "built-in")
	}
	if jig.Name != "plan" {
		t.Errorf("Name = %q, want %q", jig.Name, "plan")
	}

	jig, source, err = Resolve("bug", "")
	if err != nil {
		t.Fatalf("Resolve(bug) error: %v", err)
	}
	if source != "built-in" {
		t.Errorf("source = %q, want %q", source, "built-in")
	}
	if jig.Name != "bug" {
		t.Errorf("Name = %q, want %q", jig.Name, "bug")
	}

	jig, source, err = Resolve("spec", "")
	if err != nil {
		t.Fatalf("Resolve(spec) error: %v", err)
	}
	if source != "built-in" {
		t.Errorf("source = %q, want %q", source, "built-in")
	}
	if jig.Name != "spec" {
		t.Errorf("Name = %q, want %q", jig.Name, "spec")
	}
}

func TestResolveAlias(t *testing.T) {
	// "feature" is an alias for "plan"
	jig, source, err := Resolve("feature", "")
	if err != nil {
		t.Fatalf("Resolve(feature) error: %v", err)
	}
	if source != "built-in" {
		t.Errorf("source = %q, want %q", source, "built-in")
	}
	if jig.Name != "plan" {
		t.Errorf("Name = %q, want %q (canonical name, not alias)", jig.Name, "plan")
	}
}

func TestResolveAliasUserOverride(t *testing.T) {
	// A user-level jig named "feature" takes priority over the alias
	dir := t.TempDir()
	userContent := []byte(`---
name: feature
description: User feature jig
version: 42
status_values:
  - start
  - end
passes:
  - name: "Start"
    status: start
    output: ["out.md"]
---

# User feature
`)
	if err := os.WriteFile(filepath.Join(dir, "feature.md"), userContent, 0o644); err != nil {
		t.Fatal(err)
	}

	jig, source, err := Resolve("feature", dir)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if source != "user" {
		t.Errorf("source = %q, want %q (user-level should take priority over alias)", source, "user")
	}
	if jig.Version != 42 {
		t.Errorf("Version = %d, want %d", jig.Version, 42)
	}
}

func TestReadBuiltinRawAlias(t *testing.T) {
	// ReadBuiltinRaw("feature") should return plan jig content
	data, err := ReadBuiltinRaw("feature")
	if err != nil {
		t.Fatalf("ReadBuiltinRaw(feature) error: %v", err)
	}

	jig, err := Parse(data)
	if err != nil {
		t.Fatalf("failed to parse returned content: %v", err)
	}
	if jig.Name != "plan" {
		t.Errorf("Name = %q, want %q", jig.Name, "plan")
	}
}

func TestResolveNotFound(t *testing.T) {
	_, _, err := Resolve("nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent jig")
	}
}

func TestResolveUserOverride(t *testing.T) {
	dir := t.TempDir()

	// Write a user jig that overrides the built-in plan jig
	userContent := []byte(`---
name: plan
description: Custom plan jig
version: 99
status_values:
  - custom-start
  - custom-end
passes:
  - name: "Custom Pass"
    status: custom-start
    output: ["custom.md"]
file_structure:
  - spec.yaml
  - custom.md
---

# Custom Plan Jig
`)
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), userContent, 0o644); err != nil {
		t.Fatal(err)
	}

	jig, source, err := Resolve("plan", dir)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if source != "user" {
		t.Errorf("source = %q, want %q", source, "user")
	}
	if jig.Version != 99 {
		t.Errorf("Version = %d, want %d (user override)", jig.Version, 99)
	}
}

func TestResolveUserFallbackToBuiltin(t *testing.T) {
	dir := t.TempDir()
	// User dir exists but has no plan.md — should fall back to built-in
	jig, source, err := Resolve("plan", dir)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if source != "built-in" {
		t.Errorf("source = %q, want %q", source, "built-in")
	}
	if jig.Name != "plan" {
		t.Errorf("Name = %q, want %q", jig.Name, "plan")
	}
}

func TestListAllBuiltinOnly(t *testing.T) {
	summaries, err := ListAll("")
	if err != nil {
		t.Fatalf("ListAll error: %v", err)
	}

	if len(summaries) != 3 {
		t.Fatalf("expected 3 built-in jigs, got %d", len(summaries))
	}

	byName := make(map[string]JigSummary)
	for _, s := range summaries {
		byName[s.Name] = s
	}

	if s, ok := byName["plan"]; !ok {
		t.Error("expected plan jig from built-in source")
	} else if s.Source != "built-in" {
		t.Errorf("plan source = %q, want %q", s.Source, "built-in")
	} else if len(s.Aliases) != 1 || s.Aliases[0] != "feature" {
		t.Errorf("plan aliases = %v, want [feature]", s.Aliases)
	}

	if s, ok := byName["spec"]; !ok {
		t.Error("expected spec jig from built-in source")
	} else if s.Source != "built-in" {
		t.Errorf("spec source = %q, want %q", s.Source, "built-in")
	}

	if s, ok := byName["bug"]; !ok {
		t.Error("expected bug jig from built-in source")
	} else if s.Source != "built-in" {
		t.Errorf("bug source = %q, want %q", s.Source, "built-in")
	}
}

func TestListAllMixedSources(t *testing.T) {
	dir := t.TempDir()

	// Create a user jig that overrides plan
	userContent := []byte(`---
name: plan
description: User plan
version: 5
status_values:
  - start
  - end
passes:
  - name: "P1"
    status: start
    output: ["out.md"]
---

# User plan
`)
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), userContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a user-only jig
	customContent := []byte(`---
name: custom
description: A custom jig
version: 1
status_values:
  - doing
  - done
passes:
  - name: "Do"
    status: doing
    output: ["result.md"]
---

# Custom jig
`)
	if err := os.WriteFile(filepath.Join(dir, "custom.md"), customContent, 0o644); err != nil {
		t.Fatal(err)
	}

	summaries, err := ListAll(dir)
	if err != nil {
		t.Fatalf("ListAll error: %v", err)
	}

	byName := make(map[string]JigSummary)
	for _, s := range summaries {
		byName[s.Name] = s
	}

	// plan should come from user (override)
	if s, ok := byName["plan"]; !ok {
		t.Error("missing plan jig")
	} else if s.Source != "user" {
		t.Errorf("plan source = %q, want %q", s.Source, "user")
	} else if s.Version != 5 {
		t.Errorf("plan version = %d, want %d", s.Version, 5)
	}

	// bug should come from built-in
	if s, ok := byName["bug"]; !ok {
		t.Error("missing bug jig")
	} else if s.Source != "built-in" {
		t.Errorf("bug source = %q, want %q", s.Source, "built-in")
	}

	// custom should come from user
	if s, ok := byName["custom"]; !ok {
		t.Error("missing custom jig")
	} else if s.Source != "user" {
		t.Errorf("custom source = %q, want %q", s.Source, "user")
	}
}

func TestSaveToUser(t *testing.T) {
	dir := t.TempDir()
	jigsDir := filepath.Join(dir, "jigs")

	content := []byte(`---
name: saved
description: A saved jig
version: 1
status_values:
  - draft
  - done
passes:
  - name: "Draft"
    status: draft
    output: ["draft.md"]
---

# Saved jig
`)
	if err := SaveToUser("saved", content, jigsDir); err != nil {
		t.Fatalf("SaveToUser error: %v", err)
	}

	// Verify file was written
	readBack, err := os.ReadFile(filepath.Join(jigsDir, "saved.md"))
	if err != nil {
		t.Fatalf("failed to read saved jig: %v", err)
	}

	// Verify it parses
	jig, err := Parse(readBack)
	if err != nil {
		t.Fatalf("saved jig doesn't parse: %v", err)
	}
	if jig.Name != "saved" {
		t.Errorf("Name = %q, want %q", jig.Name, "saved")
	}
}

func TestSaveToUserCreatesDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "jigs")

	content := []byte(`---
name: test
description: test
version: 1
status_values: [x]
passes:
  - name: P
    status: x
    output: [f.md]
---
body
`)
	if err := SaveToUser("test", content, nested); err != nil {
		t.Fatalf("SaveToUser error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(nested, "test.md")); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}
}

func TestParseMinimalValid(t *testing.T) {
	content := []byte(`---
name: minimal
description: A minimal jig
version: 1
status_values:
  - active
  - done
passes:
  - name: "Work"
    status: active
    output: ["output.md"]
---

# Minimal Jig

Do the work.
`)
	jig, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if jig.Name != "minimal" {
		t.Errorf("Name = %q, want %q", jig.Name, "minimal")
	}
	if jig.TerminalStatus() != "done" {
		t.Errorf("TerminalStatus = %q, want %q", jig.TerminalStatus(), "done")
	}
}

func TestListAllNonexistentUserDir(t *testing.T) {
	summaries, err := ListAll("/nonexistent/path")
	if err != nil {
		t.Fatalf("ListAll error: %v", err)
	}
	// Should still return built-in jigs
	if len(summaries) != 3 {
		t.Errorf("expected 3 jigs from built-in, got %d", len(summaries))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsCheck(s, substr))
}

func containsCheck(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
