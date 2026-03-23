package manifest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/store"
)

func TestLoadBundleRef(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "ayunis-core"
layout = "pi"
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Bundle != "ayunis-core" {
		t.Errorf("Bundle = %q, want %q", m.Bundle, "ayunis-core")
	}
	if m.Layout != "pi" {
		t.Errorf("Layout = %q, want %q", m.Layout, "pi")
	}
}

func TestLoadCherryPick(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
instructions = "ayunis-core"
skills = ["browse", "git-workflow"]
resources = ["cursor-config"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Instructions != "ayunis-core" {
		t.Errorf("AgentsMd = %q, want %q", m.Instructions, "ayunis-core")
	}
	if len(m.Skills) != 2 {
		t.Errorf("Skills len = %d, want 2", len(m.Skills))
	}
	if m.Layout != "pi" {
		t.Errorf("Layout default = %q, want %q", m.Layout, "pi")
	}
}

func TestLoadEmptyManifestError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(``), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for empty manifest, got nil")
	}
}

func TestLoadBundleAndSkillsConflict(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "ayunis-core"
skills = ["browse"]
`), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for bundle+skills conflict, got nil")
	}
}

func setupStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"skills", "instructions", "resources", "bundles"} {
		os.MkdirAll(filepath.Join(dir, sub), 0o755)
	}
	exec.Command("git", "init", dir).Run()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return s
}

// createSkill creates a minimal skill directory with SKILL.md in the store.
func createSkill(t *testing.T, s *store.Store, name string) {
	t.Helper()
	dir := filepath.Join(s.SkillsDir(), filepath.FromSlash(name))
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+filepath.Base(name)), 0o644)
}

// singleStoreMap wraps a single store into the map format expected by Resolve.
func singleStoreMap(s *store.Store) (map[string]*store.Store, string) {
	return map[string]*store.Store{"default": s}, "default"
}

func TestResolveBundleWithOverrides(t *testing.T) {
	s := setupStore(t)

	// Create skills that the bundle references.
	createSkill(t, s, "nestjs-hexagonal-backend")
	createSkill(t, s, "git-workflow")
	createSkill(t, s, "typeorm-migrations")
	createSkill(t, s, "browse")

	bundleContent := `
[bundle]
name = "ayunis-core"
instructions = "ayunis-core"

[skills]
include = ["nestjs-hexagonal-backend", "git-workflow", "typeorm-migrations"]

[resources]
include = ["cursor-config"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "ayunis-core.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "ayunis-core"
skills_add = ["browse"]
skills_remove = ["typeorm-migrations"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if r.Instructions.Name != "ayunis-core" {
		t.Errorf("AgentsMd.Name = %q, want %q", r.Instructions.Name, "ayunis-core")
	}
	if r.Instructions.Store != "default" {
		t.Errorf("AgentsMd.Store = %q, want %q", r.Instructions.Store, "default")
	}
	if r.Layout != "pi" {
		t.Errorf("Layout = %q, want %q", r.Layout, "pi")
	}

	// Should have: nestjs-hexagonal-backend, git-workflow, browse
	// Should NOT have: typeorm-migrations
	expectedSkills := map[string]bool{
		"nestjs-hexagonal-backend": true,
		"git-workflow":             true,
		"browse":                   true,
	}
	if len(r.Skills) != len(expectedSkills) {
		t.Errorf("Skills len = %d, want %d; got %v", len(r.Skills), len(expectedSkills), assetNames(r.Skills))
	}
	for _, sk := range r.Skills {
		if !expectedSkills[sk.Name] {
			t.Errorf("unexpected skill %q", sk.Name)
		}
		if sk.Store != "default" {
			t.Errorf("skill %q store = %q, want %q", sk.Name, sk.Store, "default")
		}
	}

	if len(r.Resources) != 1 || r.Resources[0].Name != "cursor-config" {
		t.Errorf("Resources = %v, want [cursor-config]", assetNames(r.Resources))
	}
}

func TestResolveCherryPick(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "browse")
	createSkill(t, s, "git-workflow")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
instructions = "ayunis-core"
skills = ["browse", "git-workflow"]
resources = ["cursor-config"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if r.Instructions.Name != "ayunis-core" {
		t.Errorf("AgentsMd.Name = %q, want %q", r.Instructions.Name, "ayunis-core")
	}
	if r.Layout != "pi" {
		t.Errorf("Layout = %q, want %q", r.Layout, "pi")
	}

	expectedSkills := []string{"browse", "git-workflow"}
	if len(r.Skills) != len(expectedSkills) {
		t.Fatalf("Skills = %v, want %v", assetNames(r.Skills), expectedSkills)
	}
	for i, sk := range expectedSkills {
		if r.Skills[i].Name != sk {
			t.Errorf("Skills[%d].Name = %q, want %q", i, r.Skills[i].Name, sk)
		}
	}

	if len(r.Resources) != 1 || r.Resources[0].Name != "cursor-config" {
		t.Errorf("Resources = %v, want [cursor-config]", assetNames(r.Resources))
	}
}

func TestLoadBundleAndResourcesConflict(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "ayunis-core"
resources = ["x"]
`), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for bundle+resources conflict, got nil")
	}
}

func TestLoadSkillsAddWithoutBundle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
instructions = "core"
skills_add = ["browse"]
`), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for skills_add without bundle, got nil")
	}
}

