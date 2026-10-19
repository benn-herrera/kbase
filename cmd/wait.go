package main

import (
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/build"
	toolresult "kbase/internal/result"
)

// waitPoll is how often wait reads the state store.
const waitPoll = time.Second

func init() {
	var stateDir string
	var pid int
	cmd := &cobra.Command{
		Use:   "wait",
		Short: "wait for a build run to end, then report the store's status",
		Long: `wait reads the state store once a second and returns when the run named by
--pid is no longer progressing: another run has superseded it, or its own run
has ended. It then prints the status document for that store, so a command
running in the background delivers the build's outcome when it exits. --pid is
required and must be positive; --state-dir defaults as status's. It takes no
lock and reads no commit trail until that final document.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			emit := func(outcome string, fields []toolresult.Field) error {
				code, err := emitResult("wait", cmd.OutOrStdout(), outcome, fields)
				if err != nil {
					return err
				}
				if code != 0 {
					return exitCode(code)
				}
				return nil
			}
			if pid <= 0 {
				return emit(refused(nil, toolresult.Item{Check: checkUsage, Key: "--pid",
					Detail: "--pid is required and must name a run's process, a positive pid"}))
			}
			wd, err := invocationDir(cmd)
			if err != nil {
				return err
			}
			opts := build.MonitorOptions{StateDir: stateDir, WorkDir: wd, Logger: processLog.logger}
			// status resolves a relative --state-dir against the working
			// directory; build.Locate would resolve it against the repository
			// root.
			if opts.StateDir != "" && !filepath.IsAbs(opts.StateDir) {
				if opts.StateDir, err = filepath.Abs(opts.StateDir); err != nil {
					return err
				}
			}
			kbRoot, dir, outcome, fields := build.Locate(opts)
			if outcome != "" {
				return emit(outcome, fields)
			}
			placement := []toolresult.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: dir}}
			if _, err := build.WaitState(dir, pid, waitPoll); err != nil {
				return emit(failed(placement, err))
			}
			return emit(build.Status(opts))
		},
	}
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "the build's state store (default: $XDG_STATE_HOME/kbase/<key>)")
	cmd.Flags().IntVar(&pid, "pid", 0, "the process id of the build run to wait for (required)")
	rootCmd.AddCommand(mcpBinding(cmd, mcpExcluded))
}
