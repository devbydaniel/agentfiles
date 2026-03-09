package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devbydaniel/agentfiles/internal/config"
	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/registry"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// resolvedConfigPath returns the config path to use: the --config flag
// value if set, otherwise the default.
func resolvedConfigPath() string {
	if configPath != "" {
		return configPath
	}
	return config.DefaultConfigPath()
}

// openStores loads the config and opens all referenced stores,
// returning a store map and the default store name. If no config exists
// or no stores are defined, it falls back to the --store flag path
// (or ~/.agentfiles as last resort).
func openStores() (map[string]*store.Store, string, error) {
	cfg, err := config.Load(resolvedConfigPath())
	if err != nil {
		return nil, "", err
	}

	if len(cfg.Stores) > 0 {
		stores := make(map[string]*store.Store, len(cfg.Stores))
		for name := range cfg.Stores {
			p, err := cfg.ResolveStore(name)
			if err != nil {
				return nil, "", err
			}
			s, err := store.Open(p)
			if err != nil {
				return nil, "", fmt.Errorf("opening store %q: %w", name, err)
			}
			stores[name] = s
		}

		defaultStore := cfg.DefaultStore
		if defaultStore == "" {
			if len(cfg.Stores) == 1 {
				for name := range cfg.Stores {
					defaultStore = name
				}
			} else {
				return nil, "", fmt.Errorf("multiple stores configured but no default_store set; please set default_store in %s", resolvedConfigPath())
			}
		}

		// Allow --store flag to override default store when it matches a
		// configured store name.
		if storePath != "" && !looksLikePath(storePath) {
			if _, ok := stores[storePath]; ok {
				defaultStore = storePath
			}
		}

		return stores, defaultStore, nil
	}

	// Fallback: use --store flag as a single default store.
	p := fallbackStorePath()
	s, err := store.Open(p)
	if err != nil {
		return nil, "", err
	}
	return map[string]*store.Store{"default": s}, "default", nil
}

// openStore opens a single store. If --store is set, it is treated as a
// name (looked up in config) or as a path (if it looks like a filesystem
// path). Without --store, the config's default store is used, falling back
// to ~/.agentfiles.
func openStore() (*store.Store, error) {
	if storePath != "" {
		// If it looks like a path, open directly.
		if looksLikePath(storePath) {
			return store.Open(storePath)
		}
		// Otherwise treat as a store name from config.
		cfg, err := config.Load(resolvedConfigPath())
		if err != nil {
			return nil, err
		}
		p, err := cfg.ResolveStore(storePath)
		if err != nil {
			return nil, err
		}
		return store.Open(p)
	}

	// No --store flag: use config's default store.
	cfg, err := config.Load(resolvedConfigPath())
	if err != nil {
		return nil, err
	}
	if len(cfg.Stores) > 0 {
		name := cfg.DefaultStore
		if name == "" {
			if len(cfg.Stores) == 1 {
				for n := range cfg.Stores {
					name = n
				}
			} else {
				return nil, fmt.Errorf("multiple stores configured but no default_store set; please set default_store in %s", resolvedConfigPath())
			}
		}
		p, err := cfg.ResolveStore(name)
		if err != nil {
			return nil, err
		}
		return store.Open(p)
	}

	// Last resort: ~/.agentfiles
	return store.Open(fallbackStorePath())
}

// loadRegistry returns a registry from config-based repos if available,
// otherwise falls back to the store-level registry.toml.
func loadRegistry(cfg *config.Config, stores map[string]*store.Store, defaultStore string) (*registry.Registry, error) {
	if len(cfg.Repos) > 0 {
		return registry.LoadFromConfig(cfg)
	}

	// Fallback: load from store-level registry.toml.
	s, ok := stores[defaultStore]
	if !ok {
		return nil, fmt.Errorf("default store %q not found", defaultStore)
	}
	return registry.Load(s)
}

// loadConfig loads the config from the resolved path.
func loadConfig() (*config.Config, error) {
	return config.Load(resolvedConfigPath())
}

// entrySourcePath returns the absolute path to the source file/directory
// for a lock entry by looking up the entry's Store field in the store map.
// Empty Store falls back to defaultStore.
func entrySourcePath(entry *lock.Entry, stores map[string]*store.Store, defaultStore string) (string, error) {
	name := entry.Store
	if name == "" {
		name = defaultStore
	}
	s, err := store.LookupStore(stores, name)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.Root, entry.StorePath), nil
}

// looksLikePath returns true if s looks like a filesystem path rather than
// a store name.
func looksLikePath(s string) bool {
	return strings.Contains(s, string(os.PathSeparator)) ||
		strings.HasPrefix(s, ".") ||
		strings.HasPrefix(s, "~") ||
		filepath.IsAbs(s)
}

// fallbackStorePath returns the --store flag value if set, otherwise
// ~/.agentfiles.
func fallbackStorePath() string {
	if storePath != "" {
		return storePath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".agentfiles")
	}
	return filepath.Join(home, ".agentfiles")
}
