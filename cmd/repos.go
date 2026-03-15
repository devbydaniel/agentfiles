package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var reposCmd = &cobra.Command{
	Use:   "repos",
	Short: "List all repos available for af exec",
	Long: `List all repos registered in the config that can be launched with af exec.

Shows each repo's name, layout, agent CLI, and directory path.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		reg, err := loadRegistry(cfg)
		if err != nil {
			return err
		}

		if len(reg.Repos) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No repos configured.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tLAYOUT\tAGENT\tPATH")

		for _, repo := range reg.Repos {
			name := repo.Name
			if name == "" {
				name = filepath.Base(repo.Path)
			}

			agent := agentCmd(repo.Layout)
			layout := repo.Layout
			if layout == "" {
				layout = "-"
			}

			// Check if directory exists.
			path := repo.Path
			if info, err := os.Stat(path); err != nil || !info.IsDir() {
				path += " (missing)"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, layout, agent, path)
		}

		return w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(reposCmd)
}
