package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/store"
)

var addAgentName string

var addAgentCmd = &cobra.Command{
	Use:   "agent <path>",
	Short: "Add an agent file to the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Open(storePath)
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		overwritten, err := s.AddAgent(args[0], addAgentName, force)
		if err != nil {
			return err
		}
		if overwritten {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: overwriting existing agent %q\n", addAgentName)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added agent %q from %s\n", addAgentName, args[0])
		return nil
	},
}

func init() {
	addAgentCmd.Flags().StringVar(&addAgentName, "name", "", "name for the agent (required)")
	_ = addAgentCmd.MarkFlagRequired("name")
	addAgentCmd.Flags().Bool("force", false, "overwrite if already exists")
	addCmd.AddCommand(addAgentCmd)
}
