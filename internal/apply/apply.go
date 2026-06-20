// Package apply deploys assets from the store into a repository.
package apply

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/devbydaniel/agentfiles/internal/agent"
	"github.com/devbydaniel/agentfiles/internal/fsutil"
	"github.com/devbydaniel/agentfiles/internal/hooks"
	"github.com/devbydaniel/agentfiles/internal/layout"
	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// hookDeployBase is the repo- or home-relative directory under which agentfiles
// deploys directory-form hook contents. For user-level applies this becomes
// $HOME/<hookDeployBaseUser>/<name>; for repo-level it becomes
// <repoDir>/<hookDeployBaseRepo>/<name>.
//
// hookDeployBaseRepo must NOT live under ".agentfiles/" — that path is the repo
// manifest *file* (".agentfiles"), so ".agentfiles/hooks" collides with it and
// every repo-level apply fails with "open .agentfiles/hooks: not a directory".
// It sits beside the manifest instead, mirroring ".agentfiles.lock".
const (
	hookDeployBaseUser = ".local/share/agentfiles/hooks"
	hookDeployBaseRepo = ".agentfiles-hooks"
)

// isUserLayout reports whether a layout name targets user-level paths
// (e.g. $HOME/.claude/settings.json) rather than a repo root.
func isUserLayout(layoutName string) bool {
	return strings.HasPrefix(layoutName, "user-")
}

// hookDeployDir returns the repoDir-relative path where a directory-form hook's
// contents are deployed. Used both to actually copy files and as the lock
// entry's ExtraPath for pruning.
func hookDeployDir(userLevel bool, name string) string {
	if userLevel {
		return filepath.Join(hookDeployBaseUser, name)
	}
	return filepath.Join(hookDeployBaseRepo, name)
}

