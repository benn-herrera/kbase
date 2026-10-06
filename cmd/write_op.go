package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"kbase/internal/log"
	toolresult "kbase/internal/result"
	"kbase/internal/write"
)

type writeOpOptions struct {
	Op string
	// WorkDir is where the search for the repository root starts.
	WorkDir string
	// ValuesFile is the values document's path; "" reads Stdin.
	ValuesFile string
	Stdin      io.Reader
	Create     bool
	NoRefresh  bool
	// ArgumentEntry is whether the values are one entry composed from a
	// tool call's arguments rather than a document the caller wrote, so a
	// refusal's line and column would point nowhere the caller can see.
	ArgumentEntry bool
	Stdout        io.Writer
	Stderr        io.Writer
	Logger        log.Logger
}

func runWriteOp(opts writeOpOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, opts.Op); err != nil {
		return 0, err
	}
	outcome, fields := writeOp(opts)
	return emitResult(opts.Op, opts.Stdout, outcome, fields)
}

func writeOp(opts writeOpOptions) (string, []toolresult.Field) {
	root, refusals, err := kbRootFrom(opts.WorkDir)
	if err != nil {
		return failed(nil, err)
	}
	if refusals != nil {
		return refused(nil, refusals...)
	}
	fields := []toolresult.Field{{Key: "kb-root", Value: root}}
	var values []byte
	if opts.ValuesFile != "" {
		values, err = os.ReadFile(opts.ValuesFile)
	} else {
		values, err = io.ReadAll(opts.Stdin)
	}
	if err != nil {
		return failed(fields, fmt.Errorf("reading the values: %w", err))
	}
	run := write.Options{KBRoot: root, Values: values, Create: opts.Create, NoRefresh: opts.NoRefresh, Logger: opts.Logger}
	var res write.Result
	if opts.Op == write.RenderCitation {
		res = write.Run(opts.Op, run)
	} else {
		release, outcome, items, err := holdKB(root, write.RetryRemedy(opts.Op))
		if err != nil {
			return failed(fields, err)
		}
		if release == nil {
			res = write.Result{Outcome: outcome, Refusals: items}
		} else {
			defer release()
			res = write.Run(opts.Op, run)
		}
	}
	if opts.Op == write.RenderCitation {
		fields = append(fields, toolresult.Field{Key: "citations", Value: nonNil(res.Citations)})
	} else {
		if write.Inserts(opts.Op) {
			adopted := []toolresult.Record{}
			for _, a := range res.Adopted {
				adopted = append(adopted, toolresult.Record{{Key: "entry", Value: a.Entry}, {Key: "id", Value: a.ID}, {Key: "differs", Value: nonNil(a.Differs)}})
			}
			fields = append(fields, toolresult.Field{Key: "ids", Value: nonNil(res.IDs)}, toolresult.Field{Key: "minted", Value: nonNil(res.Minted)},
				toolresult.Field{Key: "adopted", Value: adopted})
		}
		fields = append(fields, toolresult.Field{Key: "written", Value: nonNil(res.Written)})
		var refreshed any
		if res.Refreshed != nil {
			refreshed = res.Refreshed
		}
		fields = append(fields, toolresult.Field{Key: "refreshed", Value: refreshed}, toolresult.Field{Key: "removed", Value: nonNil(res.Removed)})
		if opts.Op == write.ResolveDemoted {
			resolved := []toolresult.Record{}
			for _, r := range res.Resolved {
				resolved = append(resolved, toolresult.Record{{Key: "source", Value: r.Source}, {Key: "target", Value: r.Target}, {Key: "action", Value: r.Action}})
			}
			fields = append(fields, toolresult.Field{Key: "resolved", Value: resolved})
		}
	}
	if opts.ArgumentEntry {
		for i := range res.Refusals {
			res.Refusals[i].Line, res.Refusals[i].Column = 0, 0
		}
	}
	switch {
	case len(res.Refusals) > 0:
		fields = append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: res.Refusals})
	case len(res.Failures) > 0:
		fields = append(fields, toolresult.Field{Key: toolresult.FailuresKey, Value: res.Failures})
	}
	return res.Outcome, fields
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

const valuesHelp = `
Values arrive as YAML (JSON accepted) on stdin, or in the file --values names:
a document whose one key, entry, lists the entries, all written or none. A
refusal names each offending key, its position and every key the op takes.`

// writeOpCommand is one write-API op over the KB found from the working
// directory.
func writeOpCommand(op, short, long string) *cobra.Command {
	var valuesFile string
	var create, noRefresh bool
	cmd := &cobra.Command{
		Use:   op,
		Short: short,
		Long:  long + "\n" + valuesHelp,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := invocationDir(cmd)
			if err != nil {
				return err
			}
			code, err := runWriteOp(writeOpOptions{Op: op, WorkDir: wd, ValuesFile: valuesFile, Stdin: cmd.InOrStdin(),
				Create: create, NoRefresh: noRefresh, Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(), Logger: processLog.logger})
			if err != nil {
				return err
			}
			if code != 0 {
				return exitCode(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&valuesFile, "values", "", "the values document (YAML or JSON); stdin when absent")
	if op != write.RenderCitation {
		cmd.Flags().BoolVar(&noRefresh, "no-refresh", false, "skip the refresh that otherwise follows the write")
	}
	if write.CreatesRegister(op) {
		cmd.Flags().BoolVar(&create, "create", false, "create the register when it does not exist yet")
	}
	return mcpBinding(cmd, mcpBound)
}
