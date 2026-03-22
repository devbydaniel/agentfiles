package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/config"
	"github.com/devbydaniel/agentfiles/internal/store"
)

var initFromURL string
var initStoreName string

var initStoreCmd = &cobra.Command{
	Use:   "init-store [path]",
	Short: "Initialise a new agentfiles source store",
	Long: `Create a new agentfiles source store at the given path (default: ~/.agentfiles).

The store is a git-managed directory with this structure:
  skills/       Skill directories (each contains SKILL.md + supporting files)
  instructions/ Instruction files (<name>.md)
  resources/    Arbitrary file trees copied as-is into repo roots
  bundles/      Named groupings of assets (<name>.toml)

The command runs "git init" on the new directory. Idempotent — safe to
run on an existing store (no data loss).

Use --from <url> to clone an existing store from a git repository instead
of creating an empty one.

Use --name <name> to register the new store in config.toml under [stores]
with the given name.

Examples:
  af init-store                                       # default path
  af init-store ~/my-agent-store                      # custom path
  af init-store --from git@github.com:me/store.git    # clone existing
  af init-store ~/work-store --name work              # create and register`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := fallbackStorePath()
		if len(args) > 0 {
			path = args[0]
		}

		var (
			s   *store.Store
			err error
		)

		if initFromURL != "" {
			s, err = store.InitFromClone(initFromURL, path)
		} else {
			s, err = store.Init(path)
		}

		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Initialised agentfiles store at %s\n", s.Root)

		if initStoreName != "" {
			if err := registerStoreInConfig(initStoreName, s.Root); err != nil {
				return fmt.Errorf("registering store in config: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Registered as %q in %s\n", initStoreName, resolvedConfigPath())
		}

		return nil
	},
}

// registerStoreInConfig adds/updates a store entry in config.toml.
func registerStoreInConfig(name, path string) error {
	cfgPath := resolvedConfigPath()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	// Use ~ shorthand if possible.
	home, _ := os.UserHomeDir()
	displayPath := path
	if home != "" && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		displayPath = "~/" + path[len(home)+1:]
	}

	cfg.Stores[name] = displayPath

	return writeConfig(cfgPath, cfg)
}

// writeConfig writes the config back to disk. It preserves the structure
// by writing TOML manually to keep it readable.
func writeConfig(path string, cfg *config.Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var lines []string

	if cfg.DefaultStore != "" {
		lines = append(lines, fmt.Sprintf("default_store = %q", cfg.DefaultStore))
		lines = append(lines, "")
	}

	if len(cfg.Stores) > 0 {
		lines = append(lines, "[stores]")
		storeNames := make([]string, 0, len(cfg.Stores))
		for name := range cfg.Stores {
			storeNames = append(storeNames, name)
		}
		sort.Strings(storeNames)
		for _, name := range storeNames {
			lines = append(lines, fmt.Sprintf("%s = %q", name, cfg.Stores[name]))
		}
		lines = append(lines, "")
	}

	for _, repo := range cfg.Repos {
		lines = append(lines, "[[repos]]")
		if repo.Name != "" {
			lines = append(lines, fmt.Sprintf("name = %q", repo.Name))
		}
		if repo.Path != "" {
			lines = append(lines, fmt.Sprintf("path = %q", repo.Path))
		}
		if repo.Store != "" {
			lines = append(lines, fmt.Sprintf("store = %q", repo.Store))
		}
		if repo.Bundle != "" {
			lines = append(lines, fmt.Sprintf("bundle = %q", repo.Bundle))
		}
		if repo.Layout != "" {
			lines = append(lines, fmt.Sprintf("layout = %q", repo.Layout))
		}
		if len(repo.SkillsAdd) > 0 {
			lines = append(lines, fmt.Sprintf("skills_add = %s", tomlStringArray(repo.SkillsAdd)))
		}
		if len(repo.SkillsRemove) > 0 {
			lines = append(lines, fmt.Sprintf("skills_remove = %s", tomlStringArray(repo.SkillsRemove)))
		}
		lines = append(lines, "")
	}

	content := ""
	for _, l := range lines {
		content += l + "\n"
	}

	return os.WriteFile(path, []byte(content), 0o644)
}

func init() {
	initStoreCmd.Flags().StringVar(&initFromURL, "from", "", "clone store from a git repository URL")
	initStoreCmd.Flags().StringVar(&initStoreName, "name", "", "register the store in config.toml under this name")
	rootCmd.AddCommand(initStoreCmd)
}
