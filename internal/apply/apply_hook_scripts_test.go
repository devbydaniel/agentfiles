package apply

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devbydaniel/agentfiles/internal/layout"
	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// addHookFileToStore writes a single <name>.json into the store hooks dir.
func addHookFileToStore(t *testing.T, s *store.Store, name, content string) {
	t.Helper()
	dir := s.HooksDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// addHookDirToStore writes a directory-form hook with hook.json plus scripts.
func addHookDirToStore(t *testing.T, s *store.Store, name, hookJSON string, scripts map[string]string) {
	t.Helper()
	dir := filepath.Join(s.HooksDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(scripts) > 0 {
		scriptsDir := filepath.Join(dir, "scripts")
		if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		for rel, content := range scripts {
			p := filepath.Join(scriptsDir, rel)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// readFirstHookCommand reads the first Stop hook command from a settings.json.
func readFirstHookCommand(t *testing.T, settingsPath, event string) string {
	t.Helper()
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading %s: %v", settingsPath, err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("parsing settings: %v", err)
	}
	var hooks map[string][]json.RawMessage
	if err := json.Unmarshal(top["hooks"], &hooks); err != nil {
		t.Fatalf("parsing hooks: %v", err)
	}
	entries, ok := hooks[event]
	if !ok || len(entries) == 0 {
		t.Fatalf("no %s entries in settings", event)
	}
	var entry struct {
		Hooks []struct {
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(entries[0], &entry); err != nil {
		t.Fatalf("parsing entry: %v", err)
	}
	if len(entry.Hooks) == 0 {
		t.Fatalf("entry has no inner hooks")
	}
	return entry.Hooks[0].Command
}

func TestApplyHookDirRepoLevel(t *testing.T) {
	s := setupStore(t)
	addHookDirToStore(t, s, "phoenix",
		`{"event": "Stop", "hooks": [{"type": "command", "command": "${AF_HOOK_ROOT}/scripts/phoenix.sh"}]}`,
		map[string]string{"phoenix.sh": "#!/bin/sh\necho phoenix\n"},
	)

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "claude", Hooks: []string{"phoenix"}}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res.Deployed)
	}

	// Scripts copied under <repo>/.agentfiles/hooks/phoenix/.
	scriptPath := filepath.Join(repo, ".agentfiles", "hooks", "phoenix", "scripts", "phoenix.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("script not deployed: %v", err)
	}

	// Command substituted to a repo-relative path.
	got := readFirstHookCommand(t, filepath.Join(repo, ".claude", "settings.json"), "Stop")
	want := ".agentfiles/hooks/phoenix/scripts/phoenix.sh"
	if got != want {
		t.Errorf("command = %q, want %q", got, want)
	}

	// Lock records the deploy dir as ExtraPaths.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}
	entry, ok := lf.Deployed.Hooks["phoenix"]
	if !ok {
		t.Fatal("lock missing hook 'phoenix'")
	}
	if len(entry.ExtraPaths) != 1 || entry.ExtraPaths[0] != filepath.Join(".agentfiles", "hooks", "phoenix") {
		t.Errorf("ExtraPaths = %v", entry.ExtraPaths)
	}
	if entry.StorePath != filepath.Join("hooks", "phoenix")+"/" {
		t.Errorf("StorePath = %q", entry.StorePath)
	}
}

func TestApplyHookDirUserLevel(t *testing.T) {
	s := setupStore(t)
	addHookDirToStore(t, s, "phoenix",
		`{"event": "Stop", "hooks": [{"type": "command", "command": "${AF_HOOK_ROOT}/scripts/phoenix.sh"}]}`,
		map[string]string{"phoenix.sh": "#!/bin/sh\n"},
	)

	home := t.TempDir()
	m := &manifest.Manifest{Layout: "claude", Hooks: []string{"phoenix"}}

	stores, defaultStore := singleStoreMap(s)
	userLay, err := layout.GetUser("claude")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if _, err := Apply(stores, defaultStore, m, home, Options{Force: true, Layout: userLay}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Scripts copied under $HOME/.local/share/agentfiles/hooks/phoenix/.
	scriptPath := filepath.Join(home, ".local", "share", "agentfiles", "hooks", "phoenix", "scripts", "phoenix.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("script not deployed: %v", err)
	}

	// Command substituted to $HOME-prefixed form.
	got := readFirstHookCommand(t, filepath.Join(home, ".claude", "settings.json"), "Stop")
	want := "$HOME/.local/share/agentfiles/hooks/phoenix/scripts/phoenix.sh"
	if got != want {
		t.Errorf("command = %q, want %q", got, want)
	}
}

func TestApplyHookFileAndDirCoexist(t *testing.T) {
	s := setupStore(t)
	addHookFileToStore(t, s, "simple",
		`{"event": "SessionStart", "matcher": "", "hooks": [{"type": "command", "command": "echo simple"}]}`,
	)
	addHookDirToStore(t, s, "scripted",
		`{"event": "Stop", "hooks": [{"type": "command", "command": "${AF_HOOK_ROOT}/scripts/s.sh"}]}`,
		map[string]string{"s.sh": "#!/bin/sh\n"},
	)

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "claude", Hooks: []string{"simple", "scripted"}}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	simpleCmd := readFirstHookCommand(t, filepath.Join(repo, ".claude", "settings.json"), "SessionStart")
	if simpleCmd != "echo simple" {
		t.Errorf("simple command = %q", simpleCmd)
	}
	scriptedCmd := readFirstHookCommand(t, filepath.Join(repo, ".claude", "settings.json"), "Stop")
	if !strings.Contains(scriptedCmd, ".agentfiles/hooks/scripted/scripts/s.sh") {
		t.Errorf("scripted command = %q", scriptedCmd)
	}

	// Running apply a second time without changes must keep both hooks
	// present (regression against the bug where unchanged hooks were
	// dropped because they weren't in `managed`).
	if _, err := Apply(stores, defaultStore, m, repo, Options{}); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	// Both commands should still be there.
	simpleCmd2 := readFirstHookCommand(t, filepath.Join(repo, ".claude", "settings.json"), "SessionStart")
	if simpleCmd2 != "echo simple" {
		t.Errorf("after re-apply, simple command = %q", simpleCmd2)
	}
	scriptedCmd2 := readFirstHookCommand(t, filepath.Join(repo, ".claude", "settings.json"), "Stop")
	if !strings.Contains(scriptedCmd2, ".agentfiles/hooks/scripted/scripts/s.sh") {
		t.Errorf("after re-apply, scripted command = %q", scriptedCmd2)
	}
}

func TestApplyHookDirPrunedOnRemoval(t *testing.T) {
	s := setupStore(t)
	addHookDirToStore(t, s, "phoenix",
		`{"event": "Stop", "hooks": [{"type": "command", "command": "${AF_HOOK_ROOT}/scripts/phoenix.sh"}]}`,
		map[string]string{"phoenix.sh": "#!/bin/sh\n"},
	)

	repo := t.TempDir()
	stores, defaultStore := singleStoreMap(s)

	// First apply includes phoenix.
	m1 := &manifest.Manifest{Layout: "claude", Hooks: []string{"phoenix"}}
	if _, err := Apply(stores, defaultStore, m1, repo, Options{Force: true}); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	deployDir := filepath.Join(repo, ".agentfiles", "hooks", "phoenix")
	if _, err := os.Stat(deployDir); err != nil {
		t.Fatalf("deploy dir missing after first apply: %v", err)
	}

	// Second apply drops phoenix.
	m2 := &manifest.Manifest{Layout: "claude", Instructions: "", Skills: []string{}, HooksRemove: nil, Hooks: []string{}}
	m2 = &manifest.Manifest{Layout: "claude", Hooks: []string{}}
	// Need at least one cherry-picked field to pass manifest validation.
	m2.Instructions = "noop"
	addInstructionToStore(t, s, "noop", "noop")

	if _, err := Apply(stores, defaultStore, m2, repo, Options{Force: true}); err != nil {
		t.Fatalf("second Apply: %v", err)
	}

	if _, err := os.Stat(deployDir); !os.IsNotExist(err) {
		t.Errorf("deploy dir still exists after removal: %v", err)
	}

	// Settings.json should no longer have the phoenix entry.
	data, err := os.ReadFile(filepath.Join(repo, ".claude", "settings.json"))
	if err == nil {
		if strings.Contains(string(data), ".agentfiles/hooks/phoenix") {
			t.Errorf("settings.json still references phoenix: %s", data)
		}
	}
}

func TestApplyHookFileFormUnchanged(t *testing.T) {
	// Regression: file-form hooks continue to work unchanged.
	s := setupStore(t)
	addHookFileToStore(t, s, "greet",
		`{"event": "SessionStart", "hooks": [{"type": "command", "command": "echo hi"}]}`,
	)

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "claude", Hooks: []string{"greet"}}
	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := readFirstHookCommand(t, filepath.Join(repo, ".claude", "settings.json"), "SessionStart")
	if got != "echo hi" {
		t.Errorf("command = %q", got)
	}

	// No deploy dir should be created for file-form.
	if _, err := os.Stat(filepath.Join(repo, ".agentfiles", "hooks", "greet")); !os.IsNotExist(err) {
		t.Errorf("unexpected deploy dir for file-form hook")
	}

	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}
	entry := lf.Deployed.Hooks["greet"]
	if entry == nil {
		t.Fatal("lock missing hook 'greet'")
	}
	if len(entry.ExtraPaths) != 0 {
		t.Errorf("ExtraPaths = %v, want empty for file-form", entry.ExtraPaths)
	}
	if entry.StorePath != filepath.Join("hooks", "greet.json") {
		t.Errorf("StorePath = %q", entry.StorePath)
	}
}
