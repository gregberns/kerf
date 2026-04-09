package jig

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed builtin/*.md
var builtinFS embed.FS

// JigDefinition represents a parsed jig file.
type JigDefinition struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Version      int      `yaml:"version"`
	StatusValues []string `yaml:"status_values"`
	Passes       []Pass   `yaml:"passes"`
	FileStructure []string `yaml:"file_structure"`
	Body         string   `yaml:"-"`
}

// Pass represents a single pass within a jig.
type Pass struct {
	Name   string   `yaml:"name"`
	Status string   `yaml:"status"`
	Output []string `yaml:"output"`
}

// JigSummary is a brief representation for listing jigs.
type JigSummary struct {
	Name        string
	Description string
	Version     int
	Source      string // "user" or "built-in"
}

// Parse parses a jig file (YAML frontmatter + markdown body) into a JigDefinition.
func Parse(content []byte) (*JigDefinition, error) {
	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var jig JigDefinition
	if err := yaml.Unmarshal(frontmatter, &jig); err != nil {
		return nil, fmt.Errorf("invalid jig frontmatter: %w", err)
	}

	if jig.Name == "" {
		return nil, fmt.Errorf("jig missing required field: name")
	}
	if len(jig.StatusValues) == 0 {
		return nil, fmt.Errorf("jig missing required field: status_values")
	}
	if len(jig.Passes) == 0 {
		return nil, fmt.Errorf("jig missing required field: passes")
	}

	jig.Body = body
	return &jig, nil
}

// splitFrontmatter splits YAML frontmatter delimited by "---" from the markdown body.
func splitFrontmatter(content []byte) ([]byte, string, error) {
	s := string(content)
	s = strings.TrimLeft(s, "\n\r ")

	if !strings.HasPrefix(s, "---") {
		return nil, "", fmt.Errorf("jig file must start with YAML frontmatter (---)")
	}

	// Find the closing ---
	rest := s[3:]
	rest = strings.TrimLeft(rest, " ")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, "", fmt.Errorf("jig file missing closing frontmatter delimiter (---)")
	}

	fm := rest[:idx]
	after := rest[idx+4:] // skip "\n---"
	// Trim leading newline from body
	after = strings.TrimLeft(after, "\r\n")

	return []byte(fm), after, nil
}

// PassForStatus returns the pass associated with the given status value, or nil if not found.
func (j *JigDefinition) PassForStatus(status string) *Pass {
	for i := range j.Passes {
		if j.Passes[i].Status == status {
			return &j.Passes[i]
		}
	}
	return nil
}

// TerminalStatus returns the last value in status_values.
func (j *JigDefinition) TerminalStatus() string {
	if len(j.StatusValues) == 0 {
		return ""
	}
	return j.StatusValues[len(j.StatusValues)-1]
}

// IsAtOrPastTerminal returns true if the given status is at or past the terminal status
// in the status_values ordering. Statuses not in the list are considered past terminal.
func (j *JigDefinition) IsAtOrPastTerminal(status string) bool {
	if len(j.StatusValues) == 0 {
		return false
	}
	terminal := j.StatusValues[len(j.StatusValues)-1]
	for _, sv := range j.StatusValues {
		if sv == status {
			return sv == terminal
		}
	}
	// Status not in list — considered past terminal (orchestrator-defined)
	return true
}

// ExpandComponents expands {component} placeholders in file structure entries
// using the provided component names.
func ExpandComponents(fileStructure []string, components []string) []string {
	var result []string
	for _, entry := range fileStructure {
		if strings.Contains(entry, "{component}") {
			for _, comp := range components {
				result = append(result, strings.ReplaceAll(entry, "{component}", comp))
			}
		} else {
			result = append(result, entry)
		}
	}
	return result
}

// InstructionsForPass extracts the markdown section for a given pass name from the body.
// It looks for a heading containing the pass name and returns everything until the next
// heading of equal or higher level.
func (j *JigDefinition) InstructionsForPass(passName string) string {
	lines := strings.Split(j.Body, "\n")
	var capturing bool
	var captureLevel int
	var buf bytes.Buffer

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for _, c := range trimmed {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			headingText := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))

			if capturing {
				// Stop if we hit a heading at the same or higher level
				if level <= captureLevel {
					break
				}
			}

			if !capturing && strings.Contains(headingText, passName) {
				capturing = true
				captureLevel = level
			}
		}

		if capturing {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}

	return strings.TrimSpace(buf.String())
}

// VersionMismatch returns true if the jig's version differs from the recorded spec version.
func (j *JigDefinition) VersionMismatch(specVersion int) bool {
	return j.Version != specVersion
}

// Resolve resolves a jig by name. It checks user-level jigs first, then built-in.
// Returns the parsed jig, the source ("user" or "built-in"), and any error.
func Resolve(name string, userJigsDir string) (*JigDefinition, string, error) {
	// 1. Check user-level jigs
	if userJigsDir != "" {
		userPath := filepath.Join(userJigsDir, name+".md")
		if data, err := os.ReadFile(userPath); err == nil {
			jig, err := Parse(data)
			if err != nil {
				return nil, "", fmt.Errorf("user jig %q is invalid: %w", name, err)
			}
			return jig, "user", nil
		}
	}

	// 2. Check built-in jigs
	builtinPath := "builtin/" + name + ".md"
	data, err := builtinFS.ReadFile(builtinPath)
	if err != nil {
		return nil, "", fmt.Errorf("jig %q not found", name)
	}

	jig, err := Parse(data)
	if err != nil {
		return nil, "", fmt.Errorf("built-in jig %q is invalid: %w", name, err)
	}
	return jig, "built-in", nil
}

// ListAll enumerates all available jigs from user-level and built-in sources.
// User jigs override built-in jigs of the same name.
func ListAll(userJigsDir string) ([]JigSummary, error) {
	seen := make(map[string]bool)
	var summaries []JigSummary

	// User jigs first (they take priority)
	if userJigsDir != "" {
		entries, err := os.ReadDir(userJigsDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				name := strings.TrimSuffix(e.Name(), ".md")
				data, err := os.ReadFile(filepath.Join(userJigsDir, e.Name()))
				if err != nil {
					continue
				}
				jig, err := Parse(data)
				if err != nil {
					continue
				}
				seen[name] = true
				summaries = append(summaries, JigSummary{
					Name:        jig.Name,
					Description: jig.Description,
					Version:     jig.Version,
					Source:      "user",
				})
			}
		}
	}

	// Built-in jigs
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return summaries, nil
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		if seen[name] {
			continue // user override takes precedence
		}
		data, err := builtinFS.ReadFile("builtin/" + e.Name())
		if err != nil {
			continue
		}
		jig, err := Parse(data)
		if err != nil {
			continue
		}
		summaries = append(summaries, JigSummary{
			Name:        jig.Name,
			Description: jig.Description,
			Version:     jig.Version,
			Source:      "built-in",
		})
	}

	return summaries, nil
}

// ReadBuiltinRaw returns the raw content of a built-in jig file.
func ReadBuiltinRaw(name string) ([]byte, error) {
	return builtinFS.ReadFile("builtin/" + name + ".md")
}

// SaveToUser writes a jig file to the user's jigs directory.
func SaveToUser(name string, content []byte, userJigsDir string) error {
	if err := os.MkdirAll(userJigsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create jigs directory: %w", err)
	}
	path := filepath.Join(userJigsDir, name+".md")
	return os.WriteFile(path, content, 0o644)
}
