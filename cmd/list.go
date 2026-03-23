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
	Use:   "list <skills|bundles|instructions|resources|agents|pi-extensions>",
	Short: "List items in the agentfiles store",
	Long: `List assets in the source store by type.

Types:
  skills          Skill directories (each containing SKILL.md)
  instructions    Instruction files (shown without .md extension)
  bundles         Bundle definitions (shown without .toml extension)
  resources       Resource directories
  agents          Agent files (shown without .md extension)
  pi-extensions   Pi extension files (.ts) or directories (with index.ts)

Examples:
  af list skills
  af list skills --flat
  af list bundles
  af list instructions
  af list agents
  af list pi-extensions`,
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

		case "bundles", "instructions", "resources":
			var dir string
			var stripExt bool
			switch kind {
			case "bundles":
				dir = s.BundlesDir()
				stripExt = true
			case "instructions":
				dir = s.InstructionsDir()
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

		case "agents":
			agents, err := s.ListAgents()
			if err != nil {
				return err
			}
			var names []string
			for _, a := range agents {
				names = append(names, a.Name)
			}
			sort.Strings(names)
			for _, n := range names {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil

		case "pi-extensions":
			exts, err := s.ListPiExtensions()
			if err != nil {
				return err
			}
			var names []string
			for _, e := range exts {
				names = append(names, e.Name)
			}
			sort.Strings(names)
			for _, n := range names {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil

		default:
			return fmt.Errorf("unknown list type %q (use skills, bundles, instructions, resources, agents, or pi-extensions)", kind)
		}
	},
}

func init() {
	listCmd.Flags().Bool("flat", false, "show only leaf names (for scripting)")
	rootCmd.AddCommand(listCmd)
}
