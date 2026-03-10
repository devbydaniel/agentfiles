package layout

// UserAllLayout produces user-level paths for all supported layouts combined.
// Every entry is a full copy — no pointers or symlinks.
type UserAllLayout struct {
	pi     UserPiLayout
	claude UserClaudeLayout
	cursor UserCursorLayout
}

func (UserAllLayout) Name() string { return "user-all" }

// AgentMdPath returns the primary agent file path (pi layout).
func (a UserAllLayout) AgentMdPath() string { return a.pi.AgentMdPath() }

// SkillPath returns the primary skill path (pi layout).
func (a UserAllLayout) SkillPath(name string) string { return a.pi.SkillPath(name) }

// PluginPath returns the primary plugin path (pi layout).
func (a UserAllLayout) PluginPath(name string) string { return a.pi.PluginPath(name) }

// AgentMdEntries returns entries for the agent file across all user layouts.
func (a UserAllLayout) AgentMdEntries() []Entry {
	return []Entry{
		{Path: a.pi.AgentMdPath()},
		{Path: a.claude.AgentMdPath()},
		{Path: a.cursor.AgentMdPath()},
	}
}

// SkillEntries returns entries for a skill across all user layouts.
func (a UserAllLayout) SkillEntries(name string) []Entry {
	return []Entry{
		{Path: a.pi.SkillPath(name)},
		{Path: a.claude.SkillPath(name)},
		{Path: a.cursor.SkillPath(name)},
	}
}

// PluginEntries returns entries for a plugin across all user layouts.
func (a UserAllLayout) PluginEntries(name string) []Entry {
	return []Entry{
		{Path: a.pi.PluginPath(name)},
		{Path: a.claude.PluginPath(name)},
		{Path: a.cursor.PluginPath(name)},
	}
}
