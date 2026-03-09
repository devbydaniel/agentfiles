package push

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devbydaniel/agentfiles/internal/apply"
	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/store"
)

func setupStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("Init store: %v", err)
	}
	return s
}

func addAgent(t *testing.T, s *store.Store, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(s.AgentsDir(), name+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func addPlugin(t *testing.T, s *store.Store, name string, files map[string]string) {
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

func addResource(t *testing.T, s *store.Store, name string, files map[string]string) {
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

func addSkill(t *testing.T, s *store.Store, name, content string) {
	t.Helper()
	dir := filepath.Join(s.SkillsDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func applyToRepo(t *testing.T, s *store.Store, m *manifest.Manifest, repo string) {
	t.Helper()
	stores := map[string]*store.Store{"default": s}
	if _, err := apply.Apply(stores, "default", m, repo, apply.Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestPushModifiedSkill(t *testing.T) {
	s := setupStore(t)
	addAgent(t, s, "main", "# Agent")
	addSkill(t, s, "browse", "# Browse skill")

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "pi", AgentsMd: "main", Skills: []string{"browse"}}
	applyToRepo(t, s, m, repo)

	// Modify the deployed SKILL.md.
	deployed := filepath.Join(repo, ".pi", "skills", "browse", "SKILL.md")
	if err := os.WriteFile(deployed, []byte("# Browse skill MODIFIED"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Push(s, repo, Options{})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}

	if len(res.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(res.Changes))
	}
	if res.Changes[0].Name != "browse" {
		t.Errorf("change name = %q, want browse", res.Changes[0].Name)
	}

	// Verify the store copy was updated.
	data, err := os.ReadFile(filepath.Join(s.SkillsDir(), "browse", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Browse skill MODIFIED" {
		t.Errorf("store skill = %q", data)
	}

	// Verify lock was updated — pushing again should find no changes.
	res2, err := Push(s, repo, Options{})
	if err != nil {
		t.Fatalf("Push (2nd): %v", err)
	}
	if len(res2.Changes) != 0 {
		t.Errorf("expected 0 changes on 2nd push, got %d", len(res2.Changes))
	}
}

func TestPushUnmodifiedNoop(t *testing.T) {
	s := setupStore(t)
	addAgent(t, s, "main", "# Agent")
	addSkill(t, s, "browse", "# Browse skill")

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "pi", AgentsMd: "main", Skills: []string{"browse"}}
	applyToRepo(t, s, m, repo)

	res, err := Push(s, repo, Options{})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(res.Changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(res.Changes))
	}
	if res.Checked != 2 { // agents_md + browse
		t.Errorf("checked = %d, want 2", res.Checked)
	}
}

func TestPushDryRun(t *testing.T) {
	s := setupStore(t)
	addAgent(t, s, "main", "# Agent")
	addSkill(t, s, "browse", "# Browse skill")

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "pi", AgentsMd: "main", Skills: []string{"browse"}}
	applyToRepo(t, s, m, repo)

	// Modify deployed file.
	deployed := filepath.Join(repo, ".pi", "skills", "browse", "SKILL.md")
	if err := os.WriteFile(deployed, []byte("# MODIFIED"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Push(s, repo, Options{DryRun: true})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(res.Changes) != 1 {
		t.Fatalf("dry-run expected 1 change, got %d", len(res.Changes))
	}

	// Store should NOT have been updated.
	data, err := os.ReadFile(filepath.Join(s.SkillsDir(), "browse", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Browse skill" {
		t.Errorf("store was modified during dry-run: %q", data)
	}

	// Lock hash should NOT have changed.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	// Do a real push now and check the old hash is still there.
	origHash := lf.Deployed.Skills["browse"].Hash
	res2, _ := Push(s, repo, Options{DryRun: true})
	lf2, _ := lock.Load(repo)
	if lf2.Deployed.Skills["browse"].Hash != origHash {
		t.Error("lock hash changed during dry-run")
	}
	_ = res2
}

func TestPushSkillFilter(t *testing.T) {
	s := setupStore(t)
	addAgent(t, s, "main", "# Agent")
	addSkill(t, s, "browse", "# Browse")
	addSkill(t, s, "git", "# Git")

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "pi", AgentsMd: "main", Skills: []string{"browse", "git"}}
	applyToRepo(t, s, m, repo)

	// Modify both skills.
	os.WriteFile(filepath.Join(repo, ".pi", "skills", "browse", "SKILL.md"), []byte("# Browse MOD"), 0o644)
	os.WriteFile(filepath.Join(repo, ".pi", "skills", "git", "SKILL.md"), []byte("# Git MOD"), 0o644)

	res, err := Push(s, repo, Options{SkillOnly: "browse"})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(res.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(res.Changes))
	}
	if res.Changes[0].Name != "browse" {
		t.Errorf("change name = %q, want browse", res.Changes[0].Name)
	}

	// Git skill in store should NOT have been updated.
	data, err := os.ReadFile(filepath.Join(s.SkillsDir(), "git", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Git" {
		t.Errorf("git skill was modified: %q", data)
	}

	// Browse skill in store SHOULD be updated.
	data, err = os.ReadFile(filepath.Join(s.SkillsDir(), "browse", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Browse MOD" {
		t.Errorf("browse skill not updated: %q", data)
	}
}

func TestPushNoLockFile(t *testing.T) {
	s := setupStore(t)
	repo := t.TempDir()

	_, err := Push(s, repo, Options{})
	if err == nil {
		t.Fatal("expected error when no lock file exists")
	}
	if got := err.Error(); got != "no lock file found — run af apply first" {
		t.Errorf("error = %q", got)
	}
}

func TestPushModifiedPlugin(t *testing.T) {
	s := setupStore(t)
	addAgent(t, s, "main", "# Agent")
	addPlugin(t, s, "myplugin", map[string]string{
		"plugin.yaml": "name: myplugin",
		"run.sh":      "#!/bin/bash\necho hello",
	})

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "pi", AgentsMd: "main", Plugins: []string{"myplugin"}}
	applyToRepo(t, s, m, repo)

	// Modify the deployed plugin file.
	deployed := filepath.Join(repo, ".pi", "plugins", "myplugin", "plugin.yaml")
	if err := os.WriteFile(deployed, []byte("name: myplugin\nversion: 2"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Push(s, repo, Options{})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(res.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(res.Changes))
	}
	if res.Changes[0].Name != "myplugin" {
		t.Errorf("change name = %q, want myplugin", res.Changes[0].Name)
	}
	if res.Changes[0].Type != lock.AssetPlugins {
		t.Errorf("change type = %q, want %q", res.Changes[0].Type, lock.AssetPlugins)
	}

	// Verify store was updated.
	data, err := os.ReadFile(filepath.Join(s.PluginsDir(), "myplugin", "plugin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "name: myplugin\nversion: 2" {
		t.Errorf("store plugin = %q", data)
	}

	// Second push should find no changes.
	res2, err := Push(s, repo, Options{})
	if err != nil {
		t.Fatalf("Push (2nd): %v", err)
	}
	if len(res2.Changes) != 0 {
		t.Errorf("expected 0 changes on 2nd push, got %d", len(res2.Changes))
	}
}

func TestPushModifiedResource(t *testing.T) {
	s := setupStore(t)
	addAgent(t, s, "main", "# Agent")
	addResource(t, s, "configs", map[string]string{
		"config.yaml":       "key: value",
		"sub/dir/notes.txt": "hello",
	})

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "pi", AgentsMd: "main", Resources: []string{"configs"}}
	applyToRepo(t, s, m, repo)

	// Verify resources were deployed to repo root.
	data, err := os.ReadFile(filepath.Join(repo, "config.yaml"))
	if err != nil {
		t.Fatalf("resource file not deployed: %v", err)
	}
	if string(data) != "key: value" {
		t.Fatalf("unexpected deployed content: %q", data)
	}

	// Modify a deployed resource file.
	if err := os.WriteFile(filepath.Join(repo, "config.yaml"), []byte("key: updated"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Push(s, repo, Options{})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(res.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(res.Changes))
	}
	if res.Changes[0].Name != "configs" {
		t.Errorf("change name = %q, want configs", res.Changes[0].Name)
	}
	if res.Changes[0].Type != lock.AssetResources {
		t.Errorf("change type = %q, want %q", res.Changes[0].Type, lock.AssetResources)
	}

	// Verify store was updated.
	data, err = os.ReadFile(filepath.Join(s.ResourcesDir(), "configs", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "key: updated" {
		t.Errorf("store resource = %q", data)
	}

	// Nested file should be unchanged.
	data, err = os.ReadFile(filepath.Join(s.ResourcesDir(), "configs", "sub", "dir", "notes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("nested resource should be unchanged: %q", data)
	}

	// Second push should find no changes.
	res2, err := Push(s, repo, Options{})
	if err != nil {
		t.Fatalf("Push (2nd): %v", err)
	}
	if len(res2.Changes) != 0 {
		t.Errorf("expected 0 changes on 2nd push, got %d", len(res2.Changes))
	}
}

func TestPushUnmodifiedPlugin(t *testing.T) {
	s := setupStore(t)
	addAgent(t, s, "main", "# Agent")
	addPlugin(t, s, "myplugin", map[string]string{"plugin.yaml": "name: myplugin"})

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "pi", AgentsMd: "main", Plugins: []string{"myplugin"}}
	applyToRepo(t, s, m, repo)

	res, err := Push(s, repo, Options{})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(res.Changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(res.Changes))
	}
}

func TestPushUnmodifiedResource(t *testing.T) {
	s := setupStore(t)
	addAgent(t, s, "main", "# Agent")
	addResource(t, s, "configs", map[string]string{"config.yaml": "key: value"})

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "pi", AgentsMd: "main", Resources: []string{"configs"}}
	applyToRepo(t, s, m, repo)

	res, err := Push(s, repo, Options{})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(res.Changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(res.Changes))
	}
}

func TestPushModifiedAgentMd(t *testing.T) {
	s := setupStore(t)
	addAgent(t, s, "main", "# Original agent")

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "pi", AgentsMd: "main"}
	applyToRepo(t, s, m, repo)

	// Modify deployed AGENTS.md.
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("# Updated agent"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Push(s, repo, Options{})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(res.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(res.Changes))
	}
	if res.Changes[0].Name != "agents_md" {
		t.Errorf("change name = %q", res.Changes[0].Name)
	}

	// Store should be updated.
	data, err := os.ReadFile(filepath.Join(s.AgentsDir(), "main.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Updated agent" {
		t.Errorf("store agent = %q", data)
	}
}
