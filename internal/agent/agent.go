// Package agent provides parsing and conversion functions for agent definition files.
// Agents are authored as Markdown files with YAML frontmatter (the canonical format)
// and converted to tool-specific formats (e.g., Codex TOML) at deploy time.
package agent

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// Parse splits a canonical agent .md file into YAML frontmatter fields and a
// Markdown body. The frontmatter is delimited by "---" lines. If no frontmatter
// is present, an empty map is returned with the entire content as body.
func Parse(data []byte) (frontmatter map[string]any, body string, err error) {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")

	// Must start with "---"
	if !strings.HasPrefix(s, "---") {
		return map[string]any{}, s, nil
	}

	// Find the closing "---"
	rest := s[3:]
	// Skip the newline after opening ---
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}

	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		// No closing delimiter — treat entire content as body.
		return map[string]any{}, s, nil
	}

	yamlContent := rest[:idx]
	afterClose := rest[idx+4:] // skip "\n---"

	// Strip leading newline from body
	if len(afterClose) > 0 && afterClose[0] == '\n' {
		afterClose = afterClose[1:]
	}

	frontmatter = make(map[string]any)
	if err := yaml.Unmarshal([]byte(yamlContent), &frontmatter); err != nil {
		return nil, "", fmt.Errorf("parsing frontmatter YAML: %w", err)
	}
	if frontmatter == nil {
		frontmatter = make(map[string]any)
	}

	return frontmatter, afterClose, nil
}

// ToMarkdown renders frontmatter and body back to the canonical Markdown format.
func ToMarkdown(frontmatter map[string]any, body string) ([]byte, error) {
	var buf bytes.Buffer

	if len(frontmatter) > 0 {
		buf.WriteString("---\n")
		yamlBytes, err := yaml.Marshal(frontmatter)
		if err != nil {
			return nil, fmt.Errorf("marshaling frontmatter: %w", err)
		}
		buf.Write(yamlBytes)
		buf.WriteString("---\n")
	}

	if body != "" {
		buf.WriteString(body)
	}

	return buf.Bytes(), nil
}

// ToCodexTOML converts a canonical agent (frontmatter + body) to Codex TOML format.
// The body becomes `developer_instructions`. All frontmatter fields are passed
// through as top-level TOML keys.
func ToCodexTOML(frontmatter map[string]any, body string) ([]byte, error) {
	out := make(map[string]any)

	// Copy all frontmatter fields to TOML output.
	for k, v := range frontmatter {
		out[k] = v
	}

	if body != "" {
		out["developer_instructions"] = body
	}

	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(out); err != nil {
		return nil, fmt.Errorf("encoding Codex TOML: %w", err)
	}
	return buf.Bytes(), nil
}

// FromCodexTOML parses a Codex TOML agent file and returns frontmatter fields
// and body (extracted from developer_instructions).
func FromCodexTOML(data []byte) (frontmatter map[string]any, body string, err error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, "", fmt.Errorf("parsing Codex TOML: %w", err)
	}

	frontmatter = make(map[string]any)

	// Extract developer_instructions as body.
	if di, ok := raw["developer_instructions"]; ok {
		s, ok := di.(string)
		if !ok {
			return nil, "", fmt.Errorf("developer_instructions must be a string, got %T", di)
		}
		body = s
		delete(raw, "developer_instructions")
	}

	// Copy remaining fields to frontmatter.
	for k, v := range raw {
		frontmatter[k] = v
	}

	return frontmatter, body, nil
}
