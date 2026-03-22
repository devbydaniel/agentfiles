package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/apply"
	"github.com/devbydaniel/agentfiles/internal/config"
	"github.com/devbydaniel/agentfiles/internal/layout"
	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/registry"
	"github.com/devbydaniel/agentfiles/internal/store"
)

var applyAllCmd = &cobra.Command{
	Use:   "apply-all",
	Short: "Deploy agent files to all repos listed in the registry",
	Long: `Read the config and deploy agent files to every listed repo.

For each repo entry the command will:
  1. Create the target directory if it doesn't exist
  2. Write/update the .agentfiles manifest from the config entry
  3. Run apply --force to deploy all assets

Config (~/.config/agentfiles/config.toml):

  default_store = "work"

  [stores]
  work = "~/.agentfiles"
  personal = "~/.agentfiles-personal"

  [[repos]]
  name = "api-server"
  path = "~/dev/api-server"
  store = "work"
  bundle = "backend"
  layout = "pi"

  [[repos]]
  name = "web-app"
  path = "~/work/web-app"
  bundle = "frontend"
  layout = "all"
  skills_add = ["personal:browse"]

If a [user] section is present in the config, user-level files are deployed
first (to ~/.claude/, ~/.pi/, etc.) before processing repos.

Examples:
  af apply-all              # deploy to all registered repos (+ user if configured)
  af apply-all --dry-run    # show what would be done, don't deploy`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		stores, defaultStore, err := openStores()
		if err != nil {
			return err
		}

		reg, err := loadRegistry(cfg)
		if err != nil {
			return err
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")

		var failed []string

		// Deploy user-level files if [user] is configured.
		if cfg.User != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "\n→ user (layout=%s)\n", cfg.User.Layout)

			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "  (dry-run) would deploy user-level files")
			} else {
				if err := applyUser(cfg, stores, defaultStore); err != nil {
					fmt.Fprintf(os.Stderr, "  error: %v\n", err)
					failed = append(failed, "user")
				}
			}
		}

		if cfg.User == nil && len(reg.Repos) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing to deploy (no [user] section and no repos in config)")
			return nil
		}

		for _, repo := range reg.Repos {
			fmt.Fprintf(cmd.OutOrStdout(), "\n→ %s (bundle=%s, layout=%s)\n", repo.Path, repo.Bundle, repo.Layout)

			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "  (dry-run) would create manifest and apply")
				continue
			}

			// Ensure target directory exists.
			if err := os.MkdirAll(repo.Path, 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "  error: creating directory: %v\n", err)
				failed = append(failed, repo.Path)
				continue
			}

			// Write/update the .agentfiles manifest.
			if err := writeManifest(repo, defaultStore); err != nil {
				fmt.Fprintf(os.Stderr, "  error: writing manifest: %v\n", err)
				failed = append(failed, repo.Path)
				continue
			}

			// Load and apply.
			m, err := manifest.Load(repo.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  error: loading manifest: %v\n", err)
				failed = append(failed, repo.Path)
				continue
			}

			repoStore := repo.Store
			if repoStore == "" {
				repoStore = defaultStore
			}

			res, err := apply.Apply(stores, repoStore, m, repo.Path, apply.Options{Force: true})
			if err != nil {
				fmt.Fprintf(os.Stderr, "  error: apply: %v\n", err)
				failed = append(failed, repo.Path)
				continue
			}

			for _, w := range res.Warnings {
				fmt.Fprintf(os.Stderr, "  %s\n", w)
			}
			msg := fmt.Sprintf("  applied %d asset(s)", res.Deployed)
			if res.Removed > 0 {
				msg += fmt.Sprintf(", removed %d stale", res.Removed)
			}
			fmt.Fprintln(cmd.OutOrStdout(), msg)
		}

		fmt.Fprintln(cmd.OutOrStdout())
		if len(failed) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Failed: %d repo(s)\n", len(failed))
			for _, p := range failed {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s\n", p)
			}
			return fmt.Errorf("%d repo(s) failed", len(failed))
		}

		fmt.Fprintf(cmd.OutOrStdout(), "All %d repo(s) deployed successfully\n", len(reg.Repos))
		return nil
	},
}

// writeManifest writes the .agentfiles manifest into the repo directory
// based on the registry entry. It always overwrites to keep the manifest
// in sync with the registry.
func writeManifest(repo registry.Repo, defaultStore string) error {
	var lines []string

	if repo.Store != "" && repo.Store != defaultStore {
		lines = append(lines, fmt.Sprintf("store = %q", repo.Store))
	}
	lines = append(lines, fmt.Sprintf("bundle = %q", repo.Bundle))
	lines = append(lines, fmt.Sprintf("layout = %q", repo.Layout))

	if len(repo.SkillsAdd) > 0 {
		lines = append(lines, fmt.Sprintf("skills_add = %s", tomlStringArray(repo.SkillsAdd)))
	}
	if len(repo.SkillsRemove) > 0 {
		lines = append(lines, fmt.Sprintf("skills_remove = %s", tomlStringArray(repo.SkillsRemove)))
	}

	content := ""
	for _, l := range lines {
		content += l + "\n"
	}

	p := filepath.Join(repo.Path, ".agentfiles")
	return os.WriteFile(p, []byte(content), 0o644)
}

// tomlStringArray formats a string slice as a TOML inline array.
func tomlStringArray(ss []string) string {
	out := "["
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%q", s)
	}
	out += "]"
	return out
}

// userFields converts a UserConfig to manifest.UserFields.
func userFields(u *config.UserConfig) manifest.UserFields {
	return manifest.UserFields{
		Bundle:       u.Bundle,
		Layout:       u.Layout,
		Instructions: u.Instructions,
		Skills:       u.Skills,
		SkillsAdd:    u.SkillsAdd,
		SkillsRemove: u.SkillsRemove,
	}
}

// applyUser deploys user-level agent files from the [user] config section.
func applyUser(cfg *config.Config, stores map[string]*store.Store, defaultStore string) error {
	m, err := manifest.FromUserConfig(userFields(cfg.User))
	if err != nil {
		return fmt.Errorf("building user manifest: %w", err)
	}

	lay, err := layout.GetUser(m.Layout)
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("determining home directory: %w", err)
	}

	userStore := cfg.User.Store
	if userStore == "" {
		userStore = defaultStore
	}

	res, err := apply.Apply(stores, userStore, m, home, apply.Options{
		Force:        true,
		LockFilePath: cfg.UserLockPath(),
		Layout:       lay,
	})
	if err != nil {
		return err
	}

	for _, w := range res.Warnings {
		fmt.Fprintf(os.Stderr, "  %s\n", w)
	}
	msg := fmt.Sprintf("  applied %d user-level asset(s)", res.Deployed)
	if res.Removed > 0 {
		msg += fmt.Sprintf(", removed %d stale", res.Removed)
	}
	fmt.Println(msg)
	return nil
}

func init() {
	applyAllCmd.Flags().Bool("dry-run", false, "show what would be done without deploying")
	rootCmd.AddCommand(applyAllCmd)
}
