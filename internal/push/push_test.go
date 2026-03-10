package push

import (
	"os"
	"path/filepath"
	"strings"
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

func pushFromRepo(t *testing.T, s *store.Store, repo string, opts Options) *Result {
	t.Helper()
	stores := map[string]*store.Store{"default": s}
	res, err := Push(stores, "default", repo, opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	return res
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

	res := pushFromRepo(t, s, repo, Options{})

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
	res2 := pushFromRepo(t, s, repo, Options{})
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

	res := pushFromRepo(t, s, repo, Options{})
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

	res := pushFromRepo(t, s, repo, Options{DryRun: true})
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
	res2 := pushFromRepo(t, s, repo, Options{DryRun: true})
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

	res := pushFromRepo(t, s, repo, Options{SkillOnly: "browse"})
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

	_, err := Push(map[string]*store.Store{"default": s}, "default", repo, Options{})
	if err == nil {
		t.Fatal("expected error when no lock file exists")
	}
	if got := err.Error(); got != "no lock file found — run af apply first" {
		t.Errorf("error = %q", got)
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

	res := pushFromRepo(t, s, repo, Options{})
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
	res2 := pushFromRepo(t, s, repo, Options{})
	if len(res2.Changes) != 0 {
		t.Errorf("expected 0 changes on 2nd push, got %d", len(res2.Changes))
	}
}

func TestPushUnmodifiedResource(t *testing.T) {
	s := setupStore(t)
	addAgent(t, s, "main", "# Agent")
	addResource(t, s, "configs", map[string]string{"config.yaml": "key: value"})

	repo := t.TempDir()
	m := &manifest.Manifest{Layout: "pi", AgentsMd: "main", Resources: []string{"configs"}}
	applyToRepo(t, s, m, repo)

	res := pushFromRepo(t, s, repo, Options{})
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

	res := pushFromRepo(t, s, repo, Options{})
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

func TestPushMultiStore(t *testing.T) {
	// Create two stores.
	workStore := setupStore(t)
	personalStore := setupStore(t)

	// Add an agent and skill to the work store (default).
	addAgent(t, workStore, "main", "# Work Agent")
	addSkill(t, workStore, "backend", "# Backend skill")

	// Add a skill to the personal store.
	addSkill(t, personalStore, "golang", "# Golang skill")

	stores := map[string]*store.Store{
		"work":     workStore,
		"personal": personalStore,
	}

	// Apply backend from work store (default).
	m2 := &manifest.Manifest{Layout: "pi", AgentsMd: "main", Skills: []string{"backend"}}
	repo2 := t.TempDir()
	if _, err := apply.Apply(stores, "work", m2, repo2, apply.Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Now manually deploy golang skill from personal store and record it.
	golangDeployed := filepath.Join(repo2, ".pi", "skills", "golang")
	if err := os.MkdirAll(golangDeployed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(golangDeployed, "SKILL.md"), []byte("# Golang skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	lf2, err := lock.Load(repo2)
	if err != nil {
		t.Fatalf("Load lock: %v", err)
	}
	golangHash, err := lock.HashDir(golangDeployed)
	if err != nil {
		t.Fatal(err)
	}
	lf2.Deployed.Skills["golang"] = &lock.Entry{
		Store:        "personal",
		StorePath:    "skills/golang",
		DeployedPath: ".pi/skills/golang",
		Hash:         golangHash,
	}
	if err := lock.Save(repo2, lf2); err != nil {
		t.Fatal(err)
	}

	// Modify both deployed skills.
	if err := os.WriteFile(filepath.Join(repo2, ".pi", "skills", "backend", "SKILL.md"), []byte("# Backend MODIFIED"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo2, ".pi", "skills", "golang", "SKILL.md"), []byte("# Golang MODIFIED"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Push — should route changes to the correct stores.
	res, err := Push(stores, "work", repo2, Options{})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(res.Changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(res.Changes))
	}

	// Verify backend was pushed to work store.
	data, err := os.ReadFile(filepath.Join(workStore.SkillsDir(), "backend", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Backend MODIFIED" {
		t.Errorf("work store backend = %q", data)
	}

	// Verify golang was pushed to personal store.
	data, err = os.ReadFile(filepath.Join(personalStore.SkillsDir(), "golang", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Golang MODIFIED" {
		t.Errorf("personal store golang = %q", data)
	}

	// Verify work store does NOT have golang.
	if _, err := os.Stat(filepath.Join(workStore.SkillsDir(), "golang")); !os.IsNotExist(err) {
		t.Error("golang skill should not exist in work store")
	}

	// Second push should find no changes.
	res2, err := Push(stores, "work", repo2, Options{})
	if err != nil {
		t.Fatalf("Push (2nd): %v", err)
	}
	if len(res2.Changes) != 0 {
		t.Errorf("expected 0 changes on 2nd push, got %d", len(res2.Changes))
	}
}

func TestPushUnknownStoreError(t *testing.T) {
	s := setupStore(t)
	addAgent(t, s, "main", "# Agent")
	addSkill(t, s, "browse", "# Browse")

	repo := t.TempDir()
	stores := map[string]*store.Store{"default": s}
	if _, err := apply.Apply(stores, "default", &manifest.Manifest{
		Layout: "pi", AgentsMd: "main", Skills: []string{"browse"},
	}, repo, apply.Options{Force: true}); err != nil {
		t.Fatal(err)
	}

	// Manually set a store name that doesn't exist in the map.
	lf, _ := lock.Load(repo)
	lf.Deployed.Skills["browse"].Store = "nonexistent"
	lock.Save(repo, lf)

	_, err := Push(stores, "default", repo, Options{})
	if err == nil {
		t.Fatal("expected error for unknown store")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention store name: %v", err)
	}
}
