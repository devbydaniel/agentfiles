package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var addSkillCmd = &cobra.Command{
	Use:   "skill <path>",
	Short: "Add a skill directory to the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		name, overwritten, err := s.AddSkill(args[0], force)
		if err != nil {
			return err
		}
		if overwritten {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: overwriting existing %q\n", name)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added skill %q from %s\n", name, args[0])
		return nil
	},
}

func init() {
	addSkillCmd.Flags().Bool("force", false, "overwrite if already exists")
	addCmd.AddCommand(addSkillCmd)
}
