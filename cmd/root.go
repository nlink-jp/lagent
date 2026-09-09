// Package cmd holds the cobra command tree. main.go injects the version
// (from `git describe` via -X main.version) through Execute.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Execute runs the root command with the injected version string.
func Execute(version string) {
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "lagent [first message]",
	Short: "Sandboxed CLI agent runtime on a local LLM (OpenAI-compatible API)",
	Long: `lagent is a sandboxed CLI agent runtime backed by a local LLM served
over an OpenAI-compatible API (LM Studio, Ollama): file read/write,
sandboxed shell commands and MCP servers, with mutating calls gated by
the operator. It is a separate product line from gem-agent, built on the
same design; see docs/en/lagent-rfp.md.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Phase 1 (Core) lands the agent loop here. The scaffold only
		// answers --version and version, which the scaffold checklist
		// requires to be identical and pinned by a test.
		return fmt.Errorf("lagent: the agent loop is not implemented yet (Phase 1)")
	},
}

// versionCmd mirrors --version so `lagent version` and `lagent --version`
// print the same line (CONVENTIONS.md §Scaffold checklist).
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
