package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// The back end override: the one way a run's command is chosen from outside
// kbase, and the only one that exists.
//
// It is not an interface. `--harness` names a back end kbase ships and nothing
// else, so the command a run executes is kbase's own composition; this variable
// stands a command in its place for the length of one run, and the integration
// suite is its only caller. It is absent from --help, from every file kbase
// writes and from the documentation, because a user who needs it does not exist:
// a back end that a user assembles is what kbase refuses to support.
//
// Two things keep it from being a silent substitution. It is announced once on
// stderr, and the run's header records it (score.RunHeader.CommandOverride), so a
// run that executed a command nobody chose says so in its own record. And it is
// removed from the child's environment: a back end that starts kbase, or kbase's
// own tests running one another, must not hand the substitution down.
const backEndCommandEnv = "KBASE_SCORE_BACK_END_COMMAND"

// backEndCommand is the command the override names, and whether it names one at
// all. The value is an argv: the program and its arguments separated by
// whitespace, with no quoting — an element holding a space cannot be named, and
// nothing here grows a grammar to say otherwise. A blank value, or one of
// whitespace alone, names no command: the variable a shell profile exported empty
// must not turn into a run of the program "".
func backEndCommand(getenv func(string) string) []string {
	argv := strings.Fields(getenv(backEndCommandEnv))
	if len(argv) == 0 {
		return nil
	}
	return argv
}

// announceBackEndCommand is the one line a run that saw the override writes,
// naming the variable and the argv: the sentence that keeps a stray inherited
// variable from scoring a KB silently.
func announceBackEndCommand(stderr io.Writer, argv []string) {
	fmt.Fprintf(stderr, "%s names the command %q — kbase runs it in place of its own back end's command\n",
		backEndCommandEnv, argv)
}

// withoutBackEndCommand is env with the override's own assignment removed. The
// child of a run is handed this rather than the environment the run started in,
// so nothing the run executes inherits a command nobody chose.
func withoutBackEndCommand(env []string) []string {
	out := make([]string, 0, len(env))
	prefix := backEndCommandEnv + "="
	for _, assignment := range env {
		if strings.HasPrefix(assignment, prefix) {
			continue
		}
		out = append(out, assignment)
	}
	return out
}

// backEndEnv is the environment an item's process starts with when the override
// names its command: kbase's own, with the override taken out.
func backEndEnv() []string { return withoutBackEndCommand(os.Environ()) }
