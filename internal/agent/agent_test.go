package agent

import (
	"strings"
	"testing"
)

func TestParseWithFrontmatter(t *testing.T) {
	input := `---
name: code-reviewer
description: Reviews code
---
You are a code reviewer.
`
	fm, body, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if fm["name"] != "code-reviewer" {
		t.Errorf("name = %v", fm["name"])
	}
	if fm["description"] != "Reviews code" {
		t.Errorf("description = %v", fm["description"])
	}
	if body != "You are a code reviewer.\n" {
		t.Errorf("body = %q", body)
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	input := `Just a plain markdown body.`
	fm, body, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}
	if body != input {
		t.Errorf("body = %q", body)
	}
}

func TestParseEmptyBody(t *testing.T) {
	input := `---
name: empty
---
`
	fm, body, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if fm["name"] != "empty" {
		t.Errorf("name = %v", fm["name"])
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

func TestParseMultilineBody(t *testing.T) {
	input := "---\nname: test\n---\nLine 1\n\nLine 2 with `backticks` and ---\n"
	fm, body, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if fm["name"] != "test" {
		t.Errorf("name = %v", fm["name"])
	}
	if body != "Line 1\n\nLine 2 with `backticks` and ---\n" {
		t.Errorf("body = %q", body)
	}
}

func TestParseToolSpecificFields(t *testing.T) {
	input := `---
name: reviewer
description: Code reviewer
tools: Read, Grep
sandbox_mode: full
model_reasoning_effort: high
---
System prompt here.
`
	fm, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if fm["tools"] != "Read, Grep" {
		t.Errorf("tools = %v", fm["tools"])
	}
	if fm["sandbox_mode"] != "full" {
		t.Errorf("sandbox_mode = %v", fm["sandbox_mode"])
	}
	if fm["model_reasoning_effort"] != "high" {
		t.Errorf("model_reasoning_effort = %v", fm["model_reasoning_effort"])
	}
}

func TestRoundTrip(t *testing.T) {
	input := `---
description: Reviews code
name: code-reviewer
---
You are a code reviewer.
`
	fm, body, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	out, err := ToMarkdown(fm, body)
	if err != nil {
		t.Fatalf("ToMarkdown: %v", err)
	}

	fm2, body2, err := Parse(out)
	if err != nil {
		t.Fatalf("re-Parse: %v", err)
	}
	if fm2["name"] != fm["name"] || fm2["description"] != fm["description"] {
		t.Errorf("frontmatter mismatch: %v vs %v", fm, fm2)
	}
	if body2 != body {
		t.Errorf("body mismatch: %q vs %q", body, body2)
	}
}

func TestToCodexTOML(t *testing.T) {
	fm := map[string]any{
		"name":        "code-reviewer",
		"description": "Reviews code",
		"model":       "sonnet",
		"tools":       "Read, Grep", // Claude-specific, should be dropped
	}
	body := "You are a code reviewer."

	data, err := ToCodexTOML(fm, body)
	if err != nil {
		t.Fatalf("ToCodexTOML: %v", err)
	}

	s := string(data)
	if !strings.Contains(s, `developer_instructions = "You are a code reviewer."`) &&
		!strings.Contains(s, "developer_instructions") {
		t.Errorf("missing developer_instructions in TOML:\n%s", s)
	}
	if !strings.Contains(s, `name = "code-reviewer"`) {
		t.Errorf("missing name in TOML:\n%s", s)
	}
	if !strings.Contains(s, `description = "Reviews code"`) {
		t.Errorf("missing description in TOML:\n%s", s)
	}
	if !strings.Contains(s, `model = "sonnet"`) {
		t.Errorf("missing model in TOML:\n%s", s)
	}
	// All frontmatter fields are passed through (including tool-specific ones like tools).
	if !strings.Contains(s, "tools") {
		t.Errorf("expected 'tools' field to be passed through in Codex TOML:\n%s", s)
	}
}

func TestToCodexTOMLCodexSpecificFields(t *testing.T) {
	fm := map[string]any{
		"name":                    "debugger",
		"description":            "Debugs code",
		"sandbox_mode":           "full",
		"model_reasoning_effort": "high",
		"nickname_candidates":    []any{"dbg", "debug"},
	}
	body := "Debug instructions."

	data, err := ToCodexTOML(fm, body)
	if err != nil {
		t.Fatalf("ToCodexTOML: %v", err)
	}

	s := string(data)
	if !strings.Contains(s, "sandbox_mode") {
		t.Errorf("missing sandbox_mode:\n%s", s)
	}
	if !strings.Contains(s, "model_reasoning_effort") {
		t.Errorf("missing model_reasoning_effort:\n%s", s)
	}
	if !strings.Contains(s, "nickname_candidates") {
		t.Errorf("missing nickname_candidates:\n%s", s)
	}
}

func TestFromCodexTOML(t *testing.T) {
	input := `name = "code-reviewer"
description = "Reviews code"
model = "sonnet"
developer_instructions = "You are a code reviewer."
`
	fm, body, err := FromCodexTOML([]byte(input))
	if err != nil {
		t.Fatalf("FromCodexTOML: %v", err)
	}
	if fm["name"] != "code-reviewer" {
		t.Errorf("name = %v", fm["name"])
	}
	if fm["description"] != "Reviews code" {
		t.Errorf("description = %v", fm["description"])
	}
	if fm["model"] != "sonnet" {
		t.Errorf("model = %v", fm["model"])
	}
	if body != "You are a code reviewer." {
		t.Errorf("body = %q", body)
	}
	// developer_instructions should not be in frontmatter
	if _, ok := fm["developer_instructions"]; ok {
		t.Error("developer_instructions should not be in frontmatter")
	}
}

func TestFromCodexTOMLRoundTrip(t *testing.T) {
	fm := map[string]any{
		"name":        "reviewer",
		"description": "Reviews",
	}
	body := "Review instructions."

	tomlData, err := ToCodexTOML(fm, body)
	if err != nil {
		t.Fatalf("ToCodexTOML: %v", err)
	}

	fm2, body2, err := FromCodexTOML(tomlData)
	if err != nil {
		t.Fatalf("FromCodexTOML: %v", err)
	}

	if fm2["name"] != "reviewer" {
		t.Errorf("name = %v", fm2["name"])
	}
	if fm2["description"] != "Reviews" {
		t.Errorf("description = %v", fm2["description"])
	}
	if body2 != body {
		t.Errorf("body = %q, want %q", body2, body)
	}
}

func TestParseInvalidYAML(t *testing.T) {
	input := "---\n: invalid: yaml: [unterminated\n---\nBody.\n"
	_, _, err := Parse([]byte(input))
	if err == nil {
		t.Fatal("expected error for invalid YAML frontmatter, got nil")
	}
	if !strings.Contains(err.Error(), "parsing frontmatter YAML") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestToMarkdownNoFrontmatter(t *testing.T) {
	out, err := ToMarkdown(map[string]any{}, "Just a body.")
	if err != nil {
		t.Fatalf("ToMarkdown: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "---") {
		t.Errorf("should not have frontmatter delimiters:\n%s", s)
	}
	if s != "Just a body." {
		t.Errorf("output = %q", s)
	}
}

func TestToMarkdownEmptyBody(t *testing.T) {
	out, err := ToMarkdown(map[string]any{"name": "test"}, "")
	if err != nil {
		t.Fatalf("ToMarkdown: %v", err)
	}
	s := string(out)
	if !strings.HasPrefix(s, "---\n") {
		t.Errorf("should start with frontmatter:\n%s", s)
	}
	if !strings.Contains(s, "name: test") {
		t.Errorf("missing name field:\n%s", s)
	}
}
