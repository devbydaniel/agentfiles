package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DeployBaseUser and DeployBaseRepo are the home- and repo-relative directories
// under which agentfiles deploys directory-form hook contents (used by
// internal/apply). They double as an ownership signal during merging: an entry
// whose commands all point into one of these directories is agentfiles-managed
// even when its _agentfiles marker is missing — other writers of the settings
// file (notably Claude Code) re-serialize hook entries and drop unknown fields.
//
// DeployBaseRepo must NOT live under ".agentfiles/" — that path is the repo
// manifest *file* (".agentfiles"), so ".agentfiles/hooks" collides with it and
// every repo-level apply fails with "open .agentfiles/hooks: not a directory".
// It sits beside the manifest instead, mirroring ".agentfiles.lock".
const (
	DeployBaseUser = ".local/share/agentfiles/hooks"
	DeployBaseRepo = ".agentfiles-hooks"
)

// MergeIntoSettings merges managed hooks into a settings/hooks JSON file.
//
// For Claude's settings.json it preserves all non-hooks top-level keys.
// For Codex/Cursor hooks.json files it preserves any existing structure.
//
// format controls how entries are written:
//
//	FormatNested (Claude/Codex) — inject { matcher, hooks, _agentfiles } entries
//	FormatFlat (Cursor) — flatten each inner hook to { command, type, matcher, _agentfiles }
func MergeIntoSettings(settingsPath string, managed map[string]*HookFile, format string) error {
	// Read existing file or start fresh.
	var topLevel map[string]json.RawMessage
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading %s: %w", settingsPath, err)
		}
		topLevel = make(map[string]json.RawMessage)
	} else {
		if err := json.Unmarshal(data, &topLevel); err != nil {
			return fmt.Errorf("parsing %s: %w", settingsPath, err)
		}
	}

	// Parse existing hooks section.
	existingHooks := make(map[string][]json.RawMessage)
	if raw, ok := topLevel["hooks"]; ok {
		if err := json.Unmarshal(raw, &existingHooks); err != nil {
			return fmt.Errorf("parsing hooks section in %s: %w", settingsPath, err)
		}
	}

	// Remove all previously managed entries: those with an _agentfiles field,
	// plus marker-stripped survivors whose commands identify them as ours.
	for event, entries := range existingHooks {
		var kept []json.RawMessage
		for _, entry := range entries {
			if !hasAgentfilesMarker(entry) && !isStrippedManagedEntry(entry) {
				kept = append(kept, entry)
			}
		}
		if len(kept) > 0 {
			existingHooks[event] = kept
		} else {
			delete(existingHooks, event)
		}
	}

	// Add new managed entries.
	// Sort names for deterministic output.
	names := make([]string, 0, len(managed))
	for name := range managed {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		hf := managed[name]
		event := hf.Event
		if format == FormatFlat {
			event = EventNameForCursor(event)
		}

		switch format {
		case FormatFlat:
			flatEntries, err := ToFlatEntries(name, hf)
			if err != nil {
				return fmt.Errorf("converting hook %q to flat format: %w", name, err)
			}
			existingHooks[event] = append(existingHooks[event], flatEntries...)

		default: // FormatNested (Claude/Codex)
			entry, err := buildNestedEntry(name, hf)
			if err != nil {
				return fmt.Errorf("building hook entry %q: %w", name, err)
			}
			existingHooks[event] = append(existingHooks[event], entry)
		}
	}

	// For Cursor, ensure version field is present.
	if format == FormatFlat {
		if _, ok := topLevel["version"]; !ok {
			topLevel["version"] = json.RawMessage(`1`)
		}
	}

	// Write hooks back to top-level.
	if len(existingHooks) > 0 {
		hooksData, err := json.Marshal(existingHooks)
		if err != nil {
			return fmt.Errorf("marshaling hooks: %w", err)
		}
		topLevel["hooks"] = hooksData
	} else {
		delete(topLevel, "hooks")
	}

	// Marshal with ordered keys for deterministic output.
	output, err := marshalOrdered(topLevel)
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	output = append(output, '\n')

	return atomicWrite(settingsPath, output)
}

