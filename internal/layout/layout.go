// Package layout defines how instruction files are placed in a repository.
package layout

// Entry describes a single file or directory to be written.
type Entry struct {
	Path string // Destination path relative to repo root.
}

// Layout maps instruction-file concepts to repository paths.
type Layout interface {
	// Name returns the layout identifier (e.g. "pi", "claude").
	Name() string
	// InstructionMdPath returns the path for the top-level instructions file.
	InstructionMdPath() string
	// SkillPath returns the directory path for a named skill.
	SkillPath(name string) string
	// InstructionMdEntries returns entries for the instructions file.
	InstructionMdEntries() []Entry
	// SkillEntries returns entries for a named skill.
	SkillEntries(name string) []Entry
	// AgentEntries returns entries for a named agent.
	// Returns nil for tools that don't support agents (e.g., pi).
	AgentEntries(name string) []Entry
	// PiExtensionEntries returns entries for a named pi extension.
	// Returns nil for tools that don't support pi extensions (all except pi).
	PiExtensionEntries(name string) []Entry
}
