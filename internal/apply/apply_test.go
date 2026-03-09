package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// setupStore creates a temp store with git init'd and required subdirs.
func setupStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("Init store: %v", err)
	}
	return s
}

// singleStoreMap wraps a single store into the map+default format.
func singleStoreMap(s *store.Store) (map[string]*store.Store, string) {
	return map[string]*store.Store{"default": s}, "default"
}

// addAgentToStore writes a .md file into the store agents dir.
func addAgentToStore(t *testing.T, s *store.Store, name, content string) {
	t.Helper()
	p := filepath.Join(s.AgentsDir(), name+".md")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// addSkillToStore writes a skill dir with SKILL.md into the store.
func addSkillToStore(t *testing.T, s *store.Store, name, content string) {
	t.Helper()
	dir := filepath.Join(s.SkillsDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// addPluginToStore writes a plugin dir with files into the store.
func addPluginToStore(t *testing.T, s *store.Store, name string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(s.PluginsDir(), name)
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// addResourceToStore writes a resource dir with files into the store.
func addResourceToStore(t *testing.T, s *store.Store, name string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(s.ResourcesDir(), name)
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestApplyPiLayout(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "main", "# Agent instructions")
	addSkillToStore(t, s, "golang", "# Go skill")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:   "pi",
		AgentsMd: "main",
		Skills:   []string{"golang"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Check AGENTS.md
	data, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not found: %v", err)
	}
	if string(data) != "# Agent instructions" {
		t.Errorf("AGENTS.md content = %q", data)
	}

	// Check skill
	data, err = os.ReadFile(filepath.Join(repo, ".pi", "skills", "golang", "SKILL.md"))
	if err != nil {
		t.Fatalf("skill not found: %v", err)
	}
	if string(data) != "# Go skill" {
		t.Errorf("SKILL.md content = %q", data)
	}
}

func TestApplyClaudeLayout(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "main", "# Claude instructions")
	addSkillToStore(t, s, "testing", "# Test skill")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:   "claude",
		AgentsMd: "main",
		Skills:   []string{"testing"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md not found: %v", err)
	}
	if string(data) != "# Claude instructions" {
		t.Errorf("CLAUDE.md content = %q", data)
	}

	data, err = os.ReadFile(filepath.Join(repo, ".claude", "skills", "testing", "SKILL.md"))
	if err != nil {
		t.Fatalf("skill not found: %v", err)
	}
	if string(data) != "# Test skill" {
		t.Errorf("SKILL.md content = %q", data)
	}
}

func TestApplyCursorLayout(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "main", "# Cursor instructions")
	addSkillToStore(t, s, "refactor", "# Refactor skill")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:   "cursor",
		AgentsMd: "main",
		Skills:   []string{"refactor"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 2 {
		t.Errorf("deployed = %d, want 2", res.Deployed)
	}

	// .cursorrules
	data, err := os.ReadFile(filepath.Join(repo, ".cursorrules"))
	if err != nil {
		t.Fatalf(".cursorrules not found: %v", err)
	}
	if string(data) != "# Cursor instructions" {
		t.Errorf(".cursorrules content = %q", data)
	}

	// .cursor/skills/refactor/SKILL.md
	data, err = os.ReadFile(filepath.Join(repo, ".cursor", "skills", "refactor", "SKILL.md"))
	if err != nil {
		t.Fatalf("skill not found: %v", err)
	}
	if string(data) != "# Refactor skill" {
		t.Errorf("SKILL.md content = %q", data)
	}
}

func TestApplyAllLayout(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "main", "# All instructions")
	addSkillToStore(t, s, "debug", "# Debug skill")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:   "all",
		AgentsMd: "main",
		Skills:   []string{"debug"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// AGENTS.md (primary, regular)
	data, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not found: %v", err)
	}
	if string(data) != "# All instructions" {
		t.Errorf("AGENTS.md content = %q", data)
	}

	// CLAUDE.md (full copy)
	data, err = os.ReadFile(filepath.Join(repo, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md not found: %v", err)
	}
	if string(data) != "# All instructions" {
		t.Errorf("CLAUDE.md content = %q, want same as AGENTS.md", data)
	}

	// .pi/skills/debug/SKILL.md (regular)
	data, err = os.ReadFile(filepath.Join(repo, ".pi", "skills", "debug", "SKILL.md"))
	if err != nil {
		t.Fatalf("pi skill not found: %v", err)
	}
	if string(data) != "# Debug skill" {
		t.Errorf("pi SKILL.md content = %q", data)
	}

	// .claude/skills/debug should be a regular copy
	claudeSkillPath := filepath.Join(repo, ".claude", "skills", "debug", "SKILL.md")
	data, err = os.ReadFile(claudeSkillPath)
	if err != nil {
		t.Fatalf(".claude skill not found: %v", err)
	}
	if string(data) != "# Debug skill" {
		t.Errorf(".claude SKILL.md content = %q", data)
	}
}

func TestApplyResources(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "main", "# Agent")
	addResourceToStore(t, s, "configs", map[string]string{
		"config.yaml":       "key: value",
		"sub/dir/notes.txt": "hello",
	})

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:    "pi",
		AgentsMd:  "main",
		Resources: []string{"configs"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo, "config.yaml"))
	if err != nil {
		t.Fatalf("resource file not found: %v", err)
	}
	if string(data) != "key: value" {
		t.Errorf("config.yaml = %q", data)
	}

	data, err = os.ReadFile(filepath.Join(repo, "sub", "dir", "notes.txt"))
	if err != nil {
		t.Fatalf("nested resource not found: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("notes.txt = %q", data)
	}
}

func TestApplyWritesLockFile(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "main", "# Agent")
	addSkillToStore(t, s, "golang", "# Go")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:   "pi",
		AgentsMd: "main",
		Skills:   []string{"golang"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}
	if lf.Deployed.AgentsMD == nil {
		t.Fatal("lock file missing agents_md entry")
	}
	// Source should be store-relative, not absolute.
	if strings.HasPrefix(lf.Deployed.AgentsMD.StorePath, "/") {
		t.Errorf("agents_md source is absolute: %s", lf.Deployed.AgentsMD.StorePath)
	}
	if lf.Deployed.AgentsMD.StorePath != filepath.Join("agents", "main.md") {
		t.Errorf("agents_md source = %q, want %q", lf.Deployed.AgentsMD.StorePath, filepath.Join("agents", "main.md"))
	}
	if lf.Deployed.AgentsMD.Store != "default" {
		t.Errorf("agents_md store = %q, want %q", lf.Deployed.AgentsMD.Store, "default")
	}

	skill, ok := lf.Deployed.Skills["golang"]
	if !ok {
		t.Fatal("lock file missing skill 'golang'")
	}
	if strings.HasPrefix(skill.StorePath, "/") {
		t.Errorf("skill source is absolute: %s", skill.StorePath)
	}
	if skill.StorePath != "skills/golang/" {
		t.Errorf("skill source = %q, want %q", skill.StorePath, "skills/golang/")
	}
	if skill.Store != "default" {
		t.Errorf("skill store = %q, want %q", skill.Store, "default")
	}
}

func TestApplyWritesLockForPluginsAndResources(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "main", "# Agent")
	addPluginToStore(t, s, "myplugin", map[string]string{
		"plugin.yaml": "name: myplugin",
	})
	addResourceToStore(t, s, "myresource", map[string]string{
		"data.txt": "hello",
	})

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:    "pi",
		AgentsMd:  "main",
		Plugins:   []string{"myplugin"},
		Resources: []string{"myresource"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}

	plugin, ok := lf.Deployed.Plugins["myplugin"]
	if !ok {
		t.Fatal("lock file missing plugin 'myplugin'")
	}
	if plugin.StorePath != "plugins/myplugin/" {
		t.Errorf("plugin source = %q, want %q", plugin.StorePath, "plugins/myplugin/")
	}
	if plugin.Hash == "" {
		t.Error("plugin hash is empty")
	}

	resource, ok := lf.Deployed.Resources["myresource"]
	if !ok {
		t.Fatal("lock file missing resource 'myresource'")
	}
	if resource.StorePath != "resources/myresource/" {
		t.Errorf("resource source = %q, want %q", resource.StorePath, "resources/myresource/")
	}
	if resource.Hash == "" {
		t.Error("resource hash is empty")
	}
}

