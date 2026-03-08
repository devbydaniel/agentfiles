package layout

import (
	"path/filepath"
)

// AllLayout produces paths for all supported layouts combined.
// It uses PiLayout as the primary layout and creates pointer/symlink
// entries for the others.
type AllLayout struct {
	pi     PiLayout
	claude ClaudeLayout
	cursor CursorLayout
}

func (AllLayout) Name() string { return "all" }

// AgentMdPath returns the primary agent file path (AGENTS.md).
func (a AllLayout) AgentMdPath() string { return a.pi.AgentMdPath() }

// SkillPath returns the primary skill path (pi layout).
func (a AllLayout) SkillPath(name string) string { return a.pi.SkillPath(name) }

// PluginPath returns the primary plugin path (pi layout).
func (a AllLayout) PluginPath(name string) string { return a.pi.PluginPath(name) }

// AgentMdEntries returns entries for the agent file across all layouts.
// The pi entry is regular; CLAUDE.md is a pointer; .cursorrules is regular.
func (a AllLayout) AgentMdEntries() []Entry {
	return []Entry{
		{Path: a.pi.AgentMdPath(), Kind: KindRegular},
		{Path: a.claude.AgentMdPath(), Kind: KindPointer, Target: "@AGENTS.md"},
		{Path: a.cursor.AgentMdPath(), Kind: KindRegular},
	}
}

// SkillEntries returns entries for a skill across all layouts.
// The pi entry is regular; .claude/skills/<name> symlinks to .pi/skills/<name>.
// Cursor gets a regular entry.
func (a AllLayout) SkillEntries(name string) []Entry {
	piPath := a.pi.SkillPath(name)
	claudePath := a.claude.SkillPath(name)
	return []Entry{
		{Path: piPath, Kind: KindRegular},
		{Path: claudePath, Kind: KindSymlink, Target: relTarget(claudePath, piPath)},
		{Path: a.cursor.SkillPath(name), Kind: KindRegular},
	}
}

// PluginEntries returns entries for a plugin across all layouts.
func (a AllLayout) PluginEntries(name string) []Entry {
	piPath := a.pi.PluginPath(name)
	claudePath := a.claude.PluginPath(name)
	return []Entry{
		{Path: piPath, Kind: KindRegular},
		{Path: claudePath, Kind: KindSymlink, Target: relTarget(claudePath, piPath)},
		{Path: a.cursor.PluginPath(name), Kind: KindRegular},
	}
}

// relTarget computes the relative path from the directory containing symlinkPath to targetPath.
func relTarget(symlinkPath, targetPath string) string {
	rel, err := filepath.Rel(filepath.Dir(symlinkPath), targetPath)
	if err != nil {
		// Fallback — should never happen with well-formed paths.
		return targetPath
	}
	return rel
}
