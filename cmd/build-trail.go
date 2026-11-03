package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/buildstore"
)

func buildTrailCommand() *cobra.Command {
	return monitorCommand("build-trail", "list the build's stage boundaries, each with its commit and the command that rewinds to it",
		`build-trail lists the build's history over the project's KB, read-only:
each stage boundary, oldest first, with the command that rewinds to it — the
reset, then `+argmark.Call("build", map[string]any{"resume": true})+` with the flags the build's last run was
launched with — and started-at, the commit the build began from, with the
command that starts over: the reset alone. It runs none of them: run a
boundary's commands as listed, flags included — the reset in a shell. The
reset is `+"`git reset --keep <commit>`"+`: it refuses rather than discarding
uncommitted changes in the files it touches.
It takes no target: each boundary and started-at carries its own command.

Its result: head; boundaries, oldest first, each with stage, commit, subject
and command; started-at, with commit and command, null where the trail
records none; dirty, false or the worktree's changed paths, the first 20 named
and the rest counted. It is refused outside a git worktree, where status would
refuse the state store, and where the trail records a stage over an unrecorded
earlier one (check: trail).`, buildstore.BuildTrail)
}