func TestLoadSkillsRemoveWithoutBundle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
skills = ["browse"]
skills_remove = ["browse"]
`), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for skills_remove without bundle, got nil")
	}
}

func TestResolveMissingBundle(t *testing.T) {
	s := setupStore(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`bundle = "nonexistent"`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	_, err = manifest.Resolve(m, stores, defaultStore)
	if err == nil {
		t.Fatal("expected error for missing bundle, got nil")
	}
}

func TestResolveCrossStoreSkillsAdd(t *testing.T) {
	personal := setupStore(t)
	work := setupStore(t)

	createSkill(t, work, "golang")
	createSkill(t, personal, "my-skill")

	bundleContent := `
[bundle]
name = "backend"
instructions = "backend"

[skills]
include = ["golang"]
`
	os.WriteFile(filepath.Join(work.BundlesDir(), "backend.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "backend"
skills_add = ["personal:my-skill"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores := map[string]*store.Store{
		"work":     work,
		"personal": personal,
	}
	r, err := manifest.Resolve(m, stores, "work")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Should have golang from work and my-skill from personal
	if len(r.Skills) != 2 {
		t.Fatalf("Skills len = %d, want 2; got %v", len(r.Skills), assetNames(r.Skills))
	}

	// golang should be from work
	found := false
	for _, sk := range r.Skills {
		if sk.Name == "golang" && sk.Store == "work" {
			found = true
		}
	}
	if !found {
		t.Error("expected golang skill from work store")
	}

	// my-skill should be from personal
	found = false
	for _, sk := range r.Skills {
		if sk.Name == "my-skill" && sk.Store == "personal" {
			found = true
		}
	}
	if !found {
		t.Error("expected my-skill from personal store")
	}
}

func TestResolveCrossStoreSkillsRemove(t *testing.T) {
	personal := setupStore(t)
	work := setupStore(t)

	createSkill(t, work, "golang")
	createSkill(t, work, "my-skill")
	createSkill(t, personal, "my-skill")

	bundleContent := `
[bundle]
name = "backend"
instructions = "backend"