func TestApplySkillOnly(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "main", "# Agent")
	addSkillToStore(t, s, "golang", "# Go")
	addSkillToStore(t, s, "python", "# Python")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:   "pi",
		AgentsMd: "main",
		Skills:   []string{"golang", "python"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true, SkillOnly: "golang"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// golang should exist
	if _, err := os.Stat(filepath.Join(repo, ".pi", "skills", "golang", "SKILL.md")); err != nil {
		t.Error("golang skill not deployed")
	}

	// python should NOT exist
	if _, err := os.Stat(filepath.Join(repo, ".pi", "skills", "python", "SKILL.md")); err == nil {
		t.Error("python skill should not have been deployed")
	}

	// AGENTS.md should NOT exist (skill-only skips agent)
	if _, err := os.Stat(filepath.Join(repo, "AGENTS.md")); err == nil {
		t.Error("AGENTS.md should not have been deployed with --skill flag")
	}
}

func TestApplySkillNotInManifest(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "main", "# Agent")
	addSkillToStore(t, s, "golang", "# Go")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:   "pi",
		AgentsMd: "main",
		Skills:   []string{"golang"},
	}

	stores, defaultStore := singleStoreMap(s)
	_, err := Apply(stores, defaultStore, m, repo, Options{SkillOnly: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for skill not in manifest")
	}
	if !strings.Contains(err.Error(), "not in manifest") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestApplySkillNotInStore(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "main", "# Agent")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:   "pi",
		AgentsMd: "main",
		Skills:   []string{"nonexistent"},
	}

	stores, defaultStore := singleStoreMap(s)
	_, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err == nil {
		t.Fatal("expected error for skill not in store")
	}
	if !strings.Contains(err.Error(), "not found in store") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestApplyNoForceSkipsExisting(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "main", "# New content")

	repo := t.TempDir()
	// Pre-create the file
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("# Existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &manifest.Manifest{
		Layout:   "pi",
		AgentsMd: "main",
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: false})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Should still have old content (skipped).
	data, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Existing" {
		t.Errorf("file was overwritten: %q", data)
	}

	// Lock should NOT record the skipped agent.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}
	if lf.Deployed.AgentsMD != nil {
		t.Error("lock file should not record skipped agent")
	}

	if res.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", res.Skipped)
	}
	if res.Deployed != 0 {
		t.Errorf("deployed = %d, want 0", res.Deployed)
	}
}

