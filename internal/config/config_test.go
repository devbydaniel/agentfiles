package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithStores(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	content := `
default_store = "work"

[stores]
personal = "` + filepath.Join(dir, "personal") + `"
work = "` + filepath.Join(dir, "work") + `"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DefaultStore != "work" {
		t.Errorf("DefaultStore = %q, want %q", cfg.DefaultStore, "work")
	}
	if len(cfg.Stores) != 2 {
		t.Errorf("len(Stores) = %d, want 2", len(cfg.Stores))
	}
	if cfg.Stores["personal"] != filepath.Join(dir, "personal") {
		t.Errorf("Stores[personal] = %q", cfg.Stores["personal"])
	}
	if cfg.Stores["work"] != filepath.Join(dir, "work") {
		t.Errorf("Stores[work] = %q", cfg.Stores["work"])
	}
}

func TestLoadNonexistent(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.DefaultStore != "" {
		t.Errorf("DefaultStore should be empty, got %q", cfg.DefaultStore)
	}
	if len(cfg.Stores) != 0 {
		t.Errorf("Stores should be empty, got %d entries", len(cfg.Stores))
	}
}

func TestLoadEmptyConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	os.WriteFile(cfgPath, []byte(""), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Stores) != 0 {
		t.Errorf("Stores should be empty, got %d entries", len(cfg.Stores))
	}
}

func TestResolveStore(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "mystore")
	os.MkdirAll(storeDir, 0755)

	cfg := &Config{
		Stores: map[string]string{
			"test": storeDir,
		},
	}

	resolved, err := cfg.ResolveStore("test")
	if err != nil {
		t.Fatalf("ResolveStore: %v", err)
	}
	if resolved != storeDir {
		t.Errorf("resolved = %q, want %q", resolved, storeDir)
	}
}

func TestResolveStoreMissing(t *testing.T) {
	cfg := &Config{
		Stores: map[string]string{},
	}
	_, err := cfg.ResolveStore("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing store")
	}
}

func TestResolveStorePathNotExist(t *testing.T) {
	cfg := &Config{
		Stores: map[string]string{
			"bad": "/nonexistent/store/path",
		},
	}
	_, err := cfg.ResolveStore("bad")
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
}

func TestResolveStoreNotDirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notadir")
	os.WriteFile(filePath, []byte("x"), 0644)

	cfg := &Config{
		Stores: map[string]string{
			"file": filePath,
		},
	}
	_, err := cfg.ResolveStore("file")
	if err == nil {
		t.Fatal("expected error for file (not directory)")
	}
}

func TestLoadMalformedTOML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	os.WriteFile(cfgPath, []byte("[invalid toml @@!!"), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for malformed TOML")
	}
}

func TestLoadDefaultStoreNotInStores(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `
default_store = "missing"

[stores]
personal = "/tmp/personal"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error when default_store not in [stores]")
	}
}

func TestLoadRepos(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	os.MkdirAll(storeDir, 0755)

	cfgPath := filepath.Join(dir, "config.toml")
	content := `
default_store = "work"

[stores]
work = "` + storeDir + `"

[[repos]]
name = "api"
path = "/tmp/api"
bundle = "backend"
store = "work"

[[repos]]
name = "web"
path = "/tmp/web"
bundle = "frontend"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].Name != "api" {
		t.Errorf("repo 0 name = %q, want api", cfg.Repos[0].Name)
	}
	if cfg.Repos[0].Store != "work" {
		t.Errorf("repo 0 store = %q, want work", cfg.Repos[0].Store)
	}
	if cfg.Repos[1].Name != "web" {
		t.Errorf("repo 1 name = %q, want web", cfg.Repos[1].Name)
	}
	if cfg.Repos[1].Store != "work" {
		t.Errorf("repo 1 store = %q, want work (default)", cfg.Repos[1].Store)
	}
}

func TestLoadReposStoreDefault(t *testing.T) {
	dir := t.TempDir()
	personalDir := filepath.Join(dir, "personal")
	workDir := filepath.Join(dir, "work")
	os.MkdirAll(personalDir, 0755)
	os.MkdirAll(workDir, 0755)

	cfgPath := filepath.Join(dir, "config.toml")
	content := `
default_store = "personal"

[stores]
personal = "` + personalDir + `"
work = "` + workDir + `"

[[repos]]
path = "/tmp/project"
bundle = "backend"
store = "work"

[[repos]]
path = "/tmp/oss"
bundle = "oss"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].Store != "work" {
		t.Errorf("repo 0 store = %q, want work", cfg.Repos[0].Store)
	}
	if cfg.Repos[1].Store != "personal" {
		t.Errorf("repo 1 store = %q, want personal (default)", cfg.Repos[1].Store)
	}
}