[skills]
include = ["golang", "my-skill"]
`
	os.WriteFile(filepath.Join(work.BundlesDir(), "backend.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "backend"
skills_add = ["personal:my-skill"]
skills_remove = ["personal:my-skill"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores := map[string]*store.Store{
		"work":     work,
		"personal": personal,
	}
	r, err := manifest.Resolve(m, stores, "work")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// skills_remove = ["personal:my-skill"] should only remove my-skill from
	// personal store, leaving the work store's my-skill (from the bundle) intact.
	// Expected: golang (work), my-skill (work)
	if len(r.Skills) != 2 {
		t.Fatalf("Skills len = %d, want 2; got %v", len(r.Skills), assetNames(r.Skills))
	}

	for _, sk := range r.Skills {
		if sk.Name == "my-skill" && sk.Store == "personal" {
			t.Error("personal:my-skill should have been removed by store-qualified skills_remove")
		}
	}

	found := false
	for _, sk := range r.Skills {
		if sk.Name == "my-skill" && sk.Store == "work" {
			found = true
		}
	}
	if !found {
		t.Error("expected my-skill from work store to survive store-qualified removal")
	}
}

func TestResolveCrossStoreCherryPick(t *testing.T) {
	personal := setupStore(t)
	work := setupStore(t)
	createSkill(t, work, "backend")
	createSkill(t, personal, "utils")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
instructions = "personal:my-agent"
skills = ["work:backend", "personal:utils"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores := map[string]*store.Store{
		"work":     work,
		"personal": personal,
	}
	r, err := manifest.Resolve(m, stores, "work")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if r.Instructions.Name != "my-agent" || r.Instructions.Store != "personal" {
		t.Errorf("AgentsMd = %+v, want {Name:my-agent Store:personal}", r.Instructions)
	}

	if len(r.Skills) != 2 {
		t.Fatalf("Skills len = %d, want 2", len(r.Skills))
	}
	if r.Skills[0].Name != "backend" || r.Skills[0].Store != "work" {
		t.Errorf("Skills[0] = %+v, want {Name:backend Store:work}", r.Skills[0])
	}
	if r.Skills[1].Name != "utils" || r.Skills[1].Store != "personal" {
		t.Errorf("Skills[1] = %+v, want {Name:utils Store:personal}", r.Skills[1])
	}
}

func TestResolveUnknownStoreError(t *testing.T) {
	s := setupStore(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
skills = ["unknown:some-skill"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	_, err = manifest.Resolve(m, stores, defaultStore)
	if err == nil {
		t.Fatal("expected error for unknown store reference, got nil")
	}
}

func TestFromUserConfigBundle(t *testing.T) {
	m, err := manifest.FromUserConfig(manifest.UserFields{
		Bundle:    "my-bundle",
		Layout:    "claude",
		SkillsAdd: []string{"extra"},
	})
	if err != nil {
		t.Fatalf("FromUserConfig: %v", err)
	}
	if m.Bundle != "my-bundle" {
		t.Errorf("Bundle = %q, want %q", m.Bundle, "my-bundle")
	}
	if m.Layout != "claude" {
		t.Errorf("Layout = %q, want %q", m.Layout, "claude")
	}
	if len(m.SkillsAdd) != 1 || m.SkillsAdd[0] != "extra" {
		t.Errorf("SkillsAdd = %v, want [extra]", m.SkillsAdd)
	}
}

func TestFromUserConfigCherryPick(t *testing.T) {
	m, err := manifest.FromUserConfig(manifest.UserFields{
		Instructions: "my-agent",
		Skills:   []string{"browse", "git"},
	})
	if err != nil {
		t.Fatalf("FromUserConfig: %v", err)
	}
	if m.Instructions != "my-agent" {
		t.Errorf("AgentsMd = %q, want %q", m.Instructions, "my-agent")
	}
	if len(m.Skills) != 2 {
		t.Errorf("Skills len = %d, want 2", len(m.Skills))
	}
	// Default layout for user is "all"
	if m.Layout != "all" {
		t.Errorf("Layout = %q, want %q", m.Layout, "all")
	}
}

func TestFromUserConfigDefaultLayout(t *testing.T) {
	m, err := manifest.FromUserConfig(manifest.UserFields{
		Instructions: "core",
	})
	if err != nil {
		t.Fatalf("FromUserConfig: %v", err)
	}
	if m.Layout != "all" {
		t.Errorf("Layout = %q, want %q (default for user)", m.Layout, "all")
	}
}

func TestFromUserConfigMixedError(t *testing.T) {
	_, err := manifest.FromUserConfig(manifest.UserFields{
		Bundle: "my-bundle",
		Skills: []string{"browse"},
	})
	if err == nil {
		t.Fatal("expected error for bundle+cherry-pick, got nil")
	}
}

func TestFromUserConfigEmptyError(t *testing.T) {
	_, err := manifest.FromUserConfig(manifest.UserFields{})
	if err == nil {
		t.Fatal("expected error for empty fields, got nil")
	}
}

func TestFromUserConfigSkillsAddWithoutBundleError(t *testing.T) {
	_, err := manifest.FromUserConfig(manifest.UserFields{
		Instructions:  "core",
		SkillsAdd: []string{"extra"},
	})
	if err == nil {
		t.Fatal("expected error for skills_add without bundle, got nil")
	}
}

// createAgent creates a minimal agent .md file in the store.
func createAgent(t *testing.T, s *store.Store, name string) {
	t.Helper()
	dir := filepath.Join(s.Root, "agents")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, name+".md"), []byte("---\nname: "+name+"\n---\nHello"), 0o644)
}

// --- Agent manifest tests ---

func TestLoadCherryPickWithAgents(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
agents = ["code-reviewer", "debugger"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.Agents) != 2 {
		t.Errorf("Agents len = %d, want 2", len(m.Agents))
	}
}

func TestLoadAgentsAddWithoutBundleError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
agents = ["code-reviewer"]
agents_add = ["debugger"]
`), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for agents_add without bundle, got nil")
	}
}

