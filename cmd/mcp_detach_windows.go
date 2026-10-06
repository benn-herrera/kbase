//go:build windows

package main

import "syscall"

// windowsDetachedProcess is DETACHED_PROCESS, which package syscall does not
// name: the process gets no console of its own and shares none.
const windowsDetachedProcess = 0x00000008

// detachedProcess starts a process in a process group of its own and without
// the server's console, so a console signal to the server does not reach it.
func detachedProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | windowsDetachedProcess}
}
