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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
