package registry

import (
	"testing"

	"github.com/devbydaniel/agentfiles/internal/config"
)

// --- LoadFromConfig tests ---

func TestLoadFromConfig(t *testing.T) {
	cfg := &config.Config{
		DefaultStore: "work",
		Stores:       map[string]string{"work": "/tmp/work"},
		Repos: []config.Repo{
			{
				Name:   "api",
				Path:   "/dev/api",
				Store:  "work",
				Bundle: "backend",
				Layout: "pi",
			},
			{
				Name:   "web",
				Path:   "/dev/web",
				Store:  "work",
				Bundle: "frontend",
				Layout: "claude",
			},
		},
	}

	reg, err := LoadFromConfig(cfg)
	if err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}
	if len(reg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(reg.Repos))
	}
	if reg.Repos[0].Store != "work" {
		t.Errorf("repo 0 store = %q, want work", reg.Repos[0].Store)
	}
	if reg.Repos[0].Bundle != "backend" {
		t.Errorf("repo 0 bundle = %q, want backend", reg.Repos[0].Bundle)
	}
	if reg.Repos[1].Layout != "claude" {
		t.Errorf("repo 1 layout = %q, want claude", reg.Repos[1].Layout)
	}
}

func TestLoadFromConfigValidation(t *testing.T) {
	// Missing bundle should error
	cfg := &config.Config{
		Repos: []config.Repo{
			{
				Name: "api",
				Path: "/dev/api",
			},
		},
	}
	_, err := LoadFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for missing bundle")
	}
}

func TestLoadFromConfigDuplicatePath(t *testing.T) {
	cfg := &config.Config{
		Repos: []config.Repo{
			{Path: "/dev/api", Bundle: "a"},
			{Path: "/dev/api", Bundle: "b"},
		},
	}
	_, err := LoadFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate path")
	}
}

func TestLoadFromConfigEmpty(t *testing.T) {
	cfg := &config.Config{}
	reg, err := LoadFromConfig(cfg)
	if err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}
	if len(reg.Repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(reg.Repos))
	}
}

func TestLoadFromConfigSkillsPreserved(t *testing.T) {
	cfg := &config.Config{
		Repos: []config.Repo{
			{
				Path:         "/dev/api",
				Bundle:       "backend",
				Layout:       "pi",
				SkillsAdd:    []string{"extra", "personal:golang"},
				SkillsRemove: []string{"unwanted"},
			},
		},
	}
	reg, err := LoadFromConfig(cfg)
	if err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}
	repo := reg.Repos[0]
	if len(repo.SkillsAdd) != 2 || repo.SkillsAdd[1] != "personal:golang" {
		t.Errorf("skills_add = %v", repo.SkillsAdd)
	}
	if len(repo.SkillsRemove) != 1 || repo.SkillsRemove[0] != "unwanted" {
		t.Errorf("skills_remove = %v", repo.SkillsRemove)
	}
}
