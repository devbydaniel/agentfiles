package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/apply"
	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/store"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Deploy agent files from the store into the current repository",
	Long: `Deploy agent files from the source store into the current repository.

Reads the .agentfiles manifest in the current directory, resolves the
referenced bundle or cherry-picked assets from the store, and copies
them to the paths determined by the configured layout.

Alternatively, use --bundle and --layout to deploy without a manifest
file. This is useful in cloud agent environments (Codex Cloud, Cursor
Background Agents) where no config or manifest exists on disk:

  af apply --store https://github.com/user/store.git \
           --bundle backend --layout codex --force

When --store is a git URL, the store is cloned automatically.

What gets deployed (per layout):
  pi:      AGENTS.md, .agents/skills/<name>/
  claude:  .claude/CLAUDE.md, .claude/skills/<name>/
  cursor:  AGENTS.md, .agents/skills/<name>/
  codex:   AGENTS.md, .agents/skills/<name>/
  all:     All of the above (pi primary + claude + cursor + codex copies)

Resources are always copied to the repo root regardless of layout.

Creates/updates .agentfiles.lock to track what was deployed and content
hashes. The lock file is used by "af push", "af diff", and "af status".

Without --force, existing files are skipped with a warning and not
recorded in the lock file. Use --force to overwrite.

Examples:
  af apply                    # deploy everything in the manifest
  af apply --force            # overwrite existing files
  af apply --skill browse     # deploy only the "browse" skill
  af apply --store https://github.com/user/store.git --bundle backend --layout codex --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		bundleFlag, _ := cmd.Flags().GetString("bundle")
		layoutFlag, _ := cmd.Flags().GetString("layout")

		// Clone from URL if --store is a git URL (only supported here).
		var stores map[string]*store.Store
		var defaultStore string
		var err error
		if storePath != "" && looksLikeURL(storePath) {
			var tmpDir string
			stores, defaultStore, tmpDir, err = cloneStoreFromURL(storePath)
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)
		} else {
			stores, defaultStore, err = openStores()
			if err != nil {
				return err
			}
		}

		var m *manifest.Manifest
		if bundleFlag != "" {
			if layoutFlag == "" {
				return fmt.Errorf("--layout is required when using --bundle")
			}
			m, err = manifest.FromUserConfig(manifest.UserFields{
				Bundle: bundleFlag,
				Layout: layoutFlag,
			})
			if err != nil {
				return err
			}
		} else {
			if layoutFlag != "" {
				fmt.Fprintln(os.Stderr, "warning: --layout is ignored without --bundle")
			}
			m, err = manifest.Load(".")
			if err != nil {
				return err
			}
		}

		// Allow the manifest to override the default store.
		if m.Store != "" {
			defaultStore = m.Store
		}

		force, _ := cmd.Flags().GetBool("force")
		skill, _ := cmd.Flags().GetString("skill")

		res, err := apply.Apply(stores, defaultStore, m, ".", apply.Options{
			Force:     force,
			SkillOnly: skill,
		})
		if err != nil {
			return err
		}

		for _, w := range res.Warnings {
			fmt.Fprintln(os.Stderr, w)
		}
		msg := fmt.Sprintf("Applied %d asset(s)", res.Deployed)
		if res.Removed > 0 {
			msg += fmt.Sprintf(", removed %d stale", res.Removed)
		}
		fmt.Println(msg)
		return nil
	},
}

func init() {
	applyCmd.Flags().Bool("force", false, "overwrite existing files without prompting")
	applyCmd.Flags().String("skill", "", "deploy only the named skill")
	applyCmd.Flags().String("bundle", "", "bundle name (bypasses .agentfiles manifest)")
	applyCmd.Flags().String("layout", "", "layout name (required with --bundle, e.g. codex, claude, cursor, pi, all)")
	rootCmd.AddCommand(applyCmd)
}
