package cmd

import (
	"fmt"
	"os"

	"github.com/danielbenner/agentfiles/internal/store"
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
	rootCmd.PersistentFlags().StringVar(&storePath, "store", store.DefaultStorePath(), "path to the agentfiles store")
}
