package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd mirrors --version so `lagent version` and `lagent --version`
// print the same line (CONVENTIONS.md §Scaffold checklist): a Homebrew
// formula's `brew test` runs the flag, and the subcommand must not fall
// through to the agent as a first message.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s version %s\n", rootCmd.Name(), rootCmd.Version)
		return err
	},
}

func init() {
	rootCmd.SetVersionTemplate("{{.Name}} version {{.Version}}\n")
	rootCmd.AddCommand(versionCmd)
}