func TestLoadAgentsRemoveWithoutBundleError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
agents = ["code-reviewer"]
agents_remove = ["code-reviewer"]
`), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for agents_remove without bundle, got nil")
	}
}

func TestLoadBundleAndAgentsConflict(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "core"
agents = ["code-reviewer"]
`), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for bundle+agents conflict, got nil")
	}
}

func TestResolveCherryPickAgents(t *testing.T) {
	s := setupStore(t)
	createAgent(t, s, "code-reviewer")
	createAgent(t, s, "debugger")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
agents = ["code-reviewer", "debugger"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Agents) != 2 {
		t.Fatalf("Agents len = %d, want 2; got %v", len(r.Agents), assetNames(r.Agents))
	}
	if r.Agents[0].Name != "code-reviewer" || r.Agents[1].Name != "debugger" {
		t.Errorf("Agents = %v, want [code-reviewer, debugger]", assetNames(r.Agents))
	}
}

func TestResolveBundleWithAgents(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "browse")

	bundleContent := `
[bundle]
name = "test"
instructions = "core"

[skills]
include = ["browse"]

[agents]
include = ["code-reviewer", "debugger"]
exclude = ["debugger"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "test.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`bundle = "test"`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Agents) != 1 {
		t.Fatalf("Agents len = %d, want 1; got %v", len(r.Agents), assetNames(r.Agents))
	}
	if r.Agents[0].Name != "code-reviewer" {
		t.Errorf("Agents[0] = %q, want code-reviewer", r.Agents[0].Name)
	}
}

func TestResolveBundleAgentsAddRemove(t *testing.T) {
	s := setupStore(t)

	bundleContent := `
[bundle]
name = "test"
instructions = "core"

