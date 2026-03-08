package manifest

import (
	"github.com/danielbenner/agentfiles/internal/bundle"
	"github.com/danielbenner/agentfiles/internal/store"
)

// ResolvedManifest contains the fully expanded lists after resolving
// bundle references and applying overrides.
type ResolvedManifest struct {
	Layout    string
	AgentsMd  string
	Skills    []string
	Plugins   []string
	Resources []string
}

// Resolve expands a Manifest (possibly referencing a bundle) into a
// flat ResolvedManifest. It applies SkillsAdd and SkillsRemove overrides.
func Resolve(m *Manifest, s *store.Store) (*ResolvedManifest, error) {
	r := &ResolvedManifest{
		Layout: m.Layout,
	}

	if m.Bundle != "" {
		b, err := bundle.Load(s, m.Bundle)
		if err != nil {
			return nil, err
		}

		r.AgentsMd = b.AgentsMd()
		r.Skills = filterExcluded(b.Skills.Include, b.Skills.Exclude)
		r.Plugins = filterExcluded(b.Plugins.Include, b.Plugins.Exclude)
		r.Resources = filterExcluded(b.Resources.Include, b.Resources.Exclude)
	} else {
		r.AgentsMd = m.AgentsMd
		r.Skills = copySlice(m.Skills)
		r.Plugins = copySlice(m.Plugins)
		r.Resources = copySlice(m.Resources)
	}

	// Apply overrides.
	if len(m.SkillsAdd) > 0 {
		for _, sk := range m.SkillsAdd {
			if !contains(r.Skills, sk) {
				r.Skills = append(r.Skills, sk)
			}
		}
	}
	if len(m.SkillsRemove) > 0 {
		r.Skills = filterExcluded(r.Skills, m.SkillsRemove)
	}

	return r, nil
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

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func copySlice(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}
