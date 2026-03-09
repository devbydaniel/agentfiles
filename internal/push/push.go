// Package push copies modified deployed files back to the store.
package push

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/devbydaniel/agentfiles/internal/fsutil"
	"github.com/devbydaniel/agentfiles/internal/lock"
	"github.com/devbydaniel/agentfiles/internal/store"
)

// Change describes a single asset that was (or would be) pushed.
type Change struct {
	Name    string // Asset name (or "agents_md" for the agent file).
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
	DryRun    bool
	SkillOnly string
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
	lockPath := filepath.Join(repoDir, lock.FileName)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no lock file found — run af apply first")
	}

	lf, err := lock.Load(repoDir)
	if err != nil {
		return nil, fmt.Errorf("loading lock file: %w", err)
	}

	res := &Result{}

	// Push agents_md (unless filtering by skill).
	if opts.SkillOnly == "" && lf.Deployed.AgentsMD != nil {
		e := lf.Deployed.AgentsMD
		s, err := entryStore(stores, defaultStore, e)
		if err != nil {
			return nil, fmt.Errorf("pushing agent md: %w", err)
		}
		deployed := filepath.Join(repoDir, e.DeployedPath)
		ch, err := pushFile(deployed, filepath.Join(s.Root, e.StorePath), e, "agents_md", lock.AssetAgentsMD, opts.DryRun)
		if err != nil {
			return nil, fmt.Errorf("pushing agent md: %w", err)
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
		if opts.SkillOnly != "" && name != opts.SkillOnly {
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

	// Push plugins (unless filtering by skill).
	if opts.SkillOnly == "" {
		for name, e := range lf.Deployed.Plugins {
			s, err := entryStore(stores, defaultStore, e)
			if err != nil {
				return nil, fmt.Errorf("pushing plugin %q: %w", name, err)
			}
			deployed := filepath.Join(repoDir, e.DeployedPath)
			ch, err := pushDir(deployed, filepath.Join(s.Root, e.StorePath), e, name, lock.AssetPlugins, opts.DryRun)
			if err != nil {
				return nil, fmt.Errorf("pushing plugin %q: %w", name, err)
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

	// Save updated lock file (only if not dry-run and there were changes).
	if !opts.DryRun && len(res.Changes) > 0 {
		if err := lock.Save(repoDir, lf); err != nil {
			return nil, fmt.Errorf("saving lock file: %w", err)
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
