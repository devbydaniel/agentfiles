package lock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAndReadBack(t *testing.T) {
	dir := t.TempDir()

	lf := &LockFile{}
	lf.Deployed.Skills = make(map[string]*Entry)
	mustRecord(t, lf, AssetInstructions, "", "instructions/ayunis-core.md", "AGENTS.md", "abc123")
	mustRecord(t, lf, AssetSkills, "browse", "skills/browse/", ".agents/skills/browse", "def456")
	mustRecord(t, lf, AssetSkills, "git-workflow", "skills/git-workflow/", ".agents/skills/git-workflow", "ghi789")

	if err := Save(dir, lf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Deployed.Instructions == nil {
		t.Fatal("instructions entry missing")
	}
	if got.Deployed.Instructions.StorePath != "instructions/ayunis-core.md" || got.Deployed.Instructions.Hash != "abc123" {
		t.Errorf("instructions mismatch: %+v", got.Deployed.Instructions)
	}

	for _, name := range []string{"browse", "git-workflow"} {
		e, ok := got.Deployed.Skills[name]
		if !ok {
			t.Errorf("skill %q missing", name)
			continue
		}
		if name == "browse" && (e.StorePath != "skills/browse/" || e.Hash != "def456") {
			t.Errorf("browse mismatch: %+v", e)
		}
		if name == "git-workflow" && (e.StorePath != "skills/git-workflow/" || e.Hash != "ghi789") {
			t.Errorf("git-workflow mismatch: %+v", e)
		}
	}
}

func TestHashFileChange(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.txt")

	os.WriteFile(p, []byte("hello"), 0644)
	h1, err := Hash(p)
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile(p, []byte("hello modified"), 0644)
	h2, err := Hash(p)
	if err != nil {
		t.Fatal(err)
	}

	if h1 == h2 {
		t.Error("hash should change when file content changes")
	}
}

func TestHashDirDeterministic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("bbb"), 0644)

	h1, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Touch files to change mtime
	time.Sleep(10 * time.Millisecond)
	now := time.Now()
	os.Chtimes(filepath.Join(dir, "a.txt"), now, now)
	os.Chtimes(filepath.Join(dir, "sub", "b.txt"), now, now)

	h2, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if h1 != h2 {
		t.Errorf("HashDir should be deterministic: %s != %s", h1, h2)
	}
}

func TestLoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	lf, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if lf.Deployed.Instructions != nil {
		t.Error("expected nil instructions")
	}
	if len(lf.Deployed.Skills) != 0 {
		t.Error("expected empty skills map")
	}
}

func TestRecordUnknownAssetType(t *testing.T) {
	lf := &LockFile{}
	if err := lf.Record(RecordParams{AssetType: "bogus"}); err == nil {
		t.Fatal("expected error for unknown asset type")
	}
}

func TestHashDirContentChange(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("bbb"), 0644)

	h1, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Change file content → hash must differ.
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa-changed"), 0644)
	h2, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Error("HashDir should change when file content changes")
	}

	// Restore content, add a new file → hash must differ from original.
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("ccc"), 0644)
	h3, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h3 {
		t.Error("HashDir should change when a file is added")
	}

	// Remove a file → hash must differ.
	os.Remove(filepath.Join(dir, "c.txt"))
	os.Remove(filepath.Join(dir, "sub", "b.txt"))
	h4, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h4 {
		t.Error("HashDir should change when a file is removed")
	}
}

func mustRecord(t *testing.T, lf *LockFile, assetType, name, source, path, hash string) {
	t.Helper()
	if err := lf.Record(RecordParams{AssetType: assetType, Name: name, SourcePath: source, DeployedPath: path, Hash: hash}); err != nil {
		t.Fatalf("Record(%q, %q): %v", assetType, name, err)
	}
}

