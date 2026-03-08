package store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// config represents the agentfiles config.toml structure.
type config struct {
	Source string `toml:"source"`
}

// DefaultStorePath returns the default store path. It checks
// ~/.config/agentfiles/config.toml for a custom "source" setting,
// falling back to ~/.agentfiles.
func DefaultStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".agentfiles")
	}

	configPath := filepath.Join(home, ".config", "agentfiles", "config.toml")
	data, err := os.ReadFile(configPath)
	if err == nil {
		var cfg config
		if toml.Unmarshal(data, &cfg) == nil && cfg.Source != "" {
			p := cfg.Source
			// Expand ~ prefix
			if strings.HasPrefix(p, "~/") {
				p = filepath.Join(home, p[2:])
			}
			return p
		}
	}

	return filepath.Join(home, ".agentfiles")
}
