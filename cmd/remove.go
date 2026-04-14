package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var removeGlobal bool

var removeCmd = &cobra.Command{
	Use:               "remove <name>",
	Aliases:           []string{"rm"},
	Short:             "Remove a workflow from local (default) or global scope",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeWorkflowNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ensureYAMLExt(args[0])
		if err := validateWorkflowName(name); err != nil {
			return err
		}
		dir, err := scopeDir(removeGlobal)
		if err != nil {
			return err
		}
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("%s not found", path)
			}
			return err
		}
		fmt.Printf("✓ Removed %s\n", path)
		return nil
	},
}

func init() {
	removeCmd.Flags().BoolVarP(&removeGlobal, "global", "g", false, "Remove from global scope (~/.flowcmd)")
	rootCmd.AddCommand(removeCmd)
}
