package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devbydaniel/agentfiles/internal/apply"
	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/push"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// TestIntegrationRoundTrip exercises the full lifecycle:
//
//  1. init-store
//  2. add skill, agent, plugin, resource
//  3. create bundle TOML
//  4. init repo (write .agentfiles pointing at bundle, pi layout)
//  5. apply → verify files at correct pi layout paths
//  6. modify a skill's SKILL.md in the repo
//  7. push → verify change reflected in store
//  8. init + apply a second repo with same bundle
//  9. verify second repo picks up pushed change
//
// 10. verify lock file accuracy throughout
func TestIntegrationRoundTrip(t *testing.T) {
	tmp := t.TempDir()

	// ── 1. init-store ──────────────────────────────────────────────
	storePath := filepath.Join(tmp, "store")
	s, err := store.Init(storePath)
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	// ── 2. add assets ──────────────────────────────────────────────

	// skill: create a source skill dir with SKILL.md
	skillSrc := filepath.Join(tmp, "src-skill", "my-skill")
	mustMkdir(t, skillSrc)
	mustWrite(t, filepath.Join(skillSrc, "SKILL.md"), "# My Skill\nOriginal content\n")
	mustWrite(t, filepath.Join(skillSrc, "helper.sh"), "#!/bin/bash\necho hello\n")

	name, _, err := s.AddSkill(skillSrc, false)
	if err != nil {
		t.Fatalf("add skill: %v", err)
	}
	if name != "my-skill" {
		t.Fatalf("expected skill name 'my-skill', got %q", name)
	}

	// agent: create a source agent file
	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Agent Instructions\nBe helpful.\n")

	_, err = s.AddAgent(agentSrc, "default", false)
	if err != nil {
		t.Fatalf("add agent: %v", err)
	}

	// plugin: create a source plugin dir
	pluginSrc := filepath.Join(tmp, "src-plugin", "my-plugin")
	mustMkdir(t, pluginSrc)
	mustWrite(t, filepath.Join(pluginSrc, "plugin.lua"), "-- plugin code\n")

	pname, _, err := s.AddPlugin(pluginSrc, false)
	if err != nil {
		t.Fatalf("add plugin: %v", err)
	}
	if pname != "my-plugin" {
		t.Fatalf("expected plugin name 'my-plugin', got %q", pname)
	}

	// resource: create a source resource dir with files
	resSrc := filepath.Join(tmp, "src-resource", "configs")
	mustMkdir(t, resSrc)
	mustWrite(t, filepath.Join(resSrc, ".editorconfig"), "root = true\n")
	mustWrite(t, filepath.Join(resSrc, ".prettierrc"), "{}\n")

	rname, _, err := s.AddResource(resSrc, false)
	if err != nil {
		t.Fatalf("add resource: %v", err)
	}
	if rname != "configs" {
		t.Fatalf("expected resource name 'configs', got %q", rname)
	}

	// ── 3. create bundle TOML ──────────────────────────────────────
	bundleTOML := `[bundle]
name = "test-bundle"
agents_md = "default"

[skills]
include = ["my-skill"]

[plugins]
include = ["my-plugin"]

[resources]
include = ["configs"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "test-bundle.toml"), bundleTOML)

	// ── 4. init repo ───────────────────────────────────────────────
	repo1 := filepath.Join(tmp, "repo1")
	mustMkdir(t, repo1)
	mustWrite(t, filepath.Join(repo1, ".agentfiles"), `bundle = "test-bundle"`+"\n"+`layout = "pi"`+"\n")

	// ── 5. apply → verify pi layout paths ──────────────────────────
	m, err := manifest.Load(repo1)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	res, err := apply.Apply(map[string]*store.Store{"default": s}, "default", m, repo1, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 4 { // agent + skill + plugin + resource
		t.Fatalf("expected 4 deployed, got %d", res.Deployed)
	}

	// Verify AGENTS.md
	assertFileContains(t, filepath.Join(repo1, "AGENTS.md"), "Agent Instructions")

	// Verify .pi/skills/my-skill/SKILL.md
	skillDeployed := filepath.Join(repo1, ".pi", "skills", "my-skill", "SKILL.md")
	assertFileContains(t, skillDeployed, "Original content")

	// Verify .pi/skills/my-skill/helper.sh
	assertFileExists(t, filepath.Join(repo1, ".pi", "skills", "my-skill", "helper.sh"))

	// Verify .pi/plugins/my-plugin/plugin.lua
	assertFileContains(t, filepath.Join(repo1, ".pi", "plugins", "my-plugin", "plugin.lua"), "plugin code")

	// Verify resources deployed to repo root
	assertFileContains(t, filepath.Join(repo1, ".editorconfig"), "root = true")
	assertFileContains(t, filepath.Join(repo1, ".prettierrc"), "{}")

	// ── 5b. verify lock file ───────────────────────────────────────
	lf1, err := lock.Load(repo1)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}
	if lf1.Deployed.AgentsMD == nil {
		t.Fatal("lock missing agents_md entry")
	}
	if lf1.Deployed.AgentsMD.Hash == "" {
		t.Fatal("lock agents_md hash is empty")
	}
	if _, ok := lf1.Deployed.Skills["my-skill"]; !ok {
		t.Fatal("lock missing skill 'my-skill'")
	}
	if _, ok := lf1.Deployed.Plugins["my-plugin"]; !ok {
		t.Fatal("lock missing plugin 'my-plugin'")
	}
	if _, ok := lf1.Deployed.Resources["configs"]; !ok {
		t.Fatal("lock missing resource 'configs'")
	}

	// Record hashes for later comparison.
	skillHashBefore := lf1.Deployed.Skills["my-skill"].Hash

	// ── 6. modify skill SKILL.md in repo ───────────────────────────
	mustWrite(t, skillDeployed, "# My Skill\nModified in repo\n")

	// ── 7. push → verify change in store ───────────────────────────
	pushRes, err := push.Push(map[string]*store.Store{"default": s}, "default", repo1, push.Options{})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(pushRes.Changes) == 0 {
		t.Fatal("push reported no changes after modifying skill")
	}

	// Find the skill change.
	foundSkillChange := false
	for _, ch := range pushRes.Changes {
		if ch.Name == "my-skill" && ch.Type == lock.AssetSkills {
			foundSkillChange = true
			if ch.OldHash == ch.NewHash {
				t.Fatal("push skill change has identical old/new hash")
			}
		}
	}
	if !foundSkillChange {
		t.Fatal("push did not report my-skill change")
	}

	// Verify store file was updated.
	storeSkillMD := filepath.Join(s.SkillsDir(), "my-skill", "SKILL.md")
	assertFileContains(t, storeSkillMD, "Modified in repo")

	// Verify lock was updated.
	lf1After, err := lock.Load(repo1)
	if err != nil {
		t.Fatalf("load lock after push: %v", err)
	}
	skillHashAfter := lf1After.Deployed.Skills["my-skill"].Hash
	if skillHashAfter == skillHashBefore {
		t.Fatal("lock hash not updated after push")
	}

	// ── 8. init + apply second repo ────────────────────────────────
	repo2 := filepath.Join(tmp, "repo2")
	mustMkdir(t, repo2)
	mustWrite(t, filepath.Join(repo2, ".agentfiles"), `bundle = "test-bundle"`+"\n"+`layout = "pi"`+"\n")

	m2, err := manifest.Load(repo2)
	if err != nil {
		t.Fatalf("load manifest repo2: %v", err)
	}

	res2, err := apply.Apply(map[string]*store.Store{"default": s}, "default", m2, repo2, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply repo2: %v", err)
	}
	if res2.Deployed != 4 {
		t.Fatalf("expected 4 deployed in repo2, got %d", res2.Deployed)
	}

	// ── 9. verify second repo has pushed change ────────────────────
	skill2Deployed := filepath.Join(repo2, ".pi", "skills", "my-skill", "SKILL.md")
	assertFileContains(t, skill2Deployed, "Modified in repo")

	// ── 10. verify lock file accuracy in repo2 ─────────────────────
	lf2, err := lock.Load(repo2)
	if err != nil {
		t.Fatalf("load lock repo2: %v", err)
	}
	if lf2.Deployed.Skills["my-skill"].Hash != skillHashAfter {
		t.Fatalf("repo2 lock hash %q != repo1 post-push hash %q",
			lf2.Deployed.Skills["my-skill"].Hash, skillHashAfter)
	}
	// Agent, plugin, resource hashes should match between repos.
	if lf2.Deployed.AgentsMD.Hash != lf1After.Deployed.AgentsMD.Hash {
		t.Fatal("agent hash mismatch between repos")
	}
	if lf2.Deployed.Plugins["my-plugin"].Hash != lf1After.Deployed.Plugins["my-plugin"].Hash {
		t.Fatal("plugin hash mismatch between repos")
	}
	if lf2.Deployed.Resources["configs"].Hash != lf1After.Deployed.Resources["configs"].Hash {
		t.Fatal("resource hash mismatch between repos")
	}

	// Verify a second push from repo2 reports no changes (everything in sync).
	pushRes2, err := push.Push(map[string]*store.Store{"default": s}, "default", repo2, push.Options{})
	if err != nil {
		t.Fatalf("push repo2: %v", err)
	}
	if len(pushRes2.Changes) != 0 {
		t.Fatalf("expected no changes on push from fresh repo2, got %d", len(pushRes2.Changes))
	}
}

// TestIntegrationAllLayout exercises the "all" layout which creates symlinks
// and pointer files across pi, claude, and cursor directories.
func TestIntegrationAllLayout(t *testing.T) {
	tmp := t.TempDir()

	storePath := filepath.Join(tmp, "store")
	s, err := store.Init(storePath)
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	// Add a skill and agent.
	skillSrc := filepath.Join(tmp, "src-skill", "my-skill")
	mustMkdir(t, skillSrc)
	mustWrite(t, filepath.Join(skillSrc, "SKILL.md"), "# My Skill\n")

	if _, _, err := s.AddSkill(skillSrc, false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Agent\n")
	if _, err := s.AddAgent(agentSrc, "default", false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	// Add a plugin.
	pluginSrc := filepath.Join(tmp, "src-plugin", "my-plugin")
	mustMkdir(t, pluginSrc)
	mustWrite(t, filepath.Join(pluginSrc, "plugin.lua"), "-- code\n")
	if _, _, err := s.AddPlugin(pluginSrc, false); err != nil {
		t.Fatalf("add plugin: %v", err)
	}

	// Bundle
	bundleTOML := `[bundle]
name = "all-bundle"
agents_md = "default"

[skills]
include = ["my-skill"]

[plugins]
include = ["my-plugin"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "all-bundle.toml"), bundleTOML)

	// Create repo with "all" layout.
	repo := filepath.Join(tmp, "repo-all")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), "bundle = \"all-bundle\"\nlayout = \"all\"\n")

	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	res, err := apply.Apply(map[string]*store.Store{"default": s}, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 3 { // agent + skill + plugin
		t.Fatalf("expected 3 deployed, got %d", res.Deployed)
	}

	// Primary pi layout files should exist.
	assertFileContains(t, filepath.Join(repo, "AGENTS.md"), "# Agent")
	assertFileExists(t, filepath.Join(repo, ".pi", "skills", "my-skill", "SKILL.md"))
	assertFileExists(t, filepath.Join(repo, ".pi", "plugins", "my-plugin", "plugin.lua"))

	// CLAUDE.md should be a full copy.
	assertFileContains(t, filepath.Join(repo, "CLAUDE.md"), "# Agent")

	// Claude skill and plugin should be regular copies.
	assertFileExists(t, filepath.Join(repo, ".claude", "skills", "my-skill", "SKILL.md"))
	assertFileExists(t, filepath.Join(repo, ".claude", "plugins", "my-plugin", "plugin.lua"))

	// Cursor layout files should exist as regular files.
	assertFileExists(t, filepath.Join(repo, ".cursorrules"))
	assertFileExists(t, filepath.Join(repo, ".cursor", "skills", "my-skill", "SKILL.md"))
	assertFileExists(t, filepath.Join(repo, ".cursor", "plugins", "my-plugin", "plugin.lua"))
}

