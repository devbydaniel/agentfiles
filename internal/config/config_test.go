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
