package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielbenner/agentfiles/internal/manifest"
)

func TestInitCreatesManifest(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"init", "--bundle", "assistant", "--layout", "pi"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(filepath.Join(dir, ".agentfiles"))
	if err != nil {
		t.Fatalf("reading .agentfiles: %v", err)
	}

	content := string(data)
	if content == "" {
		t.Fatal(".agentfiles is empty")
	}

	// Verify it contains expected values
	if !bytes.Contains(data, []byte(`"assistant"`)) {
		t.Errorf("expected bundle=assistant in content: %s", content)
	}
	if !bytes.Contains(data, []byte(`"pi"`)) {
		t.Errorf("expected layout=pi in content: %s", content)
	}

	// Verify output suggests .gitignore
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("AGENTS.md")) {
		t.Errorf("expected AGENTS.md in gitignore suggestions, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte(".agentfiles.lock")) {
		t.Errorf("expected .agentfiles.lock in gitignore suggestions, got: %s", out)
	}
}

func TestInitExistingError(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	// Create existing .agentfiles
	os.WriteFile(filepath.Join(dir, ".agentfiles"), []byte("bundle = \"foo\"\n"), 0644)

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"init", "--bundle", "test", "--layout", "pi"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for existing .agentfiles")
	}
	if err.Error() != "already initialized, use af apply" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInitParsableByManifestLoad(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"init", "--bundle", "assistant", "--layout", "claude"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatalf("manifest.Load failed: %v", err)
	}

	if m.Bundle != "assistant" {
		t.Errorf("bundle = %q, want %q", m.Bundle, "assistant")
	}
	if m.Layout != "claude" {
		t.Errorf("layout = %q, want %q", m.Layout, "claude")
	}
}
