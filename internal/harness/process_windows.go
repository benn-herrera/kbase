//go:build windows

package harness

import (
	"os/exec"
	"syscall"
)

// processGroup starts the command in a process group of its own. Windows names
// no process group kbase can signal as one: without a job object the group is
// only what a console signal is scoped to, so a command's own children are the
// command's to end.
func processGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// killGroup kills the command's process.
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
