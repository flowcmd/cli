package cmd

import (
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed templates/hello.yml
var helloSample []byte

const sampleName = "hello.yml"

var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a .flowcmd/ directory with a starter workflow",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := filepath.Join(scopeDirName, sampleName)
		err := writeIfAbsent(path, helloSample, initForce)
		if errors.Is(err, errAlreadyExists) {
			fmt.Printf("• %s already exists (use --force to overwrite)\n", path)
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Printf("✓ Initialized %s/\n✓ Created %s\n\nTry it:\n  flowcmd run hello\n", scopeDirName, path)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing files")
	rootCmd.AddCommand(initCmd)
}
