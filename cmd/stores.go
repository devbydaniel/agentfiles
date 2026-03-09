package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/devbydaniel/agentfiles/internal/config"
	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// openStoresForLock loads the config and opens all referenced stores,
// returning a store map and the default store name. If no config exists
// or no stores are defined, it falls back to the --store flag path.
func openStoresForLock() (map[string]*store.Store, string, error) {
	cfg, err := config.Load(config.DefaultConfigPath())
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
				return nil, "", fmt.Errorf("multiple stores configured but no default_store set; please set default_store in %s", config.DefaultConfigPath())
			}
		}

		return stores, defaultStore, nil
	}

	// Fallback: use --store flag as a single default store.
	s, err := store.Open(storePath)
	if err != nil {
		return nil, "", err
	}
	return map[string]*store.Store{"default": s}, "default", nil
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
