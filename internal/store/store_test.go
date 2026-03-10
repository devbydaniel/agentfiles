package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInitCreatesStructure(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "test-store")

	s, err := Init(storePath)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	for _, sub := range subdirs {
		p := filepath.Join(s.Root, sub)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("missing subdir %q: %v", sub, err)
		} else if !info.IsDir() {
			t.Errorf("%q is not a directory", sub)
		}
	}

	// Check .git exists
	gitDir := filepath.Join(s.Root, ".git")
	info, err := os.Stat(gitDir)
	if err != nil || !info.IsDir() {
		t.Errorf(".git directory missing or not a dir")
	}
}

func TestInitIdempotent(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "test-store")

	s1, err := Init(storePath)
	if err != nil {
		t.Fatalf("first Init failed: %v", err)
	}

	// Write a file inside skills/ to verify no data loss
	marker := filepath.Join(s1.SkillsDir(), "marker.txt")
	if err := os.WriteFile(marker, []byte("hello"), 0o644); err != nil {
		t.Fatalf("writing marker: %v", err)
	}

	s2, err := Init(storePath)
	if err != nil {
		t.Fatalf("second Init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(s2.SkillsDir(), "marker.txt"))
	if err != nil {
		t.Fatalf("marker file lost after second Init: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("marker content = %q, want %q", string(data), "hello")
	}
}

func TestOpenValid(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "test-store")

	_, err := Init(storePath)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	s, err := Open(storePath)
	if err != nil {
		t.Fatalf("Open failed on valid store: %v", err)
	}

	if s.SkillsDir() != filepath.Join(s.Root, "skills") {
		t.Errorf("SkillsDir() = %q", s.SkillsDir())
	}
}

func TestOpenInvalidDir(t *testing.T) {
	dir := t.TempDir()
	_, err := Open(dir)
	if err == nil {
		t.Fatal("Open should fail on a random directory without store structure")
	}
}

func TestOpenMissingGitDir(t *testing.T) {
	dir := t.TempDir()
	// Create all subdirs but no .git — Open should reject it.
	for _, sub := range subdirs {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_, err := Open(dir)
	if err == nil {
		t.Fatal("Open should fail when .git is missing")
	}
}

func TestOpenNonexistent(t *testing.T) {
	_, err := Open("/tmp/nonexistent-agentfiles-store-xyz")
	if err == nil {
		t.Fatal("Open should fail on nonexistent path")
	}
}

func TestDirMethods(t *testing.T) {
	s := &Store{Root: "/tmp/store"}
	checks := map[string]string{
		"skills":    s.SkillsDir(),
		"agents":    s.AgentsDir(),
		"resources": s.ResourcesDir(),
		"bundles":   s.BundlesDir(),
	}
	for name, got := range checks {
		want := filepath.Join("/tmp/store", name)
		if got != want {
			t.Errorf("%sDir() = %q, want %q", name, got, want)
		}
	}
}

// createBareRepo creates a bare git repo with the required store subdirectories.
func createBareRepo(t *testing.T) string {
	t.Helper()

	// Create a normal store, commit, then clone --bare.
	tmp := t.TempDir()
	srcPath := filepath.Join(tmp, "src")
	s, err := Init(srcPath)
	if err != nil {
		t.Fatalf("Init for bare repo source: %v", err)
	}

	// Add a marker file and commit so the clone is non-empty.
	marker := filepath.Join(s.SkillsDir(), "hello.txt")
	if err := os.WriteFile(marker, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"git", "-C", srcPath, "add", "-A"},
		{"git", "-C", srcPath, "-c", "user.name=test", "-c", "user.email=t@t", "commit", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			t.Fatalf("command %v failed: %v", args, err)
		}
	}

	barePath := filepath.Join(tmp, "bare.git")
	cmd := exec.Command("git", "clone", "--bare", "--", srcPath, barePath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		t.Fatalf("git clone --bare: %v", err)
	}

	return barePath
}

func TestInitFromClone(t *testing.T) {
	bareRepo := createBareRepo(t)
	dest := filepath.Join(t.TempDir(), "cloned-store")

	s, err := InitFromClone(bareRepo, dest)
	if err != nil {
		t.Fatalf("InitFromClone failed: %v", err)
	}

	// Verify subdirs
	for _, sub := range subdirs {
		p := filepath.Join(s.Root, sub)
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			t.Errorf("missing subdir %q after clone", sub)
		}
	}

	// Verify .git
	gi, err := os.Stat(filepath.Join(s.Root, ".git"))
	if err != nil || !gi.IsDir() {
		t.Error(".git directory missing after clone")
	}

	// Verify marker file survived
	data, err := os.ReadFile(filepath.Join(s.SkillsDir(), "hello.txt"))
	if err != nil {
		t.Fatalf("marker file missing: %v", err)
	}
	if string(data) != "hi" {
		t.Errorf("marker = %q, want %q", string(data), "hi")
	}
}

func TestInitFromClonePathExists(t *testing.T) {
	bareRepo := createBareRepo(t)
	dest := filepath.Join(t.TempDir(), "existing")

	// Pre-create the destination so git clone will fail.
	if err := os.MkdirAll(filepath.Join(dest, "blocker"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := InitFromClone(bareRepo, dest)
	if err == nil {
		t.Fatal("InitFromClone should fail when destination already exists")
	}
}

func TestInitFromCloneBadURL(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "bad-clone")
	_, err := InitFromClone("https://invalid.example.test/no-such-repo.git", dest)
	if err == nil {
		t.Fatal("InitFromClone should fail with invalid URL")
	}
}
