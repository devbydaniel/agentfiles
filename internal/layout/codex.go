package layout

import "path/filepath"

// CodexLayout places files according to the OpenAI Codex convention.
type CodexLayout struct{}

func (CodexLayout) Name() string                 { return "codex" }
func (CodexLayout) InstructionMdPath() string    { return "AGENTS.md" }
func (CodexLayout) SkillPath(name string) string { return filepath.Join(".agents", name) }

func (l CodexLayout) InstructionMdEntries() []Entry {
	return []Entry{{Path: l.InstructionMdPath()}}
}
func (l CodexLayout) SkillEntries(name string) []Entry {
	return []Entry{{Path: l.SkillPath(name)}}
}
func (CodexLayout) AgentEntries(name string) []Entry {
	return []Entry{{Path: filepath.Join(".codex", "agents", name+".toml")}}
}
