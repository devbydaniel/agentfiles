package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list <skills|bundles|agents|resources>",
	Short: "List items in the agentfiles store",
	Long: `List assets in the source store by type.

Types:
  skills      Skill directories (each containing SKILL.md)
  agents      Agent instruction files (shown without .md extension)
  bundles     Bundle definitions (shown without .toml extension)
  resources   Resource directories

Examples:
  af list skills
  af list skills --flat
  af list bundles
  af list agents`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}

		kind := args[0]

		switch kind {
		case "skills":
			flat, _ := cmd.Flags().GetBool("flat")
			skills, err := s.ListSkills()
			if err != nil {
				return err
			}
			var names []string
			for _, sk := range skills {
				if flat {
					names = append(names, sk.LeafName)
				} else {
					names = append(names, sk.GroupPath)
				}
			}
			sort.Strings(names)
			for _, n := range names {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil

		case "bundles", "agents", "resources":
			var dir string
			var stripExt bool
			switch kind {
			case "bundles":
				dir = s.BundlesDir()
				stripExt = true
			case "agents":
				dir = s.AgentsDir()
				stripExt = true
			case "resources":
				dir = s.ResourcesDir()
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}

			var names []string
			for _, e := range entries {
				name := e.Name()
				if strings.HasPrefix(name, ".") {
					continue
				}
				if kind == "resources" && !e.IsDir() {
					continue
				}
				if stripExt {
					name = strings.TrimSuffix(name, filepath.Ext(name))
				}
				names = append(names, name)
			}
			sort.Strings(names)

			for _, n := range names {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil

		default:
			return fmt.Errorf("unknown list type %q (use skills, bundles, agents, or resources)", kind)
		}
	},
}

func init() {
	listCmd.Flags().Bool("flat", false, "show only leaf names (for scripting)")
	rootCmd.AddCommand(listCmd)
}
