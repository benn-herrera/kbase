package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/buildstore"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
)

// monitorCommand is status, cancel or build-trail: one read of the state
// store the operation lock names, else the default, over the KB the working
// directory is in, its result written as the verb's document.
func monitorCommand(use, short, long string, verb func(buildstore.MonitorOptions) (string, []toolresult.Field)) *cobra.Command {
	var stateDir string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := invocationDir(cmd)
			if err != nil {
				return err
			}
			outcome, fields := verb(buildstore.MonitorOptions{StateDir: stateDir, WorkDir: wd, Logger: processLog.logger})
			code, err := emitResult(use, cmd.OutOrStdout(), outcome, fields)
			if err != nil {
				return err
			}
			if code != 0 {
				return exitCode(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&stateDir, "state-dir", "", monitorStateDirUsage)
	return cmd
}

// stateStoreDefault is where a run's state store is when nothing names one.
const stateStoreDefault = "the KB's own store under $XDG_STATE_HOME/kbase"

// The state-dir usages: a run's own, build's and score's, and a monitor's,
// which first follows the run holding the operation lock.
const (
	runStateDirUsage     = "the run's state store (default: " + stateStoreDefault + ")"
	monitorStateDirUsage = "the run's state store (default: the store of the run holding the operation lock, else " + stateStoreDefault + ")"
)

func statusCommand() *cobra.Command {
	return monitorCommand("status", "report the current or last run's state, progress and resume call: a build or a score run",
		`status reports the current or last run over the project's KB, detached or in
the foreground, of either kind — a build or a score run, whichever is newest.

For a build, "state" is none, running, cancelled, interrupted, failed, bounded
or finished; none means no build stands: kb-root/ is absent, or was removed
with no build since. "state-dir", "pid", "started", "updated", "ended" and
"no-inference" name the run and its timing. "stages" marks each stage done or
not done, and "current" gives the current stage's "stage", "units-done" and
"units-total"; each completed stage's commit, with the command that rewinds to
it, is in `+argmark.Call("build-trail", nil)+`. "recent-refusals", "recent-fallbacks",
"cache-entries" and "cache-bytes", the last two the answer cache's size, are
the state store's history and outlive the build. "resume" is the call that
resumes the build.

For a score run, "state" is running, interrupted, done, cancelled, failed,
refused or retry. "state-dir", "pid", "started", "run" and "pass" name the run
and its timing. "counts", "scored" and "failures" give its progress. "resume"
is the call that resumes the run.`, score.Status)
}
