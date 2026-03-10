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
	Store        string `toml:"store,omitempty"`
	StorePath    string `toml:"source"`
	DeployedPath string `toml:"path"`
	Hash         string `toml:"hash"`
}

type DeployedMap struct {
	AgentsMD  *Entry            `toml:"agents_md,omitempty"`
	Skills    map[string]*Entry `toml:"skills,omitempty"`
	Plugins   map[string]*Entry `toml:"plugins,omitempty"`
	Resources map[string]*Entry `toml:"resources,omitempty"`
}

type LockFile struct {
	Deployed DeployedMap `toml:"deployed"`
}

// Load reads a lock file from the given directory using the standard file name.
func Load(dir string) (*LockFile, error) {
	return LoadFrom(filepath.Join(dir, FileName))
}

// LoadFrom reads a lock file from an explicit path.
func LoadFrom(path string) (*LockFile, error) {
	lf := &LockFile{}
	lf.Deployed.Skills = make(map[string]*Entry)
	lf.Deployed.Plugins = make(map[string]*Entry)
	lf.Deployed.Resources = make(map[string]*Entry)

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
	if lf.Deployed.Plugins == nil {
		lf.Deployed.Plugins = make(map[string]*Entry)
	}
	if lf.Deployed.Resources == nil {
		lf.Deployed.Resources = make(map[string]*Entry)
	}
	return lf, nil
}

// Save writes a lock file to the given directory using the standard file name.
func Save(dir string, lf *LockFile) error {
	return SaveTo(filepath.Join(dir, FileName), lf)
}

// SaveTo writes a lock file to an explicit path.
func SaveTo(path string, lf *LockFile) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
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

// RecordParams holds the parameters for recording a deployed asset in the lock file.
type RecordParams struct {
	AssetType    string
	Name         string
	StoreName    string
	SourcePath   string
	DeployedPath string
	Hash         string
}

func (lf *LockFile) Record(p RecordParams) error {
	assetType := p.AssetType
	name := p.Name
	entry := &Entry{Store: p.StoreName, StorePath: p.SourcePath, DeployedPath: p.DeployedPath, Hash: p.Hash}
	switch assetType {
	case AssetAgentsMD:
		lf.Deployed.AgentsMD = entry
	case AssetSkills:
		if lf.Deployed.Skills == nil {
			lf.Deployed.Skills = make(map[string]*Entry)
		}
		lf.Deployed.Skills[name] = entry
	case AssetPlugins:
		if lf.Deployed.Plugins == nil {
			lf.Deployed.Plugins = make(map[string]*Entry)
		}
		lf.Deployed.Plugins[name] = entry
	case AssetResources:
		if lf.Deployed.Resources == nil {
			lf.Deployed.Resources = make(map[string]*Entry)
		}
		lf.Deployed.Resources[name] = entry
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

// HashDirMapped hashes files using the directory structure from structDir but
// reading file contents from contentDir. Each relative path found in structDir
// is read from contentDir instead.
func HashDirMapped(structDir, contentDir string) (string, error) {
	var paths []string
	err := filepath.WalkDir(structDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(structDir, path)
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
		fh, err := Hash(filepath.Join(contentDir, rel))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "hash:%s\n", fh)
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
