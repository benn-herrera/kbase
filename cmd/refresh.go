package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"kbase/internal/build"
	"kbase/internal/filelock"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
	"kbase/internal/write"
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
	checkLock     = "lock"
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

// holdKB takes the write lock of the KB at root for a verb that writes it,
// refusing while a run — a build or a scoring run — holds the operation lock:
// at once, and again once the lock is taken, for a run that started during the
// wait. Where release is nil, outcome and items are the verb's answer.
func holdKB(root, remedy string) (release func(), outcome string, items []toolresult.Item, err error) {
	return holdWriting(root, remedy, true)
}

// holdWriting is holdKB for a verb that writes the KB at root. probe is
// whether to refuse while a run holds the operation lock: a scoring run's own
// landing and refresh clear it, the run that holds that lock being the writer.
func holdWriting(root, remedy string, probe bool) (release func(), outcome string, items []toolresult.Item, err error) {
	if probe {
		if running, err := build.RunningOperation(root); err != nil || running != nil {
			return nil, toolresult.Refused, running, err
		}
	}
	release, outcome, items, err = holdWrite(root, remedy)
	if release == nil || err != nil {
		return release, outcome, items, err
	}
	if probe {
		if running, err := build.RunningOperation(root); err != nil || running != nil {
			release()
			return nil, toolresult.Refused, running, err
		}
	}
	return release, "", nil, nil
}

// holdWrite takes the KB write lock at root, waiting out a writer in progress;
// where release is nil, outcome and items are the verb's answer.
func holdWrite(root, remedy string) (release func(), outcome string, items []toolresult.Item, err error) {
	release, err = write.LockKB(root)
	if errors.Is(err, filelock.ErrHeld) {
		return nil, toolresult.Retry, []toolresult.Item{{Check: checkLock, Path: filepath.Dir(root), Remedy: remedy,
			Detail: "another write op or refresh held the KB's write lock past the wait; nothing was written"}}, nil
	}
	if err != nil {
		return nil, "", nil, err
	}
	return release, "", nil, nil
}

// openKB is the KB at root as the loader opens it, or the items it refuses.
func openKB(root string) (*kb.Source, []toolresult.Item, error) {
	src, err := kbload.Open(root)
	var refusal toolresult.Refusal
	if errors.As(err, &refusal) {
		return nil, refusal, nil
	}
	return src, nil, err
}

// withMigration appends the record a migrated KB's result carries after
// kb-root, and writes the sentence that says what the load converted to
// stderr: once per call, on every verb that loads the KB.
func withMigration(fields []toolresult.Field, src *kb.Source, verb string, stderr io.Writer) []toolresult.Field {
	if m, ok := src.Migration(); ok {
		return append(fields, toolresult.Field{Key: "migration", Value: migrationNotice(verb, stderr, &m)})
	}
	return fields
}

// migrationNotice is one migrated KB's conditional record and the sentence
// that earns it.
func migrationNotice(verb string, stderr io.Writer, m *kb.Migration) toolresult.Record {
	fmt.Fprintf(stderr, "%s: the KB's metadata format %s was converted to %s in memory: %s; kbase refresh writes the conversion\n",
		verb, m.From, m.To, m.Note)
	return toolresult.Record{{Key: "from", Value: m.From}, {Key: "to", Value: m.To}, {Key: "note", Value: m.Note}}
}

// emitResult writes a verb's result document and returns its exit code.
func emitResult(verb string, stdout io.Writer, outcome string, fields []toolresult.Field) (int, error) {
	if err := toolresult.Emit(stdout, outcome, fields...); err != nil {
		return 0, fmt.Errorf("%s: writing the result: %w", verb, err)
	}
	return exitCodes[outcome], nil
}

// sayNoDot tells the human on stderr that a verb's refresh drew no
// claim-graph sheet for want of Graphviz dot.
func sayNoDot(stderr io.Writer, verb string) {
	fmt.Fprintf(stderr, "%s: no Graphviz dot on PATH; the claim-graph sheets were not rendered\n", verb)
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
	untouched := []toolresult.Field{{Key: "kb-root", Value: root}, {Key: "written", Value: []string{}}, {Key: "removed", Value: []string{}}}
	release, outcome, items, err := holdKB(root, "re-run kbase refresh")
	if err != nil {
		return failed([]toolresult.Field{{Key: "kb-root", Value: root}}, err)
	}
	if release == nil {
		return outcome, append(untouched, toolresult.Field{Key: toolresult.RefusalsKey, Value: items})
	}
	defer release()
	src, refusals, err := openKB(root)
	if err != nil {
		return failed([]toolresult.Field{{Key: "kb-root", Value: root}}, err)
	}
	if refusals != nil {
		return refused(untouched, refusals...)
	}
	fields := withMigration([]toolresult.Field{{Key: "kb-root", Value: root}}, src, "refresh", opts.Stderr)
	written, removed, noDot, err := index.RefreshReporting(src, opts.Logger)
	if noDot {
		sayNoDot(opts.Stderr, "refresh")
	}
	slices.Sort(written)
	written = slices.Compact(written)
	slices.Sort(removed)
	fields = append(fields, toolresult.Field{Key: "written", Value: append([]string{}, written...)},
		toolresult.Field{Key: "removed", Value: append([]string{}, removed...)})
	var refusal toolresult.Refusal
	switch {
	case errors.As(err, &refusal):
		return refused(fields, refusal...)
	case err != nil:
		return failed(fields, err)
	case len(written) == 0 && len(removed) == 0:
		return toolresult.Unchanged, fields
	}
	return toolresult.Done, fields
}

// maintenanceCommand is a verb over the KB found from the working directory.
func maintenanceCommand(use, short, long string, run func(maintenanceOptions) (int, error)) *cobra.Command {
	return mcpBinding(&cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := invocationDir(cmd)
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
	}, mcpBound)
}

func init() {
	rootCmd.AddCommand(maintenanceCommand("refresh",
		"derive the KB's derived metadata, .index/ and claim-graph sheets",
		`refresh derives every derived field of the KB at kb-root/ beside the
repository's .git — subtree aggregates, strength lines and annotations,
leaf-references footers — and writes .index/*.yaml and the claim-graph
sheets, drawn through Graphviz dot: kb-root/claim-graph.svg and, where two
or more volumes hold nodes, kb-root/claim-graph-digest.svg and
<volume>/claim-graph.svg (see render-claim-graph), removing a digest or volume sheet the KB no
longer calls for. With no
dot on PATH it draws nothing, says so on stderr, and writes a placeholder
kb-root/claim-graph.svg only where none exists. A KB in an older metadata
format is rewritten whole in the current one, the entry point stamped last,
and the files the old format alone used are removed. It writes only files
whose bytes change. Its result is one YAML document on stdout.`, runRefresh))
}
