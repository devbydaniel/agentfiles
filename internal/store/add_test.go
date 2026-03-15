package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Init(filepath.Join(dir, "store"))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

func TestAddSkillCopiesDir(t *testing.T) {
	s := setupStore(t)

	// Create a source skill directory.
	src := filepath.Join(t.TempDir(), "test-skill")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Test Skill"), 0o644)
	os.MkdirAll(filepath.Join(src, "sub"), 0o755)
	os.WriteFile(filepath.Join(src, "sub", "helper.sh"), []byte("#!/bin/bash"), 0o755)

	name, overwritten, err := s.AddSkill(src, false)
	if err != nil {
		t.Fatalf("AddSkill: %v", err)
	}
	if name != "test-skill" {
		t.Errorf("name = %q, want %q", name, "test-skill")
	}
	if overwritten {
		t.Error("expected overwritten=false")
	}

	// Verify files copied.
	data, err := os.ReadFile(filepath.Join(s.SkillsDir(), "test-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("SKILL.md missing: %v", err)
	}
	if string(data) != "# Test Skill" {
		t.Errorf("SKILL.md content = %q", data)
	}

	data, err = os.ReadFile(filepath.Join(s.SkillsDir(), "test-skill", "sub", "helper.sh"))
	if err != nil {
		t.Fatalf("sub/helper.sh missing: %v", err)
	}
	if string(data) != "#!/bin/bash" {
		t.Errorf("sub/helper.sh content = %q", data)
	}

	// Verify file permissions preserved.
	info, err := os.Stat(filepath.Join(s.SkillsDir(), "test-skill", "sub", "helper.sh"))
	if err != nil {
		t.Fatalf("stat helper.sh: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("helper.sh lost execute permission: %v", info.Mode())
	}
}

func TestAddAgentCopiesFile(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "AGENTS.md")
	os.WriteFile(src, []byte("# Agent instructions"), 0o644)

	overwritten, err := s.AddAgent(src, "test", false)
	if err != nil {
		t.Fatalf("AddAgent: %v", err)
	}
	if overwritten {
		t.Error("expected overwritten=false")
	}

	data, err := os.ReadFile(filepath.Join(s.AgentsDir(), "test.md"))
	if err != nil {
		t.Fatalf("test.md missing: %v", err)
	}
	if string(data) != "# Agent instructions" {
		t.Errorf("content = %q", data)
	}
}

func TestAddAgentAlreadyExistsNoForce(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "agent.md")
	os.WriteFile(src, []byte("v1"), 0o644)

	if _, err := s.AddAgent(src, "myagent", false); err != nil {
		t.Fatalf("first AddAgent: %v", err)
	}

	os.WriteFile(src, []byte("v2"), 0o644)
	_, err := s.AddAgent(src, "myagent", false)
	if err == nil {
		t.Fatal("expected error on duplicate add without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err)
	}

	// Original content preserved.
	data, _ := os.ReadFile(filepath.Join(s.AgentsDir(), "myagent.md"))
	if string(data) != "v1" {
		t.Errorf("content = %q, want v1", data)
	}
}

func TestAddAgentAlreadyExistsWithForce(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "agent.md")
	os.WriteFile(src, []byte("v1"), 0o644)

	if _, err := s.AddAgent(src, "myagent", false); err != nil {
		t.Fatalf("first AddAgent: %v", err)
	}

	os.WriteFile(src, []byte("v2"), 0o644)
	overwritten, err := s.AddAgent(src, "myagent", true)
	if err != nil {
		t.Fatalf("AddAgent with force: %v", err)
	}
	if !overwritten {
		t.Error("expected overwritten=true")
	}

	data, _ := os.ReadFile(filepath.Join(s.AgentsDir(), "myagent.md"))
	if string(data) != "v2" {
		t.Errorf("content = %q, want v2", data)
	}
}

func TestAddAgentRejectsDirectory(t *testing.T) {
	s := setupStore(t)

	src := t.TempDir()
	_, err := s.AddAgent(src, "myagent", false)
	if err == nil {
		t.Fatal("expected error when source is a directory")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("error = %q, want 'is a directory'", err)
	}
}

func TestAddAgentRejectsPathTraversal(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "agent.md")
	os.WriteFile(src, []byte("data"), 0o644)

	for _, bad := range []string{"../escape", "foo/bar", `foo\bar`, "a..b/c"} {
		_, err := s.AddAgent(src, bad, false)
		if err == nil {
			t.Errorf("expected error for name %q", bad)
		}
	}
}

func TestAddSkillAlreadyExistsNoForce(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "my-skill")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("v1"), 0o644)

	if _, _, err := s.AddSkill(src, false); err != nil {
		t.Fatalf("first AddSkill: %v", err)
	}

	// Second add without force should fail.
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("v2"), 0o644)
	_, _, err := s.AddSkill(src, false)
	if err == nil {
		t.Fatal("expected error on duplicate add without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err)
	}

	// Original content preserved.
	data, _ := os.ReadFile(filepath.Join(s.SkillsDir(), "my-skill", "SKILL.md"))
	if string(data) != "v1" {
		t.Errorf("content = %q, want v1", data)
	}
}

func TestAddSkillAlreadyExistsWithForce(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "my-skill")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("v1"), 0o644)

	if _, _, err := s.AddSkill(src, false); err != nil {
		t.Fatalf("first AddSkill: %v", err)
	}

	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("v2"), 0o644)
	_, overwritten, err := s.AddSkill(src, true)
	if err != nil {
		t.Fatalf("AddSkill with force: %v", err)
	}
	if !overwritten {
		t.Error("expected overwritten=true")
	}

	data, _ := os.ReadFile(filepath.Join(s.SkillsDir(), "my-skill", "SKILL.md"))
	if string(data) != "v2" {
		t.Errorf("content = %q, want v2", data)
	}
}

func TestAddSourceNotExists(t *testing.T) {
	s := setupStore(t)

	_, _, err := s.AddSkill("/nonexistent/path/xyz", false)
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q", err)
	}

	_, err = s.AddAgent("/nonexistent/file.md", "x", false)
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestAddResourcePreservesStructure(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "test-resource")
	os.MkdirAll(filepath.Join(src, "a", "b"), 0o755)
	os.WriteFile(filepath.Join(src, "top.txt"), []byte("top"), 0o644)
	os.WriteFile(filepath.Join(src, "a", "mid.txt"), []byte("mid"), 0o644)
	os.WriteFile(filepath.Join(src, "a", "b", "deep.txt"), []byte("deep"), 0o644)

	name, _, err := s.AddResource(src, false)
	if err != nil {
		t.Fatalf("AddResource: %v", err)
	}
	if name != "test-resource" {
		t.Errorf("name = %q, want %q", name, "test-resource")
	}

	base := filepath.Join(s.ResourcesDir(), "test-resource")
	for _, tc := range []struct {
		path, want string
	}{
		{"top.txt", "top"},
		{filepath.Join("a", "mid.txt"), "mid"},
		{filepath.Join("a", "b", "deep.txt"), "deep"},
	} {
		data, err := os.ReadFile(filepath.Join(base, tc.path))
		if err != nil {
			t.Errorf("%s missing: %v", tc.path, err)
		} else if string(data) != tc.want {
			t.Errorf("%s = %q, want %q", tc.path, data, tc.want)
		}
	}
}
