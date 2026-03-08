package cmd

import (
	"fmt"
	"os"

	"github.com/danielbenner/agentfiles/internal/apply"
	"github.com/danielbenner/agentfiles/internal/manifest"
	"github.com/danielbenner/agentfiles/internal/store"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Deploy agent files from the store into the current repository",
	Long:  "Reads .agentfiles from the current directory, resolves assets from the store, and copies them into the repo according to the configured layout.",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Open(storePath)
		if err != nil {
			return err
		}

		m, err := manifest.Load(".")
		if err != nil {
			return err
		}

		force, _ := cmd.Flags().GetBool("force")
		skill, _ := cmd.Flags().GetString("skill")

		res, err := apply.Apply(s, m, ".", apply.Options{
			Force:     force,
			SkillOnly: skill,
		})
		if err != nil {
			return err
		}

		for _, w := range res.Warnings {
			fmt.Fprintln(os.Stderr, w)
		}
		fmt.Printf("Deployed %d asset(s)\n", res.Deployed)
		return nil
	},
}

func init() {
	applyCmd.Flags().Bool("force", false, "overwrite existing files without prompting")
	applyCmd.Flags().String("skill", "", "deploy only the named skill")
	rootCmd.AddCommand(applyCmd)
}
