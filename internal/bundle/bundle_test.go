package bundle_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/devbydaniel/agentfiles/internal/bundle"
	"github.com/devbydaniel/agentfiles/internal/store"
)

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

func TestLoadBundle(t *testing.T) {
	s := setupStore(t)

	content := `
[bundle]
name = "ayunis-core"
agents_md = "ayunis-core"

[skills]
include = ["nestjs-hexagonal-backend", "git-workflow"]
exclude = ["git-workflow"]

[plugins]
include = ["my-plugin"]

[resources]
include = ["cursor-config", "shared-scripts"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "ayunis-core.toml"), []byte(content), 0o644)

	b, err := bundle.Load(s, "ayunis-core")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if b.Name() != "ayunis-core" {
		t.Errorf("Name = %q, want %q", b.Name(), "ayunis-core")
	}
	if b.AgentsMd() != "ayunis-core" {
		t.Errorf("AgentsMd = %q, want %q", b.AgentsMd(), "ayunis-core")
	}
	if len(b.Skills.Include) != 2 {
		t.Errorf("Skills.Include len = %d, want 2", len(b.Skills.Include))
	}
	if len(b.Skills.Exclude) != 1 {
		t.Errorf("Skills.Exclude len = %d, want 1", len(b.Skills.Exclude))
	}
	if len(b.Plugins.Include) != 1 {
		t.Errorf("Plugins.Include len = %d, want 1", len(b.Plugins.Include))
	}
	if len(b.Resources.Include) != 2 {
		t.Errorf("Resources.Include len = %d, want 2", len(b.Resources.Include))
	}
}

func TestLoadBundleMissing(t *testing.T) {
	s := setupStore(t)

	_, err := bundle.Load(s, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing bundle, got nil")
	}
}
