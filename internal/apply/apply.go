// Package apply deploys assets from the store into a repository.
package apply

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/devbydaniel/agentfiles/internal/agent"
	"github.com/devbydaniel/agentfiles/internal/layout"
	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// Options controls apply behaviour.
type Options struct {
	Force        bool          // Overwrite existing files without prompting.
	SkillOnly    string        // If non-empty, deploy only this skill.
	LockFilePath string        // If non-empty, use this path for the lock file instead of <repoDir>/.agentfiles.lock.
	Layout       layout.Layout // If non-nil, use this layout instead of looking up by name.
}

// ApplyResult summarises what Apply did.
type ApplyResult struct {
	Deployed int
	Skipped  int
	Removed  int
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

	var lay layout.Layout
	if opts.Layout != nil {
		lay = opts.Layout
	} else {
		lay, err = layout.Get(resolved.Layout)
		if err != nil {
			return nil, err
		}
	}

	var oldLF *lock.LockFile
	if opts.LockFilePath != "" {
		oldLF, err = lock.LoadFrom(opts.LockFilePath)
	} else {
		oldLF, err = lock.Load(repoDir)
	}
	if err != nil {
		return nil, fmt.Errorf("loading lock file: %w", err)
	}

	// Start a fresh lock file so only currently-deployed assets are tracked.
	lf := &lock.LockFile{}
	lf.Deployed.Skills = make(map[string]*lock.Entry)
	lf.Deployed.Resources = make(map[string]*lock.Entry)
	lf.Deployed.Agents = make(map[string]*lock.Entry)
	lf.Deployed.PiExtensions = make(map[string]*lock.Entry)

	res := &ApplyResult{}

	// Deploy instruction md (unless skill-only filter is set).
	if opts.SkillOnly == "" && resolved.Instructions.Name != "" {
		s, err := store.LookupStore(stores, resolved.Instructions.Store)
		if err != nil {
			return nil, fmt.Errorf("instruction %q: %w", resolved.Instructions.Name, err)
		}
		src := filepath.Join(s.InstructionsDir(), resolved.Instructions.Name+".md")
		if _, err := os.Stat(src); err != nil {
			return nil, fmt.Errorf("instruction %q not found in store %q", resolved.Instructions.Name, resolved.Instructions.Store)
		}
		entries := lay.InstructionMdEntries()
		allSkipped := true
		for _, e := range entries {
			skipped, warn, err := deployEntry(repoDir, e, src, opts.Force)
			if err != nil {
				return nil, fmt.Errorf("deploying instruction md to %s: %w", e.Path, err)
			}
			if warn != "" {
				res.Warnings = append(res.Warnings, warn)
			}
			if !skipped {
				allSkipped = false
			}
		}
		h, err := lock.Hash(src)
		if err != nil {
			return nil, fmt.Errorf("hashing instruction md: %w", err)
		}
		relSource := filepath.Join("instructions", resolved.Instructions.Name+".md")
		deployedPath := primaryPath(entries)
		lk := lockKey(resolved.Instructions.Name, resolved.Instructions.Store, defaultStore)
		if err := lf.Record(lock.RecordParams{AssetType: lock.AssetInstructions, Name: lk, StoreName: resolved.Instructions.Store, SourcePath: relSource, DeployedPath: deployedPath, Hash: h}); err != nil {
			return nil, err
		}
		if allSkipped {
			res.Skipped++
		} else {
			res.Deployed++
		}
	}

	// Deploy skills.
	resolvedSkills := resolved.Skills
	if opts.SkillOnly != "" {
		found := false
		for _, sk := range resolved.Skills {
			// Match against StorePath (qualified) or Name (leaf) for convenience.
			if sk.StorePath == opts.SkillOnly || sk.Name == opts.SkillOnly {
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
		src := filepath.Join(s.SkillsDir(), filepath.FromSlash(skill.StorePath))
		if _, err := os.Stat(src); err != nil {
			return nil, fmt.Errorf("skill %q not found in store %q", skill.StorePath, skill.Store)
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
		h, err := lock.HashDir(src)
		if err != nil {
			return nil, fmt.Errorf("hashing skill %q: %w", skill.Name, err)
		}
		relSource := filepath.Join("skills", filepath.FromSlash(skill.StorePath)) + "/"
		deployedPath := primaryPath(entries)
		lk := lockKey(skill.StorePath, skill.Store, defaultStore)
		if err := lf.Record(lock.RecordParams{AssetType: lock.AssetSkills, Name: lk, StoreName: skill.Store, SourcePath: relSource, DeployedPath: deployedPath, Hash: h}); err != nil {
			return nil, err
		}
		if allSkipped {
			res.Skipped++
		} else {
			res.Deployed++
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
			h, err := lock.HashDir(src)
			if err != nil {
				return nil, fmt.Errorf("hashing resource %q: %w", resource.Name, err)
			}
			relSource := filepath.Join("resources", resource.Name) + "/"
			lk := lockKey(resource.Name, resource.Store, defaultStore)
			if err := lf.Record(lock.RecordParams{AssetType: lock.AssetResources, Name: lk, StoreName: resource.Store, SourcePath: relSource, DeployedPath: resource.Name, Hash: h}); err != nil {
				return nil, err
			}
			if skipped {
				res.Skipped++
			} else {
				res.Deployed++
			}
		}
	}

	// Deploy agents (unless skill-only filter is set).
	if opts.SkillOnly == "" {
		for _, ag := range resolved.Agents {
			entries := lay.AgentEntries(ag.Name)
			if len(entries) == 0 {
				continue // layout doesn't support agents (e.g., pi)
			}
			s, err := store.LookupStore(stores, ag.Store)
			if err != nil {
				return nil, fmt.Errorf("agent %q: %w", ag.Name, err)
			}
			src := filepath.Join(s.AgentsDir(), ag.Name+".md")
			srcData, err := os.ReadFile(src)
			if err != nil {
				return nil, fmt.Errorf("agent %q not found in store %q", ag.Name, ag.Store)
			}
			allSkipped := true
			for _, e := range entries {
				data, err := agentDataForEntry(e, srcData)
				if err != nil {
					return nil, fmt.Errorf("converting agent %q for %s: %w", ag.Name, e.Path, err)
				}
				skipped, warn, deployErr := deployFile(filepath.Join(repoDir, e.Path), data, opts.Force)
				if deployErr != nil {
					return nil, fmt.Errorf("deploying agent %q to %s: %w", ag.Name, e.Path, deployErr)
				}
				if warn != "" {
					res.Warnings = append(res.Warnings, warn)
				}
				if !skipped {
					allSkipped = false
				}
			}
			// Hash the canonical (parsed→re-serialized) form so that the lock
			// hash matches what push computes after round-tripping through
			// format conversions (e.g., Codex TOML → MD).
			fm, body, parseErr := agent.Parse(srcData)
			if parseErr != nil {
				return nil, fmt.Errorf("parsing agent %q for hashing: %w", ag.Name, parseErr)
			}
			canonical, canonErr := agent.ToMarkdown(fm, body)
			if canonErr != nil {
				return nil, fmt.Errorf("canonicalizing agent %q for hashing: %w", ag.Name, canonErr)
			}
			h := lock.HashBytes(canonical)
			relSource := filepath.Join("agents", ag.Name+".md")
			deployedPath := primaryPath(entries)
			lk := lockKey(ag.Name, ag.Store, defaultStore)
			if err := lf.Record(lock.RecordParams{AssetType: lock.AssetAgents, Name: lk, StoreName: ag.Store, SourcePath: relSource, DeployedPath: deployedPath, Hash: h}); err != nil {
				return nil, err
			}
			if allSkipped {
				res.Skipped++
			} else {
				res.Deployed++
			}
		}
	}

	// Deploy pi_extensions (unless skill-only filter is set).
	if opts.SkillOnly == "" {
		for _, ext := range resolved.PiExtensions {
			entries := lay.PiExtensionEntries(ext.Name)
			if len(entries) == 0 {
				continue // layout doesn't support pi extensions (e.g., claude, cursor, codex)
			}
			s, err := store.LookupStore(stores, ext.Store)
			if err != nil {
				return nil, fmt.Errorf("pi_extension %q: %w", ext.Name, err)
			}

			// Detect whether this is a single .ts file or a directory extension.
			fileSrc := filepath.Join(s.PiExtensionsDir(), ext.Name+".ts")
			dirSrc := filepath.Join(s.PiExtensionsDir(), ext.Name)

			isDir := false
			if info, err := os.Stat(dirSrc); err == nil && info.IsDir() {
				isDir = true
			} else if _, err := os.Stat(fileSrc); err != nil {
				return nil, fmt.Errorf("pi_extension %q not found in store %q", ext.Name, ext.Store)
			}

			allSkipped := true
			for _, e := range entries {
				if isDir {
					// Directory extension: deploy to <entry.Path>/
					skipped, warn, err := deployEntry(repoDir, e, dirSrc, opts.Force)
					if err != nil {
						return nil, fmt.Errorf("deploying pi_extension %q to %s: %w", ext.Name, e.Path, err)
					}
					if warn != "" {
						res.Warnings = append(res.Warnings, warn)
					}
					if !skipped {
						allSkipped = false
					}
				} else {
					// Single .ts file: deploy to <entry.Path>.ts
					data, err := os.ReadFile(fileSrc)
					if err != nil {
						return nil, fmt.Errorf("reading pi_extension %q: %w", ext.Name, err)
					}
					skipped, warn, err := deployFile(filepath.Join(repoDir, e.Path+".ts"), data, opts.Force)
					if err != nil {
						return nil, fmt.Errorf("deploying pi_extension %q to %s.ts: %w", ext.Name, e.Path, err)
					}
					if warn != "" {
						res.Warnings = append(res.Warnings, warn)
					}
					if !skipped {
						allSkipped = false
					}
				}
			}

			var h string
			var relSource string
			var deployedPath string
			if isDir {
				h, err = lock.HashDir(dirSrc)
				if err != nil {
					return nil, fmt.Errorf("hashing pi_extension %q: %w", ext.Name, err)
				}
				relSource = filepath.Join("pi_extensions", ext.Name) + "/"
				deployedPath = primaryPath(entries)
			} else {
				h, err = lock.Hash(fileSrc)
				if err != nil {
					return nil, fmt.Errorf("hashing pi_extension %q: %w", ext.Name, err)
				}
				relSource = filepath.Join("pi_extensions", ext.Name+".ts")
				deployedPath = primaryPath(entries) + ".ts"
			}
			lk := lockKey(ext.Name, ext.Store, defaultStore)
			if err := lf.Record(lock.RecordParams{AssetType: lock.AssetPiExtensions, Name: lk, StoreName: ext.Store, SourcePath: relSource, DeployedPath: deployedPath, Hash: h}); err != nil {
				return nil, err
			}
			if allSkipped {
				res.Skipped++
			} else {
				res.Deployed++
			}
		}
	}

	// Clean up assets that were in the old lock but not in the new one.
	// Skip cleanup when deploying a single skill (partial deploy).
	if opts.SkillOnly == "" {
		removed := pruneStale(repoDir, oldLF, lf)
		res.Removed = removed
	}

	if opts.LockFilePath != "" {
		err = lock.SaveTo(opts.LockFilePath, lf)
	} else {
		err = lock.Save(repoDir, lf)
	}
	if err != nil {
		return nil, fmt.Errorf("saving lock file: %w", err)
	}

	return res, nil
}

// pruneStale removes deployed files/directories that are tracked in oldLF but
// absent from newLF. Returns the number of assets removed.
func pruneStale(repoDir string, oldLF, newLF *lock.LockFile) int {
	removed := 0

	// Collect all deployed paths from the new lock.
	newPaths := make(map[string]bool)
	if newLF.Deployed.Instructions != nil {
		newPaths[newLF.Deployed.Instructions.DeployedPath] = true
	}
	for _, e := range newLF.Deployed.Skills {
		newPaths[e.DeployedPath] = true
	}
	for _, e := range newLF.Deployed.Resources {
		newPaths[e.DeployedPath] = true
	}
	for _, e := range newLF.Deployed.Agents {
		newPaths[e.DeployedPath] = true
	}
	for _, e := range newLF.Deployed.PiExtensions {
		newPaths[e.DeployedPath] = true
	}

	// Remove old instructions if no longer present.
	if oldLF.Deployed.Instructions != nil && !newPaths[oldLF.Deployed.Instructions.DeployedPath] {
		if removeDeployed(repoDir, oldLF.Deployed.Instructions.DeployedPath) {
			removed++
		}
	}

	// Remove old skills.
	for _, e := range oldLF.Deployed.Skills {
		if !newPaths[e.DeployedPath] {
			if removeDeployed(repoDir, e.DeployedPath) {
				removed++
			}
		}
	}

	// Remove old resources.
	for _, e := range oldLF.Deployed.Resources {
		if !newPaths[e.DeployedPath] {
			if removeDeployed(repoDir, e.DeployedPath) {
				removed++
			}
		}
	}

	// Remove old agents.
	for _, e := range oldLF.Deployed.Agents {
		if !newPaths[e.DeployedPath] {
			if removeDeployed(repoDir, e.DeployedPath) {
				removed++
			}
		}
	}

	// Remove old pi_extensions.
	for _, e := range oldLF.Deployed.PiExtensions {
		if !newPaths[e.DeployedPath] {
			if removeDeployed(repoDir, e.DeployedPath) {
				removed++
			}
		}
	}

	return removed
}

// removeDeployed removes a deployed file or directory. Returns true if something was removed.
func removeDeployed(repoDir, deployedPath string) bool {
	full := filepath.Join(repoDir, deployedPath)
	if _, err := os.Lstat(full); err != nil {
		return false
	}
	os.RemoveAll(full)
	return true
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

// agentDataForEntry returns the data to write for an agent entry.
// If the entry path ends with ".toml", the source Markdown is converted to
// Codex TOML format. Otherwise, the source is parsed and re-serialized to
// canonical Markdown so deployed files always match the lock hash.
func agentDataForEntry(e layout.Entry, srcData []byte) ([]byte, error) {
	fm, body, err := agent.Parse(srcData)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(e.Path, ".toml") {
		return agent.ToCodexTOML(fm, body)
	}
	return agent.ToMarkdown(fm, body)
}

// primaryPath returns the Path of the first entry.
func primaryPath(entries []layout.Entry) string {
	if len(entries) > 0 {
		return entries[0].Path
	}
	return ""
}
