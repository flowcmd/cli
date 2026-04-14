//go:build unix

package engine

import (
	"os/exec"
	"syscall"
	"time"
)

// configureCmd puts the shell child in its own process group so a
// context cancellation can SIGKILL the entire tree — otherwise
// grandchildren (e.g. `sleep` launched by `sh -c`) may keep stdout
// pipes open and block Wait() until they exit on their own.
// WaitDelay bounds that wait when pipes are still held.
func configureCmd(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
	}
	c.WaitDelay = 100 * time.Millisecond
}
