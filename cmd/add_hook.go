package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var addHookCmd = &cobra.Command{
	Use:   "hook <path>",
	Short: "Add a hook file or directory to the store",
	Long: `Add a hook to the source store.

Accepts either form:
  af add hook ./my-hook.json            → store/hooks/my-hook.json
  af add hook ./my-hook/                → store/hooks/my-hook/
                                          (directory must contain hook.json
                                           plus an optional scripts/ subdir)

Directory-form hooks may reference their own scripts via the
${AF_HOOK_ROOT} placeholder, which agentfiles substitutes at apply time
with a shell-portable path to the deployed hook root.`,
	Args: cobra.ExactArgs(1),
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
