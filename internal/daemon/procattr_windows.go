//go:build windows

package daemon

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr sets platform-specific process attributes for daemon detachment.
// On Windows, we use CREATE_NEW_PROCESS_GROUP to detach from the console.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
