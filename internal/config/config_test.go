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

// --- Repo loading + merge tests ---

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
	// Named repos without local entries are skipped
	if len(cfg.Repos) != 0 {
		t.Fatalf("expected 0 repos (no local entries), got %d", len(cfg.Repos))
	}
}

func TestLoadReposWithLocal(t *testing.T) {
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
bundle = "frontend"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	localPath := filepath.Join(dir, "config.local.toml")
	localContent := `
[[repos]]
name = "api"
path = "/dev/api"

[[repos]]
name = "web"
path = "/dev/web"
`
	os.WriteFile(localPath, []byte(localContent), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.Repos))
	}
	// api has explicit store
	if cfg.Repos[0].Store != "work" {
		t.Errorf("api store = %q, want work", cfg.Repos[0].Store)
	}
	if cfg.Repos[0].Path != "/dev/api" {
		t.Errorf("api path = %q, want /dev/api", cfg.Repos[0].Path)
	}
	// web has no store, should default to default_store
	if cfg.Repos[1].Store != "work" {
		t.Errorf("web store = %q, want work (default)", cfg.Repos[1].Store)
	}
	if cfg.Repos[1].Path != "/dev/web" {
		t.Errorf("web path = %q, want /dev/web", cfg.Repos[1].Path)
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

func TestLoadReposLocalSkip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `
[[repos]]
name = "api"
bundle = "backend"

[[repos]]
name = "web"
bundle = "frontend"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	localPath := filepath.Join(dir, "config.local.toml")
	localContent := `
[[repos]]
name = "api"
path = "/dev/api"

[[repos]]
name = "web"
path = "/dev/web"
skip = true
`
	os.WriteFile(localPath, []byte(localContent), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Fatalf("expected 1 repo (web skipped), got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].Name != "api" {
		t.Errorf("expected api, got %s", cfg.Repos[0].Name)
	}
}

func TestLoadReposLocalOverrides(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `
[[repos]]
name = "api"
bundle = "backend"
layout = "pi"
skills_add = ["base"]
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	localPath := filepath.Join(dir, "config.local.toml")
	localContent := `
[[repos]]
name = "api"
path = "/dev/api"
layout = "claude"
skills_add = ["local-extra"]
skills_remove = ["unwanted"]
`
	os.WriteFile(localPath, []byte(localContent), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(cfg.Repos))
	}
	repo := cfg.Repos[0]
	if repo.Layout != "claude" {
		t.Errorf("layout = %q, want claude", repo.Layout)
	}
	if len(repo.SkillsAdd) != 2 || repo.SkillsAdd[0] != "base" || repo.SkillsAdd[1] != "local-extra" {
		t.Errorf("skills_add = %v, want [base local-extra]", repo.SkillsAdd)
	}
	if len(repo.SkillsRemove) != 1 || repo.SkillsRemove[0] != "unwanted" {
		t.Errorf("skills_remove = %v, want [unwanted]", repo.SkillsRemove)
	}
}

func TestLoadReposLocalDuplicate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `
[[repos]]
name = "api"
bundle = "backend"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	localPath := filepath.Join(dir, "config.local.toml")
	localContent := `
[[repos]]
name = "api"
path = "/dev/api1"

[[repos]]
name = "api"
path = "/dev/api2"
`
	os.WriteFile(localPath, []byte(localContent), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for duplicate local name")
	}
}

func TestLoadReposTildeExpansion(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `
[[repos]]
name = "api"
bundle = "backend"
`
	os.WriteFile(cfgPath, []byte(content), 0644)

	localPath := filepath.Join(dir, "config.local.toml")
	localContent := `
[[repos]]
name = "api"
path = "~/dev/api"
`
	os.WriteFile(localPath, []byte(localContent), 0644)

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

func TestLoadReposLocalOnly(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	os.WriteFile(cfgPath, []byte(""), 0644)

	localPath := filepath.Join(dir, "config.local.toml")
	localContent := `
[[repos]]
name = "devonly"
path = "/dev/devonly"
bundle = "devtools"
`
	os.WriteFile(localPath, []byte(localContent), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].Name != "devonly" {
		t.Errorf("name = %q, want devonly", cfg.Repos[0].Name)
	}
	if cfg.Repos[0].Bundle != "devtools" {
		t.Errorf("bundle = %q, want devtools", cfg.Repos[0].Bundle)
	}
	if cfg.Repos[0].Layout != "pi" {
		t.Errorf("layout = %q, want pi (default)", cfg.Repos[0].Layout)
	}
}

func TestLoadReposEmptyPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	os.WriteFile(cfgPath, []byte(""), 0644)

	localPath := filepath.Join(dir, "config.local.toml")
	localContent := `
[[repos]]
name = "nopath"
bundle = "backend"
`
	os.WriteFile(localPath, []byte(localContent), 0644)

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

func TestTildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	// Create a temp dir inside home to test tilde expansion
	// We use expandPath directly since ResolveStore also validates existence
	expanded, err := expandPath("~/some/path")
	if err != nil {
		t.Fatalf("expandPath: %v", err)
	}
	expected := filepath.Join(home, "some", "path")
	if expanded != expected {
		t.Errorf("expandPath(~/some/path) = %q, want %q", expanded, expected)
	}
}