// RemoveManaged removes entries with matching _agentfiles values from a
// settings/hooks JSON file. It is a no-op if the file does not exist.
func RemoveManaged(settingsPath string, names []string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var topLevel map[string]json.RawMessage
	if err := json.Unmarshal(data, &topLevel); err != nil {
		return err
	}

	raw, ok := topLevel["hooks"]
	if !ok {
		return nil
	}

	var hooksMap map[string][]json.RawMessage
	if err := json.Unmarshal(raw, &hooksMap); err != nil {
		return err
	}

	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	changed := false
	for event, entries := range hooksMap {
		var kept []json.RawMessage
		for _, entry := range entries {
			marker := extractAgentfilesMarker(entry)
			if marker != "" && nameSet[marker] {
				changed = true
				continue
			}
			kept = append(kept, entry)
		}
		if len(kept) > 0 {
			hooksMap[event] = kept
		} else {
			delete(hooksMap, event)
			changed = true
		}
	}

	if !changed {
		return nil
	}

	if len(hooksMap) > 0 {
		hooksData, err := json.Marshal(hooksMap)
		if err != nil {
			return err
		}
		topLevel["hooks"] = hooksData
	} else {
		delete(topLevel, "hooks")
	}

	output, err := marshalOrdered(topLevel)
	if err != nil {
		return err
	}
	output = append(output, '\n')
	return atomicWrite(settingsPath, output)
}

// buildNestedEntry creates a Claude/Codex nested hook entry with _agentfiles marker.
func buildNestedEntry(name string, hf *HookFile) (json.RawMessage, error) {
	entry := make(map[string]any)
	if hf.Matcher != "" {
		entry["matcher"] = hf.Matcher
	}
	var hks []any
	for _, raw := range hf.Hooks {
		var h any
		if err := json.Unmarshal(raw, &h); err != nil {
			return nil, fmt.Errorf("parsing inner hook for %q: %w", name, err)
		}
		hks = append(hks, h)
	}
	entry["hooks"] = hks
	entry["_agentfiles"] = name
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshaling hook entry %q: %w", name, err)
	}
	return data, nil
}

// isStrippedManagedEntry reports whether an unmarked entry is agentfiles-managed
// anyway: it has at least one command, and every command points into an
// agentfiles hook deploy dir. Covers both the nested shape
// ({matcher, hooks: [{command}]}) and the flat Cursor shape ({command, ...}).
// File-form hooks with commands outside the deploy dirs are not reclaimable
// this way; they still rely on the _agentfiles marker.
func isStrippedManagedEntry(entry json.RawMessage) bool {
	var obj struct {
		Command string `json:"command"`
		Hooks   []struct {
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(entry, &obj); err != nil {
		return false
	}
	cmds := make([]string, 0, len(obj.Hooks)+1)
	if obj.Command != "" {
		cmds = append(cmds, obj.Command)
	}
	for _, h := range obj.Hooks {
		cmds = append(cmds, h.Command)
	}
	if len(cmds) == 0 {
		return false
	}
	for _, cmd := range cmds {
		if !strings.Contains(cmd, DeployBaseUser+"/") && !strings.Contains(cmd, DeployBaseRepo+"/") {
			return false
		}
	}
	return true
}

// hasAgentfilesMarker returns true if the JSON entry contains an "_agentfiles" field.
func hasAgentfilesMarker(entry json.RawMessage) bool {
	return extractAgentfilesMarker(entry) != ""
}

// extractAgentfilesMarker returns the _agentfiles field value, or "" if absent.
func extractAgentfilesMarker(entry json.RawMessage) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(entry, &obj); err != nil {
		return ""
	}
	raw, ok := obj["_agentfiles"]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// marshalOrdered serializes a top-level JSON object with sorted keys and
// 2-space indentation to produce deterministic output.
func marshalOrdered(m map[string]json.RawMessage) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Re-build as an ordered structure using json.Marshal on a slice of
	// key-value pairs isn't possible directly; instead we build a temporary
	// ordered map via a slice and encode manually through a helper struct.
	ordered := make([]kv, len(keys))
	for i, k := range keys {
		var val any
		if err := json.Unmarshal(m[k], &val); err != nil {
			return nil, fmt.Errorf("re-parsing key %q: %w", k, err)
		}
		ordered[i] = kv{Key: k, Val: val}
	}

	// Build the final object manually to preserve key order.
	buf := []byte("{\n")
	for i, item := range ordered {
		keyJSON, _ := json.Marshal(item.Key)
		valJSON, err := json.MarshalIndent(item.Val, "  ", "  ")
		if err != nil {
			return nil, err
		}
		buf = append(buf, "  "...)
		buf = append(buf, keyJSON...)
		buf = append(buf, ": "...)
		buf = append(buf, valJSON...)
		if i < len(ordered)-1 {
			buf = append(buf, ',')
		}
		buf = append(buf, '\n')
	}
	buf = append(buf, '}')
	return buf, nil
}

type kv struct {
	Key string
	Val any
}

// atomicWrite writes data to path using a temp file + rename to avoid
// partial writes on crash.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".settings.tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
