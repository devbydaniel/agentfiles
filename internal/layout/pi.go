package layout

import "path/filepath"

// PiLayout places files according to the pi convention.
type PiLayout struct{}

func (PiLayout) Name() string                 { return "pi" }
func (PiLayout) InstructionMdPath() string    { return "AGENTS.md" }
func (PiLayout) SkillPath(name string) string { return filepath.Join(".pi", "skills", name) }

func (l PiLayout) InstructionMdEntries() []Entry {
	return []Entry{{Path: l.InstructionMdPath()}}
}
func (l PiLayout) SkillEntries(name string) []Entry {
	return []Entry{{Path: l.SkillPath(name)}}
}
func (PiLayout) AgentEntries(name string) []Entry {
	return nil
}
