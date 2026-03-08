// Package apply deploys assets from the store into a repository.
package apply

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/danielbenner/agentfiles/internal/layout"
	"github.com/danielbenner/agentfiles/internal/lock"
	"github.com/danielbenner/agentfiles/internal/manifest"
	"github.com/danielbenner/agentfiles/internal/store"
)

// Options controls apply behaviour.
type Options struct {
	Force     bool   // Overwrite existing files without prompting.
	SkillOnly string // If non-empty, deploy only this skill.
}

// ApplyResult summarises what Apply did.
type ApplyResult struct {
	Deployed int
	Skipped  int
	Warnings []string
}

// Apply resolves the manifest, determines the layout, and copies assets
// from the store into repoDir. It writes a lock file after deployment.
func Apply(s *store.Store, m *manifest.Manifest, repoDir string, opts Options) (*ApplyResult, error) {
	resolved, err := manifest.Resolve(m, s)
	if err != nil {
		return nil, fmt.Errorf("resolving manifest: %w", err)
	}

	lay, err := layout.Get(resolved.Layout)
	if err != nil {
		return nil, err
	}

	lf, err := lock.Load(repoDir)
	if err != nil {
		return nil, fmt.Errorf("loading lock file: %w", err)
	}

	res := &ApplyResult{}

	// Deploy agent md (unless skill-only filter is set).
	if opts.SkillOnly == "" && resolved.AgentsMd != "" {
		src := filepath.Join(s.AgentsDir(), resolved.AgentsMd+".md")
		if _, err := os.Stat(src); err != nil {
			return nil, fmt.Errorf("agent %q not found in store", resolved.AgentsMd)
		}
		entries := lay.AgentMdEntries()
		allSkipped := true
		for _, e := range entries {
			skipped, warn, err := deployEntry(repoDir, e, src, opts.Force)
			if err != nil {
				return nil, fmt.Errorf("deploying agent md to %s: %w", e.Path, err)
			}
			if warn != "" {
				res.Warnings = append(res.Warnings, warn)
			}
			if !skipped {
				allSkipped = false
			}
		}
		if allSkipped {
			res.Skipped++
		} else {
			h, err := lock.Hash(src)
			if err != nil {
				return nil, fmt.Errorf("hashing agent md: %w", err)
			}
			relSource := filepath.Join("agents", resolved.AgentsMd+".md")
			if err := lf.Record(lock.AssetAgentsMD, resolved.AgentsMd, relSource, h); err != nil {
				return nil, err
			}
			res.Deployed++
		}
	}

	// Deploy skills.
	skills := resolved.Skills
	if opts.SkillOnly != "" {
		if !contains(resolved.Skills, opts.SkillOnly) {
			return nil, fmt.Errorf("skill %q not in manifest", opts.SkillOnly)
		}
		skills = []string{opts.SkillOnly}
	}
	for _, name := range skills {
		src := filepath.Join(s.SkillsDir(), name)
		if _, err := os.Stat(src); err != nil {
			return nil, fmt.Errorf("skill %q not found in store", name)
		}
		entries := lay.SkillEntries(name)
		allSkipped := true
		for _, e := range entries {
			skipped, warn, err := deployEntry(repoDir, e, src, opts.Force)
			if err != nil {
				return nil, fmt.Errorf("deploying skill %q to %s: %w", name, e.Path, err)
			}
			if warn != "" {
				res.Warnings = append(res.Warnings, warn)
			}
			if !skipped {
				allSkipped = false
			}
		}
		if allSkipped {
			res.Skipped++
		} else {
			h, err := lock.HashDir(src)
			if err != nil {
				return nil, fmt.Errorf("hashing skill %q: %w", name, err)
			}
			relSource := filepath.Join("skills", name) + "/"
			if err := lf.Record(lock.AssetSkills, name, relSource, h); err != nil {
				return nil, err
			}
			res.Deployed++
		}
	}

	// Deploy plugins (unless skill-only filter).
	if opts.SkillOnly == "" {
		for _, name := range resolved.Plugins {
			src := filepath.Join(s.PluginsDir(), name)
			if _, err := os.Stat(src); err != nil {
				return nil, fmt.Errorf("plugin %q not found in store", name)
			}
			entries := lay.PluginEntries(name)
			allSkipped := true
			for _, e := range entries {
				skipped, warn, err := deployEntry(repoDir, e, src, opts.Force)
				if err != nil {
					return nil, fmt.Errorf("deploying plugin %q to %s: %w", name, e.Path, err)
				}
				if warn != "" {
					res.Warnings = append(res.Warnings, warn)
				}
				if !skipped {
					allSkipped = false
				}
			}
			if allSkipped {
				res.Skipped++
			} else {
				h, err := lock.HashDir(src)
				if err != nil {
					return nil, fmt.Errorf("hashing plugin %q: %w", name, err)
				}
				relSource := filepath.Join("plugins", name) + "/"
				if err := lf.Record(lock.AssetPlugins, name, relSource, h); err != nil {
					return nil, err
				}
				res.Deployed++
			}
		}
	}

	// Deploy resources (layout-independent, copy to repo root).
	if opts.SkillOnly == "" {
		for _, name := range resolved.Resources {
			src := filepath.Join(s.ResourcesDir(), name)
			if _, err := os.Stat(src); err != nil {
				return nil, fmt.Errorf("resource %q not found in store", name)
			}
			skipped, warn, err := deployResource(repoDir, src, opts.Force)
			if err != nil {
				return nil, fmt.Errorf("deploying resource %q: %w", name, err)
			}
			if warn != "" {
				res.Warnings = append(res.Warnings, warn)
			}
			if skipped {
				res.Skipped++
			} else {
				h, err := lock.HashDir(src)
				if err != nil {
					return nil, fmt.Errorf("hashing resource %q: %w", name, err)
				}
				relSource := filepath.Join("resources", name) + "/"
				if err := lf.Record(lock.AssetResources, name, relSource, h); err != nil {
					return nil, err
				}
				res.Deployed++
			}
		}
	}

	if err := lock.Save(repoDir, lf); err != nil {
		return nil, fmt.Errorf("saving lock file: %w", err)
	}

	return res, nil
}

