package hooks

import (
	"encoding/json"
	"fmt"
	"strings"
)

// HookRootPlaceholder is the token authors use in hook.json command fields to
// refer to the deployed hook root directory. Agentfiles substitutes it at
// apply time with a shell-portable string (e.g. "$HOME/.local/share/..." or a
// repo-relative path), never with a user-specific absolute path.
const HookRootPlaceholder = "${AF_HOOK_ROOT}"

// Substitute returns a copy of hf with HookRootPlaceholder replaced by hookRoot
// in each inner hook entry's "command" field. Non-string command fields and
// entries without a command are left untouched. Other placeholders are left as
// they are.
func Substitute(hf *HookFile, hookRoot string) (*HookFile, error) {
	if hf == nil {
		return nil, nil
	}

	out := &HookFile{
		Event:   hf.Event,
		Matcher: hf.Matcher,
		Hooks:   make([]json.RawMessage, 0, len(hf.Hooks)),
	}

	for i, raw := range hf.Hooks {
		var entry map[string]json.RawMessage
		if err := json.Unmarshal(raw, &entry); err != nil {
			return nil, fmt.Errorf("parsing inner hook #%d: %w", i, err)
		}

		if cmdRaw, ok := entry["command"]; ok {
			var cmd string
			if err := json.Unmarshal(cmdRaw, &cmd); err == nil {
				if strings.Contains(cmd, HookRootPlaceholder) {
					replaced := strings.ReplaceAll(cmd, HookRootPlaceholder, hookRoot)
					newCmd, err := json.Marshal(replaced)
					if err != nil {
						return nil, fmt.Errorf("marshaling substituted command #%d: %w", i, err)
					}
					entry["command"] = newCmd
				}
			}
		}

		reMarshaled, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("re-marshaling inner hook #%d: %w", i, err)
		}
		out.Hooks = append(out.Hooks, reMarshaled)
	}

	return out, nil
}
