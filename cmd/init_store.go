package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/store"
)

var initFromURL string

var initStoreCmd = &cobra.Command{
	Use:   "init-store [path]",
	Short: "Initialise a new agentfiles source store",
	Long: `Create a new agentfiles source store at the given path (default: ~/.agentfiles).

The store is a git-managed directory with this structure:
  skills/       Skill directories (each contains SKILL.md + supporting files)
  agents/       Agent instruction files (<name>.md)
  plugins/      Plugin directories
  resources/    Arbitrary file trees copied as-is into repo roots
  bundles/      Named groupings of assets (<name>.toml)

The command runs "git init" on the new directory. Idempotent — safe to
run on an existing store (no data loss).

Use --from <url> to clone an existing store from a git repository instead
of creating an empty one.

Examples:
  af init-store                                       # default path
  af init-store ~/my-agent-store                      # custom path
  af init-store --from git@github.com:me/store.git    # clone existing`,
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