// deployEntry handles a single layout entry, writing content from src.
// Returns (skipped, warning, error).
func deployEntry(repoDir string, e layout.Entry, src string, force bool) (bool, string, error) {
	dest := filepath.Join(repoDir, e.Path)

	switch e.Kind {
	case layout.KindRegular:
		return deployCopy(dest, src, force)
	case layout.KindPointer:
		return deployFile(dest, []byte(e.Target), force)
	case layout.KindSymlink:
		return deploySymlink(dest, e.Target, force)
	default:
		return false, "", fmt.Errorf("unknown entry kind %d", e.Kind)
	}
}

// deployCopy copies a file or directory from src to dest.
func deployCopy(dest, src string, force bool) (bool, string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return false, "", err
	}

	if info.IsDir() {
		return copyDir(dest, src, force)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return false, "", err
	}
	return deployFile(dest, data, force)
}

// deployFile writes data to dest, respecting force flag.
func deployFile(dest string, data []byte, force bool) (bool, string, error) {
	if !force {
		if _, err := os.Lstat(dest); err == nil {
			warn := fmt.Sprintf("warning: skipping %s (already exists, use --force to overwrite)", dest)
			return true, warn, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false, "", err
	}
	// Remove existing (could be symlink or regular file).
	os.Remove(dest)
	return false, "", os.WriteFile(dest, data, 0o644)
}

// deploySymlink creates a symlink at dest pointing to target.
func deploySymlink(dest, target string, force bool) (bool, string, error) {
	if !force {
		if _, err := os.Lstat(dest); err == nil {
			warn := fmt.Sprintf("warning: skipping %s (already exists, use --force to overwrite)", dest)
			return true, warn, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false, "", err
	}
	os.Remove(dest)
	return false, "", os.Symlink(target, dest)
}

// copyDir recursively copies src directory to dest.
func copyDir(dest, src string, force bool) (bool, string, error) {
	anyWritten := false
	var firstWarn string
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		skipped, warn, err := deployFile(target, data, force)
		if err != nil {
			return err
		}
		if warn != "" && firstWarn == "" {
			firstWarn = warn
		}
		if !skipped {
			anyWritten = true
		}
		return nil
	})
	if err != nil {
		return false, "", err
	}
	return !anyWritten, firstWarn, nil
}

// deployResource copies the contents of a resource directory to repo root.
func deployResource(repoDir, srcDir string, force bool) (bool, string, error) {
	return copyDir(repoDir, srcDir, force)
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
