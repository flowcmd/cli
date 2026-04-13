package cmd

import (
	"fmt"

	"github.com/flowcmd/cli/internal/parser"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate <workflow.yml>",
	Short: "Validate a workflow file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wf, err := parser.Parse(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("✓ %s valid (%d steps)\n", wf.Name, len(wf.Steps))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
