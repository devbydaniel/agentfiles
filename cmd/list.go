package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielbenner/agentfiles/internal/store"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list <skills|bundles|agents|plugins|resources>",
	Short: "List items in the agentfiles store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Open(storePath)
		if err != nil {
			return err
		}

		kind := args[0]
		var dir string
		var stripExt bool

		switch kind {
		case "skills":
			dir = s.SkillsDir()
		case "bundles":
			dir = s.BundlesDir()
			stripExt = true
		case "agents":
			dir = s.AgentsDir()
			stripExt = true
		case "plugins":
			dir = s.PluginsDir()
		case "resources":
			dir = s.ResourcesDir()
		default:
			return fmt.Errorf("unknown list type %q (use skills, bundles, agents, plugins, or resources)", kind)
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
			if (kind == "skills" || kind == "plugins" || kind == "resources") && !e.IsDir() {
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
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
