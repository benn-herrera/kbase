package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
)

type maintenanceOptions struct {
	// WorkDir is where the search for the repository root starts.
	WorkDir string
	Stdout  io.Writer
	Stderr  io.Writer
	Logger  log.Logger
}

// The refusal classes of a verb that finds no KB to work on.
const (
	checkWorktree = "worktree"
	checkKBRoot   = "kb-root"
)

// kbRootFrom is the kb-root beside the .git at or above dir, or a refusal
// naming why there is none.
func kbRootFrom(dir string) (string, []toolresult.Item, error) {
	repo, err := kb.RepositoryRoot(dir)
	if err != nil {
		return "", nil, err
	}
	if repo == "" {
		return "", []toolresult.Item{{Check: checkWorktree, Path: dir,
			Detail: fmt.Sprintf("%s is not inside a git worktree with %s/ beside its .git", dir, kb.KBDir)}}, nil
	}
	root := filepath.Join(repo, kb.KBDir)
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", []toolresult.Item{{Check: checkKBRoot, Path: root,
			Detail: fmt.Sprintf("%s has no %s/ beside its .git", repo, kb.KBDir)}}, nil
	}
	return root, nil, nil
}

// emitResult writes a verb's result document and returns its exit code.
func emitResult(verb string, stdout io.Writer, outcome string, fields []toolresult.Field) (int, error) {
	if err := toolresult.Emit(stdout, outcome, fields...); err != nil {
		return 0, fmt.Errorf("%s: writing the result: %w", verb, err)
	}
	return exitCodes[outcome], nil
}

func runRefresh(opts maintenanceOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "refresh"); err != nil {
		return 0, err
	}
	outcome, fields := refresh(opts)
	return emitResult("refresh", opts.Stdout, outcome, fields)
}

func refresh(opts maintenanceOptions) (string, []toolresult.Field) {
	root, refusals, err := kbRootFrom(opts.WorkDir)
	if err != nil {
		return failed(nil, err)
	}
	if refusals != nil {
		return refused(nil, refusals...)
	}
	written, err := index.Refresh(root, opts.Logger)
	slices.Sort(written)
	written = slices.Compact(written)
	fields := []toolresult.Field{{Key: "kb-root", Value: root}, {Key: "written", Value: append([]string{}, written...)}}
	var refusal index.Refusal
	switch {
	case errors.As(err, &refusal):
		return refused(fields, refusal.Items...)
	case err != nil:
		return failed(fields, err)
	case len(written) == 0:
		return toolresult.Unchanged, fields
	}
	return toolresult.Done, fields
}

// maintenanceCommand is a verb over the KB found from the working directory.
func maintenanceCommand(use, short, long string, run func(maintenanceOptions) (int, error)) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			code, err := run(maintenanceOptions{WorkDir: wd, Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(), Logger: processLog.logger})
			if err != nil {
				return err
			}
			if code != 0 {
				return exitCode(code)
			}
			return nil
		},
	}
}

func init() {
	rootCmd.AddCommand(maintenanceCommand("refresh",
		"derive the KB's derived metadata, .index/ and claim-graph sheet",
		`refresh derives every derived field of the KB at kb-root/ beside the
repository's .git — subtree aggregates, solidity lines and annotations,
leaf-references footers — and writes .index/*.jsonl and the placeholder
claim-graph.svg (only where none exists or the existing one is the
placeholder). It writes only files whose bytes change. Its result is one YAML
document on stdout.`, runRefresh))
}
