package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/push"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push modified deployed files back to the store",
	Long: `Push locally modified agent files back to the source store.

Reads .agentfiles.lock, hashes each deployed file on disk, and compares
to the hash recorded at deploy time. Changed files are copied back to
the store at their original source path.

After pushing, commit in the store (cd ~/.agentfiles && git commit)
and run "af apply --force" in other repos to propagate changes.

Requires a prior "af apply" (needs .agentfiles.lock to exist).
Use --dry-run to preview changes without modifying the store.

Examples:
  af push                     # push all local changes
  af push --dry-run           # show what would be pushed
  af push --skill browse      # push only the browse skill`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stores, defaultStore, err := openStores()
		if err != nil {
			return err
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		skill, _ := cmd.Flags().GetString("skill")

		res, err := push.Push(stores, defaultStore, ".", push.Options{
			DryRun:    dryRun,
			SkillOnly: skill,
		})
		if err != nil {
			return err
		}

		if len(res.Changes) == 0 {
			fmt.Println("No changes to push")
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
			fmt.Printf("Pushed %d asset(s)\n", len(res.Changes))
		}
		return nil
	},
}

func init() {
	pushCmd.Flags().Bool("dry-run", false, "show what would be pushed without copying")
	pushCmd.Flags().String("skill", "", "push only the named skill")
	rootCmd.AddCommand(pushCmd)
}
