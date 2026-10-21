//go:build unix

package harness

import (
	"os/exec"
	"syscall"
)

// processGroup puts the command in a process group of its own, so a deadline or
// a cancellation kills the group rather than the one process kbase holds a pid
// for: a harness command is an agent, and an agent starts children.
func processGroup() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }

// killGroup kills the command's whole process group. A group already gone — the
// command exited as its context was cancelled — is no error to report.
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return err
	}
	return nil
}
