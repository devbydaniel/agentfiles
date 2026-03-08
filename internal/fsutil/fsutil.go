// Package fsutil provides shared file and directory copy utilities.
package fsutil

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyFile copies a single file from src to dst, creating parent directories
// and preserving file permissions.
func CopyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// CopyDir recursively copies src directory to dst, preserving structure.
func CopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return CopyFile(path, target)
	})
}

// SyncDir atomically replaces dst with the contents of src by copying to a
// temp directory first, then swapping via rename.
func SyncDir(src, dst string) error {
	parent := filepath.Dir(dst)
	tmp, err := os.MkdirTemp(parent, ".afsync-*")
	if err != nil {
		return err
	}

	if err := CopyDir(src, tmp); err != nil {
		os.RemoveAll(tmp)
		return err
	}

	// Remove old destination and swap in the new one.
	if err := os.RemoveAll(dst); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return nil
}
