package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/argmark"
)

func cancelCommand() *cobra.Command {
	return monitorCommand("cancel", "stop the running build or score run, resumably",
		`cancel signals the run holding the state store, detached or in the
foreground, of either kind — a build or a score run — and waits for it to let go: the item in flight is
abandoned with nothing written for it, and the run exits cancelled. It
continues only when asked to: `+argmark.Call("build", map[string]any{"resume": true})+` or `+argmark.Call("score", map[string]any{"resume": true})+`. Its result
names the call that resumes the run.`, cancelScore)
}
