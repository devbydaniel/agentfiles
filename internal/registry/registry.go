// Package registry provides a central registry of repos and their agent
// configurations, stored as registry.toml in the agentfiles store.
//
// The registry supports a two-file model for team use:
//
//   - registry.toml (committed) — shared config using logical names, bundles,
//     layouts, and skill overrides.
//   - registry.local.toml (gitignored) — per-developer paths and optional
//     overrides for layout, skills_add, and skills_remove.
//
// Repos in registry.toml may use either "name" (requires a local entry to
// supply the path) or "path" directly (solo/non-team use).
package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/devbydaniel/agentfiles/internal/config"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// Registry represents the merged result of registry.toml + registry.local.toml.
type Registry struct {
	Repos []Repo `toml:"repos"`
}

// Repo is a single entry in the registry mapping a directory to its
// bundle/layout configuration.
type Repo struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
	// Store is the name of the store to use for this repo. Only populated
	// by LoadFromConfig (config.toml path); the legacy Load() path does not
	// set this field since it always operates on a single store.
	Store string `toml:"store"`
	Bundle       string   `toml:"bundle"`
	Layout       string   `toml:"layout"`
	SkillsAdd    []string `toml:"skills_add"`
	SkillsRemove []string `toml:"skills_remove"`
}

// localEntry is a per-developer override parsed from registry.local.toml.
type localEntry struct {
	Name         string   `toml:"name"`
	Path         string   `toml:"path"`
	Bundle       string   `toml:"bundle"`
	Layout       string   `toml:"layout"`
	SkillsAdd    []string `toml:"skills_add"`
	SkillsRemove []string `toml:"skills_remove"`
	Skip         bool     `toml:"skip"`
}

type localFile struct {
	Repos []localEntry `toml:"repos"`
}

const (
	registryFile      = "registry.toml"
	registryLocalFile = "registry.local.toml"
)

// Load reads registry.toml and registry.local.toml from the store root,
// merges them, and returns the final registry. Returns an empty registry
// (no error) if neither file exists.
func Load(s *store.Store) (*Registry, error) {
	reg, err := loadFile(filepath.Join(s.Root, registryFile))
	if err != nil {
		return nil, err
	}

	local, err := loadLocalFile(filepath.Join(s.Root, registryLocalFile))
	if err != nil {
		return nil, err
	}

	merged, err := merge(reg, local)
	if err != nil {
		return nil, err
	}

	if err := merged.validate(); err != nil {
		return nil, err
	}

	// Expand paths and set defaults.
	home, _ := os.UserHomeDir()
	for i := range merged.Repos {
		merged.Repos[i].Path = expandPath(merged.Repos[i].Path, home)
		if merged.Repos[i].Layout == "" {
			merged.Repos[i].Layout = "pi"
		}
	}

	return merged, nil
}

// loadFile reads and parses a registry TOML file. Returns an empty
// registry if the file does not exist.
func loadFile(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{}, nil
		}
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	var r Registry
	if err := toml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
	}
	return &r, nil
}

// loadLocalFile reads and parses registry.local.toml.
func loadLocalFile(path string) (*localFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &localFile{}, nil
		}
		return nil, fmt.Errorf("reading local registry: %w", err)
	}

	var lf localFile
	if err := toml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
	}
	return &lf, nil
}

// merge combines the shared registry with local overrides.
//
// Rules:
//   - Named repos in registry.toml get their path (and optional overrides)
//     from matching local entries. Named repos without a local entry are
//     skipped (the dev doesn't have that repo checked out).
//   - Path-only repos in registry.toml work standalone (no local entry needed).
//     A local entry can still override their layout/skills.
//   - Local entries can set skip=true to exclude a repo from apply-all.
//   - Local entries with a name that doesn't exist in registry.toml are
//     added as standalone repos (dev-only repos).
func merge(reg *Registry, local *localFile) (*Registry, error) {
	// Index local entries by name and by path.
	localByName := make(map[string]*localEntry, len(local.Repos))
	for i := range local.Repos {
		e := &local.Repos[i]
		if e.Name != "" {
			if _, exists := localByName[e.Name]; exists {
				return nil, fmt.Errorf("registry.local.toml: duplicate name %q", e.Name)
			}
			localByName[e.Name] = e
		}
	}

	// Track which local entries were consumed so we can add extras.
	consumed := make(map[string]bool)

	var merged []Repo

	for _, repo := range reg.Repos {
		key := repo.Name
		if key == "" {
			key = repo.Path
		}

		le := localByName[repo.Name]

		if repo.Name != "" && le == nil {
			// Named repo without a local entry — skip (dev doesn't have it).
			continue
		}

		if le != nil {
			consumed[repo.Name] = true

			if le.Skip {
				continue
			}

			// Apply local overrides.
			if le.Path != "" {
				repo.Path = le.Path
			}
			if le.Bundle != "" {
				repo.Bundle = le.Bundle
			}
			if le.Layout != "" {
				repo.Layout = le.Layout
			}
			if len(le.SkillsAdd) > 0 {
				repo.SkillsAdd = mergeStringSlices(repo.SkillsAdd, le.SkillsAdd)
			}
			if len(le.SkillsRemove) > 0 {
				repo.SkillsRemove = mergeStringSlices(repo.SkillsRemove, le.SkillsRemove)
			}
		}

		merged = append(merged, repo)
	}

	// Add local-only entries (dev-specific repos not in the shared registry).
	for i := range local.Repos {
		e := &local.Repos[i]
		if e.Name != "" && !consumed[e.Name] && !e.Skip {
			merged = append(merged, Repo{
				Name:         e.Name,
				Path:         e.Path,
				Bundle:       e.Bundle,
				Layout:       e.Layout,
				SkillsAdd:    e.SkillsAdd,
				SkillsRemove: e.SkillsRemove,
			})
		}
	}

	return &Registry{Repos: merged}, nil
}

func (r *Registry) validate() error {
	seen := make(map[string]bool)
	for i, repo := range r.Repos {
		identifier := repo.Name
		if identifier == "" {
			identifier = repo.Path
		}
		if repo.Path == "" || repo.Path == "." {
			return fmt.Errorf("registry: repos[%d] (%s) missing path (set it in registry.local.toml or registry.toml)", i, identifier)
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

// expandPath resolves ~ to home directory and cleans the path.
func expandPath(p, home string) string {
	if strings.HasPrefix(p, "~/") && home != "" {
		p = filepath.Join(home, p[2:])
	}
	return filepath.Clean(p)
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
		})
	}

	if err := reg.validate(); err != nil {
		return nil, err
	}
	return reg, nil
}

// mergeStringSlices returns a combined slice with no duplicates.
func mergeStringSlices(base, extra []string) []string {
	have := make(map[string]bool, len(base))
	for _, s := range base {
		have[s] = true
	}
	out := make([]string, len(base))
	copy(out, base)
	for _, s := range extra {
		if !have[s] {
			out = append(out, s)
			have[s] = true
		}
	}
	return out
}
