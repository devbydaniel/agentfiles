package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var addHookCmd = &cobra.Command{
	Use:   "hook <path>",
	Short: "Add a hook file to the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		name, overwritten, err := s.AddHook(args[0], force)
		if err != nil {
			return err
		}
		if overwritten {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: overwriting existing %q\n", name)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added hook %q from %s\n", name, args[0])
		return nil
	},
}

func init() {
	addHookCmd.Flags().Bool("force", false, "overwrite if already exists")
	addCmd.AddCommand(addHookCmd)
}
