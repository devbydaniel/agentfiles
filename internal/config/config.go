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
	User         *UserConfig       `toml:"user,omitempty"`
	Repos        []Repo            `toml:"repos"`

	// loadedFrom records the path the config was loaded from, used to
	// compute sibling paths like user.lock.
	loadedFrom string
}

// UserConfig describes user-level agent file deployment.
// It mirrors the manifest fields but lives in the central config
// (no .agentfiles file in $HOME).
type UserConfig struct {
	Store        string   `toml:"store"`
	Bundle       string   `toml:"bundle"`
	Layout       string   `toml:"layout"`
	AgentsMd     string   `toml:"agents_md"`
	Skills       []string `toml:"skills"`
	Plugins      []string `toml:"plugins"`
	SkillsAdd    []string `toml:"skills_add"`
	SkillsRemove []string `toml:"skills_remove"`
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

// DefaultConfigPath returns the default config file path:
// ~/.config/agentfiles/config.toml
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "agentfiles", "config.toml")
	}
	return filepath.Join(home, ".config", "agentfiles", "config.toml")
}

// Load reads and parses a config file. If the file does not exist, an empty
// Config is returned (no error). Repo paths are expanded and defaults applied.
func Load(path string) (*Config, error) {
	cfg := &Config{
		Stores: make(map[string]string),
	}

	cfg.loadedFrom = path

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

	// Apply defaults and expand paths.
	home, _ := os.UserHomeDir()
	for i := range cfg.Repos {
		repo := &cfg.Repos[i]

		if repo.Store == "" {
			repo.Store = cfg.DefaultStore
		}
		if repo.Layout == "" {
			repo.Layout = "pi"
		}
		if home != "" {
			repo.Path = expandRepoPath(repo.Path, home)
		}
	}

	// Validate [user] section if present.
	if cfg.User != nil {
		if err := cfg.User.validate(); err != nil {
			return nil, fmt.Errorf("config %q: [user] %w", path, err)
		}
		if cfg.User.Store == "" {
			cfg.User.Store = cfg.DefaultStore
		}
		if cfg.User.Layout == "" {
			cfg.User.Layout = "all"
		}
		if cfg.User.Store != "" {
			if _, ok := cfg.Stores[cfg.User.Store]; !ok {
				return nil, fmt.Errorf("config %q: [user] references unknown store %q", path, cfg.User.Store)
			}
		}
	}

	// Validate repos.
	for i, repo := range cfg.Repos {
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

	return cfg, nil
}

// validate checks that the user config has valid manifest-style fields.
func (u *UserConfig) validate() error {
	hasBundle := u.Bundle != ""
	hasCherryPick := u.AgentsMd != "" || len(u.Skills) > 0 || len(u.Plugins) > 0

	if !hasBundle && !hasCherryPick {
		return fmt.Errorf("must set either 'bundle' or cherry-pick fields ('agents_md', 'skills', 'plugins')")
	}
	if hasBundle && hasCherryPick {
		return fmt.Errorf("cannot set both 'bundle' and cherry-pick fields ('agents_md', 'skills', 'plugins')")
	}
	if !hasBundle && (len(u.SkillsAdd) > 0 || len(u.SkillsRemove) > 0) {
		return fmt.Errorf("'skills_add' and 'skills_remove' require 'bundle' to be set")
	}
	return nil
}

// UserLockPath returns the path for the user-level lock file.
// It is placed as a sibling to the config file: ~/.config/agentfiles/user.lock.
func (c *Config) UserLockPath() string {
	if c.loadedFrom != "" {
		return filepath.Join(filepath.Dir(c.loadedFrom), "user.lock")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "agentfiles", "user.lock")
	}
	return filepath.Join(home, ".config", "agentfiles", "user.lock")
}

// expandRepoPath resolves ~ to home directory and cleans the path.
func expandRepoPath(p, home string) string {
	if strings.HasPrefix(p, "~/") && home != "" {
		p = filepath.Join(home, p[2:])
	}
	return filepath.Clean(p)
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
