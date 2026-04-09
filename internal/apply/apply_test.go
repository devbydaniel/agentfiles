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

// addInstructionToStore writes a .md file into the store instructions dir.
func addInstructionToStore(t *testing.T, s *store.Store, name, content string) {
	t.Helper()
	p := filepath.Join(s.InstructionsDir(), name+".md")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// addAgentToStore writes a .md file into the store agents dir.
func addAgentToStore(t *testing.T, s *store.Store, name, content string) {
	t.Helper()
	dir := s.AgentsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name+".md")
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
	addInstructionToStore(t, s, "main", "# Agent instructions")
	addSkillToStore(t, s, "golang", "# Go skill")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "pi",
		Instructions: "main",
		Skills:       []string{"golang"},
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
	data, err = os.ReadFile(filepath.Join(repo, ".agents", "skills", "golang", "SKILL.md"))
	if err != nil {
		t.Fatalf("skill not found: %v", err)
	}
	if string(data) != "# Go skill" {
		t.Errorf("SKILL.md content = %q", data)
	}
}

func TestApplyClaudeLayout(t *testing.T) {
	s := setupStore(t)
	addInstructionToStore(t, s, "main", "# Claude instructions")
	addSkillToStore(t, s, "testing", "# Test skill")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "claude",
		Instructions: "main",
		Skills:       []string{"testing"},
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
	addInstructionToStore(t, s, "main", "# Cursor instructions")
	addSkillToStore(t, s, "refactor", "# Refactor skill")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "cursor",
		Instructions: "main",
		Skills:       []string{"refactor"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 2 {
		t.Errorf("deployed = %d, want 2", res.Deployed)
	}

	// AGENTS.md (cursor layout now uses AGENTS.md)
	data, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not found: %v", err)
	}
	if string(data) != "# Cursor instructions" {
		t.Errorf("AGENTS.md content = %q", data)
	}

	// .agents/skills/refactor/SKILL.md
	data, err = os.ReadFile(filepath.Join(repo, ".agents", "skills", "refactor", "SKILL.md"))
	if err != nil {
		t.Fatalf("skill not found: %v", err)
	}
	if string(data) != "# Refactor skill" {
		t.Errorf("SKILL.md content = %q", data)
	}
}