// TestIntegrationCherryPick exercises cherry-pick mode (manifest without bundle).
func TestIntegrationCherryPick(t *testing.T) {
	tmp := t.TempDir()

	storePath := filepath.Join(tmp, "store")
	s, err := store.Init(storePath)
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	// Add assets.
	skillSrc := filepath.Join(tmp, "src-skill", "browse")
	mustMkdir(t, skillSrc)
	mustWrite(t, filepath.Join(skillSrc, "SKILL.md"), "# Browse\n")
	if _, _, err := s.AddSkill(skillSrc, false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	skillSrc2 := filepath.Join(tmp, "src-skill", "search")
	mustMkdir(t, skillSrc2)
	mustWrite(t, filepath.Join(skillSrc2, "SKILL.md"), "# Search\n")
	if _, _, err := s.AddSkill(skillSrc2, false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Cherry Agent\n")
	if _, err := s.AddAgent(agentSrc, "cherry", false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	pluginSrc := filepath.Join(tmp, "src-plugin", "fmt")
	mustMkdir(t, pluginSrc)
	mustWrite(t, filepath.Join(pluginSrc, "fmt.lua"), "-- fmt\n")
	if _, _, err := s.AddPlugin(pluginSrc, false); err != nil {
		t.Fatalf("add plugin: %v", err)
	}

	resSrc := filepath.Join(tmp, "src-res", "dotfiles")
	mustMkdir(t, resSrc)
	mustWrite(t, filepath.Join(resSrc, ".editorconfig"), "root = true\n")
	if _, _, err := s.AddResource(resSrc, false); err != nil {
		t.Fatalf("add resource: %v", err)
	}

	// Create repo with cherry-pick manifest (no bundle).
	repo := filepath.Join(tmp, "repo-cherry")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), `layout = "pi"
agents_md = "cherry"
skills = ["browse"]
plugins = ["fmt"]
resources = ["dotfiles"]
`)

	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	res, err := apply.Apply(map[string]*store.Store{"default": s}, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 4 { // agent + skill + plugin + resource
		t.Fatalf("expected 4 deployed, got %d", res.Deployed)
	}

	// Verify correct files deployed.
	assertFileContains(t, filepath.Join(repo, "AGENTS.md"), "Cherry Agent")
	assertFileContains(t, filepath.Join(repo, ".pi", "skills", "browse", "SKILL.md"), "# Browse")
	assertFileContains(t, filepath.Join(repo, ".pi", "plugins", "fmt", "fmt.lua"), "-- fmt")
	assertFileContains(t, filepath.Join(repo, ".editorconfig"), "root = true")

	// "search" skill should NOT be deployed.
	if _, err := os.Stat(filepath.Join(repo, ".pi", "skills", "search")); !os.IsNotExist(err) {
		t.Fatal("search skill should not be deployed in cherry-pick mode")
	}

	// Verify lock file.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}
	if _, ok := lf.Deployed.Skills["browse"]; !ok {
		t.Fatal("lock missing skill 'browse'")
	}
	if _, ok := lf.Deployed.Skills["search"]; ok {
		t.Fatal("lock should not contain 'search'")
	}
}

// TestIntegrationDiffAndStatus verifies af diff and af status work in a round-trip.
func TestIntegrationDiffAndStatus(t *testing.T) {
	tmp := t.TempDir()

	storePath := filepath.Join(tmp, "store")
	s, err := store.Init(storePath)
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	// Add assets.
	skillSrc := filepath.Join(tmp, "src-skill", "my-skill")
	mustMkdir(t, skillSrc)
	mustWrite(t, filepath.Join(skillSrc, "SKILL.md"), "# My Skill\nOriginal\n")

	if _, _, err := s.AddSkill(skillSrc, false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Agent\n")
	if _, err := s.AddAgent(agentSrc, "default", false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	bundleTOML := `[bundle]
name = "diff-test"
agents_md = "default"

[skills]
include = ["my-skill"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "diff-test.toml"), bundleTOML)

	repo := filepath.Join(tmp, "repo-diff")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), "bundle = \"diff-test\"\nlayout = \"pi\"\n")

	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	if _, err := apply.Apply(map[string]*store.Store{"default": s}, "default", m, repo, apply.Options{Force: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Load lock and verify store paths are relative.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}
	if lf.Deployed.AgentsMD == nil {
		t.Fatal("lock missing agents_md")
	}
	if strings.HasPrefix(lf.Deployed.AgentsMD.StorePath, "/") {
		t.Fatalf("expected relative store path, got %q", lf.Deployed.AgentsMD.StorePath)
	}

	// Status should show all unchanged.
	for name, e := range lf.Deployed.Skills {
		absStore := filepath.Join(s.Root, e.StorePath)
		newHash, err := lock.HashDir(absStore)
		if err != nil {
			t.Fatalf("hashing skill %q: %v", name, err)
		}
		if newHash != e.Hash {
			t.Fatalf("skill %q hash mismatch: lock=%q computed=%q", name, e.Hash, newHash)
		}
	}

	// Modify a deployed file and verify diff detects the change.
	skillDeployed := filepath.Join(repo, ".pi", "skills", "my-skill", "SKILL.md")
	mustWrite(t, skillDeployed, "# My Skill\nModified locally\n")

	// Re-load lock and check hashes diverge for deployed.
	deployedHash, err := lock.HashDir(filepath.Join(repo, ".pi", "skills", "my-skill"))
	if err != nil {
		t.Fatalf("hashing deployed skill: %v", err)
	}
	if deployedHash == lf.Deployed.Skills["my-skill"].Hash {
		t.Fatal("expected deployed hash to differ from lock after local edit")
	}

	// Push should detect the change.
	pushRes, err := push.Push(map[string]*store.Store{"default": s}, "default", repo, push.Options{})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(pushRes.Changes) == 0 {
		t.Fatal("expected push to report changes after local edit")
	}

	// After push, store should have the modified content.
	assertFileContains(t, filepath.Join(s.SkillsDir(), "my-skill", "SKILL.md"), "Modified locally")
}

// ── helpers ────────────────────────────────────────────────────────

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
}

func assertFileContains(t *testing.T, path, substr string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !strings.Contains(string(data), substr) {
		t.Fatalf("file %s does not contain %q; got: %s", path, substr, string(data))
	}
}
