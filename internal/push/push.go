// Package push copies modified deployed files back to the store.
package push

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devbydaniel/agentfiles/internal/agent"
	"github.com/devbydaniel/agentfiles/internal/fsutil"
	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// Change describes a single asset that was (or would be) pushed.
type Change struct {
	Name    string // Asset name (or "instructions" for the instruction file).
	Type    string // lock.Asset* constant.
	OldHash string
	NewHash string
}

// Result summarises a push operation.
type Result struct {
	Changes []Change
	Checked int
}

// Options controls push behaviour.
type Options struct {
	DryRun       bool
	SkillOnly    string
	LockFilePath string // If non-empty, use this path for the lock file instead of <repoDir>/.agentfiles.lock.
}

// entryStore returns the store for the given lock entry. If the entry has no
// Store field, the defaultStore is used.
func entryStore(stores map[string]*store.Store, defaultStore string, e *lock.Entry) (*store.Store, error) {
	name := e.Store
	if name == "" {
		name = defaultStore
	}
	return store.LookupStore(stores, name)
}

// Push compares deployed files to their lock hashes and copies changed files
// back to the store. Returns a result describing what changed.
func Push(stores map[string]*store.Store, defaultStore string, repoDir string, opts Options) (*Result, error) {
	lockPath := opts.LockFilePath
	if lockPath == "" {
		lockPath = filepath.Join(repoDir, lock.FileName)
	}
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no lock file found — run af apply first")
	}

	var (
		lf  *lock.LockFile
		err error
	)
	if opts.LockFilePath != "" {
		lf, err = lock.LoadFrom(opts.LockFilePath)
	} else {
		lf, err = lock.Load(repoDir)
	}
	if err != nil {
		return nil, fmt.Errorf("loading lock file: %w", err)
	}

	res := &Result{}

	// Push instructions (unless filtering by skill).
	if opts.SkillOnly == "" && lf.Deployed.Instructions != nil {
		e := lf.Deployed.Instructions
		s, err := entryStore(stores, defaultStore, e)
		if err != nil {
			return nil, fmt.Errorf("pushing instruction md: %w", err)
		}
		deployed := filepath.Join(repoDir, e.DeployedPath)
		ch, err := pushFile(deployed, filepath.Join(s.Root, e.StorePath), e, "instructions", lock.AssetInstructions, opts.DryRun)
		if err != nil {
			return nil, fmt.Errorf("pushing instruction md: %w", err)
		}
		res.Checked++
		if ch != nil {
			res.Changes = append(res.Changes, *ch)
			if !opts.DryRun {
				e.Hash = ch.NewHash
			}
		}
	}

	// Push skills.
	for name, e := range lf.Deployed.Skills {
		if opts.SkillOnly != "" && !matchSkillKey(name, opts.SkillOnly) {
			continue
		}
		s, err := entryStore(stores, defaultStore, e)
		if err != nil {
			return nil, fmt.Errorf("pushing skill %q: %w", name, err)
		}
		deployed := filepath.Join(repoDir, e.DeployedPath)
		ch, err := pushDir(deployed, filepath.Join(s.Root, e.StorePath), e, name, lock.AssetSkills, opts.DryRun)
		if err != nil {
			return nil, fmt.Errorf("pushing skill %q: %w", name, err)
		}
		res.Checked++
		if ch != nil {
			res.Changes = append(res.Changes, *ch)
			if !opts.DryRun {
				e.Hash = ch.NewHash
			}
		}
	}

	// Push resources (unless filtering by skill).
	// Resources are deployed by copying their contents into the repo root,
	// so the deployed path records the resource name; we reconstruct the
	// individual top-level children from the store's resource directory.
	if opts.SkillOnly == "" {
		for name, e := range lf.Deployed.Resources {
			s, err := entryStore(stores, defaultStore, e)
			if err != nil {
				return nil, fmt.Errorf("pushing resource %q: %w", name, err)
			}
			storeResDir := filepath.Join(s.Root, e.StorePath)
			ch, err := pushResource(repoDir, storeResDir, e, name, opts.DryRun)
			if err != nil {
				return nil, fmt.Errorf("pushing resource %q: %w", name, err)
			}
			res.Checked++
			if ch != nil {
				res.Changes = append(res.Changes, *ch)
				if !opts.DryRun {
					e.Hash = ch.NewHash
				}
			}
		}
	}

	// Push agents (unless filtering by skill).
	if opts.SkillOnly == "" {
		for name, e := range lf.Deployed.Agents {
			s, err := entryStore(stores, defaultStore, e)
			if err != nil {
				return nil, fmt.Errorf("pushing agent %q: %w", name, err)
			}
			deployed := filepath.Join(repoDir, e.DeployedPath)
			ch, err := pushAgent(deployed, filepath.Join(s.Root, e.StorePath), e, name, opts.DryRun)
			if err != nil {
				return nil, fmt.Errorf("pushing agent %q: %w", name, err)
			}
			res.Checked++
			if ch != nil {
				res.Changes = append(res.Changes, *ch)
				if !opts.DryRun {
					e.Hash = ch.NewHash
				}
			}
		}
	}

	// Push pi_extensions (unless filtering by skill).
	if opts.SkillOnly == "" {
		for name, e := range lf.Deployed.PiExtensions {
			s, err := entryStore(stores, defaultStore, e)
			if err != nil {
				return nil, fmt.Errorf("pushing pi_extension %q: %w", name, err)
			}
			deployed := filepath.Join(repoDir, e.DeployedPath)
			// Determine if file or directory from the deployed path.
			if strings.HasSuffix(e.DeployedPath, ".ts") {
				// Single .ts file.
				ch, err := pushFile(deployed, filepath.Join(s.Root, e.StorePath), e, name, lock.AssetPiExtensions, opts.DryRun)
				if err != nil {
					return nil, fmt.Errorf("pushing pi_extension %q: %w", name, err)
				}
				res.Checked++
				if ch != nil {
					res.Changes = append(res.Changes, *ch)
					if !opts.DryRun {
						e.Hash = ch.NewHash
					}
				}
			} else {
				// Directory extension.
				ch, err := pushDir(deployed, filepath.Join(s.Root, e.StorePath), e, name, lock.AssetPiExtensions, opts.DryRun)
				if err != nil {
					return nil, fmt.Errorf("pushing pi_extension %q: %w", name, err)
				}
				res.Checked++
				if ch != nil {
					res.Changes = append(res.Changes, *ch)
					if !opts.DryRun {
						e.Hash = ch.NewHash
					}
				}
			}
		}
	}

	// Save updated lock file (only if not dry-run and there were changes).
	if !opts.DryRun && len(res.Changes) > 0 {
		var saveErr error
		if opts.LockFilePath != "" {
			saveErr = lock.SaveTo(opts.LockFilePath, lf)
		} else {
			saveErr = lock.Save(repoDir, lf)
		}
		if saveErr != nil {
			return nil, fmt.Errorf("saving lock file: %w", saveErr)
		}
	}

	return res, nil
}

