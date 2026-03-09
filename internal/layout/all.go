package layout

// AllLayout produces paths for all supported layouts combined.
// Every entry is a full copy — no pointers or symlinks.
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
func (a AllLayout) AgentMdEntries() []Entry {
	return []Entry{
		{Path: a.pi.AgentMdPath()},
		{Path: a.claude.AgentMdPath()},
		{Path: a.cursor.AgentMdPath()},
	}
}

// SkillEntries returns entries for a skill across all layouts.
func (a AllLayout) SkillEntries(name string) []Entry {
	return []Entry{
		{Path: a.pi.SkillPath(name)},
		{Path: a.claude.SkillPath(name)},
		{Path: a.cursor.SkillPath(name)},
	}
}

// PluginEntries returns entries for a plugin across all layouts.
func (a AllLayout) PluginEntries(name string) []Entry {
	return []Entry{
		{Path: a.pi.PluginPath(name)},
		{Path: a.claude.PluginPath(name)},
		{Path: a.cursor.PluginPath(name)},
	}
}
