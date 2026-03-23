package store

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var subdirs = []string{"skills", "instructions", "resources", "bundles"}

// optionalSubdirs are created by Init but not required by Open (backward compat).
var optionalSubdirs = []string{"agents"}

// Store represents an opened agentfiles source store.
type Store struct {
	Root string
}

func (s *Store) SkillsDir() string       { return filepath.Join(s.Root, "skills") }
func (s *Store) InstructionsDir() string { return filepath.Join(s.Root, "instructions") }
func (s *Store) ResourcesDir() string   { return filepath.Join(s.Root, "resources") }
func (s *Store) BundlesDir() string     { return filepath.Join(s.Root, "bundles") }
func (s *Store) AgentsDir() string      { return filepath.Join(s.Root, "agents") }

// Open validates that path is a valid agentfiles store and returns a Store.
func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving store path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("store not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("store path %q is not a directory", abs)
	}

	// Verify .git directory exists — a store must be git-managed.
	gitDir := filepath.Join(abs, ".git")
	gi, err := os.Stat(gitDir)
	if err != nil || !gi.IsDir() {
		return nil, fmt.Errorf("store %q is not a git repository (missing .git)", abs)
	}

	for _, sub := range subdirs {
		p := filepath.Join(abs, sub)
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("store %q is missing required subdirectory %q", abs, sub)
		}
	}

	return &Store{Root: abs}, nil
}

// Init creates a new agentfiles store at path. It creates all required
// subdirectories and initialises a git repository. It is idempotent.
func Init(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving store path: %w", err)
	}

	for _, sub := range append(subdirs, optionalSubdirs...) {
		p := filepath.Join(abs, sub)
		if err := os.MkdirAll(p, 0o755); err != nil {
			return nil, fmt.Errorf("creating %q: %w", p, err)
		}
	}

	// git init is idempotent — safe to run on existing repos.
	gitDir := filepath.Join(abs, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		cmd := exec.Command("git", "init", "--", abs)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("git init: %w", err)
		}
	}

	return Open(abs)
}

// InitFromClone clones a git repo into path, then validates the store structure.
func InitFromClone(url, path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving store path: %w", err)
	}

	cmd := exec.Command("git", "clone", "--", url, abs)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git clone: %w", err)
	}

	// After cloning, ensure all subdirs exist (create any missing ones).
	for _, sub := range subdirs {
		p := filepath.Join(abs, sub)
		if err := os.MkdirAll(p, 0o755); err != nil {
			return nil, fmt.Errorf("creating %q: %w", p, err)
		}
	}

	return Open(abs)
}

// LookupStore retrieves a store by name from a map, returning a clear error
// if the name is not found or the value is nil.
func LookupStore(stores map[string]*Store, name string) (*Store, error) {
	s, ok := stores[name]
	if !ok || s == nil {
		return nil, fmt.Errorf("store %q not found", name)
	}
	return s, nil
}

// AgentInfo describes an agent found in the store.
type AgentInfo struct {
	Name string // Filename without .md extension.
}

// ListAgents walks agents/ for .md files and returns their names.
// Returns an empty slice if the agents/ directory does not exist.
func (s *Store) ListAgents() ([]AgentInfo, error) {
	agentsDir := s.AgentsDir()
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}

	var agents []AgentInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		agents = append(agents, AgentInfo{
			Name: strings.TrimSuffix(name, ".md"),
		})
	}
	return agents, nil
}

// SkillInfo describes a skill found in the store.
type SkillInfo struct {
	// GroupPath is the full group-qualified name, e.g., "tooling/browse" or "browse" (flat).
	GroupPath string
	// LeafName is the last path component, e.g., "browse".
	LeafName string
}

// ListSkills walks skills/ recursively and returns all directories containing SKILL.md.
// The walk stops descending into a directory once SKILL.md is found there — a skill
// directory cannot also be a group parent.
func (s *Store) ListSkills() ([]SkillInfo, error) {
	skillsDir := s.SkillsDir()
	var skills []SkillInfo

	err := filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		// Skip the root skills/ directory itself.
		if path == skillsDir {
			return nil
		}

		// Check if this directory contains SKILL.md.
		skillMD := filepath.Join(path, "SKILL.md")
		if _, statErr := os.Stat(skillMD); statErr == nil {
			rel, relErr := filepath.Rel(skillsDir, path)
			if relErr != nil {
				return relErr
			}
			// Normalize to forward slashes for consistency.
			groupPath := filepath.ToSlash(rel)
			skills = append(skills, SkillInfo{
				GroupPath: groupPath,
				LeafName:  filepath.Base(path),
			})
			return fs.SkipDir // Don't descend into skill directories.
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing skills: %w", err)
	}
	return skills, nil
}

// ResolveSkill takes a reference like "browse" or "tooling/browse" and returns
// the matching SkillInfo. For bare names (no slash), searches all groups —
// errors if ambiguous (multiple skills with the same leaf name).
// For qualified names (contains slash, no trailing slash), looks up directly.
func (s *Store) ResolveSkill(name string) (SkillInfo, error) {
	if strings.Contains(name, "/") {
		// Qualified name — direct lookup.
		skillMD := filepath.Join(s.SkillsDir(), filepath.FromSlash(name), "SKILL.md")
		if _, err := os.Stat(skillMD); err != nil {
			return SkillInfo{}, fmt.Errorf("skill %q not found in store", name)
		}
		return SkillInfo{
			GroupPath: name,
			LeafName:  filepath.Base(filepath.FromSlash(name)),
		}, nil
	}

	// Bare name — search all skills.
	all, err := s.ListSkills()
	if err != nil {
		return SkillInfo{}, err
	}

	var matches []SkillInfo
	for _, info := range all {
		if info.LeafName == name {
			matches = append(matches, info)
		}
	}

	switch len(matches) {
	case 0:
		return SkillInfo{}, fmt.Errorf("skill %q not found in store", name)
	case 1:
		return matches[0], nil
	default:
		var paths []string
		for _, m := range matches {
			paths = append(paths, m.GroupPath)
		}
		return SkillInfo{}, fmt.Errorf("ambiguous skill name %q: found in multiple groups: %s — qualify with group path", name, strings.Join(paths, ", "))
	}
}

// ExpandSkillGlob expands a skill reference. If pattern ends with "/", it's a
// group glob: returns all skills whose GroupPath starts with the prefix (recursive).
// If pattern doesn't end with "/", returns it as-is (single skill reference).
func (s *Store) ExpandSkillGlob(pattern string) ([]string, error) {
	if !strings.HasSuffix(pattern, "/") {
		return []string{pattern}, nil
	}

	prefix := strings.TrimSuffix(pattern, "/")
	all, err := s.ListSkills()
	if err != nil {
		return nil, err
	}

	var result []string
	for _, info := range all {
		if info.GroupPath == prefix || strings.HasPrefix(info.GroupPath, prefix+"/") {
			result = append(result, info.GroupPath)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no skills found matching glob %q", pattern)
	}
	return result, nil
}