// pushFile checks a single deployed file against its lock hash.
// If changed and not dry-run, copies the deployed file to storeDest.
func pushFile(deployedPath, storeDest string, e *lock.Entry, name, assetType string, dryRun bool) (*Change, error) {
	newHash, err := lock.Hash(deployedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Change{Name: name, Type: assetType, OldHash: e.Hash, NewHash: "(deleted)"}, nil
		}
		return nil, err
	}
	if newHash == e.Hash {
		return nil, nil
	}
	ch := &Change{Name: name, Type: assetType, OldHash: e.Hash, NewHash: newHash}
	if !dryRun {
		if err := fsutil.CopyFile(deployedPath, storeDest); err != nil {
			return nil, err
		}
	}
	return ch, nil
}

// pushDir checks a deployed directory against its lock hash.
// If changed and not dry-run, copies the entire directory to storeDest.
func pushDir(deployedPath, storeDest string, e *lock.Entry, name, assetType string, dryRun bool) (*Change, error) {
	newHash, err := lock.HashDir(deployedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Change{Name: name, Type: assetType, OldHash: e.Hash, NewHash: "(deleted)"}, nil
		}
		return nil, err
	}
	if newHash == e.Hash {
		return nil, nil
	}
	ch := &Change{Name: name, Type: assetType, OldHash: e.Hash, NewHash: newHash}
	if !dryRun {
		if err := fsutil.SyncDir(deployedPath, storeDest); err != nil {
			return nil, err
		}
	}
	return ch, nil
}

// pushResource handles resources which are deployed as individual children
// into the repo root. It hashes the corresponding repo-side files by
// walking the store resource directory structure, then syncs back.
func pushResource(repoDir, storeResDir string, e *lock.Entry, name string, dryRun bool) (*Change, error) {
	// If the store resource dir doesn't exist, report as deleted.
	if _, err := os.Stat(storeResDir); os.IsNotExist(err) {
		return &Change{Name: name, Type: lock.AssetResources, OldHash: e.Hash, NewHash: "(deleted)"}, nil
	}

	// Hash the deployed resource files using the store directory structure
	// to know which repo-root paths belong to this resource.
	newHash, err := hashResourceDeployed(repoDir, storeResDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &Change{Name: name, Type: lock.AssetResources, OldHash: e.Hash, NewHash: "(deleted)"}, nil
		}
		return nil, err
	}
	if newHash == e.Hash {
		return nil, nil
	}
	ch := &Change{Name: name, Type: lock.AssetResources, OldHash: e.Hash, NewHash: newHash}
	if !dryRun {
		// Copy each deployed file back into the store resource directory.
		if err := syncResourceToStore(repoDir, storeResDir); err != nil {
			return nil, err
		}
	}
	return ch, nil
}

