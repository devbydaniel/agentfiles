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
	mustRecord(t, lf, AssetAgentsMD, "", "agents/ayunis-core.md", "abc123")
	mustRecord(t, lf, AssetSkills, "browse", "skills/browse/", "def456")
	mustRecord(t, lf, AssetSkills, "git-workflow", "skills/git-workflow/", "ghi789")

	if err := Save(dir, lf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Deployed.AgentsMD == nil {
		t.Fatal("agents_md entry missing")
	}
	if got.Deployed.AgentsMD.Source != "agents/ayunis-core.md" || got.Deployed.AgentsMD.Hash != "abc123" {
		t.Errorf("agents_md mismatch: %+v", got.Deployed.AgentsMD)
	}

	for _, name := range []string{"browse", "git-workflow"} {
		e, ok := got.Deployed.Skills[name]
		if !ok {
			t.Errorf("skill %q missing", name)
			continue
		}
		if name == "browse" && (e.Source != "skills/browse/" || e.Hash != "def456") {
			t.Errorf("browse mismatch: %+v", e)
		}
		if name == "git-workflow" && (e.Source != "skills/git-workflow/" || e.Hash != "ghi789") {
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
	if lf.Deployed.AgentsMD != nil {
		t.Error("expected nil agents_md")
	}
	if len(lf.Deployed.Skills) != 0 {
		t.Error("expected empty skills map")
	}
}

func TestRecordUnknownAssetType(t *testing.T) {
	lf := &LockFile{}
	if err := lf.Record("bogus", "", "", ""); err == nil {
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

func mustRecord(t *testing.T, lf *LockFile, assetType, name, source, hash string) {
	t.Helper()
	if err := lf.Record(assetType, name, source, hash); err != nil {
		t.Fatalf("Record(%q, %q): %v", assetType, name, err)
	}
}
