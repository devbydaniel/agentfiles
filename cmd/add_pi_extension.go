package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var addPiExtensionCmd = &cobra.Command{
	Use:   "pi-extension <path>",
	Short: "Add a pi extension to the store",
	Long: `Copy a pi extension into the store's pi_extensions directory.

Accepts either a single .ts file or a directory containing index.ts.
The name is derived from the filename (minus .ts) or directory name.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		name, overwritten, err := s.AddPiExtension(args[0], force)
		if err != nil {
			return err
		}
		if overwritten {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: overwriting existing %q\n", name)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added pi-extension %q from %s\n", name, args[0])
		return nil
	},
}

func init() {
	addPiExtensionCmd.Flags().Bool("force", false, "overwrite if already exists")
	addCmd.AddCommand(addPiExtensionCmd)
}
