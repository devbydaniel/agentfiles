package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/lock"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show differences between deployed files and store sources",
	Long: `Show a line-by-line diff between deployed files in the current repo
and their corresponding source files in the store.

Reads .agentfiles.lock to find deployed assets and their store paths.
For directory assets (skills), diffs each file individually.
Prints "clean" when all deployed files match the store.

Useful for seeing what you've changed locally before running "af push",
or what's changed in the store since your last "af apply".`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stores, defaultStore, err := openStores()
		if err != nil {
			return err
		}

		lf, err := lock.Load(".")
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		found := false

		// Collect all entries
		type entryInfo struct {
			name       string
			storePath  string
			deployPath string
		}
		var entries []entryInfo

		if lf.Deployed.Instructions != nil {
			sp, err := entrySourcePath(lf.Deployed.Instructions, stores, defaultStore)
			if err != nil {
				return err
			}
			entries = append(entries, entryInfo{"instructions.md", sp, lf.Deployed.Instructions.DeployedPath})
		}
		for name, e := range lf.Deployed.Skills {
			sp, err := entrySourcePath(e, stores, defaultStore)
			if err != nil {
				return err
			}
			entries = append(entries, entryInfo{"skill:" + name, sp, e.DeployedPath})
		}
		for name, e := range lf.Deployed.Resources {
			sp, err := entrySourcePath(e, stores, defaultStore)
			if err != nil {
				return err
			}
			entries = append(entries, entryInfo{"resource:" + name, sp, e.DeployedPath})
		}
		for name, e := range lf.Deployed.Agents {
			sp, err := entrySourcePath(e, stores, defaultStore)
			if err != nil {
				return err
			}
			entries = append(entries, entryInfo{"agent:" + name, sp, e.DeployedPath})
		}
		for name, e := range lf.Deployed.PiExtensions {
			sp, err := entrySourcePath(e, stores, defaultStore)
			if err != nil {
				return err
			}
			entries = append(entries, entryInfo{"pi-extension:" + name, sp, e.DeployedPath})
		}

		for _, ei := range entries {
			srcInfo, srcErr := os.Stat(ei.storePath)
			dstInfo, dstErr := os.Stat(ei.deployPath)

			if srcErr != nil || dstErr != nil {
				continue
			}

			if srcInfo.IsDir() && dstInfo.IsDir() {
				// Diff each file in the directory
				err := filepath.WalkDir(ei.storePath, func(path string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() {
						return err
					}
					rel, _ := filepath.Rel(ei.storePath, path)
					deployFile := filepath.Join(ei.deployPath, rel)

					diff, err := fileDiff(path, deployFile, filepath.Join(ei.name, rel))
					if err != nil || diff == "" {
						return err
					}
					found = true
					fmt.Fprint(out, diff)
					return nil
				})
				if err != nil {
					return err
				}
			} else {
				diff, err := fileDiff(ei.storePath, ei.deployPath, ei.name)
				if err != nil {
					return err
				}
				if diff != "" {
					found = true
					fmt.Fprint(out, diff)
				}
			}
		}

		if !found {
			fmt.Fprintln(out, "clean")
		}
		return nil
	},
}

// fileDiff returns a simple unified-style diff between two files.
func fileDiff(srcPath, dstPath, label string) (string, error) {
	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}
	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("--- a/%s\n+++ /dev/null\n(file deleted)\n", label), nil
		}
		return "", err
	}

	if string(srcData) == string(dstData) {
		return "", nil
	}

	srcLines := strings.Split(string(srcData), "\n")
	dstLines := strings.Split(string(dstData), "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s (store)\n", label)
	fmt.Fprintf(&b, "+++ b/%s (deployed)\n", label)

	// Simple line-by-line comparison (not a true unified diff algorithm,
	// but sufficient for showing changes).
	maxLen := len(srcLines)
	if len(dstLines) > maxLen {
		maxLen = len(dstLines)
	}

	for i := 0; i < maxLen; i++ {
		var sl, dl string
		haveSrc := i < len(srcLines)
		haveDst := i < len(dstLines)
		if haveSrc {
			sl = srcLines[i]
		}
		if haveDst {
			dl = dstLines[i]
		}
		if haveSrc && haveDst && sl == dl {
			continue
		}
		if haveSrc {
			fmt.Fprintf(&b, "-%s\n", sl)
		}
		if haveDst {
			fmt.Fprintf(&b, "+%s\n", dl)
		}
	}
	return b.String(), nil
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
