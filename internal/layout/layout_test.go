package layout

import (
	"testing"
)

func TestPiLayout(t *testing.T) {
	l := PiLayout{}
	assertEqual(t, "AGENTS.md", l.AgentMdPath())
	assertEqual(t, ".pi/skills/browse", l.SkillPath("browse"))
	assertEqual(t, ".pi/plugins/myplugin", l.PluginPath("myplugin"))
}

func TestClaudeLayout(t *testing.T) {
	l := ClaudeLayout{}
	assertEqual(t, "CLAUDE.md", l.AgentMdPath())
	assertEqual(t, ".claude/skills/browse", l.SkillPath("browse"))
	assertEqual(t, ".claude/plugins/myplugin", l.PluginPath("myplugin"))
}

func TestCursorLayout(t *testing.T) {
	l := CursorLayout{}
	assertEqual(t, ".cursorrules", l.AgentMdPath())
	assertEqual(t, ".cursor/skills/browse", l.SkillPath("browse"))
	assertEqual(t, ".cursor/plugins/myplugin", l.PluginPath("myplugin"))
}

func TestGet(t *testing.T) {
	for _, name := range []string{"pi", "claude", "cursor", "all"} {
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
	assertEqual(t, "AGENTS.md", l.AgentMdPath())
	assertEqual(t, ".pi/skills/browse", l.SkillPath("browse"))
	assertEqual(t, ".pi/plugins/myplugin", l.PluginPath("myplugin"))
}

func TestAllLayoutAgentMdEntries(t *testing.T) {
	l := AllLayout{}
	entries := l.AgentMdEntries()

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	assertEntryPath(t, entries[0], "AGENTS.md")
	assertEntryPath(t, entries[1], "CLAUDE.md")
	assertEntryPath(t, entries[2], ".cursorrules")
}

func TestAllLayoutSkillEntries(t *testing.T) {
	l := AllLayout{}
	entries := l.SkillEntries("browse")

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	assertEntryPath(t, entries[0], ".pi/skills/browse")
	assertEntryPath(t, entries[1], ".claude/skills/browse")
	assertEntryPath(t, entries[2], ".cursor/skills/browse")
}

func TestAllLayoutPluginEntries(t *testing.T) {
	l := AllLayout{}
	entries := l.PluginEntries("myplugin")

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	assertEntryPath(t, entries[0], ".pi/plugins/myplugin")
	assertEntryPath(t, entries[1], ".claude/plugins/myplugin")
	assertEntryPath(t, entries[2], ".cursor/plugins/myplugin")
}

func TestUserPiLayout(t *testing.T) {
	l := UserPiLayout{}
	assertEqual(t, "AGENTS.md", l.AgentMdPath())
	assertEqual(t, ".pi/skills/browse", l.SkillPath("browse"))
	assertEqual(t, ".pi/plugins/myplugin", l.PluginPath("myplugin"))
}

func TestUserClaudeLayout(t *testing.T) {
	l := UserClaudeLayout{}
	assertEqual(t, ".claude/CLAUDE.md", l.AgentMdPath())
	assertEqual(t, ".claude/skills/browse", l.SkillPath("browse"))
	assertEqual(t, ".claude/plugins/myplugin", l.PluginPath("myplugin"))
}

func TestUserCursorLayout(t *testing.T) {
	l := UserCursorLayout{}
	assertEqual(t, ".cursor/rules/agentfiles.md", l.AgentMdPath())
	assertEqual(t, ".cursor/skills/browse", l.SkillPath("browse"))
	assertEqual(t, ".cursor/plugins/myplugin", l.PluginPath("myplugin"))
}

func TestGetUser(t *testing.T) {
	for _, name := range []string{"pi", "claude", "cursor", "all"} {
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
	assertEqual(t, "AGENTS.md", l.AgentMdPath())
	assertEqual(t, ".pi/skills/browse", l.SkillPath("browse"))
	assertEqual(t, ".pi/plugins/myplugin", l.PluginPath("myplugin"))
}

func TestUserAllLayoutAgentMdEntries(t *testing.T) {
	l := UserAllLayout{}
	entries := l.AgentMdEntries()

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

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	assertEntryPath(t, entries[0], ".pi/skills/browse")
	assertEntryPath(t, entries[1], ".claude/skills/browse")
	assertEntryPath(t, entries[2], ".cursor/skills/browse")
}

func TestUserAllLayoutPluginEntries(t *testing.T) {
	l := UserAllLayout{}
	entries := l.PluginEntries("myplugin")

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	assertEntryPath(t, entries[0], ".pi/plugins/myplugin")
	assertEntryPath(t, entries[1], ".claude/plugins/myplugin")
	assertEntryPath(t, entries[2], ".cursor/plugins/myplugin")
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
