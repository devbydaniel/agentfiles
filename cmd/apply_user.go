package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/apply"
	"github.com/devbydaniel/agentfiles/internal/layout"
	"github.com/devbydaniel/agentfiles/internal/manifest"
)

var applyUserCmd = &cobra.Command{
	Use:   "apply-user",
	Short: "Deploy agent files to user-level paths (e.g. ~/.claude/, ~/.pi/)",
	Long: `Deploy agent files from the store into user-level paths.

Reads the [user] section from the config file, resolves the referenced
bundle or cherry-picked assets, and copies them to user-level paths
determined by the configured layout.

What gets deployed (per layout):
  pi:      ~/AGENTS.md, ~/.pi/skills/<name>/
  claude:  ~/.claude/CLAUDE.md, ~/.claude/skills/<name>/
  cursor:  ~/.cursor/rules/agentfiles.md, ~/.cursor/skills/<name>/
  all:     All of the above

Lock file is stored at ~/.config/agentfiles/user.lock.

Requires a [user] section in ~/.config/agentfiles/config.toml:

  [user]
  bundle = "my-bundle"
  layout = "all"

Examples:
  af apply-user               # deploy user-level files
  af apply-user --force       # overwrite existing files`,
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

		m, err := manifest.FromUserConfig(userFields(cfg.User))
		if err != nil {
			return fmt.Errorf("building user manifest: %w", err)
		}

		lay, err := layout.GetUser(m.Layout)
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

		force, _ := cmd.Flags().GetBool("force")

		res, err := apply.Apply(stores, userStore, m, home, apply.Options{
			Force:        force,
			LockFilePath: cfg.UserLockPath(),
			Layout:       lay,
		})
		if err != nil {
			return err
		}

		for _, w := range res.Warnings {
			fmt.Fprintln(os.Stderr, w)
		}
		msg := fmt.Sprintf("Applied %d user-level asset(s)", res.Deployed)
		if res.Removed > 0 {
			msg += fmt.Sprintf(", removed %d stale", res.Removed)
		}
		fmt.Println(msg)
		return nil
	},
}

func init() {
	applyUserCmd.Flags().Bool("force", false, "overwrite existing files without prompting")
	rootCmd.AddCommand(applyUserCmd)
}
