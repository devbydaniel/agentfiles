package layout

import "fmt"

// Get returns the Layout for the given name.
// Supported names: "pi", "claude", "cursor", "all".
func Get(name string) (Layout, error) {
	switch name {
	case "pi":
		return PiLayout{}, nil
	case "claude":
		return ClaudeLayout{}, nil
	case "cursor":
		return CursorLayout{}, nil
	case "all":
		return AllLayout{}, nil
	default:
		return nil, fmt.Errorf("unknown layout: %q", name)
	}
}
