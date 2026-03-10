package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/lock"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of deployed files compared to lock and store",
	Long: `Show the sync status of every deployed asset.

Compares three versions of each asset:
  - The deployed file in the repo (what's on disk now)
  - The lock file hash (what was deployed by "af apply")
  - The store file (the current source of truth)

Possible states:
  unchanged         Deployed = lock = store (everything in sync)
  modified locally  Deployed ≠ lock (you edited the file; push to propagate)
  modified in store Store ≠ lock (store updated; apply --force to update)
  conflict          Both deployed and store differ from lock
  missing           File not found on disk or in store

Requires a prior "af apply" (needs .agentfiles.lock).`,
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

		type item struct {
			name       string
			storePath  string
			deployPath string
			lockHash   string
			isDir      bool
		}
		var items []item

		if lf.Deployed.AgentsMD != nil {
			e := lf.Deployed.AgentsMD
			sp, err := entrySourcePath(e, stores, defaultStore)
			if err != nil {
				return err
			}
			items = append(items, item{"agents.md", sp, e.DeployedPath, e.Hash, false})
		}
		for name, e := range lf.Deployed.Skills {
			sp, err := entrySourcePath(e, stores, defaultStore)
			if err != nil {
				return err
			}
			items = append(items, item{"skill:" + name, sp, e.DeployedPath, e.Hash, true})
		}
		for name, e := range lf.Deployed.Resources {
			sp, err := entrySourcePath(e, stores, defaultStore)
			if err != nil {
				return err
			}
			info, _ := os.Stat(sp)
			isDir := info != nil && info.IsDir()
			items = append(items, item{"resource:" + name, sp, e.DeployedPath, e.Hash, isDir})
		}

		sort.Slice(items, func(i, j int) bool { return items[i].name < items[j].name })

		for _, it := range items {
			deployHash, deployErr := hashPath(it.deployPath, it.isDir)
			sourceHash, sourceErr := hashPath(it.storePath, it.isDir)

			if deployErr != nil {
				fmt.Fprintf(out, "%-20s  missing (deployed)\n", it.name)
				continue
			}
			if sourceErr != nil {
				fmt.Fprintf(out, "%-20s  missing (store)\n", it.name)
				continue
			}

			localChanged := deployHash != it.lockHash
			storeChanged := sourceHash != it.lockHash

			var state string
			switch {
			case localChanged && storeChanged:
				state = "conflict"
			case localChanged:
				state = "modified locally"
			case storeChanged:
				state = "modified in store"
			default:
				state = "unchanged"
			}
			fmt.Fprintf(out, "%-20s  %s\n", it.name, state)
		}

		return nil
	},
}

func hashPath(path string, isDir bool) (string, error) {
	if isDir {
		return lock.HashDir(path)
	}
	return lock.Hash(path)
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
