package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devbydaniel/agentfiles/internal/apply"
	"github.com/devbydaniel/agentfiles/internal/layout"
	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/push"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// TestIntegrationRoundTrip exercises the full lifecycle:
//
//  1. init-store
//  2. add skill, agent, resource
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

	name, _, err := s.AddSkill(skillSrc, "", false)
	if err != nil {
		t.Fatalf("add skill: %v", err)
	}
	if name != "my-skill" {
		t.Fatalf("expected skill name 'my-skill', got %q", name)
	}

	// agent: create a source agent file
	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Agent Instructions\nBe helpful.\n")

	_, err = s.AddInstruction(agentSrc, "default", false)
	if err != nil {
		t.Fatalf("add agent: %v", err)
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

	// subagent: create a source agent .md file
	subagentSrc := filepath.Join(tmp, "src-subagent", "reviewer.md")
	mustMkdir(t, filepath.Dir(subagentSrc))
	mustWrite(t, subagentSrc, "---\nname: reviewer\ndescription: Code reviewer\n---\nReview all PRs carefully.\n")

	saName, _, err := s.AddAgent(subagentSrc, false)
	if err != nil {
		t.Fatalf("add subagent: %v", err)
	}
	if saName != "reviewer" {
		t.Fatalf("expected agent name 'reviewer', got %q", saName)
	}

	// ── 3. create bundle TOML ──────────────────────────────────────
	bundleTOML := `[bundle]
name = "test-bundle"
instructions = "default"

[skills]
include = ["my-skill"]

[resources]
include = ["configs"]

[agents]
include = ["reviewer"]
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
	if res.Deployed != 3 { // agent + skill + resource
		t.Fatalf("expected 3 deployed, got %d", res.Deployed)
	}

	// Verify AGENTS.md
	assertFileContains(t, filepath.Join(repo1, "AGENTS.md"), "Agent Instructions")

	// Verify .pi/skills/my-skill/SKILL.md
	skillDeployed := filepath.Join(repo1, ".pi", "skills", "my-skill", "SKILL.md")
	assertFileContains(t, skillDeployed, "Original content")

	// Verify .pi/skills/my-skill/helper.sh
	assertFileExists(t, filepath.Join(repo1, ".pi", "skills", "my-skill", "helper.sh"))

	// Verify resources deployed to repo root
	assertFileContains(t, filepath.Join(repo1, ".editorconfig"), "root = true")
	assertFileContains(t, filepath.Join(repo1, ".prettierrc"), "{}")

	// ── 5b. verify lock file ───────────────────────────────────────
	lf1, err := lock.Load(repo1)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}
	if lf1.Deployed.Instructions == nil {
		t.Fatal("lock missing instructions entry")
	}
	if lf1.Deployed.Instructions.Hash == "" {
		t.Fatal("lock instructions hash is empty")
	}
	if _, ok := lf1.Deployed.Skills["my-skill"]; !ok {
		t.Fatal("lock missing skill 'my-skill'")
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
	if res2.Deployed != 3 {
		t.Fatalf("expected 3 deployed in repo2, got %d", res2.Deployed)
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
	// Agent and resource hashes should match between repos.
	if lf2.Deployed.Instructions.Hash != lf1After.Deployed.Instructions.Hash {
		t.Fatal("agent hash mismatch between repos")
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

	if _, _, err := s.AddSkill(skillSrc, "", false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Agent\n")
	if _, err := s.AddInstruction(agentSrc, "default", false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	// Bundle
	bundleTOML := `[bundle]
name = "all-bundle"
instructions = "default"

[skills]
include = ["my-skill"]
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
	if res.Deployed != 2 { // agent + skill
		t.Fatalf("expected 2 deployed, got %d", res.Deployed)
	}

	// Primary pi layout files should exist.
	assertFileContains(t, filepath.Join(repo, "AGENTS.md"), "# Agent")
	assertFileExists(t, filepath.Join(repo, ".pi", "skills", "my-skill", "SKILL.md"))

	// CLAUDE.md should be a full copy.
	assertFileContains(t, filepath.Join(repo, "CLAUDE.md"), "# Agent")

	// Claude skill should be a regular copy.
	assertFileExists(t, filepath.Join(repo, ".claude", "skills", "my-skill", "SKILL.md"))

	// Cursor layout files should exist as regular files.
	assertFileExists(t, filepath.Join(repo, ".cursor", "skills", "my-skill", "SKILL.md"))
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
	if _, _, err := s.AddSkill(skillSrc, "", false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	skillSrc2 := filepath.Join(tmp, "src-skill", "search")
	mustMkdir(t, skillSrc2)
	mustWrite(t, filepath.Join(skillSrc2, "SKILL.md"), "# Search\n")
	if _, _, err := s.AddSkill(skillSrc2, "", false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Cherry Agent\n")
	if _, err := s.AddInstruction(agentSrc, "cherry", false); err != nil {
		t.Fatalf("add agent: %v", err)
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
instructions = "cherry"
skills = ["browse"]
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
	if res.Deployed != 3 { // agent + skill + resource
		t.Fatalf("expected 3 deployed, got %d", res.Deployed)
	}

	// Verify correct files deployed.
	assertFileContains(t, filepath.Join(repo, "AGENTS.md"), "Cherry Agent")
	assertFileContains(t, filepath.Join(repo, ".pi", "skills", "browse", "SKILL.md"), "# Browse")
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

	if _, _, err := s.AddSkill(skillSrc, "", false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Agent\n")
	if _, err := s.AddInstruction(agentSrc, "default", false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	bundleTOML := `[bundle]
name = "diff-test"
instructions = "default"

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
	if lf.Deployed.Instructions == nil {
		t.Fatal("lock missing instructions")
	}
	if strings.HasPrefix(lf.Deployed.Instructions.StorePath, "/") {
		t.Fatalf("expected relative store path, got %q", lf.Deployed.Instructions.StorePath)
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

// TestIntegrationMultiStore exercises cross-store apply and push:
//  1. Create two stores (personal + work).
//  2. Add a skill to personal store, a bundle + agent + skill to work store.
//  3. Build a store map with work as default.
//  4. Create a repo manifest with bundle from work + skills_add from personal.
//  5. Apply — verify both skills deployed, lock entries have correct store names.
//  6. Modify the personal skill in the repo.
//  7. Push — verify the change goes to personal store, not work store.
//  8. Apply to a second repo — verify the pushed change propagates.
func TestIntegrationMultiStore(t *testing.T) {
	tmp := t.TempDir()

	// ── 1. Create two stores ───────────────────────────────────────
	personalPath := filepath.Join(tmp, "store-personal")
	personal, err := store.Init(personalPath)
	if err != nil {
		t.Fatalf("init personal store: %v", err)
	}

	workPath := filepath.Join(tmp, "store-work")
	work, err := store.Init(workPath)
	if err != nil {
		t.Fatalf("init work store: %v", err)
	}

	// ── 2. Add assets ──────────────────────────────────────────────
	// Personal store: one skill
	pSkillSrc := filepath.Join(tmp, "src-personal-skill", "my-personal-skill")
	mustMkdir(t, pSkillSrc)
	mustWrite(t, filepath.Join(pSkillSrc, "SKILL.md"), "# Personal Skill\nOriginal content\n")

	if _, _, err := personal.AddSkill(pSkillSrc, "", false); err != nil {
		t.Fatalf("add personal skill: %v", err)
	}

	// Work store: agent, skill, bundle
	wSkillSrc := filepath.Join(tmp, "src-work-skill", "work-skill")
	mustMkdir(t, wSkillSrc)
	mustWrite(t, filepath.Join(wSkillSrc, "SKILL.md"), "# Work Skill\n")

	if _, _, err := work.AddSkill(wSkillSrc, "", false); err != nil {
		t.Fatalf("add work skill: %v", err)
	}

	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Work Agent\n")
	if _, err := work.AddInstruction(agentSrc, "default", false); err != nil {
		t.Fatalf("add work agent: %v", err)
	}

	bundleTOML := `[bundle]
name = "backend"
instructions = "default"

[skills]
include = ["work-skill"]
`
	mustWrite(t, filepath.Join(work.BundlesDir(), "backend.toml"), bundleTOML)

	// ── 3. Build store map ─────────────────────────────────────────
	stores := map[string]*store.Store{
		"personal": personal,
		"work":     work,
	}
	defaultStore := "work"

	// ── 4. Create repo manifest with cross-store skills_add ────────
	repo1 := filepath.Join(tmp, "repo1")
	mustMkdir(t, repo1)
	mustWrite(t, filepath.Join(repo1, ".agentfiles"), `bundle = "backend"
layout = "pi"
skills_add = ["personal:my-personal-skill"]
`)

	m, err := manifest.Load(repo1)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// ── 5. Apply — verify both skills deployed with correct provenance ──
	res, err := apply.Apply(stores, defaultStore, m, repo1, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// agent + work-skill + personal skill = 3 deployed
	if res.Deployed != 3 {
		t.Fatalf("expected 3 deployed, got %d", res.Deployed)
	}

	// Verify files exist.
	assertFileContains(t, filepath.Join(repo1, "AGENTS.md"), "Work Agent")
	assertFileContains(t, filepath.Join(repo1, ".pi", "skills", "work-skill", "SKILL.md"), "# Work Skill")
	assertFileContains(t, filepath.Join(repo1, ".pi", "skills", "my-personal-skill", "SKILL.md"), "# Personal Skill")

	// Verify lock entries have correct store names.
	lf, err := lock.Load(repo1)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}

	// Work skill uses default store, so lock key is just "work-skill".
	workEntry, ok := lf.Deployed.Skills["work-skill"]
	if !ok {
		t.Fatal("lock missing skill 'work-skill'")
	}
	if workEntry.Store != "work" {
		t.Fatalf("expected work-skill store='work', got %q", workEntry.Store)
	}

	// Personal skill is from non-default store, lock key is "personal:my-personal-skill".
	personalEntry, ok := lf.Deployed.Skills["personal:my-personal-skill"]
	if !ok {
		// Dump actual keys for debugging.
		var keys []string
		for k := range lf.Deployed.Skills {
			keys = append(keys, k)
		}
		t.Fatalf("lock missing skill 'personal:my-personal-skill'; keys: %v", keys)
	}
	if personalEntry.Store != "personal" {
		t.Fatalf("expected personal skill store='personal', got %q", personalEntry.Store)
	}

	personalHashBefore := personalEntry.Hash

	// ── 6. Modify personal skill in the repo ───────────────────────
	personalSkillDeployed := filepath.Join(repo1, ".pi", "skills", "my-personal-skill", "SKILL.md")
	mustWrite(t, personalSkillDeployed, "# Personal Skill\nModified in repo\n")

	// ── 7. Push — verify change goes to personal store ─────────────
	pushRes, err := push.Push(stores, defaultStore, repo1, push.Options{})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(pushRes.Changes) == 0 {
		t.Fatal("push reported no changes after modifying personal skill")
	}

	// The personal store should have the updated content.
	assertFileContains(t, filepath.Join(personal.SkillsDir(), "my-personal-skill", "SKILL.md"), "Modified in repo")

	// The work store skill should be unchanged.
	assertFileContains(t, filepath.Join(work.SkillsDir(), "work-skill", "SKILL.md"), "# Work Skill")

	// Verify lock was updated with new hash.
	lfAfter, err := lock.Load(repo1)
	if err != nil {
		t.Fatalf("load lock after push: %v", err)
	}
	personalHashAfter := lfAfter.Deployed.Skills["personal:my-personal-skill"].Hash
	if personalHashAfter == personalHashBefore {
		t.Fatal("personal skill lock hash not updated after push")
	}

	// ── 8. Apply to second repo — verify pushed change propagates ──
	repo2 := filepath.Join(tmp, "repo2")
	mustMkdir(t, repo2)
	mustWrite(t, filepath.Join(repo2, ".agentfiles"), `bundle = "backend"
layout = "pi"
skills_add = ["personal:my-personal-skill"]
`)

	m2, err := manifest.Load(repo2)
	if err != nil {
		t.Fatalf("load manifest repo2: %v", err)
	}

	res2, err := apply.Apply(stores, defaultStore, m2, repo2, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply repo2: %v", err)
	}
	if res2.Deployed != 3 {
		t.Fatalf("expected 3 deployed in repo2, got %d", res2.Deployed)
	}

	// Repo2 should have the pushed modification from repo1.
	assertFileContains(t, filepath.Join(repo2, ".pi", "skills", "my-personal-skill", "SKILL.md"), "Modified in repo")

	// Hashes should match.
	lf2, err := lock.Load(repo2)
	if err != nil {
		t.Fatalf("load lock repo2: %v", err)
	}
	if lf2.Deployed.Skills["personal:my-personal-skill"].Hash != personalHashAfter {
		t.Fatal("repo2 personal skill hash doesn't match repo1 post-push hash")
	}

	// A second push from repo2 should show no changes.
	pushRes2, err := push.Push(stores, defaultStore, repo2, push.Options{})
	if err != nil {
		t.Fatalf("push repo2: %v", err)
	}
	if len(pushRes2.Changes) != 0 {
		t.Fatalf("expected no changes on push from fresh repo2, got %d", len(pushRes2.Changes))
	}
}

// TestIntegrationUserLevelDeploy exercises user-level apply and push:
//  1. Create a store with agent, skill.
//  2. Create a bundle.
//  3. Apply with user layout + home dir + user lock path.
//  4. Verify files land at correct user-level paths.
//  5. Modify a deployed file, push, verify store is updated.
func TestIntegrationUserLevelDeploy(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	mustMkdir(t, home)

	// ── 1. Create store with assets ────────────────────────────────
	storePath := filepath.Join(tmp, "store")
	s, err := store.Init(storePath)
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	// Agent
	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# User Agent\nGlobal instructions.\n")
	if _, err := s.AddInstruction(agentSrc, "global", false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	// Skill
	skillSrc := filepath.Join(tmp, "src-skill", "browse")
	mustMkdir(t, skillSrc)
	mustWrite(t, filepath.Join(skillSrc, "SKILL.md"), "# Browse\nOriginal content\n")
	if _, _, err := s.AddSkill(skillSrc, "", false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	// ── 2. Create bundle ───────────────────────────────────────────
	bundleTOML := `[bundle]
name = "user-bundle"
instructions = "global"

[skills]
include = ["browse"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "user-bundle.toml"), bundleTOML)

	// ── 3. Build manifest and apply with user layout ───────────────
	m, err := manifest.FromUserConfig(manifest.UserFields{
		Bundle: "user-bundle",
		Layout: "all",
	})
	if err != nil {
		t.Fatalf("FromUserConfig: %v", err)
	}

	lay, err := layout.GetUser(m.Layout)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}

	stores := map[string]*store.Store{"default": s}
	userLockPath := filepath.Join(tmp, "user.lock")

	res, err := apply.Apply(stores, "default", m, home, apply.Options{
		Force:        true,
		LockFilePath: userLockPath,
		Layout:       lay,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 2 { // agent + skill
		t.Fatalf("expected 2 deployed, got %d", res.Deployed)
	}

	// ── 4. Verify files at user-level paths ────────────────────────
	// Pi layout paths
	assertFileContains(t, filepath.Join(home, "AGENTS.md"), "User Agent")
	assertFileContains(t, filepath.Join(home, ".pi", "agent", "skills", "browse", "SKILL.md"), "# Browse")

	// Claude layout paths (user-all deploys to all)
	assertFileContains(t, filepath.Join(home, ".claude", "CLAUDE.md"), "User Agent")
	assertFileExists(t, filepath.Join(home, ".claude", "skills", "browse", "SKILL.md"))

	// Cursor layout paths
	assertFileContains(t, filepath.Join(home, ".cursor", "rules", "agentfiles.md"), "User Agent")
	assertFileExists(t, filepath.Join(home, ".cursor", "skills", "browse", "SKILL.md"))

	// ── 4b. Verify user lock file ──────────────────────────────────
	lf, err := lock.LoadFrom(userLockPath)
	if err != nil {
		t.Fatalf("load user lock: %v", err)
	}
	if lf.Deployed.Instructions == nil {
		t.Fatal("user lock missing instructions entry")
	}
	if _, ok := lf.Deployed.Skills["browse"]; !ok {
		t.Fatal("user lock missing skill 'browse'")
	}
	skillHashBefore := lf.Deployed.Skills["browse"].Hash

	// ── 5. Modify deployed file, push, verify store updated ────────
	// Modify the pi-layout copy (primary for "all" layout).
	mustWrite(t, filepath.Join(home, ".pi", "agent", "skills", "browse", "SKILL.md"), "# Browse\nModified by user\n")

	pushRes, err := push.Push(stores, "default", home, push.Options{
		LockFilePath: userLockPath,
	})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(pushRes.Changes) == 0 {
		t.Fatal("push reported no changes after modifying skill")
	}

	// Store should have the updated content.
	assertFileContains(t, filepath.Join(s.SkillsDir(), "browse", "SKILL.md"), "Modified by user")

	// Lock should be updated.
	lfAfter, err := lock.LoadFrom(userLockPath)
	if err != nil {
		t.Fatalf("load lock after push: %v", err)
	}
	if lfAfter.Deployed.Skills["browse"].Hash == skillHashBefore {
		t.Fatal("skill hash not updated after push")
	}
}

// TestIntegrationUserLevelCherryPick exercises user-level cherry-pick mode.
func TestIntegrationUserLevelCherryPick(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	mustMkdir(t, home)

	storePath := filepath.Join(tmp, "store")
	s, err := store.Init(storePath)
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	// Add agent and skill.
	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Cherry Agent\n")
	if _, err := s.AddInstruction(agentSrc, "cherry", false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	skillSrc := filepath.Join(tmp, "src-skill", "git")
	mustMkdir(t, skillSrc)
	mustWrite(t, filepath.Join(skillSrc, "SKILL.md"), "# Git\n")
	if _, _, err := s.AddSkill(skillSrc, "", false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	// Cherry-pick manifest with claude layout.
	m, err := manifest.FromUserConfig(manifest.UserFields{
		Instructions: "cherry",
		Skills:   []string{"git"},
		Layout:   "claude",
	})
	if err != nil {
		t.Fatalf("FromUserConfig: %v", err)
	}

	lay, err := layout.GetUser(m.Layout)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}

	stores := map[string]*store.Store{"default": s}
	userLockPath := filepath.Join(tmp, "user.lock")

	res, err := apply.Apply(stores, "default", m, home, apply.Options{
		Force:        true,
		LockFilePath: userLockPath,
		Layout:       lay,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 2 {
		t.Fatalf("expected 2 deployed, got %d", res.Deployed)
	}

	// Verify claude user paths.
	assertFileContains(t, filepath.Join(home, ".claude", "CLAUDE.md"), "Cherry Agent")
	assertFileContains(t, filepath.Join(home, ".claude", "skills", "git", "SKILL.md"), "# Git")
}

// TestIntegrationUserLevelMultiStore exercises user-level with cross-store references.
func TestIntegrationUserLevelMultiStore(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	mustMkdir(t, home)

	// Two stores.
	personalPath := filepath.Join(tmp, "store-personal")
	personal, err := store.Init(personalPath)
	if err != nil {
		t.Fatalf("init personal store: %v", err)
	}

	workPath := filepath.Join(tmp, "store-work")
	work, err := store.Init(workPath)
	if err != nil {
		t.Fatalf("init work store: %v", err)
	}

	// Personal store: one skill.
	pSkillSrc := filepath.Join(tmp, "src-pskill", "personal-skill")
	mustMkdir(t, pSkillSrc)
	mustWrite(t, filepath.Join(pSkillSrc, "SKILL.md"), "# Personal Skill\nOriginal\n")
	if _, _, err := personal.AddSkill(pSkillSrc, "", false); err != nil {
		t.Fatalf("add personal skill: %v", err)
	}

	// Work store: agent, skill, bundle.
	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Work Agent\n")
	if _, err := work.AddInstruction(agentSrc, "default", false); err != nil {
		t.Fatalf("add work agent: %v", err)
	}

	wSkillSrc := filepath.Join(tmp, "src-wskill", "work-skill")
	mustMkdir(t, wSkillSrc)
	mustWrite(t, filepath.Join(wSkillSrc, "SKILL.md"), "# Work Skill\n")
	if _, _, err := work.AddSkill(wSkillSrc, "", false); err != nil {
		t.Fatalf("add work skill: %v", err)
	}

	bundleTOML := `[bundle]
name = "user-multi"
instructions = "default"

[skills]
include = ["work-skill"]
`
	mustWrite(t, filepath.Join(work.BundlesDir(), "user-multi.toml"), bundleTOML)

	// Build manifest with cross-store skills_add.
	m, err := manifest.FromUserConfig(manifest.UserFields{
		Bundle:    "user-multi",
		Layout:    "pi",
		SkillsAdd: []string{"personal:personal-skill"},
	})
	if err != nil {
		t.Fatalf("FromUserConfig: %v", err)
	}

	lay, err := layout.GetUser(m.Layout)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}

	storesMap := map[string]*store.Store{
		"personal": personal,
		"work":     work,
	}
	userLockPath := filepath.Join(tmp, "user.lock")

	res, err := apply.Apply(storesMap, "work", m, home, apply.Options{
		Force:        true,
		LockFilePath: userLockPath,
		Layout:       lay,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 3 { // agent + work-skill + personal-skill
		t.Fatalf("expected 3 deployed, got %d", res.Deployed)
	}

	// Verify both skills deployed.
	assertFileContains(t, filepath.Join(home, "AGENTS.md"), "Work Agent")
	assertFileContains(t, filepath.Join(home, ".pi", "agent", "skills", "work-skill", "SKILL.md"), "# Work Skill")
	assertFileContains(t, filepath.Join(home, ".pi", "agent", "skills", "personal-skill", "SKILL.md"), "# Personal Skill")

	// Verify lock entries have correct store provenance.
	lf, err := lock.LoadFrom(userLockPath)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}

	workEntry, ok := lf.Deployed.Skills["work-skill"]
	if !ok {
		t.Fatal("lock missing work-skill")
	}
	if workEntry.Store != "work" {
		t.Fatalf("work-skill store = %q, want 'work'", workEntry.Store)
	}

	personalEntry, ok := lf.Deployed.Skills["personal:personal-skill"]
	if !ok {
		var keys []string
		for k := range lf.Deployed.Skills {
			keys = append(keys, k)
		}
		t.Fatalf("lock missing 'personal:personal-skill'; keys: %v", keys)
	}
	if personalEntry.Store != "personal" {
		t.Fatalf("personal-skill store = %q, want 'personal'", personalEntry.Store)
	}

	// Modify the personal skill, push, verify it goes to personal store.
	mustWrite(t, filepath.Join(home, ".pi", "agent", "skills", "personal-skill", "SKILL.md"), "# Personal Skill\nModified\n")

	pushRes, err := push.Push(storesMap, "work", home, push.Options{
		LockFilePath: userLockPath,
	})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(pushRes.Changes) == 0 {
		t.Fatal("push reported no changes")
	}

	assertFileContains(t, filepath.Join(personal.SkillsDir(), "personal-skill", "SKILL.md"), "Modified")
	// Work store skill should be unchanged.
	assertFileContains(t, filepath.Join(work.SkillsDir(), "work-skill", "SKILL.md"), "# Work Skill")
}

// TestIntegrationApplyAllWithUser simulates what apply-all does:
// deploy user-level files AND repo-level files in sequence from the
// same store, verifying both succeed with independent lock files.
func TestIntegrationApplyAllWithUser(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	mustMkdir(t, home)

	// Create store with assets.
	storePath := filepath.Join(tmp, "store")
	s, err := store.Init(storePath)
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	agentSrc := filepath.Join(tmp, "src-agent.md")
	mustWrite(t, agentSrc, "# Shared Agent\n")
	if _, err := s.AddInstruction(agentSrc, "shared", false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	skillSrc := filepath.Join(tmp, "src-skill", "browse")
	mustMkdir(t, skillSrc)
	mustWrite(t, filepath.Join(skillSrc, "SKILL.md"), "# Browse\n")
	if _, _, err := s.AddSkill(skillSrc, "", false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	bundleTOML := `[bundle]
name = "shared-bundle"
instructions = "shared"

[skills]
include = ["browse"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "shared-bundle.toml"), bundleTOML)

	stores := map[string]*store.Store{"default": s}

	// ── User-level deploy (what apply-all does first) ──────────────
	userManifest, err := manifest.FromUserConfig(manifest.UserFields{
		Bundle: "shared-bundle",
		Layout: "all",
	})
	if err != nil {
		t.Fatalf("FromUserConfig: %v", err)
	}

	userLay, err := layout.GetUser(userManifest.Layout)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}

	userLockPath := filepath.Join(tmp, "user.lock")

	userRes, err := apply.Apply(stores, "default", userManifest, home, apply.Options{
		Force:        true,
		LockFilePath: userLockPath,
		Layout:       userLay,
	})
	if err != nil {
		t.Fatalf("user apply: %v", err)
	}
	if userRes.Deployed != 2 { // agent + skill
		t.Fatalf("user: expected 2 deployed, got %d", userRes.Deployed)
	}

	// ── Repo-level deploy (what apply-all does for each repo) ──────
	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), "bundle = \"shared-bundle\"\nlayout = \"pi\"\n")

	repoManifest, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load repo manifest: %v", err)
	}

	repoRes, err := apply.Apply(stores, "default", repoManifest, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("repo apply: %v", err)
	}
	if repoRes.Deployed != 2 {
		t.Fatalf("repo: expected 2 deployed, got %d", repoRes.Deployed)
	}

	// ── Verify both deployed independently ─────────────────────────

	// User-level paths (all layout).
	assertFileContains(t, filepath.Join(home, "AGENTS.md"), "Shared Agent")
	assertFileContains(t, filepath.Join(home, ".claude", "CLAUDE.md"), "Shared Agent")
	assertFileContains(t, filepath.Join(home, ".cursor", "rules", "agentfiles.md"), "Shared Agent")
	assertFileExists(t, filepath.Join(home, ".pi", "agent", "skills", "browse", "SKILL.md"))
	assertFileExists(t, filepath.Join(home, ".claude", "skills", "browse", "SKILL.md"))
	assertFileExists(t, filepath.Join(home, ".cursor", "skills", "browse", "SKILL.md"))

	// Repo-level paths (pi layout).
	assertFileContains(t, filepath.Join(repo, "AGENTS.md"), "Shared Agent")
	assertFileExists(t, filepath.Join(repo, ".pi", "skills", "browse", "SKILL.md"))

	// Lock files are independent.
	userLF, err := lock.LoadFrom(userLockPath)
	if err != nil {
		t.Fatalf("load user lock: %v", err)
	}
	repoLF, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("load repo lock: %v", err)
	}

	// Both should track the same assets but in separate lock files.
	if userLF.Deployed.Instructions == nil {
		t.Fatal("user lock missing instructions")
	}
	if repoLF.Deployed.Instructions == nil {
		t.Fatal("repo lock missing instructions")
	}
	if _, ok := userLF.Deployed.Skills["browse"]; !ok {
		t.Fatal("user lock missing skill 'browse'")
	}
	if _, ok := repoLF.Deployed.Skills["browse"]; !ok {
		t.Fatal("repo lock missing skill 'browse'")
	}

	// Hashes should match (same source).
	if userLF.Deployed.Instructions.Hash != repoLF.Deployed.Instructions.Hash {
		t.Fatal("agent hash differs between user and repo locks")
	}
	if userLF.Deployed.Skills["browse"].Hash != repoLF.Deployed.Skills["browse"].Hash {
		t.Fatal("skill hash differs between user and repo locks")
	}

	// Modifying repo skill and pushing should NOT affect user deployment.
	mustWrite(t, filepath.Join(repo, ".pi", "skills", "browse", "SKILL.md"), "# Browse\nRepo change\n")
	pushRes, err := push.Push(stores, "default", repo, push.Options{})
	if err != nil {
		t.Fatalf("repo push: %v", err)
	}
	if len(pushRes.Changes) == 0 {
		t.Fatal("repo push reported no changes")
	}

	// User-level file should still have original content (not synced by push).
	assertFileContains(t, filepath.Join(home, ".pi", "agent", "skills", "browse", "SKILL.md"), "# Browse\n")

	// But user push should now detect that the store changed and user is stale...
	// Actually, user push compares deployed vs lock hash, not store. So user push
	// should report no changes (user files weren't modified).
	userPushRes, err := push.Push(stores, "default", home, push.Options{
		LockFilePath: userLockPath,
	})
	if err != nil {
		t.Fatalf("user push: %v", err)
	}
	if len(userPushRes.Changes) != 0 {
		t.Fatalf("user push should report 0 changes, got %d", len(userPushRes.Changes))
	}
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

// TestIntegrationAgentRoundTrip exercises agents (subagents) in the lifecycle:
//  1. Create store with an agent .md file.
//  2. Create bundle including the agent.
//  3. Apply with claude layout — verify .md passthrough.
//  4. Modify the deployed .md agent, push back, verify store updated.
//  5. Apply to second repo — verify pushed change propagates.
func TestIntegrationAgentRoundTrip(t *testing.T) {
	tmp := t.TempDir()

	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Add agent (subagent) .md file.
	agentSrc := filepath.Join(tmp, "reviewer.md")
	mustWrite(t, agentSrc, "---\nname: reviewer\ndescription: Code reviewer\n---\nReview all PRs carefully.\n")
	if _, _, err := s.AddAgent(agentSrc, false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	// Add instructions (AGENTS.md) and skill for the bundle.
	instrSrc := filepath.Join(tmp, "instr.md")
	mustWrite(t, instrSrc, "# Instructions\n")
	if _, err := s.AddInstruction(instrSrc, "default", false); err != nil {
		t.Fatalf("add instruction: %v", err)
	}

	skillSrc := filepath.Join(tmp, "src-skill", "browse")
	mustMkdir(t, skillSrc)
	mustWrite(t, filepath.Join(skillSrc, "SKILL.md"), "# Browse\n")
	if _, _, err := s.AddSkill(skillSrc, "", false); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	// Bundle with agent.
	bundleTOML := `[bundle]
name = "agent-test"
instructions = "default"

[skills]
include = ["browse"]

[agents]
include = ["reviewer"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "agent-test.toml"), bundleTOML)

	// Create repo with claude layout.
	repo1 := filepath.Join(tmp, "repo1")
	mustMkdir(t, repo1)
	mustWrite(t, filepath.Join(repo1, ".agentfiles"), "bundle = \"agent-test\"\nlayout = \"claude\"\n")

	m, err := manifest.Load(repo1)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	stores := map[string]*store.Store{"default": s}
	res, err := apply.Apply(stores, "default", m, repo1, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// instructions + skill + agent = 3
	if res.Deployed != 3 {
		t.Fatalf("deployed = %d, want 3", res.Deployed)
	}

	// Verify agent deployed as .md at claude path.
	agentDeployed := filepath.Join(repo1, ".claude", "agents", "reviewer.md")
	assertFileContains(t, agentDeployed, "Review all PRs carefully.")
	assertFileContains(t, agentDeployed, "name: reviewer")

	// Verify lock has agent entry.
	lf, err := lock.Load(repo1)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}
	agentEntry, ok := lf.Deployed.Agents["reviewer"]
	if !ok {
		t.Fatal("lock missing agent 'reviewer'")
	}
	hashBefore := agentEntry.Hash

	// Modify the deployed agent.
	mustWrite(t, agentDeployed, "---\nname: reviewer\ndescription: Code reviewer\n---\nReview all PRs VERY carefully.\n")

	// Push — verify change goes to store.
	pushRes, err := push.Push(stores, "default", repo1, push.Options{})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(pushRes.Changes) == 0 {
		t.Fatal("push reported no changes after modifying agent")
	}
	foundAgentChange := false
	for _, ch := range pushRes.Changes {
		if ch.Name == "reviewer" && ch.Type == lock.AssetAgents {
			foundAgentChange = true
		}
	}
	if !foundAgentChange {
		t.Fatal("push did not report agent change")
	}

	// Store should have the updated content.
	assertFileContains(t, filepath.Join(s.AgentsDir(), "reviewer.md"), "VERY carefully")

	// Lock hash should have changed.
	lfAfter, err := lock.Load(repo1)
	if err != nil {
		t.Fatalf("load lock after push: %v", err)
	}
	if lfAfter.Deployed.Agents["reviewer"].Hash == hashBefore {
		t.Fatal("agent hash not updated after push")
	}

	// Apply to second repo — verify pushed change propagates.
	repo2 := filepath.Join(tmp, "repo2")
	mustMkdir(t, repo2)
	mustWrite(t, filepath.Join(repo2, ".agentfiles"), "bundle = \"agent-test\"\nlayout = \"claude\"\n")

	m2, err := manifest.Load(repo2)
	if err != nil {
		t.Fatalf("load manifest repo2: %v", err)
	}
	res2, err := apply.Apply(stores, "default", m2, repo2, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply repo2: %v", err)
	}
	if res2.Deployed != 3 {
		t.Fatalf("repo2 deployed = %d, want 3", res2.Deployed)
	}
	assertFileContains(t, filepath.Join(repo2, ".claude", "agents", "reviewer.md"), "VERY carefully")

	// Push from repo2 should show no changes.
	pushRes2, err := push.Push(stores, "default", repo2, push.Options{})
	if err != nil {
		t.Fatalf("push repo2: %v", err)
	}
	if len(pushRes2.Changes) != 0 {
		t.Fatalf("expected 0 changes from repo2 push, got %d", len(pushRes2.Changes))
	}
}

// TestIntegrationAgentCodexConversion tests Codex layout: deploy → .toml conversion,
// modify .toml, push → .md conversion back to store.
func TestIntegrationAgentCodexConversion(t *testing.T) {
	tmp := t.TempDir()

	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Agent with frontmatter.
	agentSrc := filepath.Join(tmp, "planner.md")
	mustWrite(t, agentSrc, "---\nname: planner\ndescription: Plans tasks\nmodel: o3\n---\nPlan everything step by step.\n")
	if _, _, err := s.AddAgent(agentSrc, false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	instrSrc := filepath.Join(tmp, "instr.md")
	mustWrite(t, instrSrc, "# Instructions\n")
	if _, err := s.AddInstruction(instrSrc, "default", false); err != nil {
		t.Fatalf("add instruction: %v", err)
	}

	bundleTOML := `[bundle]
name = "codex-test"
instructions = "default"

[agents]
include = ["planner"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "codex-test.toml"), bundleTOML)

	// Create repo with codex layout.
	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), "bundle = \"codex-test\"\nlayout = \"codex\"\n")

	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	stores := map[string]*store.Store{"default": s}
	res, err := apply.Apply(stores, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 2 { // instructions + agent
		t.Fatalf("deployed = %d, want 2", res.Deployed)
	}

	// Verify agent deployed as .toml.
	agentDeployed := filepath.Join(repo, ".codex", "agents", "planner.toml")
	assertFileExists(t, agentDeployed)
	data, err := os.ReadFile(agentDeployed)
	if err != nil {
		t.Fatalf("read deployed toml: %v", err)
	}
	tomlContent := string(data)
	if !strings.Contains(tomlContent, "developer_instructions") {
		t.Fatal("deployed TOML should contain developer_instructions")
	}
	if !strings.Contains(tomlContent, "planner") {
		t.Fatal("deployed TOML should contain agent name")
	}
	if !strings.Contains(tomlContent, "o3") {
		t.Fatal("deployed TOML should contain model field")
	}

	// Modify the deployed TOML.
	modified := strings.Replace(tomlContent, "Plans tasks", "Plans everything", 1)
	mustWrite(t, agentDeployed, modified)

	// Push — should convert TOML back to .md in store.
	pushRes, err := push.Push(stores, "default", repo, push.Options{})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(pushRes.Changes) == 0 {
		t.Fatal("push reported no changes after modifying codex agent")
	}

	// Store should have .md with the updated description.
	storeAgent := filepath.Join(s.AgentsDir(), "planner.md")
	assertFileContains(t, storeAgent, "Plans everything")
	// Should still be .md format (not TOML).
	assertFileContains(t, storeAgent, "---")
}

// TestIntegrationAgentAllLayout tests "all" layout deploys agents as both .md and .toml.
func TestIntegrationAgentAllLayout(t *testing.T) {
	tmp := t.TempDir()

	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	agentSrc := filepath.Join(tmp, "helper.md")
	mustWrite(t, agentSrc, "---\nname: helper\ndescription: Helps\n---\nBe helpful.\n")
	if _, _, err := s.AddAgent(agentSrc, false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	instrSrc := filepath.Join(tmp, "instr.md")
	mustWrite(t, instrSrc, "# Instructions\n")
	if _, err := s.AddInstruction(instrSrc, "default", false); err != nil {
		t.Fatalf("add instruction: %v", err)
	}

	bundleTOML := `[bundle]
name = "all-agent"
instructions = "default"

[agents]
include = ["helper"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "all-agent.toml"), bundleTOML)

	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), "bundle = \"all-agent\"\nlayout = \"all\"\n")

	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	stores := map[string]*store.Store{"default": s}
	res, err := apply.Apply(stores, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 2 { // instructions + agent
		t.Fatalf("deployed = %d, want 2", res.Deployed)
	}

	// Claude: .md
	assertFileContains(t, filepath.Join(repo, ".claude", "agents", "helper.md"), "Be helpful.")
	// Cursor: .md
	assertFileContains(t, filepath.Join(repo, ".cursor", "agents", "helper.md"), "Be helpful.")
	// Codex: .toml
	codexAgent := filepath.Join(repo, ".codex", "agents", "helper.toml")
	assertFileExists(t, codexAgent)
	codexData, _ := os.ReadFile(codexAgent)
	if !strings.Contains(string(codexData), "developer_instructions") {
		t.Fatal("codex agent should contain developer_instructions")
	}
	// Pi: nothing
	if _, err := os.Stat(filepath.Join(repo, ".pi", "agents")); !os.IsNotExist(err) {
		t.Fatal("pi layout should not deploy agents")
	}
}

// TestIntegrationAgentPiLayout verifies pi layout does not deploy agents.
func TestIntegrationAgentPiLayout(t *testing.T) {
	tmp := t.TempDir()

	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	agentSrc := filepath.Join(tmp, "helper.md")
	mustWrite(t, agentSrc, "---\nname: helper\n---\nBe helpful.\n")
	if _, _, err := s.AddAgent(agentSrc, false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	instrSrc := filepath.Join(tmp, "instr.md")
	mustWrite(t, instrSrc, "# Instructions\n")
	if _, err := s.AddInstruction(instrSrc, "default", false); err != nil {
		t.Fatalf("add instruction: %v", err)
	}

	bundleTOML := `[bundle]
name = "pi-agent"
instructions = "default"

[agents]
include = ["helper"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "pi-agent.toml"), bundleTOML)

	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), "bundle = \"pi-agent\"\nlayout = \"pi\"\n")

	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	stores := map[string]*store.Store{"default": s}
	res, err := apply.Apply(stores, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// Only instructions deployed (agent skipped for pi layout).
	if res.Deployed != 1 {
		t.Fatalf("deployed = %d, want 1 (instructions only)", res.Deployed)
	}

	// No agent files should exist.
	for _, p := range []string{
		filepath.Join(repo, ".pi", "agents"),
		filepath.Join(repo, ".claude", "agents"),
		filepath.Join(repo, ".cursor", "agents"),
		filepath.Join(repo, ".codex", "agents"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("expected no agent dir at %s", p)
		}
	}

	// Lock should NOT have agent entries (pi doesn't deploy them).
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}
	if len(lf.Deployed.Agents) != 0 {
		t.Fatalf("expected 0 agent lock entries for pi layout, got %d", len(lf.Deployed.Agents))
	}
}

// TestIntegrationAgentCherryPick exercises agents in cherry-pick mode (no bundle).
func TestIntegrationAgentCherryPick(t *testing.T) {
	tmp := t.TempDir()

	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Add two agents.
	mustWrite(t, filepath.Join(tmp, "reviewer.md"), "---\nname: reviewer\n---\nReview code.\n")
	if _, _, err := s.AddAgent(filepath.Join(tmp, "reviewer.md"), false); err != nil {
		t.Fatalf("add agent: %v", err)
	}
	mustWrite(t, filepath.Join(tmp, "planner.md"), "---\nname: planner\n---\nPlan tasks.\n")
	if _, _, err := s.AddAgent(filepath.Join(tmp, "planner.md"), false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	instrSrc := filepath.Join(tmp, "instr.md")
	mustWrite(t, instrSrc, "# Instructions\n")
	if _, err := s.AddInstruction(instrSrc, "default", false); err != nil {
		t.Fatalf("add instruction: %v", err)
	}

	// Cherry-pick only reviewer.
	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), "layout = \"claude\"\ninstructions = \"default\"\nagents = [\"reviewer\"]\n")

	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	stores := map[string]*store.Store{"default": s}
	res, err := apply.Apply(stores, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 2 { // instructions + reviewer
		t.Fatalf("deployed = %d, want 2", res.Deployed)
	}

	// reviewer deployed, planner not.
	assertFileContains(t, filepath.Join(repo, ".claude", "agents", "reviewer.md"), "Review code.")
	if _, err := os.Stat(filepath.Join(repo, ".claude", "agents", "planner.md")); !os.IsNotExist(err) {
		t.Fatal("planner should not be deployed in cherry-pick mode")
	}
}

// TestIntegrationAgentPruning verifies stale agents are removed when removed from manifest.
func TestIntegrationAgentPruning(t *testing.T) {
	tmp := t.TempDir()

	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	mustWrite(t, filepath.Join(tmp, "reviewer.md"), "---\nname: reviewer\n---\nReview code.\n")
	if _, _, err := s.AddAgent(filepath.Join(tmp, "reviewer.md"), false); err != nil {
		t.Fatalf("add agent: %v", err)
	}
	mustWrite(t, filepath.Join(tmp, "planner.md"), "---\nname: planner\n---\nPlan tasks.\n")
	if _, _, err := s.AddAgent(filepath.Join(tmp, "planner.md"), false); err != nil {
		t.Fatalf("add agent: %v", err)
	}

	instrSrc := filepath.Join(tmp, "instr.md")
	mustWrite(t, instrSrc, "# Instructions\n")
	if _, err := s.AddInstruction(instrSrc, "default", false); err != nil {
		t.Fatalf("add instruction: %v", err)
	}

	// First apply with both agents.
	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), "layout = \"claude\"\ninstructions = \"default\"\nagents = [\"reviewer\", \"planner\"]\n")

	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	stores := map[string]*store.Store{"default": s}
	if _, err := apply.Apply(stores, "default", m, repo, apply.Options{Force: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Both agents deployed.
	assertFileExists(t, filepath.Join(repo, ".claude", "agents", "reviewer.md"))
	assertFileExists(t, filepath.Join(repo, ".claude", "agents", "planner.md"))

	// Remove planner from manifest and re-apply.
	mustWrite(t, filepath.Join(repo, ".agentfiles"), "layout = \"claude\"\ninstructions = \"default\"\nagents = [\"reviewer\"]\n")

	m2, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load manifest 2: %v", err)
	}
	res2, err := apply.Apply(stores, "default", m2, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply 2: %v", err)
	}
	if res2.Removed != 1 {
		t.Fatalf("removed = %d, want 1", res2.Removed)
	}

	// reviewer still exists, planner pruned.
	assertFileExists(t, filepath.Join(repo, ".claude", "agents", "reviewer.md"))
	if _, err := os.Stat(filepath.Join(repo, ".claude", "agents", "planner.md")); !os.IsNotExist(err) {
		t.Fatal("planner should have been pruned")
	}
}

// TestIntegrationSkillGroups exercises the full lifecycle with grouped skills:
//
//  1. Create a store with grouped skills (tooling/browse, ayunis/backend) and a flat skill (search).
//  2. Create a bundle referencing "tooling/" (glob) + "search" (flat).
//  3. Apply to a repo. Verify leaf-name deployment and lock keys.
//  4. Modify a deployed skill.
//  5. Push. Verify changes land in the grouped store path.
//  6. Re-apply. Verify idempotent.
func TestIntegrationSkillGroups(t *testing.T) {
	tmp := t.TempDir()

	// ── 1. Create store with grouped + flat skills ─────────────────
	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Grouped skills.
	mustMkdir(t, filepath.Join(s.SkillsDir(), "tooling", "browse"))
	mustWrite(t, filepath.Join(s.SkillsDir(), "tooling", "browse", "SKILL.md"), "# Browse\nOriginal\n")
	mustMkdir(t, filepath.Join(s.SkillsDir(), "ayunis", "backend"))
	mustWrite(t, filepath.Join(s.SkillsDir(), "ayunis", "backend", "SKILL.md"), "# Backend\nOriginal\n")

	// Flat skill.
	mustMkdir(t, filepath.Join(s.SkillsDir(), "search"))
	mustWrite(t, filepath.Join(s.SkillsDir(), "search", "SKILL.md"), "# Search\nOriginal\n")

	// Agent.
	mustWrite(t, filepath.Join(s.InstructionsDir(), "core.md"), "# Core Agent\n")

	// ── 2. Bundle with glob + flat reference ───────────────────────
	bundleTOML := `[bundle]
name = "grouped-test"
instructions = "core"

[skills]
include = ["tooling/", "search"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "grouped-test.toml"), bundleTOML)

	// ── 3. Apply to repo ───────────────────────────────────────────
	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), "bundle = \"grouped-test\"\nlayout = \"pi\"\n")

	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	stores := map[string]*store.Store{"default": s}
	res, err := apply.Apply(stores, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// agent + browse + search = 3 (ayunis/backend NOT in the glob "tooling/")
	if res.Deployed != 3 {
		t.Fatalf("deployed = %d, want 3", res.Deployed)
	}

	// Verify leaf-name deployment paths.
	assertFileExists(t, filepath.Join(repo, ".pi", "skills", "browse", "SKILL.md"))
	assertFileExists(t, filepath.Join(repo, ".pi", "skills", "search", "SKILL.md"))
	assertFileContains(t, filepath.Join(repo, ".pi", "skills", "browse", "SKILL.md"), "# Browse")

	// Verify NO deployment at group path.
	if _, err := os.Stat(filepath.Join(repo, ".pi", "skills", "tooling")); err == nil {
		t.Error("should not deploy at group path")
	}

	// Verify lock keys are group-qualified.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}
	if _, ok := lf.Deployed.Skills["tooling/browse"]; !ok {
		t.Errorf("lock missing 'tooling/browse'; keys = %v", skillLockKeys(lf))
	}
	if _, ok := lf.Deployed.Skills["search"]; !ok {
		t.Errorf("lock missing 'search'; keys = %v", skillLockKeys(lf))
	}
	// Verify source paths.
	if sp := lf.Deployed.Skills["tooling/browse"].StorePath; sp != "skills/tooling/browse/" {
		t.Errorf("tooling/browse StorePath = %q, want skills/tooling/browse/", sp)
	}

	// ── 4. Modify deployed skill ───────────────────────────────────
	browseDeployed := filepath.Join(repo, ".pi", "skills", "browse", "SKILL.md")
	mustWrite(t, browseDeployed, "# Browse\nModified locally\n")

	// ── 5. Push → verify changes land in grouped store path ────────
	pushRes, err := push.Push(stores, "default", repo, push.Options{})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(pushRes.Changes) != 1 {
		t.Fatalf("push changes = %d, want 1", len(pushRes.Changes))
	}
	if pushRes.Changes[0].Name != "tooling/browse" {
		t.Errorf("push change name = %q, want tooling/browse", pushRes.Changes[0].Name)
	}

	// Verify store file updated at grouped path.
	storeFile := filepath.Join(s.SkillsDir(), "tooling", "browse", "SKILL.md")
	assertFileContains(t, storeFile, "Modified locally")

	// ── 6. Re-apply → verify idempotent ────────────────────────────
	res2, err := apply.Apply(stores, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	// All should be deployed (force=true), no removals.
	if res2.Removed != 0 {
		t.Errorf("re-apply removed = %d, want 0", res2.Removed)
	}

	// Push again — should be clean.
	pushRes2, err := push.Push(stores, "default", repo, push.Options{})
	if err != nil {
		t.Fatalf("push after re-apply: %v", err)
	}
	if len(pushRes2.Changes) != 0 {
		t.Errorf("expected 0 changes after re-apply+push, got %d", len(pushRes2.Changes))
	}
}

// TestIntegrationSkillGroupsCrossStore exercises cross-store grouped skills.
func TestIntegrationSkillGroupsCrossStore(t *testing.T) {
	tmp := t.TempDir()

	personal, err := store.Init(filepath.Join(tmp, "personal"))
	if err != nil {
		t.Fatalf("init personal: %v", err)
	}
	work, err := store.Init(filepath.Join(tmp, "work"))
	if err != nil {
		t.Fatalf("init work: %v", err)
	}

	// Personal flat skill.
	mustMkdir(t, filepath.Join(personal.SkillsDir(), "search"))
	mustWrite(t, filepath.Join(personal.SkillsDir(), "search", "SKILL.md"), "# Search\n")

	// Work grouped skills.
	mustMkdir(t, filepath.Join(work.SkillsDir(), "ayunis", "backend"))
	mustWrite(t, filepath.Join(work.SkillsDir(), "ayunis", "backend", "SKILL.md"), "# Backend\n")
	mustMkdir(t, filepath.Join(work.SkillsDir(), "ayunis", "frontend"))
	mustWrite(t, filepath.Join(work.SkillsDir(), "ayunis", "frontend", "SKILL.md"), "# Frontend\n")

	// Agent in personal store.
	mustWrite(t, filepath.Join(personal.InstructionsDir(), "core.md"), "# Core\n")

	// Bundle in personal store referencing cross-store glob.
	bundleTOML := `[bundle]
name = "cross-store"
instructions = "core"

[skills]
include = ["search"]
`
	mustWrite(t, filepath.Join(personal.BundlesDir(), "cross-store.toml"), bundleTOML)

	// Repo with skills_add for cross-store glob.
	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), "bundle = \"cross-store\"\nlayout = \"pi\"\nskills_add = [\"work:ayunis/\"]\n")

	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	allStores := map[string]*store.Store{
		"personal": personal,
		"work":     work,
	}
	res, err := apply.Apply(allStores, "personal", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// agent + search + backend + frontend = 4
	if res.Deployed != 4 {
		t.Fatalf("deployed = %d, want 4", res.Deployed)
	}

	// Verify deployment.
	assertFileExists(t, filepath.Join(repo, ".pi", "skills", "search", "SKILL.md"))
	assertFileExists(t, filepath.Join(repo, ".pi", "skills", "backend", "SKILL.md"))
	assertFileExists(t, filepath.Join(repo, ".pi", "skills", "frontend", "SKILL.md"))

	// Verify lock.
	lf, err := lock.Load(repo)
	if err != nil {
		t.Fatalf("load lock: %v", err)
	}
	if _, ok := lf.Deployed.Skills["search"]; !ok {
		t.Errorf("lock missing 'search'")
	}
	if _, ok := lf.Deployed.Skills["work:ayunis/backend"]; !ok {
		t.Errorf("lock missing 'work:ayunis/backend'; keys = %v", skillLockKeys(lf))
	}
	if _, ok := lf.Deployed.Skills["work:ayunis/frontend"]; !ok {
		t.Errorf("lock missing 'work:ayunis/frontend'; keys = %v", skillLockKeys(lf))
	}

	// Modify work skill, push, verify routing.
	mustWrite(t, filepath.Join(repo, ".pi", "skills", "backend", "SKILL.md"), "# Backend\nModified\n")
	pushRes, err := push.Push(allStores, "personal", repo, push.Options{})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(pushRes.Changes) != 1 {
		t.Fatalf("push changes = %d, want 1", len(pushRes.Changes))
	}
	// Change should land in work store's grouped path.
	assertFileContains(t, filepath.Join(work.SkillsDir(), "ayunis", "backend", "SKILL.md"), "Modified")
	// Personal store should be unaffected.
	if _, err := os.Stat(filepath.Join(personal.SkillsDir(), "ayunis")); err == nil {
		t.Error("personal store should not have ayunis/ directory")
	}
}

// ── Pi Extension Integration Tests ─────────────────────────────

func TestIntegrationPiExtensionRoundTrip(t *testing.T) {
	tmp := t.TempDir()

	// Init store and add a single .ts file extension.
	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	extSrc := filepath.Join(tmp, "no-model-flag.ts")
	mustWrite(t, extSrc, "// original extension\nexport default {}\n")
	name, _, err := s.AddPiExtension(extSrc, false)
	if err != nil {
		t.Fatalf("add pi-extension: %v", err)
	}
	if name != "no-model-flag" {
		t.Fatalf("name = %q, want no-model-flag", name)
	}

	// Create bundle with pi_extensions.
	bundleContent := `[bundle]
name = "test"

[pi_extensions]
include = ["no-model-flag"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "test.toml"), bundleContent)

	// Apply to repo1 with pi layout.
	repo1 := filepath.Join(tmp, "repo1")
	mustMkdir(t, repo1)
	mustWrite(t, filepath.Join(repo1, ".agentfiles"), `bundle = "test"
layout = "pi"
`)
	m, err := manifest.Load(repo1)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	stores := map[string]*store.Store{"default": s}
	res, err := apply.Apply(stores, "default", m, repo1, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res.Deployed)
	}

	// Verify deployed.
	extPath := filepath.Join(repo1, ".pi", "extensions", "no-model-flag.ts")
	assertFileExists(t, extPath)
	assertFileContains(t, extPath, "original extension")

	// Modify deployed and push.
	mustWrite(t, extPath, "// modified extension\nexport default {}\n")
	pushRes, err := push.Push(stores, "default", repo1, push.Options{})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(pushRes.Changes) != 1 {
		t.Fatalf("push changes = %d, want 1", len(pushRes.Changes))
	}

	// Verify store updated.
	assertFileContains(t, filepath.Join(s.PiExtensionsDir(), "no-model-flag.ts"), "modified extension")

	// Apply to repo2 — should pick up pushed change.
	repo2 := filepath.Join(tmp, "repo2")
	mustMkdir(t, repo2)
	mustWrite(t, filepath.Join(repo2, ".agentfiles"), `bundle = "test"
layout = "pi"
`)
	m2, _ := manifest.Load(repo2)
	_, err = apply.Apply(stores, "default", m2, repo2, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply repo2: %v", err)
	}
	assertFileContains(t, filepath.Join(repo2, ".pi", "extensions", "no-model-flag.ts"), "modified extension")
}

func TestIntegrationPiExtensionDirectory(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	// Create a directory extension source.
	extSrc := filepath.Join(tmp, "my-ext")
	mustMkdir(t, extSrc)
	mustWrite(t, filepath.Join(extSrc, "index.ts"), "// index\nexport default {}\n")
	mustWrite(t, filepath.Join(extSrc, "util.ts"), "export const x = 1\n")

	name, _, err := s.AddPiExtension(extSrc, false)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if name != "my-ext" {
		t.Fatalf("name = %q", name)
	}

	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), `pi_extensions = ["my-ext"]
layout = "pi"
`)
	m, _ := manifest.Load(repo)
	stores := map[string]*store.Store{"default": s}
	_, err = apply.Apply(stores, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	assertFileExists(t, filepath.Join(repo, ".pi", "extensions", "my-ext", "index.ts"))
	assertFileExists(t, filepath.Join(repo, ".pi", "extensions", "my-ext", "util.ts"))

	// Modify and push.
	mustWrite(t, filepath.Join(repo, ".pi", "extensions", "my-ext", "index.ts"), "// modified index\n")
	pushRes, err := push.Push(stores, "default", repo, push.Options{})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(pushRes.Changes) != 1 {
		t.Fatalf("push changes = %d, want 1", len(pushRes.Changes))
	}
	assertFileContains(t, filepath.Join(s.PiExtensionsDir(), "my-ext", "index.ts"), "modified index")
}

func TestIntegrationPiExtensionNonPiLayout(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	extSrc := filepath.Join(tmp, "ext.ts")
	mustWrite(t, extSrc, "export default {}")
	s.AddPiExtension(extSrc, false)

	bundleContent := `[bundle]
name = "test"

[pi_extensions]
include = ["ext"]
`
	mustWrite(t, filepath.Join(s.BundlesDir(), "test.toml"), bundleContent)

	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), `bundle = "test"
layout = "claude"
`)
	m, _ := manifest.Load(repo)
	stores := map[string]*store.Store{"default": s}
	res, err := apply.Apply(stores, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 0 {
		t.Errorf("deployed = %d, want 0 (claude layout ignores pi_extensions)", res.Deployed)
	}

	// No extension files should exist.
	if _, err := os.Stat(filepath.Join(repo, ".pi", "extensions")); err == nil {
		t.Error(".pi/extensions should not exist for claude layout")
	}

	// Lock should not have pi_extensions.
	lf, _ := lock.Load(repo)
	if len(lf.Deployed.PiExtensions) != 0 {
		t.Errorf("lock has %d pi_extensions, want 0", len(lf.Deployed.PiExtensions))
	}
}

func TestIntegrationPiExtensionAllLayout(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	extSrc := filepath.Join(tmp, "ext.ts")
	mustWrite(t, extSrc, "export default {}")
	s.AddPiExtension(extSrc, false)

	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), `pi_extensions = ["ext"]
layout = "all"
`)
	m, _ := manifest.Load(repo)
	stores := map[string]*store.Store{"default": s}
	res, err := apply.Apply(stores, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res.Deployed)
	}

	// Should exist at pi path.
	assertFileExists(t, filepath.Join(repo, ".pi", "extensions", "ext.ts"))

	// Should NOT exist at claude/cursor/codex paths.
	for _, dir := range []string{".claude/extensions", ".cursor/extensions", ".codex/extensions"} {
		if _, err := os.Stat(filepath.Join(repo, dir)); err == nil {
			t.Errorf("%s should not exist", dir)
		}
	}
}

func TestIntegrationPiExtensionPruning(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	for _, name := range []string{"alpha", "beta"} {
		src := filepath.Join(tmp, name+".ts")
		mustWrite(t, src, "// "+name)
		s.AddPiExtension(src, false)
	}

	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	stores := map[string]*store.Store{"default": s}

	// Deploy both.
	mustWrite(t, filepath.Join(repo, ".agentfiles"), `pi_extensions = ["alpha", "beta"]
layout = "pi"
`)
	m, _ := manifest.Load(repo)
	_, err = apply.Apply(stores, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	assertFileExists(t, filepath.Join(repo, ".pi", "extensions", "alpha.ts"))
	assertFileExists(t, filepath.Join(repo, ".pi", "extensions", "beta.ts"))

	// Remove beta from manifest and re-apply.
	mustWrite(t, filepath.Join(repo, ".agentfiles"), `pi_extensions = ["alpha"]
layout = "pi"
`)
	m2, _ := manifest.Load(repo)
	res, err := apply.Apply(stores, "default", m2, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if res.Removed != 1 {
		t.Errorf("removed = %d, want 1", res.Removed)
	}
	assertFileExists(t, filepath.Join(repo, ".pi", "extensions", "alpha.ts"))
	if _, err := os.Stat(filepath.Join(repo, ".pi", "extensions", "beta.ts")); err == nil {
		t.Error("beta.ts should have been pruned")
	}
}

func TestIntegrationPiExtensionUserLevel(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	extSrc := filepath.Join(tmp, "my-ext.ts")
	mustWrite(t, extSrc, "export default {}")
	s.AddPiExtension(extSrc, false)

	home := filepath.Join(tmp, "home")
	mustMkdir(t, home)
	lockPath := filepath.Join(tmp, "user.lock")

	m, err := manifest.FromUserConfig(manifest.UserFields{
		PiExtensions: []string{"my-ext"},
		Layout:       "pi",
	})
	if err != nil {
		t.Fatalf("FromUserConfig: %v", err)
	}

	stores := map[string]*store.Store{"default": s}
	userLay, _ := layout.GetUser("pi")
	_, err = apply.Apply(stores, "default", m, home, apply.Options{
		Force:        true,
		Layout:       userLay,
		LockFilePath: lockPath,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	// User-level pi extension lands at ~/.pi/agent/extensions/
	assertFileExists(t, filepath.Join(home, ".pi", "agent", "extensions", "my-ext.ts"))
}

func TestIntegrationPiExtensionCherryPick(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(filepath.Join(tmp, "store"))
	if err != nil {
		t.Fatalf("init-store: %v", err)
	}

	extSrc := filepath.Join(tmp, "my-ext.ts")
	mustWrite(t, extSrc, "// cherry-picked extension\n")
	s.AddPiExtension(extSrc, false)

	repo := filepath.Join(tmp, "repo")
	mustMkdir(t, repo)
	mustWrite(t, filepath.Join(repo, ".agentfiles"), `pi_extensions = ["my-ext"]
layout = "pi"
`)
	m, _ := manifest.Load(repo)
	stores := map[string]*store.Store{"default": s}
	res, err := apply.Apply(stores, "default", m, repo, apply.Options{Force: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Deployed != 1 {
		t.Errorf("deployed = %d, want 1", res.Deployed)
	}
	assertFileContains(t, filepath.Join(repo, ".pi", "extensions", "my-ext.ts"), "cherry-picked extension")
}

func skillLockKeys(lf *lock.LockFile) []string {
	keys := make([]string, 0, len(lf.Deployed.Skills))
	for k := range lf.Deployed.Skills {
		keys = append(keys, k)
	}
	return keys
}
