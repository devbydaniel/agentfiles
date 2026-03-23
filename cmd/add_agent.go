package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var addAgentCmd = &cobra.Command{
	Use:   "agent <path>",
	Short: "Add an agent file to the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		name, overwritten, err := s.AddAgent(args[0], force)
		if err != nil {
			return err
		}
		if overwritten {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: overwriting existing %q\n", name)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added agent %q from %s\n", name, args[0])
		return nil
	},
}

func init() {
	addAgentCmd.Flags().Bool("force", false, "overwrite if already exists")
	addCmd.AddCommand(addAgentCmd)
}