[agents]
include = ["code-reviewer", "debugger"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "test.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "test"
agents_add = ["security-auditor"]
agents_remove = ["debugger"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Should have: code-reviewer, security-auditor
	// Should NOT have: debugger
	if len(r.Agents) != 2 {
		t.Fatalf("Agents len = %d, want 2; got %v", len(r.Agents), assetNames(r.Agents))
	}
	names := map[string]bool{}
	for _, a := range r.Agents {
		names[a.Name] = true
	}
	if !names["code-reviewer"] {
		t.Error("expected code-reviewer")
	}
	if !names["security-auditor"] {
		t.Error("expected security-auditor")
	}
	if names["debugger"] {
		t.Error("debugger should have been removed")
	}
}

func TestResolveAgentNameCollisionCrossStore(t *testing.T) {
	personal := setupStore(t)
	work := setupStore(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
agents = ["code-reviewer", "work:code-reviewer"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores := map[string]*store.Store{
		"personal": personal,
		"work":     work,
	}
	_, err = manifest.Resolve(m, stores, "personal")
	if err == nil {
		t.Fatal("expected agent name collision error")
	}
	if !strings.Contains(err.Error(), "agent") {
		t.Errorf("error = %q, should mention agent", err)
	}
}

func TestResolveCrossStoreAgents(t *testing.T) {
	personal := setupStore(t)
	work := setupStore(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
agents = ["code-reviewer", "work:debugger"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores := map[string]*store.Store{
		"personal": personal,
		"work":     work,
	}
	r, err := manifest.Resolve(m, stores, "personal")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Agents) != 2 {
		t.Fatalf("Agents len = %d, want 2", len(r.Agents))
	}
	if r.Agents[0].Name != "code-reviewer" || r.Agents[0].Store != "personal" {
		t.Errorf("Agents[0] = %+v, want {Name:code-reviewer Store:personal}", r.Agents[0])
	}
	if r.Agents[1].Name != "debugger" || r.Agents[1].Store != "work" {
		t.Errorf("Agents[1] = %+v, want {Name:debugger Store:work}", r.Agents[1])
	}
}

// --- PiExtensions manifest tests ---

func TestLoadCherryPickWithPiExtensions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
pi_extensions = ["no-model-flag", "custom-tool"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.PiExtensions) != 2 {
		t.Errorf("PiExtensions len = %d, want 2", len(m.PiExtensions))
	}
}

func TestLoadBundleAndPiExtensionsConflict(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "core"
pi_extensions = ["no-model-flag"]
`), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for bundle+pi_extensions conflict, got nil")
	}
}

func TestLoadPiExtensionsAddWithoutBundleError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
pi_extensions = ["no-model-flag"]
pi_extensions_add = ["extra"]
`), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for pi_extensions_add without bundle, got nil")
	}
}

func TestLoadPiExtensionsRemoveWithoutBundleError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
pi_extensions = ["no-model-flag"]
pi_extensions_remove = ["no-model-flag"]
`), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for pi_extensions_remove without bundle, got nil")
	}
}

func TestResolveCherryPickPiExtensions(t *testing.T) {
	s := setupStore(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
pi_extensions = ["no-model-flag", "custom-tool"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.PiExtensions) != 2 {
		t.Fatalf("PiExtensions len = %d, want 2; got %v", len(r.PiExtensions), assetNames(r.PiExtensions))
	}
	if r.PiExtensions[0].Name != "no-model-flag" || r.PiExtensions[1].Name != "custom-tool" {
		t.Errorf("PiExtensions = %v", assetNames(r.PiExtensions))
	}
}

func TestResolveBundleWithPiExtensions(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "browse")

	bundleContent := `
[bundle]
name = "test"
instructions = "core"

[skills]
include = ["browse"]

[pi_extensions]
include = ["no-model-flag", "custom-tool"]
exclude = ["custom-tool"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "test.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`bundle = "test"`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.PiExtensions) != 1 {
		t.Fatalf("PiExtensions len = %d, want 1; got %v", len(r.PiExtensions), assetNames(r.PiExtensions))
	}
	if r.PiExtensions[0].Name != "no-model-flag" {
		t.Errorf("PiExtensions[0] = %q, want no-model-flag", r.PiExtensions[0].Name)
	}
}

