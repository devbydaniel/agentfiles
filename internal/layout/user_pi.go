package layout

import "path/filepath"

// UserPiLayout places user-level files according to the pi convention.
// Paths are relative to $HOME (the caller passes home as the deploy root).
type UserPiLayout struct{}

func (UserPiLayout) Name() string              { return "user-pi" }
func (UserPiLayout) InstructionMdPath() string { return "AGENTS.md" }
func (UserPiLayout) SkillPath(name string) string {
	return filepath.Join(".pi", "agent", "skills", name)
}

func (l UserPiLayout) InstructionMdEntries() []Entry {
	return []Entry{{Path: l.InstructionMdPath()}}
}
func (l UserPiLayout) SkillEntries(name string) []Entry {
	return []Entry{{Path: l.SkillPath(name)}}
}
