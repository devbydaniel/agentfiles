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

	name, overwritten, err := s.AddSkill(src, "", false)
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

func TestAddInstructionCopiesFile(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "AGENTS.md")
	os.WriteFile(src, []byte("# Agent instructions"), 0o644)

	overwritten, err := s.AddInstruction(src, "test", false)
	if err != nil {
		t.Fatalf("AddAgent: %v", err)
	}
	if overwritten {
		t.Error("expected overwritten=false")
	}

	data, err := os.ReadFile(filepath.Join(s.InstructionsDir(), "test.md"))
	if err != nil {
		t.Fatalf("test.md missing: %v", err)
	}
	if string(data) != "# Agent instructions" {
		t.Errorf("content = %q", data)
	}
}

func TestAddInstructionAlreadyExistsNoForce(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "agent.md")
	os.WriteFile(src, []byte("v1"), 0o644)

	if _, err := s.AddInstruction(src, "myagent", false); err != nil {
		t.Fatalf("first AddAgent: %v", err)
	}

	os.WriteFile(src, []byte("v2"), 0o644)
	_, err := s.AddInstruction(src, "myagent", false)
	if err == nil {
		t.Fatal("expected error on duplicate add without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err)
	}

	// Original content preserved.
	data, _ := os.ReadFile(filepath.Join(s.InstructionsDir(), "myagent.md"))
	if string(data) != "v1" {
		t.Errorf("content = %q, want v1", data)
	}
}

func TestAddInstructionAlreadyExistsWithForce(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "agent.md")
	os.WriteFile(src, []byte("v1"), 0o644)

	if _, err := s.AddInstruction(src, "myagent", false); err != nil {
		t.Fatalf("first AddAgent: %v", err)
	}

	os.WriteFile(src, []byte("v2"), 0o644)
	overwritten, err := s.AddInstruction(src, "myagent", true)
	if err != nil {
		t.Fatalf("AddAgent with force: %v", err)
	}
	if !overwritten {
		t.Error("expected overwritten=true")
	}

	data, _ := os.ReadFile(filepath.Join(s.InstructionsDir(), "myagent.md"))
	if string(data) != "v2" {
		t.Errorf("content = %q, want v2", data)
	}
}

func TestAddInstructionRejectsDirectory(t *testing.T) {
	s := setupStore(t)

	src := t.TempDir()
	_, err := s.AddInstruction(src, "myagent", false)
	if err == nil {
		t.Fatal("expected error when source is a directory")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("error = %q, want 'is a directory'", err)
	}
}

func TestAddInstructionRejectsPathTraversal(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "agent.md")
	os.WriteFile(src, []byte("data"), 0o644)

	for _, bad := range []string{"../escape", "foo/bar", `foo\bar`, "a..b/c"} {
		_, err := s.AddInstruction(src, bad, false)
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

	if _, _, err := s.AddSkill(src, "", false); err != nil {
		t.Fatalf("first AddSkill: %v", err)
	}

	// Second add without force should fail.
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("v2"), 0o644)
	_, _, err := s.AddSkill(src, "", false)
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

	if _, _, err := s.AddSkill(src, "", false); err != nil {
		t.Fatalf("first AddSkill: %v", err)
	}

	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("v2"), 0o644)
	_, overwritten, err := s.AddSkill(src, "", true)
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

	_, _, err := s.AddSkill("/nonexistent/path/xyz", "", false)
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q", err)
	}

	_, err = s.AddInstruction("/nonexistent/file.md", "x", false)
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

// --- Group tests ---

func TestAddSkillWithGroup(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "browse")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Browse"), 0o644)

	name, overwritten, err := s.AddSkill(src, "tooling", false)
	if err != nil {
		t.Fatalf("AddSkill: %v", err)
	}
	if name != "tooling/browse" {
		t.Errorf("name = %q, want tooling/browse", name)
	}
	if overwritten {
		t.Error("expected overwritten=false")
	}

	data, err := os.ReadFile(filepath.Join(s.SkillsDir(), "tooling", "browse", "SKILL.md"))
	if err != nil {
		t.Fatalf("SKILL.md missing: %v", err)
	}
	if string(data) != "# Browse" {
		t.Errorf("content = %q", data)
	}
}

func TestAddSkillWithDeepGroup(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "deploy")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Deploy"), 0o644)

	name, _, err := s.AddSkill(src, "infra/aws", false)
	if err != nil {
		t.Fatalf("AddSkill: %v", err)
	}
	if name != "infra/aws/deploy" {
		t.Errorf("name = %q, want infra/aws/deploy", name)
	}

	if _, err := os.Stat(filepath.Join(s.SkillsDir(), "infra", "aws", "deploy", "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not found at expected path: %v", err)
	}
}

func TestAddSkillWithGroupNoGroup(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "browse")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Browse"), 0o644)

	name, _, err := s.AddSkill(src, "", false)
	if err != nil {
		t.Fatalf("AddSkill: %v", err)
	}
	if name != "browse" {
		t.Errorf("name = %q, want browse", name)
	}
}

