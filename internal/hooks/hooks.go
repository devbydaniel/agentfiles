// Package hooks handles parsing, merging, and converting hook definitions
// for deployment into Claude Code, Codex, and Cursor settings files.
package hooks

import (
	"encoding/json"
	"fmt"
)

// HookFile is the canonical store format for a hook definition.
// Stored as JSON in store/hooks/<name>.json.
type HookFile struct {
	Event   string            `json:"event"`
	Matcher string            `json:"matcher,omitempty"`
	Hooks   []json.RawMessage `json:"hooks"`
}

// Parse reads and validates a hook file from raw JSON bytes.
func Parse(data []byte) (*HookFile, error) {
	var hf HookFile
	if err := json.Unmarshal(data, &hf); err != nil {
		return nil, fmt.Errorf("parsing hook file: %w", err)
	}
	if hf.Event == "" {
		return nil, fmt.Errorf("hook file missing required field \"event\"")
	}
	if len(hf.Hooks) == 0 {
		return nil, fmt.Errorf("hook file missing required field \"hooks\" (must be a non-empty array)")
	}
	return &hf, nil
}
