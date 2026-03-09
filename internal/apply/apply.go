// Package apply deploys assets from the store into a repository.
package apply

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/devbydaniel/agentfiles/internal/layout"
	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/store"
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



// lockKey returns the key used in the lock file for an asset. Assets from the
// default store use just the name; assets from other stores are prefixed with
// "storename:".
func lockKey(name, storeName, defaultStore string) string {
	if storeName != defaultStore {
		return storeName + ":" + name
	}
	return name
}

// Apply resolves the manifest, determines the layout, and copies assets
// from the correct stores into repoDir. It writes a lock file after deployment.
//
// stores maps store names to open Store instances. defaultStore is the name
// used for assets without an explicit store prefix.
func Apply(stores map[string]*store.Store, defaultStore string, m *manifest.Manifest, repoDir string, opts Options) (*ApplyResult, error) {
	resolved, err := manifest.Resolve(m, stores, defaultStore)
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
	if opts.SkillOnly == "" && resolved.AgentsMd.Name != "" {
		s, err := store.LookupStore(stores, resolved.AgentsMd.Store)
		if err != nil {
			return nil, fmt.Errorf("agent %q: %w", resolved.AgentsMd.Name, err)
		}
		src := filepath.Join(s.AgentsDir(), resolved.AgentsMd.Name+".md")
		if _, err := os.Stat(src); err != nil {
			return nil, fmt.Errorf("agent %q not found in store %q", resolved.AgentsMd.Name, resolved.AgentsMd.Store)
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
			relSource := filepath.Join("agents", resolved.AgentsMd.Name+".md")
			deployedPath := primaryPath(entries)
			lk := lockKey(resolved.AgentsMd.Name, resolved.AgentsMd.Store, defaultStore)
			if err := lf.Record(lock.RecordParams{AssetType: lock.AssetAgentsMD, Name: lk, StoreName: resolved.AgentsMd.Store, SourcePath: relSource, DeployedPath: deployedPath, Hash: h}); err != nil {
				return nil, err
			}
			res.Deployed++
		}
	}

	// Deploy skills.
	resolvedSkills := resolved.Skills
	if opts.SkillOnly != "" {
		found := false
		for _, sk := range resolved.Skills {
			if sk.Name == opts.SkillOnly {
				resolvedSkills = []manifest.ResolvedAsset{sk}
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("skill %q not in manifest", opts.SkillOnly)
		}
	}
	for _, skill := range resolvedSkills {
		s, err := store.LookupStore(stores, skill.Store)
		if err != nil {
			return nil, fmt.Errorf("skill %q: %w", skill.Name, err)
		}
		src := filepath.Join(s.SkillsDir(), skill.Name)
		if _, err := os.Stat(src); err != nil {
			return nil, fmt.Errorf("skill %q not found in store %q", skill.Name, skill.Store)
		}
		entries := lay.SkillEntries(skill.Name)
		allSkipped := true
		for _, e := range entries {
			skipped, warn, err := deployEntry(repoDir, e, src, opts.Force)
			if err != nil {
				return nil, fmt.Errorf("deploying skill %q to %s: %w", skill.Name, e.Path, err)
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
				return nil, fmt.Errorf("hashing skill %q: %w", skill.Name, err)
			}
			relSource := filepath.Join("skills", skill.Name) + "/"
			deployedPath := primaryPath(entries)
			lk := lockKey(skill.Name, skill.Store, defaultStore)
			if err := lf.Record(lock.RecordParams{AssetType: lock.AssetSkills, Name: lk, StoreName: skill.Store, SourcePath: relSource, DeployedPath: deployedPath, Hash: h}); err != nil {
				return nil, err
			}
			res.Deployed++
		}
	}

	// Deploy plugins (unless skill-only filter).
	if opts.SkillOnly == "" {
		for _, plugin := range resolved.Plugins {
			s, err := store.LookupStore(stores, plugin.Store)
			if err != nil {
				return nil, fmt.Errorf("plugin %q: %w", plugin.Name, err)
			}
			src := filepath.Join(s.PluginsDir(), plugin.Name)
			if _, err := os.Stat(src); err != nil {
				return nil, fmt.Errorf("plugin %q not found in store %q", plugin.Name, plugin.Store)
			}
			entries := lay.PluginEntries(plugin.Name)
			allSkipped := true
			for _, e := range entries {
				skipped, warn, err := deployEntry(repoDir, e, src, opts.Force)
				if err != nil {
					return nil, fmt.Errorf("deploying plugin %q to %s: %w", plugin.Name, e.Path, err)
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
					return nil, fmt.Errorf("hashing plugin %q: %w", plugin.Name, err)
				}
				relSource := filepath.Join("plugins", plugin.Name) + "/"
				deployedPath := primaryPath(entries)
				lk := lockKey(plugin.Name, plugin.Store, defaultStore)
				if err := lf.Record(lock.RecordParams{AssetType: lock.AssetPlugins, Name: lk, StoreName: plugin.Store, SourcePath: relSource, DeployedPath: deployedPath, Hash: h}); err != nil {
					return nil, err
				}
				res.Deployed++
			}
		}
	}

	// Deploy resources (layout-independent, copy to repo root).
	if opts.SkillOnly == "" {
		for _, resource := range resolved.Resources {
			s, err := store.LookupStore(stores, resource.Store)
			if err != nil {
				return nil, fmt.Errorf("resource %q: %w", resource.Name, err)
			}
			src := filepath.Join(s.ResourcesDir(), resource.Name)
			if _, err := os.Stat(src); err != nil {
				return nil, fmt.Errorf("resource %q not found in store %q", resource.Name, resource.Store)
			}
			skipped, warn, err := deployResource(repoDir, src, opts.Force)
			if err != nil {
				return nil, fmt.Errorf("deploying resource %q: %w", resource.Name, err)
			}
			if warn != "" {
				res.Warnings = append(res.Warnings, warn)
			}
			if skipped {
				res.Skipped++
			} else {
				h, err := lock.HashDir(src)
				if err != nil {
					return nil, fmt.Errorf("hashing resource %q: %w", resource.Name, err)
				}
				relSource := filepath.Join("resources", resource.Name) + "/"
				lk := lockKey(resource.Name, resource.Store, defaultStore)
				if err := lf.Record(lock.RecordParams{AssetType: lock.AssetResources, Name: lk, StoreName: resource.Store, SourcePath: relSource, DeployedPath: resource.Name, Hash: h}); err != nil {
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
	return deployCopy(dest, src, force)
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

// primaryPath returns the Path of the first entry.
func primaryPath(entries []layout.Entry) string {
	if len(entries) > 0 {
		return entries[0].Path
	}
	return ""
}

