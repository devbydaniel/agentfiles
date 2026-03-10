package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	storePath  string
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "af",
	Short: "agentfiles — portable context files for AI coding agents",
	Long: `agentfiles (af) manages a central source store of AI agent config files
(skills, agent instructions, resources) and deploys them into
repositories using layout presets for different tools (pi, Claude Code, Cursor).

Key concepts:
  Store     Git-managed directory (~/.agentfiles) holding all agent assets.
  Manifest  A .agentfiles TOML file in each repo declaring what it needs.
  Bundle    A named grouping of assets (store/bundles/<name>.toml).
  Layout    How assets map to files on disk (pi, claude, cursor, all).

Typical workflow:
  af init-store                       # create the store (once)
  af add skill ./my-skill/            # populate the store
  af init --bundle backend --layout pi  # set up a repo
  af apply                            # deploy files into the repo
  # ... edit files in context ...
  af push                             # send edits back to the store
  af apply --force                    # update other repos

User-level deployment (global agent files):
  af apply-user                       # deploy to ~/.claude/, ~/.pi/, etc.
  af push-user                        # push user-level edits back

Use "af <command> --help" for detailed usage of each command.`,
}

func Execute() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&storePath, "store", "", "store name (from config) or path; defaults to the config's default_store")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "path to config file (default: ~/.config/agentfiles/config.toml)")
}
