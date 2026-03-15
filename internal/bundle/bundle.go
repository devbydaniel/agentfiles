package bundle

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/devbydaniel/agentfiles/internal/store"
)

// Bundle represents a parsed bundle TOML file.
type Bundle struct {
	BundleMeta bundleMeta `toml:"bundle"`
	Skills     ItemList   `toml:"skills"`
	Resources  ItemList   `toml:"resources"`
}

type bundleMeta struct {
	Name     string `toml:"name"`
	AgentsMd string `toml:"agents_md"`
}

// ItemList holds include/exclude slices for a bundle section.
type ItemList struct {
	Include []string `toml:"include"`
	Exclude []string `toml:"exclude"`
}

// Name returns the bundle name.
func (b *Bundle) Name() string { return b.BundleMeta.Name }

// AgentsMd returns the agents_md reference.
func (b *Bundle) AgentsMd() string { return b.BundleMeta.AgentsMd }

// Load reads and parses a bundle TOML from the store's bundles directory.
func Load(s *store.Store, name string) (*Bundle, error) {
	p := filepath.Join(s.BundlesDir(), name+".toml")
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("reading bundle %q: %w", name, err)
	}

	var b Bundle
	if err := toml.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parsing bundle %q: %w", name, err)
	}

	return &b, nil
}
