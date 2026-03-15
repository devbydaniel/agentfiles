package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/registry"
)

// agentCmd returns the CLI command name for a given layout.
func agentCmd(layout string) string {
	switch layout {
	case "claude":
		return "claude"
	case "cursor":
		return "cursor-agent"
	default: // "pi", "all"
		return "pi"
	}
}

// completeRepoNames returns repo names for shell completion.
func completeRepoNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := loadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	reg, err := loadRegistry(cfg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, repo := range reg.Repos {
		name := repo.Name
		if name == "" {
			name = filepath.Base(repo.Path)
		}
		names = append(names, name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

var execCmd = &cobra.Command{
	Use:   "exec <repo-name> [-- agent-args...]",
	Short: "Launch the agent CLI for a registered repo",
	Long: `Look up a repo by name in the registry and launch the appropriate
agent CLI (pi, claude, or cursor-agent) in that repo's directory.

The agent is chosen based on the repo's layout:
  pi      → pi
  claude  → claude
  cursor  → cursor-agent
  all     → pi (primary)

Any arguments after -- are forwarded to the agent CLI.

Examples:
  af exec api-server
  af exec web-app -- -p "fix the tests"
  af exec api-server --agent claude   # override agent choice`,
	ValidArgsFunction:  completeRepoNames,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoName := args[0]
		agentArgs := args[1:]

		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		reg, err := loadRegistry(cfg)
		if err != nil {
			return err
		}

		repo, err := findRepo(reg, repoName)
		if err != nil {
			return err
		}

		agentOverride, _ := cmd.Flags().GetString("agent")
		agent := agentOverride
		if agent == "" {
			agent = agentCmd(repo.Layout)
		}

		agentPath, err := exec.LookPath(agent)
		if err != nil {
			return fmt.Errorf("agent %q not found in PATH", agent)
		}

		// Verify repo directory exists.
		info, err := os.Stat(repo.Path)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("repo directory %q does not exist (run 'af apply-all' first)", repo.Path)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "→ %s in %s (layout=%s)\n", agent, repo.Path, repo.Layout)

		// Exec into the agent process, replacing this process.
		c := exec.Command(agentPath, agentArgs...)
		c.Dir = repo.Path
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		// Use absolute path for the working directory.
		absPath, err := filepath.Abs(repo.Path)
		if err == nil {
			c.Dir = absPath
		}

		return c.Run()
	},
}

// findRepo looks up a repo by name, then falls back to matching the
// basename of the path.
func findRepo(reg *registry.Registry, name string) (*registry.Repo, error) {
	// First try exact name match.
	for i := range reg.Repos {
		if reg.Repos[i].Name == name {
			return &reg.Repos[i], nil
		}
	}

	// Fall back to path basename match.
	for i := range reg.Repos {
		if filepath.Base(reg.Repos[i].Path) == name {
			return &reg.Repos[i], nil
		}
	}

	return nil, fmt.Errorf("repo %q not found in registry", name)
}

func init() {
	execCmd.Flags().String("agent", "", "override agent CLI (pi, claude, cursor-agent)")
	rootCmd.AddCommand(execCmd)
}
