package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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
		"skills":       s.SkillsDir(),
		"instructions": s.InstructionsDir(),
		"resources":    s.ResourcesDir(),
		"bundles":      s.BundlesDir(),
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

// --- Skill discovery tests (Step 1) ---

// createSkill creates a minimal skill directory with SKILL.md.
func createSkill(t *testing.T, s *Store, groupPath string) {
	t.Helper()
	dir := filepath.Join(s.SkillsDir(), filepath.FromSlash(groupPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+filepath.Base(dir)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListSkillsFlat(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "browse")

	skills, err := s.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].GroupPath != "browse" || skills[0].LeafName != "browse" {
		t.Errorf("got %+v, want {GroupPath:browse LeafName:browse}", skills[0])
	}
}

func TestListSkillsGrouped(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "tooling/browse")
	createSkill(t, s, "ayunis/backend")

	skills, err := s.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].GroupPath < skills[j].GroupPath })
	if skills[0].GroupPath != "ayunis/backend" || skills[0].LeafName != "backend" {
		t.Errorf("skills[0] = %+v", skills[0])
	}
	if skills[1].GroupPath != "tooling/browse" || skills[1].LeafName != "browse" {
		t.Errorf("skills[1] = %+v", skills[1])
	}
}

func TestListSkillsDeeplyNested(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "infra/aws/deploy")

	skills, err := s.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].GroupPath != "infra/aws/deploy" || skills[0].LeafName != "deploy" {
		t.Errorf("got %+v", skills[0])
	}
}

func TestListSkillsStopsAtSkillDir(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "browse")
	// Add a scripts/ subdirectory inside the skill — should NOT be treated as a nested skill.
	scriptsDir := filepath.Join(s.SkillsDir(), "browse", "scripts")
	os.MkdirAll(scriptsDir, 0o755)
	os.WriteFile(filepath.Join(scriptsDir, "helper.sh"), []byte("#!/bin/bash"), 0o755)

	skills, err := s.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1 (scripts/ should not be a skill)", len(skills))
	}
}

func TestListSkillsIgnoresGroupDirsWithoutSkillMD(t *testing.T) {
	s := setupStore(t)
	// Create a group directory with no SKILL.md — not a skill.
	os.MkdirAll(filepath.Join(s.SkillsDir(), "tooling"), 0o755)
	// But a skill inside the group.
	createSkill(t, s, "tooling/browse")

	skills, err := s.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].GroupPath != "tooling/browse" {
		t.Errorf("got %+v", skills[0])
	}
}

func TestListSkillsEmpty(t *testing.T) {
	s := setupStore(t)
	skills, err := s.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}

func TestResolveSkillBare(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "tooling/browse")

	info, err := s.ResolveSkill("browse")
	if err != nil {
		t.Fatalf("ResolveSkill: %v", err)
	}
	if info.GroupPath != "tooling/browse" || info.LeafName != "browse" {
		t.Errorf("got %+v", info)
	}
}

func TestResolveSkillQualified(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "tooling/browse")

	info, err := s.ResolveSkill("tooling/browse")
	if err != nil {
		t.Fatalf("ResolveSkill: %v", err)
	}
	if info.GroupPath != "tooling/browse" || info.LeafName != "browse" {
		t.Errorf("got %+v", info)
	}
}

func TestResolveSkillFlatBare(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "browse")

	info, err := s.ResolveSkill("browse")
	if err != nil {
		t.Fatalf("ResolveSkill: %v", err)
	}
	if info.GroupPath != "browse" || info.LeafName != "browse" {
		t.Errorf("got %+v", info)
	}
}

func TestResolveSkillAmbiguous(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "a/browse")
	createSkill(t, s, "b/browse")

	_, err := s.ResolveSkill("browse")
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error = %q, want 'ambiguous'", err)
	}
}

