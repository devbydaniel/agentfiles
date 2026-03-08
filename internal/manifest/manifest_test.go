package manifest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/danielbenner/agentfiles/internal/manifest"
	"github.com/danielbenner/agentfiles/internal/store"
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

	r, err := manifest.Resolve(m, s)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if r.AgentsMd != "ayunis-core" {
		t.Errorf("AgentsMd = %q, want %q", r.AgentsMd, "ayunis-core")
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
		t.Errorf("Skills len = %d, want %d; got %v", len(r.Skills), len(expectedSkills), r.Skills)
	}
	for _, sk := range r.Skills {
		if !expectedSkills[sk] {
			t.Errorf("unexpected skill %q", sk)
		}
	}

	if len(r.Plugins) != 1 || r.Plugins[0] != "my-plugin" {
		t.Errorf("Plugins = %v, want [my-plugin]", r.Plugins)
	}
	if len(r.Resources) != 1 || r.Resources[0] != "cursor-config" {
		t.Errorf("Resources = %v, want [cursor-config]", r.Resources)
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

	r, err := manifest.Resolve(m, s)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if r.AgentsMd != "ayunis-core" {
		t.Errorf("AgentsMd = %q, want %q", r.AgentsMd, "ayunis-core")
	}
	if r.Layout != "pi" {
		t.Errorf("Layout = %q, want %q", r.Layout, "pi")
	}

	expectedSkills := []string{"browse", "git-workflow"}
	if len(r.Skills) != len(expectedSkills) {
		t.Fatalf("Skills = %v, want %v", r.Skills, expectedSkills)
	}
	for i, sk := range expectedSkills {
		if r.Skills[i] != sk {
			t.Errorf("Skills[%d] = %q, want %q", i, r.Skills[i], sk)
		}
	}

	if len(r.Plugins) != 1 || r.Plugins[0] != "my-plugin" {
		t.Errorf("Plugins = %v, want [my-plugin]", r.Plugins)
	}
	if len(r.Resources) != 1 || r.Resources[0] != "cursor-config" {
		t.Errorf("Resources = %v, want [cursor-config]", r.Resources)
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

	_, err = manifest.Resolve(m, s)
	if err == nil {
		t.Fatal("expected error for missing bundle, got nil")
	}
}
