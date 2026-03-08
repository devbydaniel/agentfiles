package layout

import "path/filepath"

// ClaudeLayout places files according to the Claude Code convention.
type ClaudeLayout struct{}

func (ClaudeLayout) Name() string                  { return "claude" }
func (ClaudeLayout) AgentMdPath() string            { return "CLAUDE.md" }
func (ClaudeLayout) SkillPath(name string) string   { return filepath.Join(".claude", "skills", name) }
func (ClaudeLayout) PluginPath(name string) string  { return filepath.Join(".claude", "plugins", name) }

func (l ClaudeLayout) AgentMdEntries() []Entry    { return []Entry{{Path: l.AgentMdPath(), Kind: KindRegular}} }
func (l ClaudeLayout) SkillEntries(name string) []Entry  { return []Entry{{Path: l.SkillPath(name), Kind: KindRegular}} }
func (l ClaudeLayout) PluginEntries(name string) []Entry { return []Entry{{Path: l.PluginPath(name), Kind: KindRegular}} }