func TestResolveSkillNotFound(t *testing.T) {
	s := setupStore(t)

	_, err := s.ResolveSkill("nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

func TestResolveSkillQualifiedNotFound(t *testing.T) {
	s := setupStore(t)

	_, err := s.ResolveSkill("tooling/nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

// --- Glob expansion tests (Step 2) ---

func TestExpandSkillGlobNonGlob(t *testing.T) {
	s := setupStore(t)
	result, err := s.ExpandSkillGlob("browse")
	if err != nil {
		t.Fatalf("ExpandSkillGlob: %v", err)
	}
	if len(result) != 1 || result[0] != "browse" {
		t.Errorf("got %v, want [browse]", result)
	}
}

func TestExpandSkillGlobSingleGroup(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "ayunis/backend")
	createSkill(t, s, "ayunis/frontend")
	createSkill(t, s, "tooling/browse")

	result, err := s.ExpandSkillGlob("ayunis/")
	if err != nil {
		t.Fatalf("ExpandSkillGlob: %v", err)
	}
	sort.Strings(result)
	if len(result) != 2 || result[0] != "ayunis/backend" || result[1] != "ayunis/frontend" {
		t.Errorf("got %v", result)
	}
}

func TestExpandSkillGlobRecursive(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "infra/aws/deploy")
	createSkill(t, s, "infra/gcp/deploy")
	createSkill(t, s, "infra/monitoring")

	result, err := s.ExpandSkillGlob("infra/")
	if err != nil {
		t.Fatalf("ExpandSkillGlob: %v", err)
	}
	sort.Strings(result)
	if len(result) != 3 {
		t.Fatalf("got %d results, want 3: %v", len(result), result)
	}
	if result[0] != "infra/aws/deploy" || result[1] != "infra/gcp/deploy" || result[2] != "infra/monitoring" {
		t.Errorf("got %v", result)
	}
}

func TestExpandSkillGlobNestedPrefix(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "infra/aws/deploy")
	createSkill(t, s, "infra/gcp/deploy")

	result, err := s.ExpandSkillGlob("infra/aws/")
	if err != nil {
		t.Fatalf("ExpandSkillGlob: %v", err)
	}
	if len(result) != 1 || result[0] != "infra/aws/deploy" {
		t.Errorf("got %v", result)
	}
}

func TestExpandSkillGlobNoMatch(t *testing.T) {
	s := setupStore(t)
	createSkill(t, s, "tooling/browse")

	_, err := s.ExpandSkillGlob("nonexistent/")
	if err == nil {
		t.Fatal("expected error for non-matching glob")
	}
	if !strings.Contains(err.Error(), "no skills found") {
		t.Errorf("error = %q", err)
	}
}

// --- Agent tests ---

func TestInitCreatesAgentsDir(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(filepath.Join(dir, "store"))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	info, err := os.Stat(s.AgentsDir())
	if err != nil || !info.IsDir() {
		t.Errorf("agents/ directory missing after Init")
	}
}

func TestOpenWithoutAgentsDir(t *testing.T) {
	// Create a store, then remove agents/ — Open should still work.
	dir := t.TempDir()
	storePath := filepath.Join(dir, "store")
	s, err := Init(storePath)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	os.RemoveAll(s.AgentsDir())

	s2, err := Open(storePath)
	if err != nil {
		t.Fatalf("Open should succeed without agents/: %v", err)
	}
	if s2 == nil {
		t.Fatal("Open returned nil store")
	}
}

func TestListAgentsEmpty(t *testing.T) {
	s := setupStore(t)
	agents, err := s.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestListAgentsNoDir(t *testing.T) {
	s := setupStore(t)
	os.RemoveAll(s.AgentsDir())
	agents, err := s.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestListAgentsMultiple(t *testing.T) {
	s := setupStore(t)
	os.WriteFile(filepath.Join(s.AgentsDir(), "reviewer.md"), []byte("# Reviewer"), 0o644)
	os.WriteFile(filepath.Join(s.AgentsDir(), "debugger.md"), []byte("# Debugger"), 0o644)

	agents, err := s.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	names := map[string]bool{}
	for _, a := range agents {
		names[a.Name] = true
	}
	if !names["reviewer"] || !names["debugger"] {
		t.Errorf("unexpected agents: %v", agents)
	}
}

func TestListAgentsIgnoresNonMD(t *testing.T) {
	s := setupStore(t)
	os.WriteFile(filepath.Join(s.AgentsDir(), "reviewer.md"), []byte("# Reviewer"), 0o644)
	os.WriteFile(filepath.Join(s.AgentsDir(), "notes.txt"), []byte("notes"), 0o644)
	os.WriteFile(filepath.Join(s.AgentsDir(), ".hidden.md"), []byte("hidden"), 0o644)
	os.MkdirAll(filepath.Join(s.AgentsDir(), "subdir"), 0o755)

	agents, err := s.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "reviewer" {
		t.Errorf("name = %q, want reviewer", agents[0].Name)
	}
}

func TestExpandSkillGlobEmptyStore(t *testing.T) {
	s := setupStore(t)

	_, err := s.ExpandSkillGlob("anything/")
	if err == nil {
		t.Fatal("expected error for empty store glob")
	}
}

// --- PiExtension tests ---

func TestInitCreatesPiExtensionsDir(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(filepath.Join(dir, "store"))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	info, err := os.Stat(s.PiExtensionsDir())
	if err != nil || !info.IsDir() {
		t.Errorf("pi_extensions/ directory missing after Init")
	}
}

func TestOpenWithoutPiExtensionsDir(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "store")
	s, err := Init(storePath)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	os.RemoveAll(s.PiExtensionsDir())

	s2, err := Open(storePath)
	if err != nil {
		t.Fatalf("Open should succeed without pi_extensions/: %v", err)
	}
	if s2 == nil {
		t.Fatal("Open returned nil store")
	}
}

func TestListPiExtensionsEmpty(t *testing.T) {
	s := setupStore(t)
	exts, err := s.ListPiExtensions()
	if err != nil {
		t.Fatalf("ListPiExtensions: %v", err)
	}
	if len(exts) != 0 {
		t.Errorf("expected 0 pi_extensions, got %d", len(exts))
	}
}

func TestListPiExtensionsNoDir(t *testing.T) {
	s := setupStore(t)
	os.RemoveAll(s.PiExtensionsDir())
	exts, err := s.ListPiExtensions()
	if err != nil {
		t.Fatalf("ListPiExtensions: %v", err)
	}
	if len(exts) != 0 {
		t.Errorf("expected 0 pi_extensions, got %d", len(exts))
	}
}

func TestListPiExtensionsSingleFile(t *testing.T) {
	s := setupStore(t)
	os.WriteFile(filepath.Join(s.PiExtensionsDir(), "no-model-flag.ts"), []byte("export default {}"), 0o644)

	exts, err := s.ListPiExtensions()
	if err != nil {
		t.Fatalf("ListPiExtensions: %v", err)
	}
	if len(exts) != 1 {
		t.Fatalf("expected 1 pi_extension, got %d", len(exts))
	}
	if exts[0].Name != "no-model-flag" || exts[0].IsDir {
		t.Errorf("got %+v, want {Name:no-model-flag IsDir:false}", exts[0])
	}
}

func TestListPiExtensionsDirectory(t *testing.T) {
	s := setupStore(t)
	extDir := filepath.Join(s.PiExtensionsDir(), "my-ext")
	os.MkdirAll(extDir, 0o755)
	os.WriteFile(filepath.Join(extDir, "index.ts"), []byte("export default {}"), 0o644)

	exts, err := s.ListPiExtensions()
	if err != nil {
		t.Fatalf("ListPiExtensions: %v", err)
	}
	if len(exts) != 1 {
		t.Fatalf("expected 1 pi_extension, got %d", len(exts))
	}
	if exts[0].Name != "my-ext" || !exts[0].IsDir {
		t.Errorf("got %+v, want {Name:my-ext IsDir:true}", exts[0])
	}
}

func TestListPiExtensionsMixed(t *testing.T) {
	s := setupStore(t)
	// Single file extension.
	os.WriteFile(filepath.Join(s.PiExtensionsDir(), "simple.ts"), []byte("export default {}"), 0o644)
	// Directory extension.
	extDir := filepath.Join(s.PiExtensionsDir(), "complex")
	os.MkdirAll(extDir, 0o755)
	os.WriteFile(filepath.Join(extDir, "index.ts"), []byte("export default {}"), 0o644)
	// Directory without index.ts — should be ignored.
	badDir := filepath.Join(s.PiExtensionsDir(), "incomplete")
	os.MkdirAll(badDir, 0o755)
	os.WriteFile(filepath.Join(badDir, "main.ts"), []byte("nope"), 0o644)
	// Non-.ts file — should be ignored.
	os.WriteFile(filepath.Join(s.PiExtensionsDir(), "readme.md"), []byte("docs"), 0o644)
	// Hidden file — should be ignored.
	os.WriteFile(filepath.Join(s.PiExtensionsDir(), ".hidden.ts"), []byte("hidden"), 0o644)

	exts, err := s.ListPiExtensions()
	if err != nil {
		t.Fatalf("ListPiExtensions: %v", err)
	}
	if len(exts) != 2 {
		t.Fatalf("expected 2 pi_extensions, got %d: %+v", len(exts), exts)
	}

	names := map[string]bool{}
	for _, e := range exts {
		names[e.Name] = true
	}
	if !names["simple"] || !names["complex"] {
		t.Errorf("unexpected extensions: %+v", exts)
	}
}