func TestResolveBundlePiExtensionsAddRemove(t *testing.T) {
	s := setupStore(t)

	bundleContent := `
[bundle]
name = "test"
instructions = "core"

[pi_extensions]
include = ["no-model-flag", "old-ext"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "test.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "test"
pi_extensions_add = ["new-ext"]
pi_extensions_remove = ["old-ext"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.PiExtensions) != 2 {
		t.Fatalf("PiExtensions len = %d, want 2; got %v", len(r.PiExtensions), assetNames(r.PiExtensions))
	}
	names := map[string]bool{}
	for _, e := range r.PiExtensions {
		names[e.Name] = true
	}
	if !names["no-model-flag"] {
		t.Error("expected no-model-flag")
	}
	if !names["new-ext"] {
		t.Error("expected new-ext")
	}
	if names["old-ext"] {
		t.Error("old-ext should have been removed")
	}
}

func TestResolveCrossStorePiExtensions(t *testing.T) {
	personal := setupStore(t)
	work := setupStore(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
pi_extensions = ["no-model-flag", "work:custom-tool"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores := map[string]*store.Store{
		"personal": personal,
		"work":     work,
	}
	r, err := manifest.Resolve(m, stores, "personal")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.PiExtensions) != 2 {
		t.Fatalf("PiExtensions len = %d, want 2", len(r.PiExtensions))
	}
	if r.PiExtensions[0].Name != "no-model-flag" || r.PiExtensions[0].Store != "personal" {
		t.Errorf("PiExtensions[0] = %+v", r.PiExtensions[0])
	}
	if r.PiExtensions[1].Name != "custom-tool" || r.PiExtensions[1].Store != "work" {
		t.Errorf("PiExtensions[1] = %+v", r.PiExtensions[1])
	}
}

func TestResolvePiExtensionNameCollisionCrossStore(t *testing.T) {
	personal := setupStore(t)
	work := setupStore(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
pi_extensions = ["no-model-flag", "work:no-model-flag"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores := map[string]*store.Store{
		"personal": personal,
		"work":     work,
	}
	_, err = manifest.Resolve(m, stores, "personal")
	if err == nil {
		t.Fatal("expected pi_extension name collision error")
	}
	if !strings.Contains(err.Error(), "pi_extension") {
		t.Errorf("error = %q, should mention pi_extension", err)
	}
}

func TestFromUserConfigWithPiExtensions(t *testing.T) {
	m, err := manifest.FromUserConfig(manifest.UserFields{
		PiExtensions: []string{"no-model-flag"},
	})
	if err != nil {
		t.Fatalf("FromUserConfig: %v", err)
	}
	if len(m.PiExtensions) != 1 || m.PiExtensions[0] != "no-model-flag" {
		t.Errorf("PiExtensions = %v, want [no-model-flag]", m.PiExtensions)
	}
	if m.Layout != "all" {
		t.Errorf("Layout = %q, want all", m.Layout)
	}
}

func TestFromUserConfigPiExtensionsAddWithoutBundleError(t *testing.T) {
	_, err := manifest.FromUserConfig(manifest.UserFields{
		Instructions:    "core",
		PiExtensionsAdd: []string{"extra"},
	})
	if err == nil {
		t.Fatal("expected error for pi_extensions_add without bundle, got nil")
	}
}

// assetNames extracts names from a slice of ResolvedAssets for test output.
func assetNames(assets []manifest.ResolvedAsset) []string {
	names := make([]string, len(assets))
	for i, a := range assets {
		names[i] = a.Store + ":" + a.Name
	}
	return names
}

// --- Skill group tests ---

func TestResolveBundleWithGlob(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "ayunis/backend")
	createSkill(t, s, "ayunis/frontend")
	createSkill(t, s, "browse")

	bundleContent := `
[bundle]
name = "test"
instructions = "core"

[skills]
include = ["ayunis/", "browse"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "test.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`bundle = "test"`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Skills) != 3 {
		t.Fatalf("Skills len = %d, want 3; got %v", len(r.Skills), assetNames(r.Skills))
	}

	// Check StorePath is set correctly.
	byName := map[string]manifest.ResolvedAsset{}
	for _, sk := range r.Skills {
		byName[sk.Name] = sk
	}
	if sk, ok := byName["backend"]; !ok || sk.StorePath != "ayunis/backend" {
		t.Errorf("expected backend with StorePath=ayunis/backend, got %+v", byName["backend"])
	}
	if sk, ok := byName["frontend"]; !ok || sk.StorePath != "ayunis/frontend" {
		t.Errorf("expected frontend with StorePath=ayunis/frontend, got %+v", byName["frontend"])
	}
	if sk, ok := byName["browse"]; !ok || sk.StorePath != "browse" {
		t.Errorf("expected browse with StorePath=browse, got %+v", byName["browse"])
	}
}

