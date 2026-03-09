package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var addResourceCmd = &cobra.Command{
	Use:   "resource <path>",
	Short: "Add a resource directory to the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		name, overwritten, err := s.AddResource(args[0], force)
		if err != nil {
			return err
		}
		if overwritten {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: overwriting existing %q\n", name)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added resource %q from %s\n", name, args[0])
		return nil
	},
}

func init() {
	addResourceCmd.Flags().Bool("force", false, "overwrite if already exists")
	addCmd.AddCommand(addResourceCmd)
}