func TestApplyMultiStore(t *testing.T) {
	// Create two stores
	work := setupStore(t)
	personal := setupStore(t)

	// Add skill to each store
	addSkillToStore(t, work, "backend", "# Backend skill")
	addSkillToStore(t, personal, "my-utils", "# My utils")
	addAgentToStore(t, work, "main", "# Agent")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:   "pi",
		AgentsMd: "main",
		Skills:   []string{"backend", "personal:my-utils"},
	}

	stores := map[string]*store.Store{
		"work":     work,
		"personal": personal,
	}
	res, err := Apply(stores, "work", m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 3 { // agent + 2 skills
		t.Errorf("deployed = %d, want 3", res.Deployed)
	}

	// Both skills deployed
	data, err := os.ReadFile(filepath.Join(repo, ".pi", "skills", "backend", "SKILL.md"))
	if err != nil {
		t.Fatalf("backend skill not found: %v", err)
	}
	if string(data) != "# Backend skill" {
		t.Errorf("backend content = %q", data)
	}

	data, err = os.ReadFile(filepath.Join(repo, ".pi", "skills", "my-utils", "SKILL.md"))
	if err != nil {
		t.Fatalf("my-utils skill not found: %v", err)
	}
	if string(data) != "# My utils" {
		t.Errorf("my-utils content = %q", data)
	}

	// Lock records correct store names
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}

	backendEntry, ok := lf.Deployed.Skills["backend"]
	if !ok {
		t.Fatal("lock missing backend skill")
	}
	if backendEntry.Store != "work" {
		t.Errorf("backend store = %q, want %q", backendEntry.Store, "work")
	}

	utilsEntry, ok := lf.Deployed.Skills["personal:my-utils"]
	if !ok {
		t.Fatal("lock missing personal:my-utils skill")
	}
	if utilsEntry.Store != "personal" {
		t.Errorf("my-utils store = %q, want %q", utilsEntry.Store, "personal")
	}

	if lf.Deployed.AgentsMD.Store != "work" {
		t.Errorf("agents_md store = %q, want %q", lf.Deployed.AgentsMD.Store, "work")
	}
}
