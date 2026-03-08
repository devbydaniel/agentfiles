package cmd

import (
	"fmt"

	"github.com/danielbenner/agentfiles/internal/store"
	"github.com/spf13/cobra"
)

var initFromURL string

var initStoreCmd = &cobra.Command{
	Use:   "init-store [path]",
	Short: "Initialise a new agentfiles source store",
	Long: `Create a new agentfiles source store at the given path (or the default store path).

The store is a git-managed directory with subdirectories for skills, agents,
plugins, resources, and bundles.

Use --from <url> to clone an existing store from a git repository.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := storePath
		if len(args) > 0 {
			path = args[0]
		}

		var (
			s   *store.Store
			err error
		)

		if initFromURL != "" {
			s, err = store.InitFromClone(initFromURL, path)
		} else {
			s, err = store.Init(path)
		}

		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Initialised agentfiles store at %s\n", s.Root)
		return nil
	},
}

func init() {
	initStoreCmd.Flags().StringVar(&initFromURL, "from", "", "clone store from a git repository URL")
	rootCmd.AddCommand(initStoreCmd)
}
