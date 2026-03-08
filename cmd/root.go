package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var storePath string

var rootCmd = &cobra.Command{
	Use:   "af",
	Short: "agentfiles — portable context files for AI coding agents",
	Long:  "agentfiles (af) manages portable context files that AI coding agents can use across projects.",
}

func Execute() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	defaultStore := filepath.Join(home, ".agentfiles")
	rootCmd.PersistentFlags().StringVar(&storePath, "store", defaultStore, "path to the agentfiles store")
}
