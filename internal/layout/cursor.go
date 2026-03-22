package layout

import "path/filepath"

// CursorLayout places files according to the Cursor convention.
type CursorLayout struct{}

func (CursorLayout) Name() string                 { return "cursor" }
func (CursorLayout) InstructionMdPath() string    { return "AGENTS.md" }
func (CursorLayout) SkillPath(name string) string { return filepath.Join(".cursor", "skills", name) }

func (l CursorLayout) InstructionMdEntries() []Entry {
	return []Entry{{Path: l.InstructionMdPath()}}
}
func (l CursorLayout) SkillEntries(name string) []Entry {
	return []Entry{{Path: l.SkillPath(name)}}
}
