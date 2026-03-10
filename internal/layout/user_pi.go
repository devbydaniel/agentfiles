package layout

import "path/filepath"

// UserPiLayout places user-level files according to the pi convention.
// Paths are relative to $HOME (the caller passes home as the deploy root).
type UserPiLayout struct{}

func (UserPiLayout) Name() string                  { return "user-pi" }
func (UserPiLayout) AgentMdPath() string           { return "AGENTS.md" }
func (UserPiLayout) SkillPath(name string) string  { return filepath.Join(".pi", "agent", "skills", name) }
func (UserPiLayout) PluginPath(name string) string { return filepath.Join(".pi", "agent", "plugins", name) }

func (l UserPiLayout) AgentMdEntries() []Entry {
	return []Entry{{Path: l.AgentMdPath()}}
}
func (l UserPiLayout) SkillEntries(name string) []Entry {
	return []Entry{{Path: l.SkillPath(name)}}
}
func (l UserPiLayout) PluginEntries(name string) []Entry {
	return []Entry{{Path: l.PluginPath(name)}}
}
