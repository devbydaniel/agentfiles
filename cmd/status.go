package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/danielbenner/agentfiles/internal/lock"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of deployed files compared to lock and store",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			items = append(items, item{"agents.md", e.StorePath, e.DeployedPath, e.Hash, false})
		}
		for name, e := range lf.Deployed.Skills {
			items = append(items, item{"skill:" + name, e.StorePath, e.DeployedPath, e.Hash, true})
		}
		for name, e := range lf.Deployed.Plugins {
			items = append(items, item{"plugin:" + name, e.StorePath, e.DeployedPath, e.Hash, true})
		}
		for name, e := range lf.Deployed.Resources {
			info, _ := os.Stat(e.StorePath)
			isDir := info != nil && info.IsDir()
			items = append(items, item{"resource:" + name, e.StorePath, e.DeployedPath, e.Hash, isDir})
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
