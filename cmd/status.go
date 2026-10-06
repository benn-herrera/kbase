package main

import (
	"github.com/spf13/cobra"

	"kbase/internal/build"
	toolresult "kbase/internal/result"
)

// monitorCommand is status or cancel: one call into build over the KB the
// working directory is in, its result written as the verb's document.
func monitorCommand(use, short, long string, verb func(build.MonitorOptions) (string, []toolresult.Field)) *cobra.Command {
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
			outcome, fields := verb(build.MonitorOptions{StateDir: stateDir, WorkDir: wd, Logger: processLog.logger})
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
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "the build's state store (default: $XDG_STATE_HOME/kbase/<key>)")
	return mcpBinding(cmd, mcpBound)
}

func init() {
	rootCmd.AddCommand(readOnly(monitorCommand("status", "report the build's state, progress and resume command",
		`status reads the build over the KB the working directory is in — its run
lock, its progress record and the kb-build: commit trail — and reports the
state (none, running, cancelled, failed, bounded, finished), the pid and
timestamps, each stage recorded or not with its commit, the current stage's
units done and total, recent refusals and fallbacks, the --no-inference flag
and the command that resumes it, as one YAML document on stdout.`, build.Status)))
}
