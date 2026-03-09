package store

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var subdirs = []string{"skills", "agents", "plugins", "resources", "bundles"}

// Store represents an opened agentfiles source store.
type Store struct {
	Root string
}

func (s *Store) SkillsDir() string    { return filepath.Join(s.Root, "skills") }
func (s *Store) AgentsDir() string    { return filepath.Join(s.Root, "agents") }
func (s *Store) PluginsDir() string   { return filepath.Join(s.Root, "plugins") }
func (s *Store) ResourcesDir() string { return filepath.Join(s.Root, "resources") }
func (s *Store) BundlesDir() string   { return filepath.Join(s.Root, "bundles") }

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

	for _, sub := range subdirs {
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