// hashResourceDeployed computes a hash of the deployed resource files in
// repoDir by walking the store resource directory to discover which paths
// belong to the resource.
func hashResourceDeployed(repoDir, storeResDir string) (string, error) {
	// Build a temporary view: for each file in the store resource dir,
	// hash the corresponding file in repoDir.
	// We reuse lock.HashDir but on a virtual mapping. Instead, we can
	// just hash the store resource dir structure but read from repoDir.
	// Simplest correct approach: hash by walking storeResDir for structure
	// but reading content from repoDir.
	return lock.HashDirMapped(storeResDir, repoDir)
}

// matchSkillKey checks if a lock key matches a skill-only filter.
// Matches against the full key (e.g., "tooling/browse" or "work:tooling/browse")
// and the leaf name (last path component, after stripping any store prefix).
func matchSkillKey(lockKey, filter string) bool {
	if lockKey == filter {
		return true
	}
	// Strip store prefix from the lock key to get the group-qualified path.
	qualifiedName := lockKey
	if idx := strings.Index(lockKey, ":"); idx > 0 {
		qualifiedName = lockKey[idx+1:]
	}
	// Match by leaf name (last path component).
	leaf := filepath.Base(qualifiedName)
	return leaf == filter || qualifiedName == filter
}

// pushAgent checks a deployed agent file against its lock hash.
// If the deployed file is a .toml (Codex), it converts back to canonical .md
// before hashing and pushing to the store. The hash is always computed on the
// canonical .md form (after conversion) to match what apply records.
func pushAgent(deployedPath, storeDest string, e *lock.Entry, name string, dryRun bool) (*Change, error) {
	// For agents, the lock hash is of the source .md file, not the deployed
	// file (which might be .toml). We need to:
	// 1. Read the deployed file
	// 2. If it's .toml, convert to .md
	// 3. Hash the converted .md (to compare with the lock hash of the source)
	// 4. If changed, write converted .md to store

	deployedData, err := os.ReadFile(deployedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Change{Name: name, Type: lock.AssetAgents, OldHash: e.Hash, NewHash: "(deleted)"}, nil
		}
		return nil, err
	}

	// Convert to canonical .md if needed.
	var mdData []byte
	if strings.HasSuffix(deployedPath, ".toml") {
		fm, body, err := agent.FromCodexTOML(deployedData)
		if err != nil {
			return nil, fmt.Errorf("converting from Codex TOML: %w", err)
		}
		mdData, err = agent.ToMarkdown(fm, body)
		if err != nil {
			return nil, fmt.Errorf("converting to Markdown: %w", err)
		}
	} else {
		fm, body, err := agent.Parse(deployedData)
		if err != nil {
			return nil, err
		}
		mdData, err = agent.ToMarkdown(fm, body)
		if err != nil {
			return nil, err
		}
	}

	// Hash the canonical .md data and compare to the lock hash (which was
	// computed from the source .md file).
	newHash := lock.HashBytes(mdData)
	if newHash == e.Hash {
		return nil, nil
	}

	ch := &Change{Name: name, Type: lock.AssetAgents, OldHash: e.Hash, NewHash: newHash}
	if !dryRun {
		// Write the canonical .md to the store path.
		if err := os.MkdirAll(filepath.Dir(storeDest), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(storeDest, mdData, 0o644); err != nil {
			return nil, err
		}
	}
	return ch, nil
}

// syncResourceToStore copies each deployed resource file from repoDir back
// to the store resource directory, preserving structure.
func syncResourceToStore(repoDir, storeResDir string) error {
	// Walk the store resource dir to find what files exist, then copy
	// each corresponding file from repoDir back.
	entries, err := os.ReadDir(storeResDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		repoPath := filepath.Join(repoDir, entry.Name())
		storePath := filepath.Join(storeResDir, entry.Name())
		if entry.IsDir() {
			if err := fsutil.SyncDir(repoPath, storePath); err != nil {
				return err
			}
		} else {
			if err := fsutil.CopyFile(repoPath, storePath); err != nil {
				return err
			}
		}
	}
	return nil
}
