package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Manifest represents a parsed .agentfiles TOML file.
type Manifest struct {
	Bundle       string   `toml:"bundle"`
	Layout       string   `toml:"layout"`
	AgentsMd     string   `toml:"agents_md"`
	Skills       []string `toml:"skills"`
	Plugins      []string `toml:"plugins"`
	Resources    []string `toml:"resources"`
	SkillsAdd    []string `toml:"skills_add"`
	SkillsRemove []string `toml:"skills_remove"`
}

// Load reads and parses an .agentfiles manifest from the given directory.
func Load(dir string) (*Manifest, error) {
	p := filepath.Join(dir, ".agentfiles")
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	// Default layout to "pi".
	if m.Layout == "" {
		m.Layout = "pi"
	}

	if err := m.validate(); err != nil {
		return nil, err
	}

	return &m, nil
}

func (m *Manifest) validate() error {
	hasBundle := m.Bundle != ""
	hasCherryPick := m.AgentsMd != "" || len(m.Skills) > 0 || len(m.Plugins) > 0 || len(m.Resources) > 0

	if !hasBundle && !hasCherryPick {
		return fmt.Errorf("manifest must set either 'bundle' or cherry-pick fields ('agents_md', 'skills', 'plugins', 'resources')")
	}
	if hasBundle && hasCherryPick {
		return fmt.Errorf("manifest cannot set both 'bundle' and cherry-pick fields ('agents_md', 'skills', 'plugins', 'resources')")
	}

	if !hasBundle && (len(m.SkillsAdd) > 0 || len(m.SkillsRemove) > 0) {
		return fmt.Errorf("'skills_add' and 'skills_remove' require 'bundle' to be set")
	}

	return nil
}