func TestLoadReposDefaultLayout(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `
[[repos]]
path = "/tmp/project"
bundle = "backend"

[[repos]]
path = "/tmp/other"
bundle = "frontend"
layout = "claude"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Repos[0].Layout != "pi" {
		t.Errorf("repo 0 layout = %q, want pi (default)", cfg.Repos[0].Layout)
	}
	if cfg.Repos[1].Layout != "claude" {
		t.Errorf("repo 1 layout = %q, want claude", cfg.Repos[1].Layout)
	}
}

func TestLoadReposTildeExpansion(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `
[[repos]]
path = "~/dev/api"
bundle = "backend"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "dev", "api")
	if cfg.Repos[0].Path != want {
		t.Errorf("path = %q, want %q", cfg.Repos[0].Path, want)
	}
}

func TestLoadReposEmptyPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `
[[repos]]
name = "nopath"
bundle = "backend"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for repo with empty path")
	}
}

func TestLoadReposUnknownStore(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	os.MkdirAll(storeDir, 0755)

	cfgPath := filepath.Join(dir, "config.toml")
	content := `
default_store = "work"

[stores]
work = "` + storeDir + `"

[[repos]]
path = "/tmp/project"
bundle = "backend"
store = "typo"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for unknown store reference")
	}
}

func TestLoadUserSection(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	os.MkdirAll(storeDir, 0755)

	cfgPath := filepath.Join(dir, "config.toml")
	content := `
default_store = "personal"

[stores]
personal = "` + storeDir + `"

[user]
bundle = "my-tools"
layout = "all"
skills_add = ["browse"]
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.User == nil {
		t.Fatal("expected User to be set")
	}
	if cfg.User.Bundle != "my-tools" {
		t.Errorf("User.Bundle = %q, want my-tools", cfg.User.Bundle)
	}
	if cfg.User.Layout != "all" {
		t.Errorf("User.Layout = %q, want all", cfg.User.Layout)
	}
	if cfg.User.Store != "personal" {
		t.Errorf("User.Store = %q, want personal (default)", cfg.User.Store)
	}
	if len(cfg.User.SkillsAdd) != 1 || cfg.User.SkillsAdd[0] != "browse" {
		t.Errorf("User.SkillsAdd = %v, want [browse]", cfg.User.SkillsAdd)
	}
}

func TestLoadUserCherryPick(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	os.MkdirAll(storeDir, 0755)

	cfgPath := filepath.Join(dir, "config.toml")
	content := `
default_store = "personal"

[stores]
personal = "` + storeDir + `"

[user]
agents_md = "my-agent"
skills = ["browse", "plan"]
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.User == nil {
		t.Fatal("expected User to be set")
	}
	if cfg.User.AgentsMd != "my-agent" {
		t.Errorf("User.AgentsMd = %q, want my-agent", cfg.User.AgentsMd)
	}
	if len(cfg.User.Skills) != 2 {
		t.Errorf("User.Skills = %v, want [browse plan]", cfg.User.Skills)
	}
}

func TestLoadNoUserSection(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	os.WriteFile(cfgPath, []byte(""), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.User != nil {
		t.Errorf("expected User to be nil, got %+v", cfg.User)
	}
}

func TestLoadUserNeitherBundleNorCherryPick(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	os.MkdirAll(storeDir, 0755)

	cfgPath := filepath.Join(dir, "config.toml")
	content := `
default_store = "personal"

[stores]
personal = "` + storeDir + `"

[user]
layout = "all"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for [user] with neither bundle nor cherry-pick")
	}
}

func TestLoadUserBothBundleAndCherryPick(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	os.MkdirAll(storeDir, 0755)

	cfgPath := filepath.Join(dir, "config.toml")
	content := `
default_store = "personal"

[stores]
personal = "` + storeDir + `"

[user]
bundle = "my-tools"
skills = ["browse"]
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for [user] with both bundle and cherry-pick")
	}
}

func TestLoadUserUnknownStore(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	os.MkdirAll(storeDir, 0755)

	cfgPath := filepath.Join(dir, "config.toml")
	content := `
default_store = "personal"

[stores]
personal = "` + storeDir + `"

[user]
bundle = "tools"
store = "nonexistent"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for [user] referencing unknown store")
	}
}

func TestUserLockPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	os.WriteFile(cfgPath, []byte(""), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := filepath.Join(dir, "user.lock")
	got := cfg.UserLockPath()
	if got != want {
		t.Errorf("UserLockPath() = %q, want %q", got, want)
	}
}

func TestUserDefaultLayout(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	os.MkdirAll(storeDir, 0755)

	cfgPath := filepath.Join(dir, "config.toml")
	content := `
default_store = "personal"

[stores]
personal = "` + storeDir + `"

[user]
bundle = "tools"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.User.Layout != "all" {
		t.Errorf("User.Layout = %q, want all (default)", cfg.User.Layout)
	}
}

func TestTildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	expanded, err := expandPath("~/some/path")
	if err != nil {
		t.Fatalf("expandPath: %v", err)
	}
	expected := filepath.Join(home, "some", "path")
	if expanded != expected {
		t.Errorf("expandPath(~/some/path) = %q, want %q", expanded, expected)
	}
}
