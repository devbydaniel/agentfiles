// Package layout defines how agent files are placed in a repository.
package layout

// Entry describes a single file or directory to be written.
type Entry struct {
	Path string // Destination path relative to repo root.
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
