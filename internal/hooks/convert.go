package hooks

import (
	"encoding/json"
	"fmt"
	"strings"
)

// cursorEventMap maps canonical PascalCase event names to Cursor's naming.
var cursorEventMap = map[string]string{
	"PreToolUse":       "preToolUse",
	"PostToolUse":      "postToolUse",
	"SessionStart":     "sessionStart",
	"SessionEnd":       "sessionEnd",
	"UserPromptSubmit": "beforeSubmitPrompt",
	"Stop":             "stop",
}

// EventNameForCursor maps a canonical PascalCase event name to Cursor's format.
// Known events use the explicit mapping table; unknown events get simple
// camelCase conversion (first letter lowercased).
func EventNameForCursor(event string) string {
	if mapped, ok := cursorEventMap[event]; ok {
		return mapped
	}
	if len(event) == 0 {
		return event
	}
	return strings.ToLower(event[:1]) + event[1:]
}

// ToFlatEntries converts a canonical nested HookFile to Cursor flat entries.
// Each inner hook becomes a separate JSON object with the hook's fields plus
// the parent's matcher and an _agentfiles ownership marker.
func ToFlatEntries(name string, hf *HookFile) ([]json.RawMessage, error) {
	var entries []json.RawMessage
	for _, raw := range hf.Hooks {
		// Parse the inner hook as a generic map.
		var inner map[string]any
		if err := json.Unmarshal(raw, &inner); err != nil {
			return nil, fmt.Errorf("parsing inner hook: %w", err)
		}
		// Add matcher from parent if present.
		if hf.Matcher != "" {
			inner["matcher"] = hf.Matcher
		}
		// Add ownership marker.
		inner["_agentfiles"] = name

		data, err := json.Marshal(inner)
		if err != nil {
			return nil, fmt.Errorf("marshaling flat entry: %w", err)
		}
		entries = append(entries, data)
	}
	return entries, nil
}
