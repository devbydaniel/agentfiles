package cmd

import "github.com/spf13/cobra"

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a skill, agent, or resource to the store",
	Long: `Copy an asset from the local filesystem into the source store.

Subcommands:
  af add skill <dir>                 Copy a skill directory → store/skills/<dirname>/
  af add agent <file> --name <name>  Copy an agent file → store/agents/<name>.md
  af add resource <dir>              Copy a resource directory → store/resources/<dirname>/

The asset name is derived from the directory basename (skills,
resources) or from the --name flag (agents). Use --force to overwrite
an existing asset in the store.`,
}

func init() {
	rootCmd.AddCommand(addCmd)
}
