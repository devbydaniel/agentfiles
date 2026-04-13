package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Manifest represents a parsed .agentfiles TOML file.
type Manifest struct {
	Store              string   `toml:"store"`
	Bundle             string   `toml:"bundle"`
	Layout             string   `toml:"layout"`
	Instructions       string   `toml:"instructions"`
	Skills             []string `toml:"skills"`
	Resources          []string `toml:"resources"`
	Agents             []string `toml:"agents"`
	PiExtensions       []string `toml:"pi_extensions"`
	Hooks              []string `toml:"hooks"`
	SkillsAdd          []string `toml:"skills_add"`
	SkillsRemove       []string `toml:"skills_remove"`
	AgentsAdd          []string `toml:"agents_add"`
	AgentsRemove       []string `toml:"agents_remove"`
	PiExtensionsAdd    []string `toml:"pi_extensions_add"`
	PiExtensionsRemove []string `toml:"pi_extensions_remove"`
	HooksAdd           []string `toml:"hooks_add"`
	HooksRemove        []string `toml:"hooks_remove"`
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

// UserFields holds the manifest-equivalent fields from a user-level config.
// This avoids importing the config package (which would be circular).
type UserFields struct {
	Bundle             string
	Layout             string
	Instructions       string
	Skills             []string
	Agents             []string
	PiExtensions       []string
	Hooks              []string
	SkillsAdd          []string
	SkillsRemove       []string
	AgentsAdd          []string
	AgentsRemove       []string
	PiExtensionsAdd    []string
	PiExtensionsRemove []string
	HooksAdd           []string
	HooksRemove        []string
}

// FromUserConfig constructs a Manifest from user config fields.
// It applies defaults (layout defaults to "all" for user-level) and validates.
func FromUserConfig(u UserFields) (*Manifest, error) {
	m := &Manifest{
		Bundle:             u.Bundle,
		Layout:             u.Layout,
		Instructions:       u.Instructions,
		Skills:             u.Skills,
		Agents:             u.Agents,
		PiExtensions:       u.PiExtensions,
		Hooks:              u.Hooks,
		SkillsAdd:          u.SkillsAdd,
		SkillsRemove:       u.SkillsRemove,
		AgentsAdd:          u.AgentsAdd,
		AgentsRemove:       u.AgentsRemove,
		PiExtensionsAdd:    u.PiExtensionsAdd,
		PiExtensionsRemove: u.PiExtensionsRemove,
		HooksAdd:           u.HooksAdd,
		HooksRemove:        u.HooksRemove,
	}

	if m.Layout == "" {
		m.Layout = "all"
	}

	if err := m.validate(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manifest) validate() error {
	hasBundle := m.Bundle != ""
	hasCherryPick := m.Instructions != "" || len(m.Skills) > 0 || len(m.Resources) > 0 || len(m.Agents) > 0 || len(m.PiExtensions) > 0 || len(m.Hooks) > 0

	if !hasBundle && !hasCherryPick {
		return fmt.Errorf("manifest must set either 'bundle' or cherry-pick fields ('instructions', 'skills', 'resources', 'agents', 'pi_extensions', 'hooks')")
	}
	if hasBundle && hasCherryPick {
		return fmt.Errorf("manifest cannot set both 'bundle' and cherry-pick fields ('instructions', 'skills', 'resources', 'agents', 'pi_extensions', 'hooks')")
	}

	if !hasBundle && (len(m.SkillsAdd) > 0 || len(m.SkillsRemove) > 0) {
		return fmt.Errorf("'skills_add' and 'skills_remove' require 'bundle' to be set")
	}

	if !hasBundle && (len(m.AgentsAdd) > 0 || len(m.AgentsRemove) > 0) {
		return fmt.Errorf("'agents_add' and 'agents_remove' require 'bundle' to be set")
	}

	if !hasBundle && (len(m.PiExtensionsAdd) > 0 || len(m.PiExtensionsRemove) > 0) {
		return fmt.Errorf("'pi_extensions_add' and 'pi_extensions_remove' require 'bundle' to be set")
	}

	if !hasBundle && (len(m.HooksAdd) > 0 || len(m.HooksRemove) > 0) {
		return fmt.Errorf("'hooks_add' and 'hooks_remove' require 'bundle' to be set")
	}

	return nil
}
