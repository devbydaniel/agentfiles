// Package registry provides a central registry of repos and their agent
// configurations. Repos are defined in ~/.config/agentfiles/config.toml
// and loaded via LoadFromConfig.
package registry

import (
	"fmt"

	"github.com/devbydaniel/agentfiles/internal/config"
)

// Registry represents repos loaded from config.
type Registry struct {
	Repos []Repo `toml:"repos"`
}

// Repo is a single entry in the registry mapping a directory to its
// bundle/layout configuration.
type Repo struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
	// Store is the name of the store to use for this repo.
	Store        string   `toml:"store"`
	Bundle       string   `toml:"bundle"`
	Layout       string   `toml:"layout"`
	SkillsAdd    []string `toml:"skills_add"`
	SkillsRemove []string `toml:"skills_remove"`
	AgentsAdd          []string `toml:"agents_add"`
	AgentsRemove       []string `toml:"agents_remove"`
	PiExtensionsAdd    []string `toml:"pi_extensions_add"`
	PiExtensionsRemove []string `toml:"pi_extensions_remove"`
	ExecArgs           []string `toml:"exec_args"`
}

func (r *Registry) validate() error {
	seen := make(map[string]bool)
	for i, repo := range r.Repos {
		identifier := repo.Name
		if identifier == "" {
			identifier = repo.Path
		}
		if repo.Path == "" || repo.Path == "." {
			return fmt.Errorf("registry: repos[%d] (%s) missing path", i, identifier)
		}
		if repo.Bundle == "" {
			return fmt.Errorf("registry: repos[%d] (%s) missing required field 'bundle'", i, identifier)
		}
		if seen[repo.Path] {
			return fmt.Errorf("registry: duplicate path %q", repo.Path)
		}
		seen[repo.Path] = true
	}
	return nil
}

// LoadFromConfig builds a Registry from a config.Config's repo list.
// The repos are already merged and expanded by config.Load, so this is a
// straightforward conversion. Validation is applied (path + bundle required,
// no duplicate paths).
func LoadFromConfig(cfg *config.Config) (*Registry, error) {
	reg := &Registry{}
	for _, cr := range cfg.Repos {
		reg.Repos = append(reg.Repos, Repo{
			Name:         cr.Name,
			Path:         cr.Path,
			Store:        cr.Store,
			Bundle:       cr.Bundle,
			Layout:       cr.Layout,
			SkillsAdd:    cr.SkillsAdd,
			SkillsRemove: cr.SkillsRemove,
			AgentsAdd:          cr.AgentsAdd,
			AgentsRemove:       cr.AgentsRemove,
			PiExtensionsAdd:    cr.PiExtensionsAdd,
			PiExtensionsRemove: cr.PiExtensionsRemove,
			ExecArgs:           cr.ExecArgs,
		})
	}

	if err := reg.validate(); err != nil {
		return nil, err
	}
	return reg, nil
}
