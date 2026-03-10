package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/store"
)

var validLayouts = []string{"pi", "claude", "cursor", "all"}

var gitignoreByLayout = map[string][]string{
	"pi":     {"AGENTS.md", ".pi/skills/"},
	"claude": {"CLAUDE.md", ".claude/skills/"},
	"cursor": {".cursorrules", ".cursor/skills/"},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize an .agentfiles manifest in the current directory",
	Long: `Create an .agentfiles manifest in the current directory.

The manifest declares which bundle (or individual assets) and layout this
repo uses. Run "af apply" after init to deploy the files.

Non-interactive mode (both flags):
  af init --bundle backend --layout pi

Interactive mode (no flags):
  Lists available bundles from the store and prompts for selection.

Layouts control where deployed files are placed:
  pi       AGENTS.md + .pi/skills/
  claude   CLAUDE.md + .claude/skills/
  cursor   .cursorrules + .cursor/skills/
  all      All of the above (pi primary, claude symlinks, cursor copies)

Layout defaults to "pi" if not specified.

Refuses to overwrite an existing .agentfiles — use "af apply" to redeploy.
Prints suggested .gitignore entries after creating the manifest.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		bundleFlag, _ := cmd.Flags().GetString("bundle")
		layoutFlag, _ := cmd.Flags().GetString("layout")

		// Check if .agentfiles already exists
		afPath := filepath.Join(".", ".agentfiles")
		if _, err := os.Stat(afPath); err == nil {
			return fmt.Errorf("already initialized, use af apply")
		}

		var bundleName, layoutName string

		if bundleFlag != "" || layoutFlag != "" {
			// Non-interactive mode: both flags required
			if bundleFlag == "" {
				return fmt.Errorf("--bundle is required when using --layout")
			}
			if layoutFlag == "" {
				layoutFlag = "pi"
			}
			if !isValidLayout(layoutFlag) {
				return fmt.Errorf("invalid layout %q, must be one of: %s", layoutFlag, strings.Join(validLayouts, ", "))
			}
			bundleName = bundleFlag
			layoutName = layoutFlag
		} else {
			// Interactive mode
			s, err := openStore()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}

			bundles, err := listBundles(s)
			if err != nil {
				return fmt.Errorf("listing bundles: %w", err)
			}
			if len(bundles) == 0 {
				return fmt.Errorf("no bundles found in store")
			}

			reader := bufio.NewReader(os.Stdin)

			fmt.Fprintln(cmd.OutOrStdout(), "Available bundles:")
			for i, b := range bundles {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d) %s\n", i+1, b)
			}
			fmt.Fprint(cmd.OutOrStdout(), "Select bundle: ")
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			idx, err := strconv.Atoi(line)
			if err != nil || idx < 1 || idx > len(bundles) {
				return fmt.Errorf("invalid selection: %s", line)
			}
			bundleName = bundles[idx-1]

			fmt.Fprintf(cmd.OutOrStdout(), "Available layouts: %s\n", strings.Join(validLayouts, ", "))
			fmt.Fprint(cmd.OutOrStdout(), "Select layout [pi]: ")
			line, _ = reader.ReadString('\n')
			line = strings.TrimSpace(line)
			if line == "" {
				line = "pi"
			}
			if !isValidLayout(line) {
				return fmt.Errorf("invalid layout %q", line)
			}
			layoutName = line
		}

		// Write .agentfiles
		content := fmt.Sprintf("bundle = %q\nlayout = %q\n", bundleName, layoutName)
		if err := os.WriteFile(afPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing .agentfiles: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Created .agentfiles (bundle=%s, layout=%s)\n", bundleName, layoutName)

		// Suggest .gitignore additions
		suggestions := gitignoreSuggestions(layoutName)
		fmt.Fprintln(cmd.OutOrStdout(), "\nSuggested .gitignore additions:")
		for _, s := range suggestions {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", s)
		}

		return nil
	},
}

func isValidLayout(l string) bool {
	for _, v := range validLayouts {
		if v == l {
			return true
		}
	}
	return false
}

func listBundles(s *store.Store) ([]string, error) {
	entries, err := os.ReadDir(s.BundlesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".toml") {
			names = append(names, strings.TrimSuffix(name, ".toml"))
		}
	}
	sort.Strings(names)
	return names, nil
}

func gitignoreSuggestions(layout string) []string {
	var suggestions []string

	if layout == "all" {
		for _, l := range []string{"pi", "claude", "cursor"} {
			suggestions = append(suggestions, gitignoreByLayout[l]...)
		}
	} else {
		suggestions = append(suggestions, gitignoreByLayout[layout]...)
	}

	suggestions = append(suggestions, ".agentfiles.lock")
	return suggestions
}

func init() {
	initCmd.Flags().String("bundle", "", "bundle name to use")
	initCmd.Flags().String("layout", "", "layout name (pi, claude, cursor, all)")
	rootCmd.AddCommand(initCmd)
}
