package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config represents the agentfiles configuration from
// ~/.config/agentfiles/config.toml.
type Config struct {
	DefaultStore string            `toml:"default_store"`
	Stores       map[string]string `toml:"stores"`
	Repos        []Repo            `toml:"repos"`
}

// Repo is a single repo entry in the config.
type Repo struct {
	Name         string   `toml:"name"`
	Path         string   `toml:"path"`
	Store        string   `toml:"store"`
	Bundle       string   `toml:"bundle"`
	Layout       string   `toml:"layout"`
	SkillsAdd    []string `toml:"skills_add"`
	SkillsRemove []string `toml:"skills_remove"`
}

// localEntry is a per-developer override parsed from config.local.toml.
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

// DefaultConfigPath returns the default config file path:
// ~/.config/agentfiles/config.toml
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "agentfiles", "config.toml")
	}
	return filepath.Join(home, ".config", "agentfiles", "config.toml")
}

// Load reads a config file from path and its companion local file
// (same directory, config.local.toml). If the file does not exist, an empty
// Config is returned (no error). Repos are merged with local overrides
// and paths are expanded.
func Load(path string) (*Config, error) {
	cfg := &Config{
		Stores: make(map[string]string),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}

	if cfg.Stores == nil {
		cfg.Stores = make(map[string]string)
	}

	if cfg.DefaultStore != "" {
		if _, ok := cfg.Stores[cfg.DefaultStore]; !ok {
			return nil, fmt.Errorf("parsing config %q: default_store %q not found in [stores]", path, cfg.DefaultStore)
		}
	}

	// Load local overrides from config.local.toml in the same directory.
	localPath := filepath.Join(filepath.Dir(path), "config.local.toml")
	local, err := loadLocalFile(localPath)
	if err != nil {
		return nil, err
	}

	// Merge repos with local overrides.
	merged, err := mergeRepos(cfg.Repos, local, cfg.DefaultStore)
	if err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}

	// Validate merged repos.
	for i, repo := range merged {
		if repo.Path == "" || repo.Path == "." {
			identifier := repo.Name
			if identifier == "" {
				identifier = fmt.Sprintf("repos[%d]", i)
			}
			return nil, fmt.Errorf("config %q: %s has empty path", path, identifier)
		}
		if repo.Store != "" {
			if _, ok := cfg.Stores[repo.Store]; !ok {
				identifier := repo.Name
				if identifier == "" {
					identifier = repo.Path
				}
				return nil, fmt.Errorf("config %q: repo %s references unknown store %q", path, identifier, repo.Store)
			}
		}
	}

	cfg.Repos = merged

	return cfg, nil
}

// loadLocalFile reads and parses config.local.toml.
func loadLocalFile(path string) (*localFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &localFile{}, nil
		}
		return nil, fmt.Errorf("reading local config: %w", err)
	}

	var lf localFile
	if err := toml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
	}
	return &lf, nil
}

// mergeRepos combines shared repos with local overrides.
//
// Rules:
//   - Named repos get their path (and optional overrides) from matching local
//     entries. Named repos without a local entry are skipped.
//   - Path-only repos work standalone (no local entry needed).
//   - Local entries can set skip=true to exclude a repo.
//   - Local entries with a name not in shared repos are added as standalone.
//   - Store defaults to defaultStore when empty.
func mergeRepos(repos []Repo, local *localFile, defaultStore string) ([]Repo, error) {
	localByName := make(map[string]*localEntry, len(local.Repos))
	for i := range local.Repos {
		e := &local.Repos[i]
		if e.Name != "" {
			if _, exists := localByName[e.Name]; exists {
				return nil, fmt.Errorf("config.local.toml: duplicate name %q", e.Name)
			}
			localByName[e.Name] = e
		}
	}

	consumed := make(map[string]bool)
	home, _ := os.UserHomeDir()
	var merged []Repo

	for _, repo := range repos {
		le := localByName[repo.Name]

		if repo.Name != "" && le == nil {
			// Named repo without a local entry — skip.
			continue
		}

		if le != nil {
			consumed[repo.Name] = true
			if le.Skip {
				continue
			}
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

		// Default store.
		if repo.Store == "" {
			repo.Store = defaultStore
		}
		// Default layout.
		if repo.Layout == "" {
			repo.Layout = "pi"
		}
		// Expand path.
		if home != "" {
			repo.Path = expandRepoPath(repo.Path, home)
		}

		merged = append(merged, repo)
	}

	// Add local-only entries.
	for i := range local.Repos {
		e := &local.Repos[i]
		if e.Name != "" && !consumed[e.Name] && !e.Skip {
			layout := e.Layout
			if layout == "" {
				layout = "pi"
			}
			p := e.Path
			if home != "" {
				p = expandRepoPath(p, home)
			}
			merged = append(merged, Repo{
				Name:         e.Name,
				Path:         p,
				Store:        defaultStore,
				Bundle:       e.Bundle,
				Layout:       layout,
				SkillsAdd:    e.SkillsAdd,
				SkillsRemove: e.SkillsRemove,
			})
		}
	}

	return merged, nil
}

// expandRepoPath resolves ~ to home directory and cleans the path.
func expandRepoPath(p, home string) string {
	if strings.HasPrefix(p, "~/") && home != "" {
		p = filepath.Join(home, p[2:])
	}
	return filepath.Clean(p)
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

// ResolveStore returns the expanded absolute path for the named store.
// It expands ~ prefixes and validates that the path exists on disk.
func (c *Config) ResolveStore(name string) (string, error) {
	raw, ok := c.Stores[name]
	if !ok {
		return "", fmt.Errorf("store %q not found in config", name)
	}

	expanded, err := expandPath(raw)
	if err != nil {
		return "", fmt.Errorf("expanding store %q path: %w", name, err)
	}

	info, err := os.Stat(expanded)
	if err != nil {
		return "", fmt.Errorf("store %q path %q: %w", name, expanded, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("store %q path %q is not a directory", name, expanded)
	}

	return expanded, nil
}

// expandPath expands a leading ~ to the user's home directory and returns
// the absolute path.
func expandPath(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			p = home
		} else {
			p = filepath.Join(home, p[2:])
		}
	}
	return filepath.Abs(p)
}