func TestResolveCherryPickQualifiedName(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "tooling/browse")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
skills = ["tooling/browse"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Skills) != 1 {
		t.Fatalf("Skills len = %d, want 1", len(r.Skills))
	}
	if r.Skills[0].Name != "browse" {
		t.Errorf("Name = %q, want browse", r.Skills[0].Name)
	}
	if r.Skills[0].StorePath != "tooling/browse" {
		t.Errorf("StorePath = %q, want tooling/browse", r.Skills[0].StorePath)
	}
}

func TestResolveLeafNameCollisionError(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "tooling/browse")
	createSkill(t, s, "legacy/browse")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
skills = ["tooling/browse", "legacy/browse"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	_, err = manifest.Resolve(m, stores, defaultStore)
	if err == nil {
		t.Fatal("expected leaf name collision error")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("error = %q, want 'collision'", err)
	}
}

func TestResolveLeafNameCollisionCrossStore(t *testing.T) {
	personal := setupStore(t)
	work := setupStore(t)
	createSkill(t, personal, "browse")
	createSkill(t, work, "tooling/browse")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
skills = ["browse", "work:tooling/browse"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores := map[string]*store.Store{
		"personal": personal,
		"work":     work,
	}
	_, err = manifest.Resolve(m, stores, "personal")
	if err == nil {
		t.Fatal("expected leaf name collision error across stores")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("error = %q, want 'collision'", err)
	}
}

func TestResolveCrossStoreGlob(t *testing.T) {
	personal := setupStore(t)
	work := setupStore(t)
	createSkill(t, work, "ayunis/backend")
	createSkill(t, work, "ayunis/frontend")
	createSkill(t, personal, "browse")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
skills = ["browse", "work:ayunis/"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores := map[string]*store.Store{
		"personal": personal,
		"work":     work,
	}
	r, err := manifest.Resolve(m, stores, "personal")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Skills) != 3 {
		t.Fatalf("Skills len = %d, want 3; got %v", len(r.Skills), assetNames(r.Skills))
	}
}

func TestResolveSkillsAddWithGlob(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "base")
	createSkill(t, s, "ayunis/backend")
	createSkill(t, s, "ayunis/frontend")

	bundleContent := `
[bundle]
name = "test"
instructions = "core"

[skills]
include = ["base"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "test.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "test"
skills_add = ["ayunis/"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Skills) != 3 {
		t.Fatalf("Skills len = %d, want 3; got %v", len(r.Skills), assetNames(r.Skills))
	}
}

func TestResolveSkillsRemoveSpecific(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "ayunis/backend")
	createSkill(t, s, "ayunis/frontend")
	createSkill(t, s, "browse")

	bundleContent := `
[bundle]
name = "test"
instructions = "core"

[skills]
include = ["ayunis/", "browse"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "test.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "test"
skills_remove = ["ayunis/backend"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Skills) != 2 {
		t.Fatalf("Skills len = %d, want 2; got %v", len(r.Skills), assetNames(r.Skills))
	}
	for _, sk := range r.Skills {
		if sk.Name == "backend" {
			t.Error("backend should have been removed")
		}
	}
}

func TestResolveSkillsRemoveGlob(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "ayunis/backend")
	createSkill(t, s, "ayunis/frontend")
	createSkill(t, s, "browse")

	bundleContent := `
[bundle]
name = "test"
instructions = "core"

[skills]
include = ["ayunis/", "browse"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "test.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "test"
skills_remove = ["ayunis/"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Skills) != 1 {
		t.Fatalf("Skills len = %d, want 1; got %v", len(r.Skills), assetNames(r.Skills))
	}
	if r.Skills[0].Name != "browse" {
		t.Errorf("expected browse, got %q", r.Skills[0].Name)
	}
}

func TestResolveBundleExcludeGlob(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "ayunis/backend")
	createSkill(t, s, "ayunis/frontend")
	createSkill(t, s, "browse")

	bundleContent := `
[bundle]
name = "test"
instructions = "core"

[skills]
include = ["ayunis/", "browse"]
exclude = ["ayunis/"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "test.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`bundle = "test"`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Skills) != 1 {
		t.Fatalf("Skills len = %d, want 1; got %v", len(r.Skills), assetNames(r.Skills))
	}
	if r.Skills[0].Name != "browse" {
		t.Errorf("expected browse, got %q", r.Skills[0].Name)
	}
}

