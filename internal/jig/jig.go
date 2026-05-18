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

//go:embed builtin/templates
var templatesFS embed.FS

// JigDefinition represents a parsed jig file.
type JigDefinition struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	Version       int      `yaml:"version"`
	Aliases       []string `yaml:"aliases"`
	Phase         string   `yaml:"phase"`
	Tools         []string `yaml:"tools"`
	Composable    bool     `yaml:"composable"`
	StatusValues  []string `yaml:"status_values"`
	Passes        []Pass   `yaml:"passes"`
	FileStructure []string `yaml:"file_structure"`
	Body          string   `yaml:"-"`
}

// Pass represents a single pass within a jig.
type Pass struct {
	Name   string   `yaml:"name"`
	Status string   `yaml:"status"`
	Output []string `yaml:"output"`
	Tools  []string `yaml:"tools"`
}

// JigSummary is a brief representation for listing jigs.
type JigSummary struct {
	Name        string
	Description string
	Version     int
	Source      string // "user" or "built-in"
	Aliases     []string
	Phase       string
	Tools       []string
	Composable  bool
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

// ReviewForPass extracts the "Done when reviewer approves on:" block for the
// named pass from the jig's markdown body. It returns the block text (without
// the bold heading) and a boolean indicating whether the pass declares review
// criteria. Passes without a review block (e.g., terminal "ready") return ok=false.
//
// The block ends at the first of: a blank-line-delimited paragraph beginning
// with "Review follows the protocol" (the per-pass review-protocol trailer),
// a new markdown heading, or a new bold section heading on its own line
// (e.g., "**What to do:**").
func (j *JigDefinition) ReviewForPass(passName string) (string, bool) {
	section := j.InstructionsForPass(passName)
	if section == "" {
		return "", false
	}
	const marker = "**Done when reviewer approves on:**"
	idx := strings.Index(section, marker)
	if idx < 0 {
		return "", false
	}
	rest := section[idx+len(marker):]
	lines := strings.Split(rest, "\n")
	var buf bytes.Buffer
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			break
		}
		if strings.HasPrefix(trimmed, "Review follows the protocol") {
			break
		}
		// A subsequent **Bold heading:** line ends the block.
		if strings.HasPrefix(trimmed, "**") && strings.HasSuffix(trimmed, ":**") {
			break
		}
		buf.WriteString(line)
		buf.WriteString("\n")
	}
	body := strings.TrimSpace(buf.String())
	if body == "" {
		return "", false
	}
	return body, true
}

// VersionMismatch returns true if the jig's version differs from the recorded spec version.
func (j *JigDefinition) VersionMismatch(specVersion int) bool {
	return j.Version != specVersion
}

// Resolve resolves a jig by name. It checks user-level jigs first, then built-in
// by filename, then built-in by alias.
// Returns the parsed jig, the source ("user" or "built-in"), and any error.
func Resolve(name string, userJigsDir string) (*JigDefinition, string, error) {
	// 1. Check user-level jigs by filename
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

	// 2. Check built-in jigs by filename
	builtinPath := "builtin/" + name + ".md"
	if data, err := builtinFS.ReadFile(builtinPath); err == nil {
		jig, err := Parse(data)
		if err != nil {
			return nil, "", fmt.Errorf("built-in jig %q is invalid: %w", name, err)
		}
		return jig, "built-in", nil
	}

	// 3. Check built-in jigs by alias
	if j, err := resolveBuiltinAlias(name); err == nil {
		return j, "built-in", nil
	}

	return nil, "", fmt.Errorf("jig %q not found", name)
}

// resolveBuiltinAlias scans all built-in jigs for one whose aliases include the given name.
func resolveBuiltinAlias(name string) (*JigDefinition, error) {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := builtinFS.ReadFile("builtin/" + e.Name())
		if err != nil {
			continue
		}
		j, err := Parse(data)
		if err != nil {
			continue
		}
		for _, alias := range j.Aliases {
			if alias == name {
				return j, nil
			}
		}
	}
	return nil, fmt.Errorf("no built-in jig has alias %q", name)
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
					Phase:       jig.Phase,
					Tools:       jig.Tools,
					Composable:  jig.Composable,
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
			Aliases:     jig.Aliases,
			Phase:       jig.Phase,
			Tools:       jig.Tools,
			Composable:  jig.Composable,
		})
	}

	return summaries, nil
}

// ReadBuiltinRaw returns the raw content of a built-in jig file.
// If no file matches by filename, it resolves aliases and returns the
// canonical jig's content.
func ReadBuiltinRaw(name string) ([]byte, error) {
	// Try direct filename first.
	if data, err := builtinFS.ReadFile("builtin/" + name + ".md"); err == nil {
		return data, nil
	}

	// Try alias resolution.
	j, err := resolveBuiltinAlias(name)
	if err != nil {
		return nil, fmt.Errorf("built-in jig %q not found", name)
	}
	return builtinFS.ReadFile("builtin/" + j.Name + ".md")
}

// TemplateFor returns the template body for the given jig name and template
// basename (e.g., "01-problem-space.md.template"). It returns os.ErrNotExist
// if the jig has no template by that name.
func TemplateFor(jigName, templateName string) ([]byte, error) {
	path := "builtin/templates/" + jigName + "/" + templateName
	data, err := templatesFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("template %q for jig %q: %w", templateName, jigName, err)
	}
	return data, nil
}

// TemplateForPass returns the template body for a given jig name and pass index
// (1-based). The mapping follows the convention in specs/jig-spec.md: pass N's
// template is named NN-<slug>.md.template where the slug is derived from the
// pass's first output path with directory separators replaced by dots. If the
// pass has no template (e.g., terminal "ready" pass), os.ErrNotExist is returned.
func TemplateForPass(jigName string, pass *Pass) ([]byte, error) {
	if pass == nil || len(pass.Output) == 0 {
		return nil, os.ErrNotExist
	}
	name := templateBasenameForOutput(pass.Output[0])
	if name == "" {
		return nil, os.ErrNotExist
	}
	return TemplateFor(jigName, name)
}

// templateBasenameForOutput turns a pass output path like
// "03-research/{component}/findings.md" into "03-research.findings.md.template"
// by removing {component} segments and joining the remaining path components
// with dots.
func templateBasenameForOutput(output string) string {
	parts := strings.Split(output, "/")
	var keep []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		if p == "{component}" {
			// Drop pure {component} directory segments entirely.
			continue
		}
		// Within other segments, replace embedded {component} with the literal
		// "component" so the template basename is stable on disk.
		keep = append(keep, strings.ReplaceAll(p, "{component}", "component"))
	}
	if len(keep) == 0 {
		return ""
	}
	return strings.Join(keep, ".") + ".template"
}

// ListTemplates returns the names of all template files shipped for the given
// jig (e.g., "01-problem-space.md.template").
func ListTemplates(jigName string) ([]string, error) {
	entries, err := templatesFS.ReadDir("builtin/templates/" + jigName)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
}

// SaveToUser writes a jig file to the user's jigs directory.
func SaveToUser(name string, content []byte, userJigsDir string) error {
	if err := os.MkdirAll(userJigsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create jigs directory: %w", err)
	}
	path := filepath.Join(userJigsDir, name+".md")
	return os.WriteFile(path, content, 0o644)
}
