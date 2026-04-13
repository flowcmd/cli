package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExecute_Help(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"flowcmd", "--help"}

	rootCmd.SetOut(new(strings.Builder))
	rootCmd.SetErr(new(strings.Builder))
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	Execute()
}

// TestExecute_ErrorBranch covers the os.Exit(1) path of Execute() by
// re-running the test binary as a subprocess that registers a failing
// subcommand. Coverage is not merged from the child but the parent still
// asserts that Execute exited non-zero, validating the error branch behaves.
func TestExecute_ErrorBranch(t *testing.T) {
	if os.Getenv("FLOWCMD_EXECUTE_CHILD") == "1" {
		throwaway := &cobra.Command{
			Use:           "boom",
			SilenceUsage:  true,
			SilenceErrors: true,
			RunE:          func(*cobra.Command, []string) error { return os.ErrInvalid },
		}
		rootCmd.AddCommand(throwaway)
		os.Args = []string{"flowcmd", "boom"}
		Execute()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestExecute_ErrorBranch$")
	cmd.Env = append(os.Environ(), "FLOWCMD_EXECUTE_CHILD=1")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("child should exit non-zero; output: %s", out)
	}
}