func TestAddSkillWithGroupInvalidTraversal(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "browse")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Browse"), 0o644)

	for _, bad := range []string{"../escape", "foo/../bar", "."} {
		_, _, err := s.AddSkill(src, bad, false)
		if err == nil {
			t.Errorf("expected error for group %q", bad)
		}
	}
}

// --- AddAgent tests ---

func TestAddAgentFileCopy(t *testing.T) {
	s := setupStore(t)
	src := filepath.Join(t.TempDir(), "reviewer.md")
	os.WriteFile(src, []byte("---\nname: reviewer\n---\nReview code."), 0o644)

	name, overwritten, err := s.AddAgent(src, false)
	if err != nil {
		t.Fatalf("AddAgent: %v", err)
	}
	if name != "reviewer" {
		t.Errorf("name = %q, want reviewer", name)
	}
	if overwritten {
		t.Error("expected overwritten=false")
	}

	data, err := os.ReadFile(filepath.Join(s.AgentsDir(), "reviewer.md"))
	if err != nil {
		t.Fatalf("agent file missing: %v", err)
	}
	if !strings.Contains(string(data), "Review code.") {
		t.Errorf("content = %q", data)
	}
}

func TestAddAgentOverwriteNoForce(t *testing.T) {
	s := setupStore(t)
	src := filepath.Join(t.TempDir(), "reviewer.md")
	os.WriteFile(src, []byte("v1"), 0o644)

	if _, _, err := s.AddAgent(src, false); err != nil {
		t.Fatalf("first AddAgent: %v", err)
	}

	os.WriteFile(src, []byte("v2"), 0o644)
	_, _, err := s.AddAgent(src, false)
	if err == nil {
		t.Fatal("expected error on duplicate without force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q", err)
	}
}

func TestAddAgentOverwriteWithForce(t *testing.T) {
	s := setupStore(t)
	src := filepath.Join(t.TempDir(), "reviewer.md")
	os.WriteFile(src, []byte("v1"), 0o644)

	if _, _, err := s.AddAgent(src, false); err != nil {
		t.Fatalf("first AddAgent: %v", err)
	}

	os.WriteFile(src, []byte("v2"), 0o644)
	_, overwritten, err := s.AddAgent(src, true)
	if err != nil {
		t.Fatalf("AddAgent with force: %v", err)
	}
	if !overwritten {
		t.Error("expected overwritten=true")
	}

	data, _ := os.ReadFile(filepath.Join(s.AgentsDir(), "reviewer.md"))
	if string(data) != "v2" {
		t.Errorf("content = %q, want v2", data)
	}
}

func TestAddAgentFileRejectsDirectory(t *testing.T) {
	s := setupStore(t)
	_, _, err := s.AddAgent(t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error for directory source")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("error = %q", err)
	}
}

func TestAddAgentRejectsNonMD(t *testing.T) {
	s := setupStore(t)
	src := filepath.Join(t.TempDir(), "agent.toml")
	os.WriteFile(src, []byte("name = 'test'"), 0o644)

	_, _, err := s.AddAgent(src, false)
	if err == nil {
		t.Fatal("expected error for non-.md file")
	}
	if !strings.Contains(err.Error(), ".md extension") {
		t.Errorf("error = %q", err)
	}
}

func TestAddAgentCreatesAgentsDirOnDemand(t *testing.T) {
	s := setupStore(t)
	os.RemoveAll(s.AgentsDir())

	src := filepath.Join(t.TempDir(), "reviewer.md")
	os.WriteFile(src, []byte("content"), 0o644)

	name, _, err := s.AddAgent(src, false)
	if err != nil {
		t.Fatalf("AddAgent: %v", err)
	}
	if name != "reviewer" {
		t.Errorf("name = %q", name)
	}

	info, err := os.Stat(s.AgentsDir())
	if err != nil || !info.IsDir() {
		t.Error("agents/ dir was not created")
	}
}

func TestAddSkillWithGroupForce(t *testing.T) {
	s := setupStore(t)

	src := filepath.Join(t.TempDir(), "browse")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("v1"), 0o644)

	if _, _, err := s.AddSkill(src, "tooling", false); err != nil {
		t.Fatalf("first AddSkill: %v", err)
	}

	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("v2"), 0o644)
	_, overwritten, err := s.AddSkill(src, "tooling", true)
	if err != nil {
		t.Fatalf("AddSkill with force: %v", err)
	}
	if !overwritten {
		t.Error("expected overwritten=true")
	}

	data, _ := os.ReadFile(filepath.Join(s.SkillsDir(), "tooling", "browse", "SKILL.md"))
	if string(data) != "v2" {
		t.Errorf("content = %q, want v2", data)
	}
}

// --- AddPiExtension tests ---

