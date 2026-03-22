package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devbydaniel/agentfiles/internal/fsutil"
)

// AddSkill copies a skill directory into the store's skills directory.
// If group is non-empty, the skill is placed in skills/<group>/<leafname>/
// instead of skills/<leafname>/. Returns the group-qualified name (e.g.,
// "tooling/browse" or just "browse" for flat) and whether an existing entry
// was overwritten.
func (s *Store) AddSkill(srcPath string, group string, force bool) (string, bool, error) {
	if group != "" {
		// Validate group: no ".." components, no absolute paths.
		if filepath.IsAbs(group) {
			return "", false, fmt.Errorf("invalid group %q: must not be an absolute path", group)
		}
		for _, part := range strings.Split(filepath.ToSlash(group), "/") {
			if part == ".." || part == "." {
				return "", false, fmt.Errorf("invalid group %q: must not contain '.' or '..' components", group)
			}
		}
		parentDir := filepath.Join(s.SkillsDir(), filepath.FromSlash(group))
		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			return "", false, fmt.Errorf("creating group directory: %w", err)
		}
		leafName, overwritten, err := s.addDir(srcPath, parentDir, force)
		if err != nil {
			return "", false, err
		}
		qualifiedName := group + "/" + leafName
		return qualifiedName, overwritten, nil
	}
	return s.addDir(srcPath, s.SkillsDir(), force)
}

// AddInstruction copies a file into store/instructions/<name>.md.
// Returns whether an existing instruction was overwritten.
func (s *Store) AddInstruction(srcPath, name string, force bool) (bool, error) {
	// Validate name: reject path separators and traversal.
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return false, fmt.Errorf("invalid instruction name %q: must not contain path separators or '..'", name)
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

	dest := filepath.Join(s.InstructionsDir(), name+".md")

	// Ensure the resolved dest is still inside InstructionsDir.
	cleanDest, err := filepath.Abs(dest)
	if err != nil {
		return false, fmt.Errorf("resolving dest path: %w", err)
	}
	instructionsDir := s.InstructionsDir()
	if !strings.HasPrefix(cleanDest, instructionsDir+string(filepath.Separator)) && cleanDest != instructionsDir {
		return false, fmt.Errorf("invalid instruction name %q: resolved path escapes instructions directory", name)
	}

	overwritten := false
	if _, err := os.Stat(dest); err == nil {
		if !force {
			return false, fmt.Errorf("instruction %q already exists in store (use --force to overwrite)", name)
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
