package lock

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

const FileName = ".agentfiles.lock"

// Asset type constants for use with Record.
const (
	AssetAgentsMD  = "agents_md"
	AssetSkills    = "skills"
	AssetPlugins   = "plugins"
	AssetResources = "resources"
)

type Entry struct {
	Source string `toml:"source"`
	Hash   string `toml:"hash"`
}

type DeployedMap struct {
	AgentsMD *Entry            `toml:"agents_md,omitempty"`
	Skills   map[string]*Entry `toml:"skills,omitempty"`
}

type LockFile struct {
	Deployed DeployedMap `toml:"deployed"`
}

func Load(dir string) (*LockFile, error) {
	path := filepath.Join(dir, FileName)
	lf := &LockFile{}
	lf.Deployed.Skills = make(map[string]*Entry)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lf, nil
		}
		return nil, err
	}

	if err := toml.Unmarshal(data, lf); err != nil {
		return nil, err
	}
	if lf.Deployed.Skills == nil {
		lf.Deployed.Skills = make(map[string]*Entry)
	}
	return lf, nil
}

func Save(dir string, lf *LockFile) error {
	path := filepath.Join(dir, FileName)
	tmp, err := os.CreateTemp(dir, ".agentfiles.lock.tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	enc := toml.NewEncoder(tmp)
	if err := enc.Encode(lf); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func (lf *LockFile) Record(assetType, name, sourcePath, hash string) error {
	entry := &Entry{Source: sourcePath, Hash: hash}
	switch assetType {
	case AssetAgentsMD:
		lf.Deployed.AgentsMD = entry
	case AssetSkills:
		if lf.Deployed.Skills == nil {
			lf.Deployed.Skills = make(map[string]*Entry)
		}
		lf.Deployed.Skills[name] = entry
	case AssetPlugins, AssetResources:
		// Supported but not yet tracked in the lock file.
	default:
		return fmt.Errorf("unknown asset type: %q", assetType)
	}
	return nil
}

func Hash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func HashDir(dirPath string) (string, error) {
	var paths []string
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(paths)

	h := sha256.New()
	for _, rel := range paths {
		fmt.Fprintf(h, "file:%s\n", rel)
		fh, err := Hash(filepath.Join(dirPath, rel))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "hash:%s\n", fh)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
