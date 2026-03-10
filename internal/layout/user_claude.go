package layout

import "path/filepath"

// UserClaudeLayout places user-level files according to the Claude Code convention.
// Paths are relative to $HOME (the caller passes home as the deploy root).
type UserClaudeLayout struct{}

func (UserClaudeLayout) Name() string { return "user-claude" }
func (UserClaudeLayout) AgentMdPath() string {
	return filepath.Join(".claude", "CLAUDE.md")
}
func (UserClaudeLayout) SkillPath(name string) string {
	return filepath.Join(".claude", "skills", name)
}

func (l UserClaudeLayout) AgentMdEntries() []Entry {
	return []Entry{{Path: l.AgentMdPath()}}
}
func (l UserClaudeLayout) SkillEntries(name string) []Entry {
	return []Entry{{Path: l.SkillPath(name)}}
}
