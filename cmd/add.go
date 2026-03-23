package cmd

import "github.com/spf13/cobra"

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a skill, instruction, resource, or agent to the store",
	Long: `Copy an asset from the local filesystem into the source store.

Subcommands:
  af add skill <dir>                        Copy a skill directory → store/skills/<dirname>/
  af add instruction <file> --name <name>   Copy an instruction file → store/instructions/<name>.md
  af add resource <dir>                     Copy a resource directory → store/resources/<dirname>/
  af add agent <file>                       Copy an agent .md file → store/agents/<filename>
  af add pi-extension <path>                Copy a .ts file or directory → store/pi_extensions/

The asset name is derived from the directory basename (skills,
resources), from the --name flag (instructions), from the filename
(agents, pi-extensions), or from the directory name (pi-extensions).
Use --force to overwrite an existing asset in the store.`,
}

func init() {
	rootCmd.AddCommand(addCmd)
}
