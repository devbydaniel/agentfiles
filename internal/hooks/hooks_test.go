package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		data := []byte(`{
			"event": "PreToolUse",
			"matcher": "Bash(git *)",
			"hooks": [{"type": "command", "command": "check.sh"}]
		}`)
		hf, err := Parse(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hf.Event != "PreToolUse" {
			t.Errorf("event = %q, want %q", hf.Event, "PreToolUse")
		}
		if hf.Matcher != "Bash(git *)" {
			t.Errorf("matcher = %q, want %q", hf.Matcher, "Bash(git *)")
		}
		if len(hf.Hooks) != 1 {
			t.Errorf("hooks count = %d, want 1", len(hf.Hooks))
		}
	})

	t.Run("no matcher", func(t *testing.T) {
		data := []byte(`{"event": "SessionStart", "hooks": [{"type": "command", "command": "setup.sh"}]}`)
		hf, err := Parse(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hf.Matcher != "" {
			t.Errorf("matcher = %q, want empty", hf.Matcher)
		}
	})

	t.Run("missing event", func(t *testing.T) {
		data := []byte(`{"hooks": [{"type": "command"}]}`)
		_, err := Parse(data)
		if err == nil {
			t.Fatal("expected error for missing event")
		}
	})

	t.Run("missing hooks", func(t *testing.T) {
		data := []byte(`{"event": "PreToolUse"}`)
		_, err := Parse(data)
		if err == nil {
			t.Fatal("expected error for missing hooks")
		}
	})

	t.Run("empty hooks array", func(t *testing.T) {
		data := []byte(`{"event": "PreToolUse", "hooks": []}`)
		_, err := Parse(data)
		if err == nil {
			t.Fatal("expected error for empty hooks array")
		}
	})
}

func TestEventNameForCursor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"PreToolUse", "preToolUse"},
		{"PostToolUse", "postToolUse"},
		{"SessionStart", "sessionStart"},
		{"SessionEnd", "sessionEnd"},
		{"UserPromptSubmit", "beforeSubmitPrompt"},
		{"Stop", "stop"},
		{"CustomEvent", "customEvent"},
		{"", ""},
	}
	for _, tt := range tests {
		got := EventNameForCursor(tt.input)
		if got != tt.want {
			t.Errorf("EventNameForCursor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToFlatEntries(t *testing.T) {
	hf := &HookFile{
		Event:   "PreToolUse",
		Matcher: "Bash(git *)",
		Hooks: []json.RawMessage{
			json.RawMessage(`{"type": "command", "command": "check.sh", "timeout": 10}`),
		},
	}

	entries, err := ToFlatEntries("git-guard", hf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(entries))
	}

	var flat map[string]any
	if err := json.Unmarshal(entries[0], &flat); err != nil {
		t.Fatalf("unmarshal flat entry: %v", err)
	}
	if flat["command"] != "check.sh" {
		t.Errorf("command = %v, want check.sh", flat["command"])
	}
	if flat["matcher"] != "Bash(git *)" {
		t.Errorf("matcher = %v, want Bash(git *)", flat["matcher"])
	}
	if flat["_agentfiles"] != "git-guard" {
		t.Errorf("_agentfiles = %v, want git-guard", flat["_agentfiles"])
	}
}

func TestToFlatEntriesMultiple(t *testing.T) {
	hf := &HookFile{
		Event: "SessionStart",
		Hooks: []json.RawMessage{
			json.RawMessage(`{"type": "command", "command": "a.sh"}`),
			json.RawMessage(`{"type": "command", "command": "b.sh"}`),
		},
	}
	entries, err := ToFlatEntries("setup", hf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries count = %d, want 2", len(entries))
	}
}

func TestMergeIntoSettings_Nested(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")

	// Write existing settings with permissions and a user-defined hook.
	existing := `{
  "permissions": {"allow": ["Bash(*)"]},
  "hooks": {
    "SessionEnd": [
      {"hooks": [{"type": "command", "command": "user-cleanup.sh"}]}
    ]
  }
}`
	os.MkdirAll(filepath.Dir(settingsPath), 0o755)
	os.WriteFile(settingsPath, []byte(existing), 0o644)

	managed := map[string]*HookFile{
		"git-guard": {
			Event:   "PreToolUse",
			Matcher: "Bash(git *)",
			Hooks:   []json.RawMessage{json.RawMessage(`{"type": "command", "command": "check.sh"}`)},
		},
	}

	if err := MergeIntoSettings(settingsPath, managed, FormatNested); err != nil {
		t.Fatalf("MergeIntoSettings: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading result: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parsing result: %v", err)
	}

	// Permissions preserved.
	if _, ok := result["permissions"]; !ok {
		t.Error("permissions key missing from result")
	}

	// Hooks section exists.
	var hooks map[string][]json.RawMessage
	if err := json.Unmarshal(result["hooks"], &hooks); err != nil {
		t.Fatalf("parsing hooks: %v", err)
	}

	// User hook preserved.
	if len(hooks["SessionEnd"]) != 1 {
		t.Errorf("SessionEnd hooks = %d, want 1 (user-defined)", len(hooks["SessionEnd"]))
	}

	// Managed hook added.
	if len(hooks["PreToolUse"]) != 1 {
		t.Errorf("PreToolUse hooks = %d, want 1", len(hooks["PreToolUse"]))
	}

	// Verify _agentfiles marker.
	marker := extractAgentfilesMarker(hooks["PreToolUse"][0])
	if marker != "git-guard" {
		t.Errorf("_agentfiles = %q, want git-guard", marker)
	}
}

func TestMergeIntoSettings_Flat(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".cursor", "hooks.json")

	managed := map[string]*HookFile{
		"git-guard": {
			Event:   "PreToolUse",
			Matcher: "Bash(git *)",
			Hooks:   []json.RawMessage{json.RawMessage(`{"type": "command", "command": "check.sh"}`)},
		},
	}

	if err := MergeIntoSettings(settingsPath, managed, FormatFlat); err != nil {
		t.Fatalf("MergeIntoSettings: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading result: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parsing result: %v", err)
	}

	// Version field present.
	var version int
	if err := json.Unmarshal(result["version"], &version); err != nil {
		t.Fatalf("parsing version: %v", err)
	}
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}

	var hooks map[string][]json.RawMessage
	if err := json.Unmarshal(result["hooks"], &hooks); err != nil {
		t.Fatalf("parsing hooks: %v", err)
	}

	// Event name mapped to camelCase.
	if _, ok := hooks["preToolUse"]; !ok {
		t.Error("expected preToolUse key (camelCase), not found")
	}
	if _, ok := hooks["PreToolUse"]; ok {
		t.Error("unexpected PascalCase PreToolUse key")
	}

	// Flat format: command at top level.
	var flat map[string]any
	if err := json.Unmarshal(hooks["preToolUse"][0], &flat); err != nil {
		t.Fatalf("parsing flat entry: %v", err)
	}
	if flat["command"] != "check.sh" {
		t.Errorf("command = %v, want check.sh", flat["command"])
	}
}

