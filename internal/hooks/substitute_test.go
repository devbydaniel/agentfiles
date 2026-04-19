package hooks

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSubstitute_ReplacesPlaceholder(t *testing.T) {
	hf := &HookFile{
		Event: "Stop",
		Hooks: []json.RawMessage{
			json.RawMessage(`{"type": "command", "command": "${AF_HOOK_ROOT}/scripts/go.sh"}`),
		},
	}

	got, err := Substitute(hf, "$HOME/.local/share/agentfiles/hooks/phoenix")
	if err != nil {
		t.Fatalf("Substitute: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(got.Hooks[0], &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := "$HOME/.local/share/agentfiles/hooks/phoenix/scripts/go.sh"
	if entry["command"] != want {
		t.Errorf("command = %v, want %v", entry["command"], want)
	}
}

func TestSubstitute_NoPlaceholderIsNoOp(t *testing.T) {
	hf := &HookFile{
		Event: "Stop",
		Hooks: []json.RawMessage{
			json.RawMessage(`{"type": "command", "command": "echo hi"}`),
		},
	}

	got, err := Substitute(hf, "/anything")
	if err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	var entry map[string]any
	if err := json.Unmarshal(got.Hooks[0], &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if entry["command"] != "echo hi" {
		t.Errorf("command = %v, want unchanged", entry["command"])
	}
}

func TestSubstitute_UnknownPlaceholderPassesThrough(t *testing.T) {
	hf := &HookFile{
		Event: "Stop",
		Hooks: []json.RawMessage{
			json.RawMessage(`{"type": "command", "command": "${AF_HOOK_ROOT}/x ${AF_ROTO}/y"}`),
		},
	}

	got, err := Substitute(hf, "/root")
	if err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	var entry map[string]any
	if err := json.Unmarshal(got.Hooks[0], &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cmd, _ := entry["command"].(string)
	if !strings.Contains(cmd, "/root/x") {
		t.Errorf("expected /root/x in %q", cmd)
	}
	if !strings.Contains(cmd, "${AF_ROTO}/y") {
		t.Errorf("expected unknown placeholder preserved in %q", cmd)
	}
}

func TestSubstitute_MultipleHooks(t *testing.T) {
	hf := &HookFile{
		Event: "SessionStart",
		Hooks: []json.RawMessage{
			json.RawMessage(`{"type": "command", "command": "${AF_HOOK_ROOT}/a.sh"}`),
			json.RawMessage(`{"type": "command", "command": "${AF_HOOK_ROOT}/b.sh"}`),
		},
	}

	got, err := Substitute(hf, "/r")
	if err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	if len(got.Hooks) != 2 {
		t.Fatalf("hooks count = %d, want 2", len(got.Hooks))
	}
	for i, want := range []string{"/r/a.sh", "/r/b.sh"} {
		var entry map[string]any
		if err := json.Unmarshal(got.Hooks[i], &entry); err != nil {
			t.Fatalf("unmarshal %d: %v", i, err)
		}
		if entry["command"] != want {
			t.Errorf("hooks[%d].command = %v, want %v", i, entry["command"], want)
		}
	}
}

func TestSubstitute_PreservesEventAndMatcher(t *testing.T) {
	hf := &HookFile{
		Event:   "PreToolUse",
		Matcher: "Bash(git *)",
		Hooks: []json.RawMessage{
			json.RawMessage(`{"type": "command", "command": "${AF_HOOK_ROOT}/x.sh"}`),
		},
	}
	got, err := Substitute(hf, "/r")
	if err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	if got.Event != "PreToolUse" {
		t.Errorf("Event = %q", got.Event)
	}
	if got.Matcher != "Bash(git *)" {
		t.Errorf("Matcher = %q", got.Matcher)
	}
}

func TestSubstitute_NilInput(t *testing.T) {
	got, err := Substitute(nil, "/r")
	if err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestSubstitute_EntryWithoutCommand(t *testing.T) {
	hf := &HookFile{
		Event: "Stop",
		Hooks: []json.RawMessage{
			json.RawMessage(`{"type": "something-else"}`),
		},
	}
	got, err := Substitute(hf, "/r")
	if err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	var entry map[string]any
	if err := json.Unmarshal(got.Hooks[0], &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if entry["type"] != "something-else" {
		t.Errorf("type = %v, want something-else", entry["type"])
	}
}