func TestAddPiExtensionSingleFile(t *testing.T) {
	s := setupStore(t)
	src := filepath.Join(t.TempDir(), "no-model-flag.ts")
	os.WriteFile(src, []byte("export default {}"), 0o644)

	name, overwritten, err := s.AddPiExtension(src, false)
	if err != nil {
		t.Fatalf("AddPiExtension: %v", err)
	}
	if name != "no-model-flag" {
		t.Errorf("name = %q, want no-model-flag", name)
	}
	if overwritten {
		t.Error("expected overwritten=false")
	}

	data, err := os.ReadFile(filepath.Join(s.PiExtensionsDir(), "no-model-flag.ts"))
	if err != nil {
		t.Fatalf("file missing: %v", err)
	}
	if string(data) != "export default {}" {
		t.Errorf("content = %q", data)
	}
}

func TestAddPiExtensionDirectory(t *testing.T) {
	s := setupStore(t)
	src := filepath.Join(t.TempDir(), "my-ext")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "index.ts"), []byte("export default {}"), 0o644)
	os.WriteFile(filepath.Join(src, "util.ts"), []byte("export const x = 1"), 0o644)

	name, overwritten, err := s.AddPiExtension(src, false)
	if err != nil {
		t.Fatalf("AddPiExtension: %v", err)
	}
	if name != "my-ext" {
		t.Errorf("name = %q, want my-ext", name)
	}
	if overwritten {
		t.Error("expected overwritten=false")
	}

	data, err := os.ReadFile(filepath.Join(s.PiExtensionsDir(), "my-ext", "index.ts"))
	if err != nil {
		t.Fatalf("index.ts missing: %v", err)
	}
	if string(data) != "export default {}" {
		t.Errorf("content = %q", data)
	}

	data, err = os.ReadFile(filepath.Join(s.PiExtensionsDir(), "my-ext", "util.ts"))
	if err != nil {
		t.Fatalf("util.ts missing: %v", err)
	}
	if string(data) != "export const x = 1" {
		t.Errorf("content = %q", data)
	}
}

func TestAddPiExtensionDirWithoutIndexTs(t *testing.T) {
	s := setupStore(t)
	src := filepath.Join(t.TempDir(), "bad-ext")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "main.ts"), []byte("nope"), 0o644)

	_, _, err := s.AddPiExtension(src, false)
	if err == nil {
		t.Fatal("expected error for directory without index.ts")
	}
	if !strings.Contains(err.Error(), "index.ts") {
		t.Errorf("error = %q", err)
	}
}

func TestAddPiExtensionRejectsNonTs(t *testing.T) {
	s := setupStore(t)
	src := filepath.Join(t.TempDir(), "ext.js")
	os.WriteFile(src, []byte("module.exports = {}"), 0o644)

	_, _, err := s.AddPiExtension(src, false)
	if err == nil {
		t.Fatal("expected error for non-.ts file")
	}
	if !strings.Contains(err.Error(), ".ts extension") {
		t.Errorf("error = %q", err)
	}
}

func TestAddPiExtensionOverwriteNoForce(t *testing.T) {
	s := setupStore(t)
	src := filepath.Join(t.TempDir(), "ext.ts")
	os.WriteFile(src, []byte("v1"), 0o644)

	if _, _, err := s.AddPiExtension(src, false); err != nil {
		t.Fatalf("first add: %v", err)
	}

	os.WriteFile(src, []byte("v2"), 0o644)
	_, _, err := s.AddPiExtension(src, false)
	if err == nil {
		t.Fatal("expected error on duplicate without force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q", err)
	}
}

func TestAddPiExtensionOverwriteWithForce(t *testing.T) {
	s := setupStore(t)
	src := filepath.Join(t.TempDir(), "ext.ts")
	os.WriteFile(src, []byte("v1"), 0o644)

	if _, _, err := s.AddPiExtension(src, false); err != nil {
		t.Fatalf("first add: %v", err)
	}

	os.WriteFile(src, []byte("v2"), 0o644)
	_, overwritten, err := s.AddPiExtension(src, true)
	if err != nil {
		t.Fatalf("AddPiExtension with force: %v", err)
	}
	if !overwritten {
		t.Error("expected overwritten=true")
	}

	data, _ := os.ReadFile(filepath.Join(s.PiExtensionsDir(), "ext.ts"))
	if string(data) != "v2" {
		t.Errorf("content = %q, want v2", data)
	}
}

func TestAddPiExtensionCreatesDirOnDemand(t *testing.T) {
	s := setupStore(t)
	os.RemoveAll(s.PiExtensionsDir())

	src := filepath.Join(t.TempDir(), "ext.ts")
	os.WriteFile(src, []byte("content"), 0o644)

	name, _, err := s.AddPiExtension(src, false)
	if err != nil {
		t.Fatalf("AddPiExtension: %v", err)
	}
	if name != "ext" {
		t.Errorf("name = %q", name)
	}

	info, err := os.Stat(s.PiExtensionsDir())
	if err != nil || !info.IsDir() {
		t.Error("pi_extensions/ dir was not created")
	}
}
