package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "flowcmd",
	Short: "Execute YAML-defined workflows with shell and LLM steps",
	Long:  "flowcmd runs declarative YAML workflows where shell scripts and LLM calls are first-class citizens. Sequential and parallel steps, templates, retries, and a TUI.",
}

// SetVersionInfo wires build-time metadata (injected by goreleaser) into the
// root command's --version output.
func SetVersionInfo(version, commit, date string) {
	rootCmd.Version = fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
