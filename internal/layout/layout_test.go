package layout

import (
	"testing"
)

func TestPiLayout(t *testing.T) {
	l := PiLayout{}
	assertEqual(t, "AGENTS.md", l.InstructionMdPath())
	assertEqual(t, ".pi/skills/browse", l.SkillPath("browse"))
}

func TestClaudeLayout(t *testing.T) {
	l := ClaudeLayout{}
	assertEqual(t, "CLAUDE.md", l.InstructionMdPath())
	assertEqual(t, ".claude/skills/browse", l.SkillPath("browse"))
}

func TestCursorLayout(t *testing.T) {
	l := CursorLayout{}
	assertEqual(t, "AGENTS.md", l.InstructionMdPath())
	assertEqual(t, ".cursor/skills/browse", l.SkillPath("browse"))
}

func TestCodexLayout(t *testing.T) {
	l := CodexLayout{}
	assertEqual(t, "AGENTS.md", l.InstructionMdPath())
	assertEqual(t, ".agents/browse", l.SkillPath("browse"))
}

func TestGet(t *testing.T) {
	for _, name := range []string{"pi", "claude", "cursor", "codex", "all"} {
		l, err := Get(name)
		if err != nil {
			t.Fatalf("Get(%q) returned error: %v", name, err)
		}
		if l.Name() != name {
			t.Fatalf("Get(%q).Name() = %q", name, l.Name())
		}
	}

	_, err := Get("invalid")
	if err == nil {
		t.Fatal("Get(\"invalid\") should return error")
	}
}

func TestAllLayout(t *testing.T) {
	l := AllLayout{}

	// Primary paths use pi layout.
	assertEqual(t, "AGENTS.md", l.InstructionMdPath())
	assertEqual(t, ".pi/skills/browse", l.SkillPath("browse"))
}

func TestAllLayoutInstructionMdEntries(t *testing.T) {
	l := AllLayout{}
	entries := l.InstructionMdEntries()

	// AGENTS.md is shared by pi, cursor, and codex — deduplicated to 2 entries.
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (AGENTS.md deduplicated), got %d", len(entries))
	}

	assertEntryPath(t, entries[0], "AGENTS.md")
	assertEntryPath(t, entries[1], "CLAUDE.md")
}

func TestAllLayoutSkillEntries(t *testing.T) {
	l := AllLayout{}
	entries := l.SkillEntries("browse")

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	assertEntryPath(t, entries[0], ".pi/skills/browse")
	assertEntryPath(t, entries[1], ".claude/skills/browse")
	assertEntryPath(t, entries[2], ".cursor/skills/browse")
	assertEntryPath(t, entries[3], ".agents/browse")
}

func TestUserPiLayout(t *testing.T) {
	l := UserPiLayout{}
	assertEqual(t, "AGENTS.md", l.InstructionMdPath())
	assertEqual(t, ".pi/agent/skills/browse", l.SkillPath("browse"))
}

func TestUserClaudeLayout(t *testing.T) {
	l := UserClaudeLayout{}
	assertEqual(t, ".claude/CLAUDE.md", l.InstructionMdPath())
	assertEqual(t, ".claude/skills/browse", l.SkillPath("browse"))
}

func TestUserCursorLayout(t *testing.T) {
	l := UserCursorLayout{}
	assertEqual(t, ".cursor/rules/agentfiles.md", l.InstructionMdPath())
	assertEqual(t, ".cursor/skills/browse", l.SkillPath("browse"))
}

func TestUserCodexLayout(t *testing.T) {
	l := UserCodexLayout{}
	assertEqual(t, "AGENTS.md", l.InstructionMdPath())
	assertEqual(t, ".agents/browse", l.SkillPath("browse"))
}

func TestGetUser(t *testing.T) {
	for _, name := range []string{"pi", "claude", "cursor", "codex", "all"} {
		l, err := GetUser(name)
		if err != nil {
			t.Fatalf("GetUser(%q) returned error: %v", name, err)
		}
		if l == nil {
			t.Fatalf("GetUser(%q) returned nil", name)
		}
	}

	_, err := GetUser("invalid")
	if err == nil {
		t.Fatal("GetUser(\"invalid\") should return error")
	}
}

func TestUserAllLayout(t *testing.T) {
	l := UserAllLayout{}

	// Primary paths use pi layout.
	assertEqual(t, "AGENTS.md", l.InstructionMdPath())
	assertEqual(t, ".pi/agent/skills/browse", l.SkillPath("browse"))
}

func TestUserAllLayoutInstructionMdEntries(t *testing.T) {
	l := UserAllLayout{}
	entries := l.InstructionMdEntries()

	// AGENTS.md is shared by pi and codex — deduplicated to 3 entries.
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	assertEntryPath(t, entries[0], "AGENTS.md")
	assertEntryPath(t, entries[1], ".claude/CLAUDE.md")
	assertEntryPath(t, entries[2], ".cursor/rules/agentfiles.md")
}

func TestUserAllLayoutSkillEntries(t *testing.T) {
	l := UserAllLayout{}
	entries := l.SkillEntries("browse")

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	assertEntryPath(t, entries[0], ".pi/agent/skills/browse")
	assertEntryPath(t, entries[1], ".claude/skills/browse")
	assertEntryPath(t, entries[2], ".cursor/skills/browse")
	assertEntryPath(t, entries[3], ".agents/browse")
}

func assertEqual(t *testing.T, want, got string) {
	t.Helper()
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func assertEntryPath(t *testing.T, e Entry, path string) {
	t.Helper()
	if e.Path != path {
		t.Errorf("entry path: want %q, got %q", path, e.Path)
	}
}
