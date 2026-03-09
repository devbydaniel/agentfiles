package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/devbydaniel/agentfiles/internal/apply"
	"github.com/devbydaniel/agentfiles/internal/manifest"
	"github.com/devbydaniel/agentfiles/internal/registry"
)

var applyAllCmd = &cobra.Command{
	Use:   "apply-all",
	Short: "Deploy agent files to all repos listed in the registry",
	Long: `Read the central registry and deploy agent files to every listed repo.

For each registry entry the command will:
  1. Create the target directory if it doesn't exist
  2. Write/update the .agentfiles manifest from the registry entry
  3. Run apply --force to deploy all assets

The registry is configured in ~/.config/agentfiles/config.toml using
[[repos]] entries. Local overrides live in config.local.toml alongside it.

Config (config.toml):

  default_store = "work"

  [stores]
  work = "~/.agentfiles"
  personal = "~/.agentfiles-personal"

  [[repos]]
  name = "api-server"
  store = "work"
  bundle = "backend"
  layout = "pi"

  [[repos]]
  name = "web-app"
  bundle = "frontend"
  layout = "all"
  skills_add = ["personal:browse"]

Local overrides (config.local.toml):

  [[repos]]
  name = "api-server"
  path = "~/dev/api-server"
  layout = "claude"           # optional: override layout

  [[repos]]
  name = "web-app"
  path = "~/work/web-app"
  skip = true                 # optional: skip this repo

Named repos without a local entry are silently skipped (the dev doesn't
have that repo checked out). Repos with path set directly in config.toml
work without a local entry (solo/non-team use).

For backward compatibility, apply-all also supports the legacy registry.toml
inside a store. If no repos are found in config.toml, it falls back to the
store-level registry.

Examples:
  af apply-all              # deploy to all registered repos
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

		reg, err := loadRegistry(cfg, stores, defaultStore)
		if err != nil {
			return err
		}

		if len(reg.Repos) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No repos in registry")
			return nil
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")

		var failed []string

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
			fmt.Fprintf(cmd.OutOrStdout(), "  deployed %d asset(s)\n", res.Deployed)
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

func init() {
	applyAllCmd.Flags().Bool("dry-run", false, "show what would be done without deploying")
	rootCmd.AddCommand(applyAllCmd)
}

