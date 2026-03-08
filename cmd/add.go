package cmd

import "github.com/spf13/cobra"

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a skill, agent, plugin, or resource to the store",
}

func init() {
	rootCmd.AddCommand(addCmd)
}
