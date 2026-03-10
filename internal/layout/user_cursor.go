package layout

import "path/filepath"

// UserCursorLayout places user-level files according to the Cursor convention.
// Paths are relative to $HOME (the caller passes home as the deploy root).
// The agent md goes to .cursor/rules/agentfiles.md (Cursor's user-level rules).
type UserCursorLayout struct{}

func (UserCursorLayout) Name() string { return "user-cursor" }
func (UserCursorLayout) AgentMdPath() string {
	return filepath.Join(".cursor", "rules", "agentfiles.md")
}
func (UserCursorLayout) SkillPath(name string) string {
	return filepath.Join(".cursor", "skills", name)
}

func (l UserCursorLayout) AgentMdEntries() []Entry {
	return []Entry{{Path: l.AgentMdPath()}}
}
func (l UserCursorLayout) SkillEntries(name string) []Entry {
	return []Entry{{Path: l.SkillPath(name)}}
}
