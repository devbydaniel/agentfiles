package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/push"
)

var pushUserCmd = &cobra.Command{
	Use:   "push-user",
	Short: "Push modified user-level files back to the store",
	Long: `Push locally modified user-level agent files back to the source store.

Reads the user lock file (~/.config/agentfiles/user.lock), hashes each
deployed file on disk, and compares to the hash recorded at deploy time.
Changed files are copied back to the store.

Requires a prior "af apply-user" (needs user.lock to exist).

Examples:
  af push-user               # push all user-level changes
  af push-user --dry-run     # show what would be pushed`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if cfg.User == nil {
			return fmt.Errorf("no [user] section in config; add one to %s", resolvedConfigPath())
		}

		stores, defaultStore, err := openStores()
		if err != nil {
			return err
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("determining home directory: %w", err)
		}

		userStore := cfg.User.Store
		if userStore == "" {
			userStore = defaultStore
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")

		res, err := push.Push(stores, userStore, home, push.Options{
			DryRun:       dryRun,
			LockFilePath: cfg.UserLockPath(),
		})
		if err != nil {
			return err
		}

		if len(res.Changes) == 0 {
			fmt.Println("No user-level changes to push")
			return nil
		}

		verb := "Pushed"
		if dryRun {
			verb = "Would push"
		}
		for _, ch := range res.Changes {
			fmt.Printf("%s %s (%s)\n", verb, ch.Name, ch.Type)
		}
		if !dryRun {
			fmt.Printf("Pushed %d user-level asset(s)\n", len(res.Changes))
		}
		return nil
	},
}

func init() {
	pushUserCmd.Flags().Bool("dry-run", false, "show what would be pushed without copying")
	rootCmd.AddCommand(pushUserCmd)
}