func TestMergeIntoSettings_ReplacesOld(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// First apply.
	managed1 := map[string]*HookFile{
		"old-hook": {
			Event: "PreToolUse",
			Hooks: []json.RawMessage{json.RawMessage(`{"type": "command", "command": "old.sh"}`)},
		},
	}
	if err := MergeIntoSettings(settingsPath, managed1, FormatNested); err != nil {
		t.Fatalf("first merge: %v", err)
	}

	// Second apply with different hook.
	managed2 := map[string]*HookFile{
		"new-hook": {
			Event: "PreToolUse",
			Hooks: []json.RawMessage{json.RawMessage(`{"type": "command", "command": "new.sh"}`)},
		},
	}
	if err := MergeIntoSettings(settingsPath, managed2, FormatNested); err != nil {
		t.Fatalf("second merge: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var result map[string]json.RawMessage
	json.Unmarshal(data, &result)

	var hooks map[string][]json.RawMessage
	json.Unmarshal(result["hooks"], &hooks)

	// Only new hook should remain (old was managed → removed).
	if len(hooks["PreToolUse"]) != 1 {
		t.Fatalf("PreToolUse hooks = %d, want 1", len(hooks["PreToolUse"]))
	}
	marker := extractAgentfilesMarker(hooks["PreToolUse"][0])
	if marker != "new-hook" {
		t.Errorf("_agentfiles = %q, want new-hook", marker)
	}
}

func TestMergeIntoSettings_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "nonexistent", "settings.json")

	managed := map[string]*HookFile{
		"setup": {
			Event: "SessionStart",
			Hooks: []json.RawMessage{json.RawMessage(`{"type": "command", "command": "setup.sh"}`)},
		},
	}

	if err := MergeIntoSettings(settingsPath, managed, FormatNested); err != nil {
		t.Fatalf("MergeIntoSettings: %v", err)
	}

	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("settings file not created: %v", err)
	}
}

func TestRemoveManaged(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Set up with managed and user hooks.
	managed := map[string]*HookFile{
		"git-guard": {
			Event: "PreToolUse",
			Hooks: []json.RawMessage{json.RawMessage(`{"type": "command", "command": "check.sh"}`)},
		},
	}
	MergeIntoSettings(settingsPath, managed, FormatNested)

	// Add a user hook manually.
	data, _ := os.ReadFile(settingsPath)
	var topLevel map[string]json.RawMessage
	json.Unmarshal(data, &topLevel)
	var hooks map[string][]json.RawMessage
	json.Unmarshal(topLevel["hooks"], &hooks)
	hooks["PreToolUse"] = append(hooks["PreToolUse"], json.RawMessage(`{"hooks": [{"type": "command", "command": "user.sh"}]}`))
	hooksData, _ := json.Marshal(hooks)
	topLevel["hooks"] = hooksData
	out, _ := json.MarshalIndent(topLevel, "", "  ")
	os.WriteFile(settingsPath, out, 0o644)

	// Remove managed.
	if err := RemoveManaged(settingsPath, []string{"git-guard"}); err != nil {
		t.Fatalf("RemoveManaged: %v", err)
	}

	data, _ = os.ReadFile(settingsPath)
	json.Unmarshal(data, &topLevel)
	json.Unmarshal(topLevel["hooks"], &hooks)

	// Only user hook should remain.
	if len(hooks["PreToolUse"]) != 1 {
		t.Fatalf("PreToolUse hooks = %d, want 1", len(hooks["PreToolUse"]))
	}
	if hasAgentfilesMarker(hooks["PreToolUse"][0]) {
		t.Error("expected user hook (no marker), got managed")
	}
}

func TestRemoveManaged_NonexistentFile(t *testing.T) {
	err := RemoveManaged("/nonexistent/settings.json", []string{"foo"})
	if err != nil {
		t.Errorf("expected no error for nonexistent file, got: %v", err)
	}
}

func TestTargetsForLayout(t *testing.T) {
	tests := []struct {
		layout string
		count  int
	}{
		{"claude", 1},
		{"codex", 1},
		{"cursor", 1},
		{"all", 3},
		{"pi", 0},
		{"user-claude", 1},
		{"user-all", 3},
		{"user-pi", 0},
	}
	for _, tt := range tests {
		targets := TargetsForLayout(tt.layout)
		if len(targets) != tt.count {
			t.Errorf("TargetsForLayout(%q) = %d targets, want %d", tt.layout, len(targets), tt.count)
		}
	}
}