func TestStoreFieldRoundTrip(t *testing.T) {
	dir := t.TempDir()

	lf := &LockFile{}
	lf.Deployed.Skills = make(map[string]*Entry)
	if err := lf.Record(RecordParams{AssetType: AssetSkills, Name: "my-skill", StoreName: "personal", SourcePath: "skills/my-skill/", DeployedPath: ".agents/skills/my-skill", Hash: "aaa111"}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := lf.Record(RecordParams{AssetType: AssetSkills, Name: "other-skill", StoreName: "work", SourcePath: "skills/other-skill/", DeployedPath: ".agents/skills/other-skill", Hash: "bbb222"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if err := Save(dir, lf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	e := got.Deployed.Skills["my-skill"]
	if e == nil {
		t.Fatal("my-skill missing")
	}
	if e.Store != "personal" {
		t.Errorf("my-skill.Store = %q, want %q", e.Store, "personal")
	}

	e2 := got.Deployed.Skills["other-skill"]
	if e2 == nil {
		t.Fatal("other-skill missing")
	}
	if e2.Store != "work" {
		t.Errorf("other-skill.Store = %q, want %q", e2.Store, "work")
	}
}

func TestLegacyLockNoStoreField(t *testing.T) {
	dir := t.TempDir()

	// Write a lock file without the store field (legacy format).
	legacy := `[deployed]
[deployed.skills.browse]
source = "skills/browse/"
path = ".agents/skills/browse"
hash = "def456"
`
	os.WriteFile(filepath.Join(dir, FileName), []byte(legacy), 0644)

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	e := got.Deployed.Skills["browse"]
	if e == nil {
		t.Fatal("browse skill missing")
	}
	if e.Store != "" {
		t.Errorf("Store should be empty for legacy lock, got %q", e.Store)
	}
	if e.StorePath != "skills/browse/" {
		t.Errorf("StorePath = %q, want %q", e.StorePath, "skills/browse/")
	}
}

func TestLoadFromSaveTo(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "custom", "my.lock")

	lf := &LockFile{}
	lf.Deployed.Skills = make(map[string]*Entry)
	lf.Deployed.Resources = make(map[string]*Entry)
	mustRecord(t, lf, AssetInstructions, "", "instructions/test.md", "AGENTS.md", "abc123")
	mustRecord(t, lf, AssetSkills, "browse", "skills/browse/", ".agents/skills/browse", "def456")

	// SaveTo should create parent dirs.
	if err := SaveTo(lockPath, lf); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file not created: %v", err)
	}

	// LoadFrom should read it back.
	got, err := LoadFrom(lockPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if got.Deployed.Instructions == nil {
		t.Fatal("instructions entry missing")
	}
	if got.Deployed.Instructions.Hash != "abc123" {
		t.Errorf("instructions hash = %q, want abc123", got.Deployed.Instructions.Hash)
	}

	e := got.Deployed.Skills["browse"]
	if e == nil {
		t.Fatal("browse skill missing")
	}
	if e.Hash != "def456" {
		t.Errorf("browse hash = %q, want def456", e.Hash)
	}
}

func TestAgentRecordAndLoad(t *testing.T) {
	dir := t.TempDir()

	lf := &LockFile{}
	lf.Deployed.Skills = make(map[string]*Entry)
	lf.Deployed.Resources = make(map[string]*Entry)
	lf.Deployed.Agents = make(map[string]*Entry)
	mustRecord(t, lf, AssetAgents, "code-reviewer", "agents/code-reviewer.md", ".claude/agents/code-reviewer.md", "aaa111")
	mustRecord(t, lf, AssetAgents, "debugger", "agents/debugger.md", ".codex/agents/debugger.toml", "bbb222")

	if err := Save(dir, lf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(got.Deployed.Agents) != 2 {
		t.Fatalf("Agents len = %d, want 2", len(got.Deployed.Agents))
	}

	e := got.Deployed.Agents["code-reviewer"]
	if e == nil {
		t.Fatal("code-reviewer agent missing")
	}
	if e.StorePath != "agents/code-reviewer.md" || e.Hash != "aaa111" {
		t.Errorf("code-reviewer mismatch: %+v", e)
	}
	if e.DeployedPath != ".claude/agents/code-reviewer.md" {
		t.Errorf("code-reviewer DeployedPath = %q, want .claude/agents/code-reviewer.md", e.DeployedPath)
	}

	e2 := got.Deployed.Agents["debugger"]
	if e2 == nil {
		t.Fatal("debugger agent missing")
	}
	if e2.StorePath != "agents/debugger.md" || e2.Hash != "bbb222" {
		t.Errorf("debugger mismatch: %+v", e2)
	}
}

func TestAgentStoreFieldRoundTrip(t *testing.T) {
	dir := t.TempDir()

	lf := &LockFile{}
	lf.Deployed.Skills = make(map[string]*Entry)
	lf.Deployed.Resources = make(map[string]*Entry)
	lf.Deployed.Agents = make(map[string]*Entry)
	if err := lf.Record(RecordParams{AssetType: AssetAgents, Name: "my-agent", StoreName: "work", SourcePath: "agents/my-agent.md", DeployedPath: ".claude/agents/my-agent.md", Hash: "ccc333"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if err := Save(dir, lf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	e := got.Deployed.Agents["my-agent"]
	if e == nil {
		t.Fatal("my-agent missing")
	}
	if e.Store != "work" {
		t.Errorf("Store = %q, want work", e.Store)
	}
}

func TestLegacyLockNoAgentsField(t *testing.T) {
	dir := t.TempDir()

	// Write a lock file without the agents section (legacy format).
	legacy := `[deployed]
[deployed.skills.browse]
source = "skills/browse/"
path = ".agents/skills/browse"
hash = "def456"
`
	os.WriteFile(filepath.Join(dir, FileName), []byte(legacy), 0644)

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(got.Deployed.Agents) != 0 {
		t.Errorf("Agents should be empty for legacy lock, got %d entries", len(got.Deployed.Agents))
	}
}

func TestPiExtensionRecordAndLoad(t *testing.T) {
	dir := t.TempDir()

	lf := &LockFile{}
	lf.Deployed.Skills = make(map[string]*Entry)
	lf.Deployed.Resources = make(map[string]*Entry)
	lf.Deployed.Agents = make(map[string]*Entry)
	lf.Deployed.PiExtensions = make(map[string]*Entry)
	mustRecord(t, lf, AssetPiExtensions, "no-model-flag", "pi_extensions/no-model-flag.ts", ".pi/extensions/no-model-flag.ts", "aaa111")
	mustRecord(t, lf, AssetPiExtensions, "my-ext", "pi_extensions/my-ext/", ".pi/extensions/my-ext", "bbb222")

	if err := Save(dir, lf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(got.Deployed.PiExtensions) != 2 {
		t.Fatalf("PiExtensions len = %d, want 2", len(got.Deployed.PiExtensions))
	}

	e := got.Deployed.PiExtensions["no-model-flag"]
	if e == nil {
		t.Fatal("no-model-flag pi_extension missing")
	}
	if e.StorePath != "pi_extensions/no-model-flag.ts" || e.Hash != "aaa111" {
		t.Errorf("no-model-flag mismatch: %+v", e)
	}
	if e.DeployedPath != ".pi/extensions/no-model-flag.ts" {
		t.Errorf("DeployedPath = %q", e.DeployedPath)
	}

	e2 := got.Deployed.PiExtensions["my-ext"]
	if e2 == nil {
		t.Fatal("my-ext pi_extension missing")
	}
	if e2.StorePath != "pi_extensions/my-ext/" || e2.Hash != "bbb222" {
		t.Errorf("my-ext mismatch: %+v", e2)
	}
}

func TestPiExtensionEmptyMapInit(t *testing.T) {
	dir := t.TempDir()

	// Legacy lock with no pi_extensions section.
	legacy := `[deployed]
[deployed.skills.browse]
source = "skills/browse/"
path = ".agents/skills/browse"
hash = "def456"
`
	os.WriteFile(filepath.Join(dir, FileName), []byte(legacy), 0644)

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Deployed.PiExtensions == nil {
		t.Fatal("PiExtensions map should be initialized, got nil")
	}
	if len(got.Deployed.PiExtensions) != 0 {
		t.Errorf("PiExtensions should be empty for legacy lock, got %d entries", len(got.Deployed.PiExtensions))
	}
}

func TestPiExtensionStoreFieldRoundTrip(t *testing.T) {
	dir := t.TempDir()

	lf := &LockFile{}
	lf.Deployed.PiExtensions = make(map[string]*Entry)
	if err := lf.Record(RecordParams{AssetType: AssetPiExtensions, Name: "my-ext", StoreName: "work", SourcePath: "pi_extensions/my-ext.ts", DeployedPath: ".pi/extensions/my-ext.ts", Hash: "ccc333"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if err := Save(dir, lf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	e := got.Deployed.PiExtensions["my-ext"]
	if e == nil {
		t.Fatal("my-ext missing")
	}
	if e.Store != "work" {
		t.Errorf("Store = %q, want work", e.Store)
	}
}

func TestLoadFromNonexistent(t *testing.T) {
	lf, err := LoadFrom("/nonexistent/path/lock.toml")
	if err != nil {
		t.Fatalf("LoadFrom should return empty lock for missing file, got: %v", err)
	}
	if lf.Deployed.Instructions != nil {
		t.Error("expected nil instructions")
	}
	if len(lf.Deployed.Skills) != 0 {
		t.Error("expected empty skills map")
	}
}
