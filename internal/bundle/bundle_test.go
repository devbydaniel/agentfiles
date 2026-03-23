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

func TestLoadBundle(t *testing.T) {
	s := setupStore(t)

	content := `
[bundle]
name = "ayunis-core"
instructions = "ayunis-core"

[skills]
include = ["nestjs-hexagonal-backend", "git-workflow"]
exclude = ["git-workflow"]

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
	if b.Instructions() != "ayunis-core" {
		t.Errorf("AgentsMd = %q, want %q", b.Instructions(), "ayunis-core")
	}
	if len(b.Skills.Include) != 2 {
		t.Errorf("Skills.Include len = %d, want 2", len(b.Skills.Include))
	}
	if len(b.Skills.Exclude) != 1 {
		t.Errorf("Skills.Exclude len = %d, want 1", len(b.Skills.Exclude))
	}
	if len(b.Resources.Include) != 2 {
		t.Errorf("Resources.Include len = %d, want 2", len(b.Resources.Include))
	}
}

func TestLoadBundleWithAgents(t *testing.T) {
	s := setupStore(t)

	content := `
[bundle]
name = "with-agents"
instructions = "core"

[skills]
include = ["browse"]

[agents]
include = ["code-reviewer", "debugger"]
exclude = ["debugger"]
`
	os.WriteFile(filepath.Join(s.BundlesDir(), "with-agents.toml"), []byte(content), 0o644)

	b, err := bundle.Load(s, "with-agents")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(b.Agents.Include) != 2 {
		t.Errorf("Agents.Include len = %d, want 2", len(b.Agents.Include))
	}
	if len(b.Agents.Exclude) != 1 {
		t.Errorf("Agents.Exclude len = %d, want 1", len(b.Agents.Exclude))
	}
	if b.Agents.Include[0] != "code-reviewer" {
		t.Errorf("Agents.Include[0] = %q, want code-reviewer", b.Agents.Include[0])
	}
}

func TestLoadBundleMissing(t *testing.T) {
	s := setupStore(t)

	_, err := bundle.Load(s, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing bundle, got nil")
	}
}
