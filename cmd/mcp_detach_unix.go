//go:build unix

package main

import "syscall"

// detachedProcess puts a started process in a process group of its own, so
// a signal to the server's group — a harness stopping it — does not reach it.
func detachedProcess() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }
