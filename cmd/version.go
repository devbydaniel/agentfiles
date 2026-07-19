package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var Version = "0.1.0-dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of agentfiles",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "agentfiles version %s\n", resolveVersion())
	},
}

// resolveVersion prefers the goreleaser ldflags stamp, falling back to the
// module version Go embeds on `go install module@version` — without this,
// go-installed binaries always report the dev default.
func resolveVersion() string {
	if Version != "0.1.0-dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return Version
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
