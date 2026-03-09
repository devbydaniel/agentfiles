package manifest

import (
	"fmt"
	"strings"

	"github.com/devbydaniel/agentfiles/internal/bundle"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// ResolvedAsset pairs an asset name with the store it comes from.
type ResolvedAsset struct {
	Name  string
	Store string
}

// ResolvedManifest contains the fully expanded lists after resolving
// bundle references and applying overrides.
type ResolvedManifest struct {
	Layout    string
	AgentsMd  ResolvedAsset
	Skills    []ResolvedAsset
	Plugins   []ResolvedAsset
	Resources []ResolvedAsset
}

// Resolve expands a Manifest (possibly referencing a bundle) into a
// flat ResolvedManifest. It applies SkillsAdd and SkillsRemove overrides.
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

	if m.Bundle != "" {
		s := stores[defaultStore]
		b, err := bundle.Load(s, m.Bundle)
		if err != nil {
			return nil, err
		}

		if name := b.AgentsMd(); name != "" {
			r.AgentsMd = ResolvedAsset{Name: name, Store: defaultStore}
		}
		r.Skills = toResolvedAssets(filterExcluded(b.Skills.Include, b.Skills.Exclude), defaultStore)
		r.Plugins = toResolvedAssets(filterExcluded(b.Plugins.Include, b.Plugins.Exclude), defaultStore)
		r.Resources = toResolvedAssets(filterExcluded(b.Resources.Include, b.Resources.Exclude), defaultStore)
	} else {
		if m.AgentsMd != "" {
			storeName, name := parseStorePrefix(m.AgentsMd, defaultStore)
			r.AgentsMd = ResolvedAsset{Name: name, Store: storeName}
		}
		r.Skills = parseResolvedAssets(m.Skills, defaultStore)
		r.Plugins = parseResolvedAssets(m.Plugins, defaultStore)
		r.Resources = parseResolvedAssets(m.Resources, defaultStore)
	}

	// Apply overrides.
	if len(m.SkillsAdd) > 0 {
		for _, sk := range m.SkillsAdd {
			storeName, name := parseStorePrefix(sk, defaultStore)
			if !containsAsset(r.Skills, name, storeName) {
				r.Skills = append(r.Skills, ResolvedAsset{Name: name, Store: storeName})
			}
		}
	}
	if len(m.SkillsRemove) > 0 {
		r.Skills = filterExcludedAssets(r.Skills, m.SkillsRemove)
	}

	// Validate that all referenced stores exist.
	allAssets := []ResolvedAsset{}
	if r.AgentsMd.Name != "" {
		allAssets = append(allAssets, r.AgentsMd)
	}
	allAssets = append(allAssets, r.Skills...)
	allAssets = append(allAssets, r.Plugins...)
	allAssets = append(allAssets, r.Resources...)
	for _, a := range allAssets {
		if _, ok := stores[a.Store]; !ok {
			return nil, fmt.Errorf("store %q not found (referenced by asset %q)", a.Store, a.Name)
		}
	}

	// Validate no same-name collisions across different stores.
	if err := checkNameCollisions("skill", r.Skills); err != nil {
		return nil, err
	}
	if err := checkNameCollisions("plugin", r.Plugins); err != nil {
		return nil, err
	}
	if err := checkNameCollisions("resource", r.Resources); err != nil {
		return nil, err
	}

	return r, nil
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
