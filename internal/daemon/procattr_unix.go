//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr sets platform-specific process attributes for daemon detachment.
// On Unix, this creates a new session to detach from the controlling terminal.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session, detach from controlling terminal
	}
}
