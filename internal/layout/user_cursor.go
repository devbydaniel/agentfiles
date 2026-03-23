package layout

import "path/filepath"

// UserCursorLayout places user-level files according to the Cursor convention.
// Paths are relative to $HOME (the caller passes home as the deploy root).
// The instruction md goes to .cursor/rules/agentfiles.md (Cursor's user-level rules).
type UserCursorLayout struct{}

func (UserCursorLayout) Name() string { return "user-cursor" }
func (UserCursorLayout) InstructionMdPath() string {
	return filepath.Join(".cursor", "rules", "agentfiles.md")
}
func (UserCursorLayout) SkillPath(name string) string {
	return filepath.Join(".cursor", "skills", name)
}

func (l UserCursorLayout) InstructionMdEntries() []Entry {
	return []Entry{{Path: l.InstructionMdPath()}}
}
func (l UserCursorLayout) SkillEntries(name string) []Entry {
	return []Entry{{Path: l.SkillPath(name)}}
}
func (UserCursorLayout) AgentEntries(name string) []Entry {
	return []Entry{{Path: filepath.Join(".cursor", "agents", name+".md")}}
}
