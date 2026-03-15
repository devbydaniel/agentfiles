package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/apply"
	"github.com/devbydaniel/agentfiles/internal/manifest"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Deploy agent files from the store into the current repository",
	Long: `Deploy agent files from the source store into the current repository.

Reads the .agentfiles manifest in the current directory, resolves the
referenced bundle or cherry-picked assets from the store, and copies
them to the paths determined by the configured layout.

What gets deployed (per layout):
  pi:      AGENTS.md, .pi/skills/<name>/
  claude:  CLAUDE.md, .claude/skills/<name>/
  cursor:  .cursorrules, .cursor/skills/<name>/
  all:     All of the above (pi primary + claude + cursor copies)

Resources are always copied to the repo root regardless of layout.

Creates/updates .agentfiles.lock to track what was deployed and content
hashes. The lock file is used by "af push", "af diff", and "af status".

Without --force, existing files are skipped with a warning and not
recorded in the lock file. Use --force to overwrite.

Examples:
  af apply                    # deploy everything in the manifest
  af apply --force            # overwrite existing files
  af apply --skill browse     # deploy only the "browse" skill`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stores, defaultStore, err := openStores()
		if err != nil {
			return err
		}

		m, err := manifest.Load(".")
		if err != nil {
			return err
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
	rootCmd.AddCommand(applyCmd)
}