func TestApplyAllLayout(t *testing.T) {
	s := setupStore(t)
	addInstructionToStore(t, s, "main", "# All instructions")
	addSkillToStore(t, s, "debug", "# Debug skill")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "all",
		Instructions: "main",
		Skills:       []string{"debug"},
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

	// .agents/skills/debug/SKILL.md (regular)
	data, err = os.ReadFile(filepath.Join(repo, ".agents", "skills", "debug", "SKILL.md"))
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
	addInstructionToStore(t, s, "main", "# Agent")
	addResourceToStore(t, s, "configs", map[string]string{
		"config.yaml":       "key: value",
		"sub/dir/notes.txt": "hello",
	})

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "pi",
		Instructions: "main",
		Resources:    []string{"configs"},
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
	addInstructionToStore(t, s, "main", "# Agent")
	addSkillToStore(t, s, "golang", "# Go")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "pi",
		Instructions: "main",
		Skills:       []string{"golang"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}
	if lf.Deployed.Instructions == nil {
		t.Fatal("lock file missing instructions entry")
	}
	// Source should be store-relative, not absolute.
	if strings.HasPrefix(lf.Deployed.Instructions.StorePath, "/") {
		t.Errorf("instructions source is absolute: %s", lf.Deployed.Instructions.StorePath)
	}
	if lf.Deployed.Instructions.StorePath != filepath.Join("instructions", "main.md") {
		t.Errorf("instructions source = %q, want %q", lf.Deployed.Instructions.StorePath, filepath.Join("instructions", "main.md"))
	}
	if lf.Deployed.Instructions.Store != "default" {
		t.Errorf("instructions store = %q, want %q", lf.Deployed.Instructions.Store, "default")
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

func TestApplyWritesLockForResources(t *testing.T) {
	s := setupStore(t)
	addInstructionToStore(t, s, "main", "# Agent")
	addResourceToStore(t, s, "myresource", map[string]string{
		"data.txt": "hello",
	})

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "pi",
		Instructions: "main",
		Resources:    []string{"myresource"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
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
	addInstructionToStore(t, s, "main", "# Agent")
	addSkillToStore(t, s, "golang", "# Go")
	addSkillToStore(t, s, "python", "# Python")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "pi",
		Instructions: "main",
		Skills:       []string{"golang", "python"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true, SkillOnly: "golang"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// golang should exist
	if _, err := os.Stat(filepath.Join(repo, ".agents", "skills", "golang", "SKILL.md")); err != nil {
		t.Error("golang skill not deployed")
	}

	// python should NOT exist
	if _, err := os.Stat(filepath.Join(repo, ".agents", "skills", "python", "SKILL.md")); err == nil {
		t.Error("python skill should not have been deployed")
	}

	// AGENTS.md should NOT exist (skill-only skips agent)
	if _, err := os.Stat(filepath.Join(repo, "AGENTS.md")); err == nil {
		t.Error("AGENTS.md should not have been deployed with --skill flag")
	}
}

func TestApplySkillNotInManifest(t *testing.T) {
	s := setupStore(t)
	addInstructionToStore(t, s, "main", "# Agent")
	addSkillToStore(t, s, "golang", "# Go")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "pi",
		Instructions: "main",
		Skills:       []string{"golang"},
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
	addInstructionToStore(t, s, "main", "# Agent")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "pi",
		Instructions: "main",
		Skills:       []string{"nonexistent"},
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
	addInstructionToStore(t, s, "main", "# New content")

	repo := t.TempDir()
	// Pre-create the file
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("# Existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &manifest.Manifest{
		Layout:       "pi",
		Instructions: "main",
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

	// Lock should still record the skipped agent (to prevent stale pruning).
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}
	if lf.Deployed.Instructions == nil {
		t.Error("lock file should record skipped agent to prevent stale pruning")
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
	addInstructionToStore(t, work, "main", "# Agent")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "pi",
		Instructions: "main",
		Skills:       []string{"backend", "personal:my-utils"},
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
	data, err := os.ReadFile(filepath.Join(repo, ".agents", "skills", "backend", "SKILL.md"))
	if err != nil {
		t.Fatalf("backend skill not found: %v", err)
	}
	if string(data) != "# Backend skill" {
		t.Errorf("backend content = %q", data)
	}

	data, err = os.ReadFile(filepath.Join(repo, ".agents", "skills", "my-utils", "SKILL.md"))
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

	if lf.Deployed.Instructions.Store != "work" {
		t.Errorf("instructions store = %q, want %q", lf.Deployed.Instructions.Store, "work")
	}
}

func TestApplyPrunesStaleAssets(t *testing.T) {
	s := setupStore(t)
	stores, defaultStore := singleStoreMap(s)
	repo := t.TempDir()

	// Deploy with two skills.
	addSkillToStore(t, s, "alpha", "# Alpha")
	addSkillToStore(t, s, "beta", "# Beta")
	m := &manifest.Manifest{
		Layout: "pi",
		Skills: []string{"alpha", "beta"},
	}

	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if res.Deployed != 2 {
		t.Fatalf("deployed = %d, want 2", res.Deployed)
	}

	// Verify both exist.
	assertFileExists(t, filepath.Join(repo, ".agents", "skills", "alpha", "SKILL.md"))
	assertFileExists(t, filepath.Join(repo, ".agents", "skills", "beta", "SKILL.md"))

	// Re-deploy with only alpha (beta removed from manifest).
	m2 := &manifest.Manifest{
		Layout: "pi",
		Skills: []string{"alpha"},
	}
	res2, err := Apply(stores, defaultStore, m2, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if res2.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res2.Deployed)
	}
	if res2.Removed != 1 {
		t.Errorf("removed = %d, want 1", res2.Removed)
	}

	// Alpha still exists, beta is gone.
	assertFileExists(t, filepath.Join(repo, ".agents", "skills", "alpha", "SKILL.md"))
	if _, err := os.Stat(filepath.Join(repo, ".agents", "skills", "beta")); err == nil {
		t.Error("beta skill should have been pruned")
	}

	// Lock should only have alpha.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}
	if _, ok := lf.Deployed.Skills["alpha"]; !ok {
		t.Error("lock missing alpha")
	}
	if _, ok := lf.Deployed.Skills["beta"]; ok {
		t.Error("lock should not contain beta")
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
}

// --- Grouped skill tests ---

func TestApplyGroupedSkill(t *testing.T) {
	s := setupStore(t)
	addSkillToStore(t, s, "tooling/browse", "# Browse skill")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout: "pi",
		Skills: []string{"tooling/browse"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res.Deployed)
	}

	// Deploys using leaf name, not group path.
	data, err := os.ReadFile(filepath.Join(repo, ".agents", "skills", "browse", "SKILL.md"))
	if err != nil {
		t.Fatalf("skill not found at leaf path: %v", err)
	}
	if string(data) != "# Browse skill" {
		t.Errorf("content = %q", data)
	}

	// Should NOT exist at the group path.
	if _, err := os.Stat(filepath.Join(repo, ".agents", "skills", "tooling", "browse")); err == nil {
		t.Error("skill should not be deployed at group path")
	}
}

func TestApplyGroupedSkillLock(t *testing.T) {
	s := setupStore(t)
	addSkillToStore(t, s, "tooling/browse", "# Browse")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout: "pi",
		Skills: []string{"tooling/browse"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}

	// Lock key uses group-qualified path.
	entry, ok := lf.Deployed.Skills["tooling/browse"]
	if !ok {
		t.Fatalf("lock missing 'tooling/browse'; keys = %v", lockKeys(lf.Deployed.Skills))
	}
	if entry.StorePath != "skills/tooling/browse/" {
		t.Errorf("StorePath = %q, want skills/tooling/browse/", entry.StorePath)
	}
}

func TestApplyGroupedSkillMultiStoreLock(t *testing.T) {
	work := setupStore(t)
	personal := setupStore(t)
	addSkillToStore(t, work, "tooling/browse", "# Browse")
	addSkillToStore(t, personal, "search", "# Search")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout: "pi",
		Skills: []string{"tooling/browse", "personal:search"},
	}

	stores := map[string]*store.Store{
		"work":     work,
		"personal": personal,
	}
	if _, err := Apply(stores, "work", m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}

	// Default store skill uses group path as key.
	if _, ok := lf.Deployed.Skills["tooling/browse"]; !ok {
		t.Errorf("lock missing 'tooling/browse'; keys = %v", lockKeys(lf.Deployed.Skills))
	}
	// Non-default store uses store prefix + group path.
	if _, ok := lf.Deployed.Skills["personal:search"]; !ok {
		t.Errorf("lock missing 'personal:search'; keys = %v", lockKeys(lf.Deployed.Skills))
	}
}

func TestApplyTwoGroupedSkills(t *testing.T) {
	s := setupStore(t)
	addSkillToStore(t, s, "tooling/browse", "# Browse")
	addSkillToStore(t, s, "ayunis/backend", "# Backend")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout: "pi",
		Skills: []string{"tooling/browse", "ayunis/backend"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 2 {
		t.Errorf("deployed = %d, want 2", res.Deployed)
	}

	assertFileExists(t, filepath.Join(repo, ".agents", "skills", "browse", "SKILL.md"))
	assertFileExists(t, filepath.Join(repo, ".agents", "skills", "backend", "SKILL.md"))
}

func TestApplySkillOnlyByStorePath(t *testing.T) {
	s := setupStore(t)
	addSkillToStore(t, s, "tooling/browse", "# Browse")
	addSkillToStore(t, s, "tooling/search", "# Search")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout: "pi",
		Skills: []string{"tooling/browse", "tooling/search"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true, SkillOnly: "tooling/browse"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertFileExists(t, filepath.Join(repo, ".agents", "skills", "browse", "SKILL.md"))
	if _, err := os.Stat(filepath.Join(repo, ".agents", "skills", "search")); err == nil {
		t.Error("search should not be deployed")
	}
}

func TestApplySkillOnlyByLeafName(t *testing.T) {
	s := setupStore(t)
	addSkillToStore(t, s, "tooling/browse", "# Browse")
	addSkillToStore(t, s, "search", "# Search")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout: "pi",
		Skills: []string{"tooling/browse", "search"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true, SkillOnly: "browse"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertFileExists(t, filepath.Join(repo, ".agents", "skills", "browse", "SKILL.md"))
	if _, err := os.Stat(filepath.Join(repo, ".agents", "skills", "search")); err == nil {
		t.Error("search should not be deployed")
	}
}

// lockKeys returns all keys from a lock entry map.
func lockKeys(m map[string]*lock.Entry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// --- Subagent tests ---

const testAgentMd = `---
name: code-reviewer
description: Reviews code
---
You are a code reviewer.
`

func TestApplyAgentClaudeLayout(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "code-reviewer", testAgentMd)

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout: "claude",
		Agents: []string{"code-reviewer"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res.Deployed)
	}

	// Should be deployed as canonical .md (parsed and re-serialized).
	data, err := os.ReadFile(filepath.Join(repo, ".claude", "agents", "code-reviewer.md"))
	if err != nil {
		t.Fatalf("agent not found: %v", err)
	}
	// yaml.Marshal alphabetizes keys, so frontmatter order may differ from source.
	if !strings.Contains(string(data), "name: code-reviewer") {
		t.Errorf("agent content missing name: %q", data)
	}
	if !strings.Contains(string(data), "You are a code reviewer.") {
		t.Errorf("agent content missing body: %q", data)
	}

	// Lock should record the agent.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}
	entry, ok := lf.Deployed.Agents["code-reviewer"]
	if !ok {
		t.Fatal("lock missing agent 'code-reviewer'")
	}
	if entry.StorePath != filepath.Join("agents", "code-reviewer.md") {
		t.Errorf("agent StorePath = %q", entry.StorePath)
	}
	if entry.DeployedPath != ".claude/agents/code-reviewer.md" {
		t.Errorf("agent DeployedPath = %q", entry.DeployedPath)
	}
}

func TestApplyAgentCodexLayout(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "code-reviewer", testAgentMd)

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout: "codex",
		Agents: []string{"code-reviewer"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res.Deployed)
	}

	// Should be deployed as .toml (converted).
	data, err := os.ReadFile(filepath.Join(repo, ".codex", "agents", "code-reviewer.toml"))
	if err != nil {
		t.Fatalf("agent not found: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "code-reviewer") {
		t.Errorf("TOML missing name: %s", content)
	}
	if !strings.Contains(content, "You are a code reviewer.") {
		t.Errorf("TOML missing developer_instructions: %s", content)
	}
}

func TestApplyAgentAllLayout(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "code-reviewer", testAgentMd)

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout: "all",
		Agents: []string{"code-reviewer"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res.Deployed)
	}

	// Claude .md should exist.
	assertFileExists(t, filepath.Join(repo, ".claude", "agents", "code-reviewer.md"))
	// Cursor .md should exist.
	assertFileExists(t, filepath.Join(repo, ".cursor", "agents", "code-reviewer.md"))
	// Codex .toml should exist.
	assertFileExists(t, filepath.Join(repo, ".codex", "agents", "code-reviewer.toml"))
}

func TestApplyAgentPiLayout(t *testing.T) {
	s := setupStore(t)
	addAgentToStore(t, s, "code-reviewer", testAgentMd)

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout: "pi",
		Agents: []string{"code-reviewer"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// Pi layout returns nil for AgentEntries, so nothing deployed for agents.
	if res.Deployed != 0 {
		t.Errorf("deployed = %d, want 0", res.Deployed)
	}

	// No agent files should exist.
	if _, err := os.Stat(filepath.Join(repo, ".claude", "agents")); err == nil {
		t.Error("no agent dirs should exist for pi layout")
	}
}

func TestApplyAgentPrunesStale(t *testing.T) {
	s := setupStore(t)
	stores, defaultStore := singleStoreMap(s)
	repo := t.TempDir()

	addAgentToStore(t, s, "alpha", "---\nname: alpha\n---\nalpha body\n")
	addAgentToStore(t, s, "beta", "---\nname: beta\n---\nbeta body\n")

	m := &manifest.Manifest{
		Layout: "claude",
		Agents: []string{"alpha", "beta"},
	}
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	assertFileExists(t, filepath.Join(repo, ".claude", "agents", "alpha.md"))
	assertFileExists(t, filepath.Join(repo, ".claude", "agents", "beta.md"))

	// Re-deploy with only alpha.
	m2 := &manifest.Manifest{
		Layout: "claude",
		Agents: []string{"alpha"},
	}
	res2, err := Apply(stores, defaultStore, m2, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if res2.Removed != 1 {
		t.Errorf("removed = %d, want 1", res2.Removed)
	}

	assertFileExists(t, filepath.Join(repo, ".claude", "agents", "alpha.md"))
	if _, err := os.Stat(filepath.Join(repo, ".claude", "agents", "beta.md")); err == nil {
		t.Error("beta agent should have been pruned")
	}
}

// --- PiExtension tests ---

// addPiExtensionFileToStore writes a single .ts file into store pi_extensions dir.
func addPiExtensionFileToStore(t *testing.T, s *store.Store, name, content string) {
	t.Helper()
	dir := s.PiExtensionsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".ts"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// addPiExtensionDirToStore writes a directory extension with index.ts.
func addPiExtensionDirToStore(t *testing.T, s *store.Store, name string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(s.PiExtensionsDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
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

func TestApplyPiExtensionFilePiLayout(t *testing.T) {
	s := setupStore(t)
	addPiExtensionFileToStore(t, s, "no-model-flag", "export default {}")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "pi",
		PiExtensions: []string{"no-model-flag"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res.Deployed)
	}

	// Should be at .pi/extensions/no-model-flag.ts
	data, err := os.ReadFile(filepath.Join(repo, ".pi", "extensions", "no-model-flag.ts"))
	if err != nil {
		t.Fatalf("extension file not found: %v", err)
	}
	if string(data) != "export default {}" {
		t.Errorf("content = %q", data)
	}

	// Lock should record it.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}
	entry, ok := lf.Deployed.PiExtensions["no-model-flag"]
	if !ok {
		t.Fatal("lock missing pi_extension 'no-model-flag'")
	}
	if entry.StorePath != filepath.Join("pi_extensions", "no-model-flag.ts") {
		t.Errorf("StorePath = %q", entry.StorePath)
	}
	if entry.DeployedPath != ".pi/extensions/no-model-flag.ts" {
		t.Errorf("DeployedPath = %q", entry.DeployedPath)
	}
	if entry.Hash == "" {
		t.Error("hash is empty")
	}
}

func TestApplyPiExtensionDirPiLayout(t *testing.T) {
	s := setupStore(t)
	addPiExtensionDirToStore(t, s, "my-ext", map[string]string{
		"index.ts": "export default {}",
		"util.ts":  "export const x = 1",
	})

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "pi",
		PiExtensions: []string{"my-ext"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res.Deployed)
	}

	// Should be at .pi/extensions/my-ext/
	data, err := os.ReadFile(filepath.Join(repo, ".pi", "extensions", "my-ext", "index.ts"))
	if err != nil {
		t.Fatalf("index.ts not found: %v", err)
	}
	if string(data) != "export default {}" {
		t.Errorf("content = %q", data)
	}

	data, err = os.ReadFile(filepath.Join(repo, ".pi", "extensions", "my-ext", "util.ts"))
	if err != nil {
		t.Fatalf("util.ts not found: %v", err)
	}
	if string(data) != "export const x = 1" {
		t.Errorf("content = %q", data)
	}

	// Lock should record it.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}
	entry, ok := lf.Deployed.PiExtensions["my-ext"]
	if !ok {
		t.Fatal("lock missing pi_extension 'my-ext'")
	}
	if entry.StorePath != filepath.Join("pi_extensions", "my-ext")+"/" {
		t.Errorf("StorePath = %q", entry.StorePath)
	}
	if entry.DeployedPath != ".pi/extensions/my-ext" {
		t.Errorf("DeployedPath = %q", entry.DeployedPath)
	}
}

func TestApplyPiExtensionClaudeLayoutSkipped(t *testing.T) {
	s := setupStore(t)
	addPiExtensionFileToStore(t, s, "no-model-flag", "export default {}")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "claude",
		PiExtensions: []string{"no-model-flag"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// Claude layout returns nil for PiExtensionEntries → nothing deployed.
	if res.Deployed != 0 {
		t.Errorf("deployed = %d, want 0", res.Deployed)
	}

	// No files should exist.
	if _, err := os.Stat(filepath.Join(repo, ".pi", "extensions")); err == nil {
		t.Error("no pi extension dirs should exist for claude layout")
	}

	// Lock should NOT have pi_extensions.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("loading lock: %v", err)
	}
	if len(lf.Deployed.PiExtensions) != 0 {
		t.Errorf("lock has %d pi_extensions, want 0", len(lf.Deployed.PiExtensions))
	}
}

func TestApplyPiExtensionAllLayout(t *testing.T) {
	s := setupStore(t)
	addPiExtensionFileToStore(t, s, "no-model-flag", "export default {}")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "all",
		PiExtensions: []string{"no-model-flag"},
	}

	stores, defaultStore := singleStoreMap(s)
	res, err := Apply(stores, defaultStore, m, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res.Deployed)
	}

	// Should be at .pi/extensions/no-model-flag.ts (only pi path in all layout).
	assertFileExists(t, filepath.Join(repo, ".pi", "extensions", "no-model-flag.ts"))

	// Should NOT be at claude/cursor/codex paths.
	if _, err := os.Stat(filepath.Join(repo, ".claude", "extensions")); err == nil {
		t.Error(".claude/extensions should not exist")
	}
}

func TestApplyPiExtensionPrunesStale(t *testing.T) {
	s := setupStore(t)
	stores, defaultStore := singleStoreMap(s)
	repo := t.TempDir()

	addPiExtensionFileToStore(t, s, "alpha", "// alpha")
	addPiExtensionFileToStore(t, s, "beta", "// beta")

	m := &manifest.Manifest{
		Layout:       "pi",
		PiExtensions: []string{"alpha", "beta"},
	}
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	assertFileExists(t, filepath.Join(repo, ".pi", "extensions", "alpha.ts"))
	assertFileExists(t, filepath.Join(repo, ".pi", "extensions", "beta.ts"))

	// Re-deploy with only alpha.
	m2 := &manifest.Manifest{
		Layout:       "pi",
		PiExtensions: []string{"alpha"},
	}
	res2, err := Apply(stores, defaultStore, m2, repo, Options{Force: true})
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if res2.Removed != 1 {
		t.Errorf("removed = %d, want 1", res2.Removed)
	}

	assertFileExists(t, filepath.Join(repo, ".pi", "extensions", "alpha.ts"))
	if _, err := os.Stat(filepath.Join(repo, ".pi", "extensions", "beta.ts")); err == nil {
		t.Error("beta extension should have been pruned")
	}
}

func TestApplyPiExtensionSkillOnlySkips(t *testing.T) {
	s := setupStore(t)
	addSkillToStore(t, s, "golang", "# Go")
	addPiExtensionFileToStore(t, s, "no-model-flag", "export default {}")

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:       "pi",
		Skills:       []string{"golang"},
		PiExtensions: []string{"no-model-flag"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true, SkillOnly: "golang"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Skill should be deployed.
	assertFileExists(t, filepath.Join(repo, ".agents", "skills", "golang", "SKILL.md"))

	// Pi extension should NOT be deployed (skill-only mode).
	if _, err := os.Stat(filepath.Join(repo, ".pi", "extensions", "no-model-flag.ts")); err == nil {
		t.Error("pi_extension should not be deployed with --skill flag")
	}
}

func TestApplySkillOnlySkipsAgents(t *testing.T) {
	s := setupStore(t)
	addSkillToStore(t, s, "golang", "# Go")
	addAgentToStore(t, s, "code-reviewer", testAgentMd)

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout: "claude",
		Skills: []string{"golang"},
		Agents: []string{"code-reviewer"},
	}

	stores, defaultStore := singleStoreMap(s)
	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true, SkillOnly: "golang"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Skill should be deployed.
	assertFileExists(t, filepath.Join(repo, ".claude", "skills", "golang", "SKILL.md"))

	// Agent should NOT be deployed.
	if _, err := os.Stat(filepath.Join(repo, ".claude", "agents", "code-reviewer.md")); err == nil {
		t.Error("agent should not be deployed with --skill flag")
	}
}
