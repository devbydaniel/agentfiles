package layout

import "path/filepath"

// ClaudeLayout places files according to the Claude Code convention.
type ClaudeLayout struct{}

func (ClaudeLayout) Name() string                 { return "claude" }
func (ClaudeLayout) InstructionMdPath() string    { return "CLAUDE.md" }
func (ClaudeLayout) SkillPath(name string) string { return filepath.Join(".claude", "skills", name) }

func (l ClaudeLayout) InstructionMdEntries() []Entry {
	return []Entry{{Path: l.InstructionMdPath()}}
}
func (l ClaudeLayout) SkillEntries(name string) []Entry {
	return []Entry{{Path: l.SkillPath(name)}}
}
