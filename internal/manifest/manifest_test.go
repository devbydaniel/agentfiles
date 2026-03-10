package manifest_test

import (
	"os"
	"os/exec"
	"path/filepath"
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
agents_md = "ayunis-core"
skills = ["browse", "git-workflow"]
resources = ["cursor-config"]
`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.AgentsMd != "ayunis-core" {
		t.Errorf("AgentsMd = %q, want %q", m.AgentsMd, "ayunis-core")
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
	for _, sub := range []string{"skills", "agents", "plugins", "resources", "bundles"} {
		os.MkdirAll(filepath.Join(dir, sub), 0o755)
	}
	exec.Command("git", "init", dir).Run()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return s
}

// singleStoreMap wraps a single store into the map format expected by Resolve.
func singleStoreMap(s *store.Store) (map[string]*store.Store, string) {
	return map[string]*store.Store{"default": s}, "default"
}

func TestResolveBundleWithOverrides(t *testing.T) {
	s := setupStore(t)

	bundleContent := `
[bundle]
name = "ayunis-core"
agents_md = "ayunis-core"

[skills]
include = ["nestjs-hexagonal-backend", "git-workflow", "typeorm-migrations"]

[plugins]
include = ["my-plugin"]

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

	if r.AgentsMd.Name != "ayunis-core" {
		t.Errorf("AgentsMd.Name = %q, want %q", r.AgentsMd.Name, "ayunis-core")
	}
	if r.AgentsMd.Store != "default" {
		t.Errorf("AgentsMd.Store = %q, want %q", r.AgentsMd.Store, "default")
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

	if len(r.Plugins) != 1 || r.Plugins[0].Name != "my-plugin" {
		t.Errorf("Plugins = %v, want [my-plugin]", assetNames(r.Plugins))
	}
	if len(r.Resources) != 1 || r.Resources[0].Name != "cursor-config" {
		t.Errorf("Resources = %v, want [cursor-config]", assetNames(r.Resources))
	}
}

func TestResolveCherryPick(t *testing.T) {
	s := setupStore(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
agents_md = "ayunis-core"
skills = ["browse", "git-workflow"]
plugins = ["my-plugin"]
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

	if r.AgentsMd.Name != "ayunis-core" {
		t.Errorf("AgentsMd.Name = %q, want %q", r.AgentsMd.Name, "ayunis-core")
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

	if len(r.Plugins) != 1 || r.Plugins[0].Name != "my-plugin" {
		t.Errorf("Plugins = %v, want [my-plugin]", assetNames(r.Plugins))
	}
	if len(r.Resources) != 1 || r.Resources[0].Name != "cursor-config" {
		t.Errorf("Resources = %v, want [cursor-config]", assetNames(r.Resources))
	}
}

func TestLoadPluginsOnlyCherryPick(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`plugins = ["x"]`), 0o644)

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.Plugins) != 1 || m.Plugins[0] != "x" {
		t.Errorf("Plugins = %v, want [x]", m.Plugins)
	}
}

func TestLoadBundleAndPluginsConflict(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
bundle = "ayunis-core"
plugins = ["x"]
`), 0o644)

	_, err := manifest.Load(dir)
	if err == nil {
		t.Fatal("expected error for bundle+plugins conflict, got nil")
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
agents_md = "core"
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

	bundleContent := `
[bundle]
name = "backend"
agents_md = "backend"

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

	bundleContent := `
[bundle]
name = "backend"
agents_md = "backend"

[skills]
include = ["golang", "my-skill"]
`
	os.WriteFile(filepath.Join(work.BundlesDir(), "backend.toml"), []byte(bundleContent), 0o644)

	// Also add my-skill to personal store so both stores have it
	os.MkdirAll(filepath.Join(personal.SkillsDir(), "my-skill"), 0o755)

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

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte(`
agents_md = "personal:my-agent"
skills = ["work:backend", "personal:utils"]
plugins = ["work:formatter"]
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

	if r.AgentsMd.Name != "my-agent" || r.AgentsMd.Store != "personal" {
		t.Errorf("AgentsMd = %+v, want {Name:my-agent Store:personal}", r.AgentsMd)
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

	if len(r.Plugins) != 1 || r.Plugins[0].Name != "formatter" || r.Plugins[0].Store != "work" {
		t.Errorf("Plugins = %v, want [{Name:formatter Store:work}]", r.Plugins)
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
		AgentsMd: "my-agent",
		Skills:   []string{"browse", "git"},
		Plugins:  []string{"fmt"},
	})
	if err != nil {
		t.Fatalf("FromUserConfig: %v", err)
	}
	if m.AgentsMd != "my-agent" {
		t.Errorf("AgentsMd = %q, want %q", m.AgentsMd, "my-agent")
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
		AgentsMd: "core",
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
		AgentsMd:  "core",
		SkillsAdd: []string{"extra"},
	})
	if err == nil {
		t.Fatal("expected error for skills_add without bundle, got nil")
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
