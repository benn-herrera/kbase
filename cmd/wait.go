package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/buildstore"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
)

// waitPoll is how often wait reads the state store.
const waitPoll = time.Second

func waitCommand() *cobra.Command {
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
				return emit(refused(nil, toolresult.Item{Check: toolresult.CheckUsage, Key: "--pid",
					Detail: "--pid is required and must name a run's process, a positive pid"}))
			}
			wd, err := invocationDir(cmd)
			if err != nil {
				return err
			}
			opts := buildstore.MonitorOptions{StateDir: stateDir, WorkDir: wd, Logger: processLog.logger}
			kbRoot, dir, outcome, fields := buildstore.Locate(opts)
			if outcome != "" {
				return emit(outcome, fields)
			}
			// The run removes the operation lock when it ends, so the final
			// document reads the store located here rather than resolving again.
			opts.StateDir = dir
			placement := []toolresult.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: dir}}
			// A score run's pid is recorded in its run directory; where the pid
			// names one, wait polls that run and reports the document it wrote.
			_, err = score.RunByPID(dir, pid)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return emit(failed(placement, err))
			}
			if err == nil {
				doc, code, err := score.WaitRun(dir, pid, waitPoll)
				if err != nil {
					return emit(failed(placement, err))
				}
				if _, err := fmt.Fprint(cmd.OutOrStdout(), doc); err != nil {
					return err
				}
				if code != 0 {
					return exitCode(code)
				}
				return nil
			}
			if _, err := buildstore.WaitState(dir, pid, waitPoll); err != nil {
				return emit(failed(placement, err))
			}
			return emit(buildstore.Status(opts))
		},
	}
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "the build's state store (default: the store the operation lock names, else $XDG_STATE_HOME/kbase/<key>)")
	cmd.Flags().IntVar(&pid, "pid", 0, "the process id of the build run to wait for (required)")
	return cmd
}
