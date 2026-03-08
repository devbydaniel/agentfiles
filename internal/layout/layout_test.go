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

	// AGENTS.md — regular
	assertEntry(t, entries[0], "AGENTS.md", KindRegular, "")
	// CLAUDE.md — pointer to @AGENTS.md
	assertEntry(t, entries[1], "CLAUDE.md", KindPointer, "@AGENTS.md")
	// .cursorrules — regular
	assertEntry(t, entries[2], ".cursorrules", KindRegular, "")
}

func TestAllLayoutSkillEntries(t *testing.T) {
	l := AllLayout{}
	entries := l.SkillEntries("browse")

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// pi — regular
	assertEntry(t, entries[0], ".pi/skills/browse", KindRegular, "")
	// claude — symlink to pi
	assertEntry(t, entries[1], ".claude/skills/browse", KindSymlink, "../../.pi/skills/browse")
	// cursor — regular
	assertEntry(t, entries[2], ".cursor/skills/browse", KindRegular, "")
}

func TestAllLayoutPluginEntries(t *testing.T) {
	l := AllLayout{}
	entries := l.PluginEntries("myplugin")

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	assertEntry(t, entries[0], ".pi/plugins/myplugin", KindRegular, "")
	assertEntry(t, entries[1], ".claude/plugins/myplugin", KindSymlink, "../../.pi/plugins/myplugin")
	assertEntry(t, entries[2], ".cursor/plugins/myplugin", KindRegular, "")
}

func assertEqual(t *testing.T, want, got string) {
	t.Helper()
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func assertEntry(t *testing.T, e Entry, path string, kind FileKind, target string) {
	t.Helper()
	if e.Path != path {
		t.Errorf("entry path: want %q, got %q", path, e.Path)
	}
	if e.Kind != kind {
		t.Errorf("entry %q kind: want %d, got %d", path, kind, e.Kind)
	}
	if e.Target != target {
		t.Errorf("entry %q target: want %q, got %q", path, target, e.Target)
	}
}
