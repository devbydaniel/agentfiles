package manifest

import (
	"fmt"
	"strings"

	"github.com/devbydaniel/agentfiles/internal/bundle"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// ResolvedAsset pairs an asset name with the store it comes from.
type ResolvedAsset struct {
	Name      string // Leaf name for deployment (e.g., "browse")
	Store     string // Store name
	StorePath string // Group-qualified path (e.g., "tooling/browse" or "browse" for flat)
}

// ResolvedManifest contains the fully expanded lists after resolving
// bundle references and applying overrides.
type ResolvedManifest struct {
	Layout       string
	Instructions ResolvedAsset
	Skills       []ResolvedAsset
	Resources    []ResolvedAsset
	Agents       []ResolvedAsset
	PiExtensions []ResolvedAsset
}

// Resolve expands a Manifest (possibly referencing a bundle) into a
// flat ResolvedManifest. It applies SkillsAdd/SkillsRemove and
// AgentsAdd/AgentsRemove overrides.
//
// stores maps store names to open Store instances. defaultStore is the name
// used when an asset has no "storename:" prefix. Bundle resolution always
// uses the default store.
func Resolve(m *Manifest, stores map[string]*store.Store, defaultStore string) (*ResolvedManifest, error) {
	if _, ok := stores[defaultStore]; !ok {
		return nil, fmt.Errorf("default store %q not found in stores map", defaultStore)
	}

	r := &ResolvedManifest{
		Layout: m.Layout,
	}

	var skillRefs []skillRef

	if m.Bundle != "" {
		s := stores[defaultStore]
		b, err := bundle.Load(s, m.Bundle)
		if err != nil {
			return nil, err
		}

		if name := b.Instructions(); name != "" {
			r.Instructions = ResolvedAsset{Name: name, Store: defaultStore}
		}

		// Expand globs in bundle skills include/exclude, then filter.
		expandedInclude, err := expandGlobList(s, b.Skills.Include)
		if err != nil {
			return nil, fmt.Errorf("expanding bundle skills.include: %w", err)
		}
		expandedExclude, err := expandGlobList(s, b.Skills.Exclude)
		if err != nil {
			return nil, fmt.Errorf("expanding bundle skills.exclude: %w", err)
		}
		filtered := filterExcluded(expandedInclude, expandedExclude)
		for _, name := range filtered {
			skillRefs = append(skillRefs, skillRef{name: name, storeName: defaultStore})
		}

		r.Resources = toResolvedAssets(filterExcluded(b.Resources.Include, b.Resources.Exclude), defaultStore)
		r.Agents = toResolvedAssets(filterExcluded(b.Agents.Include, b.Agents.Exclude), defaultStore)
		r.PiExtensions = toResolvedAssets(filterExcluded(b.PiExtensions.Include, b.PiExtensions.Exclude), defaultStore)
	} else {
		if m.Instructions != "" {
			storeName, name := parseStorePrefix(m.Instructions, defaultStore)
			r.Instructions = ResolvedAsset{Name: name, Store: storeName}
		}

		// Expand globs in cherry-pick skills.
		for _, raw := range m.Skills {
			storeName, name := parseStorePrefix(raw, defaultStore)
			s, err := store.LookupStore(stores, storeName)
			if err != nil {
				return nil, fmt.Errorf("skill %q: %w", raw, err)
			}
			expanded, err := s.ExpandSkillGlob(name)
			if err != nil {
				return nil, fmt.Errorf("expanding skill %q: %w", raw, err)
			}
			for _, n := range expanded {
				skillRefs = append(skillRefs, skillRef{name: n, storeName: storeName})
			}
		}

		r.Resources = parseResolvedAssets(m.Resources, defaultStore)
		r.Agents = parseResolvedAssets(m.Agents, defaultStore)
		r.PiExtensions = parseResolvedAssets(m.PiExtensions, defaultStore)
	}

	// Apply overrides: expand globs in skills_add and skills_remove.
	if len(m.SkillsAdd) > 0 {
		for _, raw := range m.SkillsAdd {
			storeName, name := parseStorePrefix(raw, defaultStore)
			s, err := store.LookupStore(stores, storeName)
			if err != nil {
				return nil, fmt.Errorf("skills_add %q: %w", raw, err)
			}
			expanded, err := s.ExpandSkillGlob(name)
			if err != nil {
				return nil, fmt.Errorf("expanding skills_add %q: %w", raw, err)
			}
			for _, n := range expanded {
				if !containsSkillRef(skillRefs, n, storeName) {
					skillRefs = append(skillRefs, skillRef{name: n, storeName: storeName})
				}
			}
		}
	}
	if len(m.SkillsRemove) > 0 {
		// Expand globs in skills_remove, then filter.
		var expandedRemove []string
		for _, raw := range m.SkillsRemove {
			storeName, name := parseStorePrefix(raw, defaultStore)
			s, err := store.LookupStore(stores, storeName)
			if err != nil {
				return nil, fmt.Errorf("skills_remove %q: %w", raw, err)
			}
			expanded, err := s.ExpandSkillGlob(name)
			if err != nil {
				// Glob with no matches is a warning, not error — just skip.
				continue
			}
			for _, n := range expanded {
				// Re-add store prefix for filterExcludedRefs.
				if storeName != defaultStore {
					expandedRemove = append(expandedRemove, storeName+":"+n)
				} else {
					expandedRemove = append(expandedRemove, n)
				}
			}
		}
		skillRefs = filterExcludedRefs(skillRefs, expandedRemove, defaultStore)
	}

	// Apply agents overrides.
	if len(m.AgentsAdd) > 0 {
		for _, raw := range m.AgentsAdd {
			storeName, name := parseStorePrefix(raw, defaultStore)
			if _, err := store.LookupStore(stores, storeName); err != nil {
				return nil, fmt.Errorf("agents_add %q: %w", raw, err)
			}
			if !containsAsset(r.Agents, name, storeName) {
				r.Agents = append(r.Agents, ResolvedAsset{Name: name, Store: storeName})
			}
		}
	}
	if len(m.AgentsRemove) > 0 {
		for _, raw := range m.AgentsRemove {
			storeName, _ := parseStorePrefix(raw, defaultStore)
			if _, err := store.LookupStore(stores, storeName); err != nil {
				return nil, fmt.Errorf("agents_remove %q: %w", raw, err)
			}
		}
		r.Agents = filterExcludedAssets(r.Agents, m.AgentsRemove)
	}

	// Apply pi_extensions overrides.
	if len(m.PiExtensionsAdd) > 0 {
		for _, raw := range m.PiExtensionsAdd {
			storeName, name := parseStorePrefix(raw, defaultStore)
			if _, err := store.LookupStore(stores, storeName); err != nil {
				return nil, fmt.Errorf("pi_extensions_add %q: %w", raw, err)
			}
			if !containsAsset(r.PiExtensions, name, storeName) {
				r.PiExtensions = append(r.PiExtensions, ResolvedAsset{Name: name, Store: storeName})
			}
		}
	}
	if len(m.PiExtensionsRemove) > 0 {
		for _, raw := range m.PiExtensionsRemove {
			storeName, _ := parseStorePrefix(raw, defaultStore)
			if _, err := store.LookupStore(stores, storeName); err != nil {
				return nil, fmt.Errorf("pi_extensions_remove %q: %w", raw, err)
			}
		}
		r.PiExtensions = filterExcludedAssets(r.PiExtensions, m.PiExtensionsRemove)
	}

	// Resolve each skill ref via store.ResolveSkill to get SkillInfo.
	for _, ref := range skillRefs {
		s, err := store.LookupStore(stores, ref.storeName)
		if err != nil {
			return nil, fmt.Errorf("skill %q: %w", ref.name, err)
		}
		info, err := s.ResolveSkill(ref.name)
		if err != nil {
			return nil, fmt.Errorf("resolving skill %q in store %q: %w", ref.name, ref.storeName, err)
		}
		r.Skills = append(r.Skills, ResolvedAsset{
			Name:      info.LeafName,
			Store:     ref.storeName,
			StorePath: info.GroupPath,
		})
	}

	// Validate that all referenced stores exist.
	allAssets := []ResolvedAsset{}
	if r.Instructions.Name != "" {
		allAssets = append(allAssets, r.Instructions)
	}
	allAssets = append(allAssets, r.Skills...)
	allAssets = append(allAssets, r.Resources...)
	allAssets = append(allAssets, r.Agents...)
	allAssets = append(allAssets, r.PiExtensions...)
	for _, a := range allAssets {
		if _, ok := stores[a.Store]; !ok {
			return nil, fmt.Errorf("store %q not found (referenced by asset %q)", a.Store, a.Name)
		}
	}

	// Validate no leaf-name collisions across skills (same leaf from different groups or stores).
	if err := checkLeafNameCollisions(r.Skills); err != nil {
		return nil, err
	}
	if err := checkNameCollisions("resource", r.Resources); err != nil {
		return nil, err
	}
	if err := checkNameCollisions("agent", r.Agents); err != nil {
		return nil, err
	}
	if err := checkNameCollisions("pi_extension", r.PiExtensions); err != nil {
		return nil, err
	}

	return r, nil
}

// skillRef is a store-tagged skill reference (pre-resolution, post-glob-expansion).
type skillRef struct {
	name      string // bare or group-qualified, no trailing slash
	storeName string
}

// expandGlobList expands all glob patterns in a string list against the given store.
func expandGlobList(s *store.Store, patterns []string) ([]string, error) {
	var result []string
	for _, p := range patterns {
		expanded, err := s.ExpandSkillGlob(p)
		if err != nil {
			return nil, err
		}
		result = append(result, expanded...)
	}
	return result, nil
}

// containsSkillRef checks if a skill reference already exists in the list.
func containsSkillRef(refs []skillRef, name, storeName string) bool {
	for _, r := range refs {
		if r.name == name && r.storeName == storeName {
			return true
		}
	}
	return false
}

// filterExcludedRefs removes skill refs matching the expanded exclude list.
func filterExcludedRefs(refs []skillRef, exclude []string, defaultStore string) []skillRef {
	if len(exclude) == 0 {
		return refs
	}
	type entry struct {
		name      string
		storeName string
	}
	entries := make([]entry, len(exclude))
	for i, raw := range exclude {
		if idx := strings.Index(raw, ":"); idx > 0 {
			entries[i] = entry{name: raw[idx+1:], storeName: raw[:idx]}
		} else {
			entries[i] = entry{name: raw}
		}
	}
	var out []skillRef
	for _, r := range refs {
		excluded := false
		for _, ex := range entries {
			if r.name == ex.name && (ex.storeName == "" || ex.storeName == r.storeName) {
				excluded = true
				break
			}
		}
		if !excluded {
			out = append(out, r)
		}
	}
	return out
}

// checkLeafNameCollisions returns an error if two skills resolve to the same
// leaf name, regardless of group path or store.
func checkLeafNameCollisions(skills []ResolvedAsset) error {
	seen := make(map[string]ResolvedAsset, len(skills))
	for _, sk := range skills {
		if prev, ok := seen[sk.Name]; ok {
			return fmt.Errorf("skill leaf name %q collision: %s (store %s) and %s (store %s)", sk.Name, prev.StorePath, prev.Store, sk.StorePath, sk.Store)
		}
		seen[sk.Name] = sk
	}
	return nil
}

// parseStorePrefix splits "storename:assetname" into (storeName, assetName).
// If no prefix, returns (defaultStore, raw).
func parseStorePrefix(raw string, defaultStore string) (string, string) {
	if idx := strings.Index(raw, ":"); idx > 0 {
		return raw[:idx], raw[idx+1:]
	}
	return defaultStore, raw
}

// parseResolvedAssets converts a string slice into ResolvedAssets, parsing
// store prefixes.
func parseResolvedAssets(names []string, defaultStore string) []ResolvedAsset {
	if names == nil {
		return nil
	}
	out := make([]ResolvedAsset, len(names))
	for i, n := range names {
		storeName, name := parseStorePrefix(n, defaultStore)
		out[i] = ResolvedAsset{Name: name, Store: storeName}
	}
	return out
}

// toResolvedAssets converts plain names to ResolvedAssets with the given store.
func toResolvedAssets(names []string, storeName string) []ResolvedAsset {
	if names == nil {
		return nil
	}
	out := make([]ResolvedAsset, len(names))
	for i, n := range names {
		out[i] = ResolvedAsset{Name: n, Store: storeName}
	}
	return out
}

func containsAsset(assets []ResolvedAsset, name, storeName string) bool {
	for _, a := range assets {
		if a.Name == name && a.Store == storeName {
			return true
		}
	}
	return false
}

// excludeEntry represents a parsed skills_remove entry.
type excludeEntry struct {
	Name  string
	Store string // empty means match all stores
}

// filterExcludedAssets removes assets matching the exclude list.
// Exclude entries are parsed for store prefixes: "storename:skill" removes
// only from that store, while "skill" (no prefix) removes from all stores.
func filterExcludedAssets(assets []ResolvedAsset, exclude []string) []ResolvedAsset {
	if len(exclude) == 0 {
		return assets
	}
	entries := make([]excludeEntry, len(exclude))
	for i, raw := range exclude {
		if idx := strings.Index(raw, ":"); idx > 0 {
			entries[i] = excludeEntry{Name: raw[idx+1:], Store: raw[:idx]}
		} else {
			entries[i] = excludeEntry{Name: raw}
		}
	}
	out := make([]ResolvedAsset, 0, len(assets))
	for _, a := range assets {
		excluded := false
		for _, ex := range entries {
			if a.Name == ex.Name && (ex.Store == "" || ex.Store == a.Store) {
				excluded = true
				break
			}
		}
		if !excluded {
			out = append(out, a)
		}
	}
	return out
}

// checkNameCollisions returns an error if two assets have the same name but
// come from different stores, which would cause deploy path and lock key
// collisions.
func checkNameCollisions(assetType string, assets []ResolvedAsset) error {
	seen := make(map[string]string, len(assets)) // name -> store
	for _, a := range assets {
		if prev, ok := seen[a.Name]; ok && prev != a.Store {
			return fmt.Errorf("%s %q appears in multiple stores (%s, %s); same-name assets from different stores are not supported", assetType, a.Name, prev, a.Store)
		}
		seen[a.Name] = a.Store
	}
	return nil
}

func filterExcluded(include, exclude []string) []string {
	if len(exclude) == 0 {
		return copySlice(include)
	}
	ex := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		ex[e] = true
	}
	out := make([]string, 0, len(include))
	for _, s := range include {
		if !ex[s] {
			out = append(out, s)
		}
	}
	return out
}

func copySlice(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}