// hookRootToken returns the shell-portable string that replaces ${AF_HOOK_ROOT}
// in a hook's commands. User-level uses $HOME-prefixed absolute form so the
// hook is portable across machines; repo-level uses a relative path resolved
// against the hook's cwd at execution time (the project root).
func hookRootToken(userLevel bool, name string) string {
	if userLevel {
		return "$HOME/" + hookDeployBaseUser + "/" + name
	}
	return hookDeployBaseRepo + "/" + name
}

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
	lf.Deployed.Hooks = make(map[string]*lock.Entry)

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
		deployedPath, extraPaths := primaryPath(entries), extraPaths(entries)
		lk := lockKey(resolved.Instructions.Name, resolved.Instructions.Store, defaultStore)
		if err := lf.Record(lock.RecordParams{AssetType: lock.AssetInstructions, Name: lk, StoreName: resolved.Instructions.Store, SourcePath: relSource, DeployedPath: deployedPath, ExtraPaths: extraPaths, Hash: h}); err != nil {
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
		deployedPath, extras := primaryPath(entries), extraPaths(entries)
		lk := lockKey(skill.StorePath, skill.Store, defaultStore)
		if err := lf.Record(lock.RecordParams{AssetType: lock.AssetSkills, Name: lk, StoreName: skill.Store, SourcePath: relSource, DeployedPath: deployedPath, ExtraPaths: extras, Hash: h}); err != nil {
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
			deployedPath, extras := primaryPath(entries), extraPaths(entries)
			lk := lockKey(ag.Name, ag.Store, defaultStore)
			if err := lf.Record(lock.RecordParams{AssetType: lock.AssetAgents, Name: lk, StoreName: ag.Store, SourcePath: relSource, DeployedPath: deployedPath, ExtraPaths: extras, Hash: h}); err != nil {
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
			var extras []string
			if isDir {
				h, err = lock.HashDir(dirSrc)
				if err != nil {
					return nil, fmt.Errorf("hashing pi_extension %q: %w", ext.Name, err)
				}
				relSource = filepath.Join("pi_extensions", ext.Name) + "/"
				deployedPath = primaryPath(entries)
				extras = extraPaths(entries)
			} else {
				h, err = lock.Hash(fileSrc)
				if err != nil {
					return nil, fmt.Errorf("hashing pi_extension %q: %w", ext.Name, err)
				}
				relSource = filepath.Join("pi_extensions", ext.Name+".ts")
				deployedPath = primaryPath(entries) + ".ts"
				for _, p := range extraPaths(entries) {
					extras = append(extras, p+".ts")
				}
			}
			lk := lockKey(ext.Name, ext.Store, defaultStore)
			if err := lf.Record(lock.RecordParams{AssetType: lock.AssetPiExtensions, Name: lk, StoreName: ext.Store, SourcePath: relSource, DeployedPath: deployedPath, ExtraPaths: extras, Hash: h}); err != nil {
				return nil, err
			}
			if allSkipped {
				res.Skipped++
			} else {
				res.Deployed++
			}
		}
	}

	// Deploy hooks (merge into settings files for Claude/Codex/Cursor).
	if opts.SkillOnly == "" && len(resolved.Hooks) > 0 {
		targets := hooks.TargetsForLayout(lay.Name())
		if len(targets) > 0 {
			userLevel := isUserLayout(lay.Name())
			// All resolved hooks are placed into managed so MergeIntoSettings,
			// which strips every _agentfiles entry before re-adding, doesn't
			// drop entries whose source content was unchanged on this apply.
			managed := make(map[string]*hooks.HookFile)
			needsMerge := false
			for _, hk := range resolved.Hooks {
				s, err := store.LookupStore(stores, hk.Store)
				if err != nil {
					return nil, fmt.Errorf("hook %q: %w", hk.Name, err)
				}
				hookPath, isDir, err := s.HookPath(hk.Name)
				if err != nil {
					return nil, fmt.Errorf("hook %q in store %q: %w", hk.Name, hk.Store, err)
				}

				var (
					hf              *hooks.HookFile
					hash            string
					sourcePath      string
					extraPaths      []string
					deployedHookDir string
				)

				if isDir {
					loaded, _, err := hooks.LoadFromDir(hookPath)
					if err != nil {
						return nil, fmt.Errorf("loading hook %q: %w", hk.Name, err)
					}
					dirHash, err := lock.HashDir(hookPath)
					if err != nil {
						return nil, fmt.Errorf("hashing hook %q: %w", hk.Name, err)
					}
					hash = dirHash
					sourcePath = filepath.Join("hooks", hk.Name) + "/"

					hookRootRel := hookDeployDir(userLevel, hk.Name)
					deployedHookDir = hookRootRel
					extraPaths = []string{hookRootRel}

					subbed, err := hooks.Substitute(loaded, hookRootToken(userLevel, hk.Name))
					if err != nil {
						return nil, fmt.Errorf("substituting placeholders in hook %q: %w", hk.Name, err)
					}
					hf = subbed
				} else {
					data, err := os.ReadFile(hookPath)
					if err != nil {
						return nil, fmt.Errorf("hook %q not found in store %q", hk.Name, hk.Store)
					}
					parsed, err := hooks.Parse(data)
					if err != nil {
						return nil, fmt.Errorf("parsing hook %q: %w", hk.Name, err)
					}
					hf = parsed
					hash = lock.HashBytes(data)
					sourcePath = filepath.Join("hooks", hk.Name+".json")
				}

				lk := lockKey(hk.Name, hk.Store, defaultStore)

				oldEntry, existed := oldLF.Deployed.Hooks[lk]
				if existed && oldEntry.Hash == hash && !opts.Force {
					res.Skipped++
				} else {
					needsMerge = true
					res.Deployed++
					if isDir {
						fullDest := filepath.Join(repoDir, deployedHookDir)
						if err := os.RemoveAll(fullDest); err != nil {
							return nil, fmt.Errorf("clearing hook deploy dir %s: %w", fullDest, err)
						}
						if err := fsutil.CopyDir(hookPath, fullDest); err != nil {
							return nil, fmt.Errorf("deploying hook %q scripts: %w", hk.Name, err)
						}
					}
				}

				// Dir-form hooks: even on skip, ensure scripts exist on disk
				// (a previous apply may have been interrupted after the lock
				// was updated but before the scripts were written).
				if isDir {
					fullDest := filepath.Join(repoDir, deployedHookDir)
					if _, err := os.Stat(fullDest); os.IsNotExist(err) {
						if err := fsutil.CopyDir(hookPath, fullDest); err != nil {
							return nil, fmt.Errorf("restoring hook %q scripts: %w", hk.Name, err)
						}
					}
				}

				managed[hk.Name] = hf

				if err := lf.Record(lock.RecordParams{
					AssetType:    lock.AssetHooks,
					Name:         lk,
					StoreName:    hk.Store,
					SourcePath:   sourcePath,
					DeployedPath: targets[0].Path,
					ExtraPaths:   extraPaths,
					Hash:         hash,
				}); err != nil {
					return nil, err
				}
			}
			if needsMerge {
				for _, t := range targets {
					fullPath := filepath.Join(repoDir, t.Path)
					if err := hooks.MergeIntoSettings(fullPath, managed, t.Format); err != nil {
						return nil, fmt.Errorf("deploying hooks to %s: %w", t.Path, err)
					}
				}
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

	// Collect all deployed paths from the new lock (primary + extras) so we
	// don't remove a path that's still legitimately in use under a different
	// asset's multi-entry deployment.
	newPaths := make(map[string]bool)
	addPaths := func(e *lock.Entry) {
		for _, p := range e.AllDeployedPaths() {
			newPaths[p] = true
		}
	}
	if newLF.Deployed.Instructions != nil {
		addPaths(newLF.Deployed.Instructions)
	}
	for _, e := range newLF.Deployed.Skills {
		addPaths(e)
	}
	for _, e := range newLF.Deployed.Resources {
		addPaths(e)
	}
	for _, e := range newLF.Deployed.Agents {
		addPaths(e)
	}
	for _, e := range newLF.Deployed.PiExtensions {
		addPaths(e)
	}
	for _, e := range newLF.Deployed.Hooks {
		addPaths(e)
	}

	// Build set of new hook names.
	newHookNames := make(map[string]bool)
	for name := range newLF.Deployed.Hooks {
		newHookNames[name] = true
	}

	// pruneEntry removes every deployed path for the entry that isn't present
	// in the new lock. Counted as one asset if anything was removed, to keep
	// the summary as "N stale assets" rather than "N stale files".
	pruneEntry := func(e *lock.Entry) {
		any := false
		for _, p := range e.AllDeployedPaths() {
			if newPaths[p] {
				continue
			}
			if removeDeployed(repoDir, p) {
				any = true
			}
		}
		if any {
			removed++
		}
	}

	// Remove old instructions if no longer present.
	if oldLF.Deployed.Instructions != nil {
		pruneEntry(oldLF.Deployed.Instructions)
	}

	// Remove old skills.
	for _, e := range oldLF.Deployed.Skills {
		pruneEntry(e)
	}

	// Remove old resources.
	for _, e := range oldLF.Deployed.Resources {
		pruneEntry(e)
	}

	// Remove old agents.
	for _, e := range oldLF.Deployed.Agents {
		pruneEntry(e)
	}

	// Remove old pi_extensions.
	for _, e := range oldLF.Deployed.PiExtensions {
		pruneEntry(e)
	}

	// Remove old hooks from settings files.
	// Try all possible target files (not just the current layout) so that
	// layout changes don't leave orphaned entries.
	var staleHookNames []string
	for name, e := range oldLF.Deployed.Hooks {
		if !newHookNames[name] {
			staleHookNames = append(staleHookNames, name)
			// Directory-form hooks record their deployed content dir in
			// ExtraPaths; remove it so leftover scripts don't linger.
			for _, p := range e.ExtraPaths {
				removeDeployed(repoDir, p)
			}
		}
	}
	if len(staleHookNames) > 0 {
		for _, t := range hooks.AllTargets() {
			fullPath := filepath.Join(repoDir, t.Path)
			if err := hooks.RemoveManaged(fullPath, staleHookNames); err != nil {
				// Log but continue — partial cleanup is better than none.
				_ = err
			}
		}
		removed += len(staleHookNames)
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

// extraPaths returns the Paths of all entries after the first. Used alongside
// primaryPath so multi-entry layouts (e.g. UserAllLayout) record every copy
// they wrote, which pruneStale needs to clean up when an asset is removed.
func extraPaths(entries []layout.Entry) []string {
	if len(entries) <= 1 {
		return nil
	}
	out := make([]string, 0, len(entries)-1)
	for _, e := range entries[1:] {
		out = append(out, e.Path)
	}
	return out
}
