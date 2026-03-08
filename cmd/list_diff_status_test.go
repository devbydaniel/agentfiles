package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielbenner/agentfiles/internal/lock"
)

// setupStore creates a minimal store with git init and required subdirs.
func setupStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"skills", "agents", "plugins", "resources", "bundles"} {
		os.MkdirAll(filepath.Join(dir, sub), 0o755)
	}
	gitIn(t, dir, "init")
	gitIn(t, dir, "config", "user.email", "test@test.com")
	gitIn(t, dir, "config", "user.name", "test")
	return dir
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s failed: %v\n%s", args, dir, err, out)
	}
}

func TestListSkillsShowsNames(t *testing.T) {
	storeDir := setupStore(t)

	os.MkdirAll(filepath.Join(storeDir, "skills", "web-search"), 0o755)
	os.WriteFile(filepath.Join(storeDir, "skills", "web-search", "SKILL.md"), []byte("# web-search"), 0o644)
	os.MkdirAll(filepath.Join(storeDir, "skills", "browse"), 0o755)
	os.WriteFile(filepath.Join(storeDir, "skills", "browse", "SKILL.md"), []byte("# browse"), 0o644)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"list", "skills", "--store", storeDir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "browse") {
		t.Errorf("expected 'browse' in output, got: %q", out)
	}
	if !strings.Contains(out, "web-search") {
		t.Errorf("expected 'web-search' in output, got: %q", out)
	}
}

func TestListBundlesEmpty(t *testing.T) {
	storeDir := setupStore(t)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"list", "bundles", "--store", storeDir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := strings.TrimSpace(buf.String()); got != "" {
		t.Errorf("expected empty output, got: %q", got)
	}
}

func TestDiffClean(t *testing.T) {
	storeDir := setupStore(t)
	projDir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(projDir)

	// Write the source file into the store
	os.MkdirAll(filepath.Join(storeDir, "agents"), 0o755)
	srcFile := filepath.Join(storeDir, "agents", "default.md")
	os.WriteFile(srcFile, []byte("hello world\n"), 0o644)

	deployFile := filepath.Join(projDir, "AGENTS.md")
	os.WriteFile(deployFile, []byte("hello world\n"), 0o644)

	hash, _ := lock.Hash(srcFile)

	lf := &lock.LockFile{}
	lf.Deployed.Skills = make(map[string]*lock.Entry)
	lf.Deployed.Plugins = make(map[string]*lock.Entry)
	lf.Deployed.Resources = make(map[string]*lock.Entry)
	lf.Deployed.AgentsMD = &lock.Entry{
		StorePath:    "agents/default.md",
		DeployedPath: "AGENTS.md",
		Hash:         hash,
	}
	lock.Save(projDir, lf)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"diff", "--store", storeDir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := strings.TrimSpace(buf.String())
	if out != "clean" {
		t.Errorf("expected 'clean', got: %q", out)
	}
}

func TestDiffWithLocalEdit(t *testing.T) {
	storeDir := setupStore(t)
	projDir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(projDir)

	os.MkdirAll(filepath.Join(storeDir, "agents"), 0o755)
	srcFile := filepath.Join(storeDir, "agents", "default.md")
	os.WriteFile(srcFile, []byte("line one\nline two\n"), 0o644)

	deployFile := filepath.Join(projDir, "AGENTS.md")
	os.WriteFile(deployFile, []byte("line one\nline CHANGED\n"), 0o644)

	hash, _ := lock.Hash(srcFile)

	lf := &lock.LockFile{}
	lf.Deployed.Skills = make(map[string]*lock.Entry)
	lf.Deployed.Plugins = make(map[string]*lock.Entry)
	lf.Deployed.Resources = make(map[string]*lock.Entry)
	lf.Deployed.AgentsMD = &lock.Entry{
		StorePath:    "agents/default.md",
		DeployedPath: "AGENTS.md",
		Hash:         hash,
	}
	lock.Save(projDir, lf)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"diff", "--store", storeDir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "---") {
		t.Errorf("expected diff header, got: %q", out)
	}
	if !strings.Contains(out, "+line CHANGED") {
		t.Errorf("expected '+line CHANGED' in diff, got: %q", out)
	}
}

func TestStatusShowsCorrectStates(t *testing.T) {
	storeDir := setupStore(t)
	projDir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(projDir)

	// Create resource files inside the store's resources directory
	storeResDir := filepath.Join(storeDir, "resources", "myres")
	os.MkdirAll(storeResDir, 0o755)

	// Unchanged
	srcUnchanged := filepath.Join(storeResDir, "unchanged.txt")
	os.WriteFile(srcUnchanged, []byte("same\n"), 0o644)
	deployUnchanged := filepath.Join(projDir, "unchanged.txt")
	os.WriteFile(deployUnchanged, []byte("same\n"), 0o644)
	hashUnchanged, _ := lock.Hash(srcUnchanged)

	// Modified locally
	srcLocal := filepath.Join(storeResDir, "local.txt")
	os.WriteFile(srcLocal, []byte("original\n"), 0o644)
	deployLocal := filepath.Join(projDir, "local.txt")
	os.WriteFile(deployLocal, []byte("edited locally\n"), 0o644)
	hashLocal, _ := lock.Hash(srcLocal)

	// Modified in store
	srcStore := filepath.Join(storeResDir, "remote.txt")
	deployStore := filepath.Join(projDir, "remote.txt")
	os.WriteFile(deployStore, []byte("old content\n"), 0o644)
	hashStore, _ := lock.Hash(deployStore)
	os.WriteFile(srcStore, []byte("new store content\n"), 0o644)

	// Conflict
	srcConflict := filepath.Join(storeResDir, "conflict.txt")
	deployConflict := filepath.Join(projDir, "conflict.txt")
	tmpBase := filepath.Join(projDir, ".tmpbase")
	os.WriteFile(tmpBase, []byte("base\n"), 0o644)
	hashConflict, _ := lock.Hash(tmpBase)
	os.Remove(tmpBase)
	os.WriteFile(srcConflict, []byte("store changed\n"), 0o644)
	os.WriteFile(deployConflict, []byte("local changed\n"), 0o644)

	lf := &lock.LockFile{}
	lf.Deployed.Skills = make(map[string]*lock.Entry)
	lf.Deployed.Plugins = make(map[string]*lock.Entry)
	lf.Deployed.Resources = map[string]*lock.Entry{
		"unchanged": {StorePath: "resources/myres/unchanged.txt", DeployedPath: "unchanged.txt", Hash: hashUnchanged},
		"local":     {StorePath: "resources/myres/local.txt", DeployedPath: "local.txt", Hash: hashLocal},
		"remote":    {StorePath: "resources/myres/remote.txt", DeployedPath: "remote.txt", Hash: hashStore},
		"conflict":  {StorePath: "resources/myres/conflict.txt", DeployedPath: "conflict.txt", Hash: hashConflict},
	}
	lock.Save(projDir, lf)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"status", "--store", storeDir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()

	checks := map[string]string{
		"resource:unchanged": "unchanged",
		"resource:local":     "modified locally",
		"resource:remote":    "modified in store",
		"resource:conflict":  "conflict",
	}

	for name, wantState := range checks {
		found := false
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, name) && strings.Contains(line, wantState) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q with state %q in output:\n%s", name, wantState, out)
		}
	}
}
