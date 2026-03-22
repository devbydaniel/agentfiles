package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var addInstructionName string

var addInstructionCmd = &cobra.Command{
	Use:   "instruction <path>",
	Short: "Add an instruction file to the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		overwritten, err := s.AddInstruction(args[0], addInstructionName, force)
		if err != nil {
			return err
		}
		if overwritten {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: overwriting existing instruction %q\n", addInstructionName)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added instruction %q from %s\n", addInstructionName, args[0])
		return nil
	},
}

func init() {
	addInstructionCmd.Flags().StringVar(&addInstructionName, "name", "", "name for the instruction (required)")
	_ = addInstructionCmd.MarkFlagRequired("name")
	addInstructionCmd.Flags().Bool("force", false, "overwrite if already exists")
	addCmd.AddCommand(addInstructionCmd)
}
