//go:build windows

package engine

import "os/exec"

// configureCmd is a no-op on Windows; exec.CommandContext already attaches
// the child to a job object that tears down the whole tree on cancel.
func configureCmd(_ *exec.Cmd) {}