func TestResolveMixedFlatAndGrouped(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "browse")
	createSkill(t, s, "tooling/web-search")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
skills = ["browse", "tooling/web-search"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Skills) != 2 {
		t.Fatalf("Skills len = %d, want 2", len(r.Skills))
	}

	byName := map[string]manifest.ResolvedAsset{}
	for _, sk := range r.Skills {
		byName[sk.Name] = sk
	}
	if sk := byName["browse"]; sk.StorePath != "browse" {
		t.Errorf("browse StorePath = %q, want browse", sk.StorePath)
	}
	if sk := byName["web-search"]; sk.StorePath != "tooling/web-search" {
		t.Errorf("web-search StorePath = %q, want tooling/web-search", sk.StorePath)
	}
}

func TestResolveSkillsAddCrossStoreGlob(t *testing.T) {
	personal := setupStore(t)
	work := setupStore(t)
	createSkill(t, personal, "browse")
	createSkill(t, work, "ayunis/backend")
	createSkill(t, work, "ayunis/frontend")

	bundleContent := `
[bundle]
name = "test"
instructions = "core"

[skills]
include = ["browse"]
`
	os.WriteFile(filepath.Join(personal.BundlesDir(), "test.toml"), []byte(bundleContent), 0o644)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "test"
skills_add = ["work:ayunis/"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores := map[string]*store.Store{
		"personal": personal,
		"work":     work,
	}
	r, err := manifest.Resolve(m, stores, "personal")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.Skills) != 3 {
		t.Fatalf("Skills len = %d, want 3; got %v", len(r.Skills), assetNames(r.Skills))
	}

	// Verify work skills have correct store
	for _, sk := range r.Skills {
		if sk.Name == "backend" || sk.Name == "frontend" {
			if sk.Store != "work" {
				t.Errorf("skill %q store = %q, want work", sk.Name, sk.Store)
			}
		}
	}
}

func TestResolveStorePath(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "tooling/browse")
	createSkill(t, s, "search")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
skills = ["tooling/browse", "search"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	stores, defaultStore := singleStoreMap(s)
	r, err := manifest.Resolve(m, stores, defaultStore)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	for _, sk := range r.Skills {
		switch sk.Name {
		case "browse":
			if sk.StorePath != "tooling/browse" {
				t.Errorf("browse StorePath = %q, want tooling/browse", sk.StorePath)
			}
		case "search":
			if sk.StorePath != "search" {
				t.Errorf("search StorePath = %q, want search", sk.StorePath)
			}
		}
	}
}
