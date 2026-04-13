package hooks

import "path/filepath"

// FormatNested is the Claude/Codex hook entry format (grouped with inner hooks array).
const FormatNested = "nested"

// FormatFlat is the Cursor hook entry format (flat entries with command at top level).
const FormatFlat = "flat"

// Target describes a hooks deployment target.
type Target struct {
	Path   string // Destination path relative to repo/home root.
	Format string // FormatNested or FormatFlat.
}

var (
	claudeTarget = Target{Path: filepath.Join(".claude", "settings.json"), Format: FormatNested}
	codexTarget  = Target{Path: filepath.Join(".codex", "hooks.json"), Format: FormatNested}
	cursorTarget = Target{Path: filepath.Join(".cursor", "hooks.json"), Format: FormatFlat}
)

// AllTargets returns all possible hooks target files across all layouts.
// Used during pruning to ensure stale entries are cleaned up even after
// layout changes.
func AllTargets() []Target {
	return []Target{claudeTarget, codexTarget, cursorTarget}
}

// TargetsForLayout returns the hooks deployment targets for a layout name.
func TargetsForLayout(layoutName string) []Target {
	switch layoutName {
	case "claude", "user-claude":
		return []Target{claudeTarget}
	case "codex", "user-codex":
		return []Target{codexTarget}
	case "cursor", "user-cursor":
		return []Target{cursorTarget}
	case "all", "user-all":
		return []Target{claudeTarget, codexTarget, cursorTarget}
	default:
		return nil
	}
}
