package layout

import "path/filepath"

// PiLayout places files according to the pi agent convention.
type PiLayout struct{}

func (PiLayout) Name() string                 { return "pi" }
func (PiLayout) AgentMdPath() string          { return "AGENTS.md" }
func (PiLayout) SkillPath(name string) string { return filepath.Join(".pi", "skills", name) }

func (l PiLayout) AgentMdEntries() []Entry {
	return []Entry{{Path: l.AgentMdPath()}}
}
func (l PiLayout) SkillEntries(name string) []Entry {
	return []Entry{{Path: l.SkillPath(name)}}
}
