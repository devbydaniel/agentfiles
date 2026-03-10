package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devbydaniel/agentfiles/internal/fsutil"
)

// AddSkill copies a skill directory into store/skills/<dirname>/.
// Returns the derived name and whether an existing entry was overwritten.
func (s *Store) AddSkill(srcPath string, force bool) (string, bool, error) {
	return s.addDir(srcPath, s.SkillsDir(), force)
}

// AddAgent copies a file into store/agents/<name>.md.
// Returns whether an existing agent was overwritten.
func (s *Store) AddAgent(srcPath, name string, force bool) (bool, error) {
	// Validate name: reject path separators and traversal.
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return false, fmt.Errorf("invalid agent name %q: must not contain path separators or '..'", name)
	}

	abs, err := filepath.Abs(srcPath)
	if err != nil {
		return false, fmt.Errorf("resolving source path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return false, fmt.Errorf("source path %q does not exist", srcPath)
	}
	if info.IsDir() {
		return false, fmt.Errorf("source path %q is a directory, expected a file", srcPath)
	}

	dest := filepath.Join(s.AgentsDir(), name+".md")

	// Ensure the resolved dest is still inside AgentsDir.
	cleanDest, err := filepath.Abs(dest)
	if err != nil {
		return false, fmt.Errorf("resolving dest path: %w", err)
	}
	agentsDir := s.AgentsDir()
	if !strings.HasPrefix(cleanDest, agentsDir+string(filepath.Separator)) && cleanDest != agentsDir {
		return false, fmt.Errorf("invalid agent name %q: resolved path escapes agents directory", name)
	}

	overwritten := false
	if _, err := os.Stat(dest); err == nil {
		if !force {
			return false, fmt.Errorf("agent %q already exists in store (use --force to overwrite)", name)
		}
		overwritten = true
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return false, fmt.Errorf("reading source: %w", err)
	}
	return overwritten, os.WriteFile(dest, data, 0o644)
}

// AddResource copies a resource directory into store/resources/<dirname>/ preserving structure.
// Returns the derived name and whether an existing entry was overwritten.
func (s *Store) AddResource(srcPath string, force bool) (string, bool, error) {
	return s.addDir(srcPath, s.ResourcesDir(), force)
}

// addDir copies srcPath (must be a directory) into parentDir/<basename>/.
// Returns the derived name and whether an existing entry was overwritten.
func (s *Store) addDir(srcPath, parentDir string, force bool) (string, bool, error) {
	abs, err := filepath.Abs(srcPath)
	if err != nil {
		return "", false, fmt.Errorf("resolving source path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", false, fmt.Errorf("source path %q does not exist", srcPath)
	}
	if !info.IsDir() {
		return "", false, fmt.Errorf("source path %q is not a directory", srcPath)
	}

	name := filepath.Base(abs)
	dest := filepath.Join(parentDir, name)

	overwritten := false
	if _, err := os.Stat(dest); err == nil {
		if !force {
			return "", false, fmt.Errorf("%q already exists in store (use --force to overwrite)", name)
		}
		overwritten = true
		if err := os.RemoveAll(dest); err != nil {
			return "", false, fmt.Errorf("removing existing %q: %w", name, err)
		}
	}

	return name, overwritten, fsutil.CopyDir(abs, dest)
}
