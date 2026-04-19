// Package hooks handles parsing, merging, and converting hook definitions
// for deployment into Claude Code, Codex, and Cursor settings files.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// HookJSONFilename is the canonical filename used inside directory-form hooks.
const HookJSONFilename = "hook.json"

// ScriptsDirname is the conventional subdirectory name inside directory-form
// hooks that holds script files referenced via ${AF_HOOK_ROOT}/scripts/....
const ScriptsDirname = "scripts"

// HookFile is the canonical store format for a hook definition.
// Stored as JSON in store/hooks/<name>.json (file form) or
// store/hooks/<name>/hook.json (directory form).
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

// LoadFromDir reads a directory-form hook: a directory containing hook.json
// and an optional scripts/ subdirectory. Returns the parsed hook and a
// hasScripts flag that is true when scripts/ exists as a directory.
func LoadFromDir(dir string) (*HookFile, bool, error) {
	hookPath := filepath.Join(dir, HookJSONFilename)
	data, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, fmt.Errorf("hook directory %q is missing %s", dir, HookJSONFilename)
		}
		return nil, false, fmt.Errorf("reading %s: %w", hookPath, err)
	}
	hf, err := Parse(data)
	if err != nil {
		return nil, false, err
	}

	hasScripts := false
	scriptsPath := filepath.Join(dir, ScriptsDirname)
	if info, err := os.Stat(scriptsPath); err == nil && info.IsDir() {
		hasScripts = true
	}

	return hf, hasScripts, nil
}
