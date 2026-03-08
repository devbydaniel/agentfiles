package layout

import "path/filepath"

// CursorLayout places files according to the Cursor convention.
type CursorLayout struct{}

func (CursorLayout) Name() string                  { return "cursor" }
func (CursorLayout) AgentMdPath() string            { return ".cursorrules" }
func (CursorLayout) SkillPath(name string) string   { return filepath.Join(".cursor", "skills", name) }
func (CursorLayout) PluginPath(name string) string  { return filepath.Join(".cursor", "plugins", name) }

func (l CursorLayout) AgentMdEntries() []Entry    { return []Entry{{Path: l.AgentMdPath(), Kind: KindRegular}} }
func (l CursorLayout) SkillEntries(name string) []Entry  { return []Entry{{Path: l.SkillPath(name), Kind: KindRegular}} }
func (l CursorLayout) PluginEntries(name string) []Entry { return []Entry{{Path: l.PluginPath(name), Kind: KindRegular}} }
