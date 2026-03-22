package layout

import "fmt"

// Get returns the Layout for the given name.
// Supported names: "pi", "claude", "cursor", "codex", "all".
func Get(name string) (Layout, error) {
	switch name {
	case "pi":
		return PiLayout{}, nil
	case "claude":
		return ClaudeLayout{}, nil
	case "cursor":
		return CursorLayout{}, nil
	case "codex":
		return CodexLayout{}, nil
	case "all":
		return AllLayout{}, nil
	default:
		return nil, fmt.Errorf("unknown layout: %q", name)
	}
}

// GetUser returns the user-level Layout for the given name.
// User layouts deploy to $HOME-relative paths (e.g. ~/.claude/CLAUDE.md).
// Supported names: "pi", "claude", "cursor", "codex", "all".
func GetUser(name string) (Layout, error) {
	switch name {
	case "pi":
		return UserPiLayout{}, nil
	case "claude":
		return UserClaudeLayout{}, nil
	case "cursor":
		return UserCursorLayout{}, nil
	case "codex":
		return UserCodexLayout{}, nil
	case "all":
		return UserAllLayout{}, nil
	default:
		return nil, fmt.Errorf("unknown user layout: %q", name)
	}
}
