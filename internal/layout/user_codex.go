package layout

import "path/filepath"

// UserCodexLayout places user-level files according to the OpenAI Codex convention.
// Paths are relative to $HOME (the caller passes home as the deploy root).
type UserCodexLayout struct{}

func (UserCodexLayout) Name() string              { return "user-codex" }
func (UserCodexLayout) InstructionMdPath() string { return "AGENTS.md" }
func (UserCodexLayout) SkillPath(name string) string {
	return filepath.Join(".agents", "skills", name)
}

func (l UserCodexLayout) InstructionMdEntries() []Entry {
	return []Entry{{Path: l.InstructionMdPath()}}
}
func (l UserCodexLayout) SkillEntries(name string) []Entry {
	return []Entry{{Path: l.SkillPath(name)}}
}
func (UserCodexLayout) AgentEntries(name string) []Entry {
	return []Entry{{Path: filepath.Join(".codex", "agents", name+".toml")}}
}
func (UserCodexLayout) PiExtensionEntries(name string) []Entry {
	return nil
}
