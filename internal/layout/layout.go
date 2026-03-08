// Package layout defines how agent files are placed in a repository.
package layout

// FileKind describes special handling for a file entry.
type FileKind int

const (
	// KindRegular is a normal file written with its own content.
	KindRegular FileKind = iota
	// KindPointer is a file whose content is a reference to another file (e.g. @AGENTS.md).
	KindPointer
	// KindSymlink is a symlink to another path.
	KindSymlink
)

// Entry describes a single file to be written, with optional special handling.
type Entry struct {
	Path   string   // Destination path relative to repo root.
	Kind   FileKind // How the file should be created.
	Target string   // For KindPointer: content to write. For KindSymlink: link target.
}

// Layout maps agent-file concepts to repository paths.
type Layout interface {
	// Name returns the layout identifier (e.g. "pi", "claude").
	Name() string
	// AgentMdPath returns the path for the top-level agent instructions file.
	AgentMdPath() string
	// SkillPath returns the directory path for a named skill.
	SkillPath(name string) string
	// PluginPath returns the directory path for a named plugin.
	PluginPath(name string) string
	// AgentMdEntries returns entries for the agent instructions file.
	AgentMdEntries() []Entry
	// SkillEntries returns entries for a named skill.
	SkillEntries(name string) []Entry
	// PluginEntries returns entries for a named plugin.
	PluginEntries(name string) []Entry
}
