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
	checkKBRoot   = kb.CheckKBRoot
	checkLock     = kb.CheckLock
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
	if item, held := write.HeldLockItem(err, root, remedy); held {
		return nil, toolresult.Retry, []toolresult.Item{item}, nil
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

// degradedFaults is the faults that degrade a KB's load, as help names them.
const degradedFaults = "an edge authored twice, an edge naming no node, a cycle among the scoring edges"

// degradedRefusedHelp is the help sentence of a verb refuseDegraded guards.
const degradedRefusedHelp = "A KB whose load is degraded is refused, naming each fault; " + index.RefreshRemedy + " is the remedy."

// refuseDegraded is the answer of a verb that does not run over a degraded
// load: the records' refusal or a failure to read them, or a check: degraded
// item per fault the load resolved, each with the remedy kbase refresh. stop
// is false where the load is clean.
func refuseDegraded(fields []toolresult.Field, src *kb.Source) (outcome string, out []toolresult.Field, stop bool) {
	return refusalOrFailure(fields, index.RefuseDegraded(src))
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
// that earns it, each of the conversion's warnings a line of its own.
func migrationNotice(verb string, stderr io.Writer, m *kb.Migration) toolresult.Record {
	fmt.Fprintf(stderr, "%s: the KB's metadata format %s was converted to %s in memory: %s; kbase refresh saves the conversion\n",
		verb, m.From, m.To, m.Note)
	warnings := []toolresult.Item{}
	for _, w := range m.Warnings {
		at := w.Path
		if w.Line > 0 {
			at = fmt.Sprintf("%s:%d", w.Path, w.Line)
		}
		fmt.Fprintf(stderr, "%s: conversion warning: %s: %s\n", verb, at, w.Detail)
		warnings = append(warnings, toolresult.Item{Path: w.Path, Line: w.Line, Detail: w.Detail})
	}
	return toolresult.Record{{Key: "from", Value: m.From}, {Key: "to", Value: m.To}, {Key: "note", Value: m.Note},
		{Key: "warnings", Value: warnings}}
}

// emitResult writes a verb's result document and returns its exit code.
func emitResult(verb string, stdout io.Writer, outcome string, fields []toolresult.Field) (int, error) {
	if err := toolresult.Emit(stdout, outcome, fields...); err != nil {
		return 0, fmt.Errorf("%s: writing the result: %w", verb, err)
	}
	return toolresult.ExitCode(outcome), nil
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
	untouched := []toolresult.Field{{Key: "kb-root", Value: root}, {Key: "written", Value: []string{}}, {Key: "removed", Value: []string{}},
		{Key: "warnings", Value: []toolresult.Item{}}}
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
	if err := restamp(src); err != nil {
		return failed(append(fields, untouched[1:]...), err)
	}
	r, err := index.RefreshWithWarnings(src, opts.Logger)
	if r.NoDot {
		sayNoDot(opts.Stderr, "refresh")
	}
	written := slices.Compact(slices.Sorted(slices.Values(r.Written)))
	removed := slices.Sorted(slices.Values(r.Removed))
	fields = append(fields, toolresult.Field{Key: "written", Value: append([]string{}, written...)},
		toolresult.Field{Key: "removed", Value: append([]string{}, removed...)})
	warnings := toolresult.Field{Key: "warnings", Value: append([]toolresult.Item{}, r.Warnings...)}
	var refusal toolresult.Refusal
	switch {
	case errors.As(err, &refusal):
		return refused(append(fields, warnings), refusal...)
	case err != nil:
		return failed(append(fields, warnings), err)
	}
	if !r.NoDot {
		fields = append(fields, toolresult.Field{Key: "sheet", Value: sheetDrawn})
	} else {
		fields = append(fields, toolresult.Field{Key: "sheet", Value: sheetPlaceholder}, toolresult.Field{Key: "dot", Value: dotNotFound})
	}
	fields = append(fields, warnings)
	if len(written) == 0 && len(removed) == 0 {
		return toolresult.Unchanged, fields
	}
	return toolresult.Done, fields
}

// Whether refresh drew the sheets through Graphviz dot or, with no dot on
// PATH, stood the placeholder in for the root sheet.
const (
	sheetPlaceholder = "placeholder"
	sheetDrawn       = "drawn"
	dotNotFound      = "not found on PATH"
)

// maintenanceCommand is a verb over the KB found from the working directory.
func maintenanceCommand(use, short, long string, run func(maintenanceOptions) (int, error)) *cobra.Command {
	return &cobra.Command{
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
	}
}

func refreshCommand() *cobra.Command {
	return maintenanceCommand("refresh",
		"render the KB's register pages and claim-graph sheets; repair a degraded load and save a migration",
		`refresh renders the KB at kb-root/ beside the repository's .git from its
records: every register page the KB calls for — kb-root/`+kb.RegisterFile+` and
<volume>/`+kb.RegisterFile+`, generated and never read as a source — removing a
page it no longer calls for, and the claim-graph sheets, drawn through Graphviz dot:
kb-root/claim-graph.svg always; where two or more volumes hold nodes,
kb-root/claim-graph-digest.svg and <volume>/claim-graph.svg for each volume
holding a node — removing a digest or volume sheet the KB no longer calls for.
With no dot on PATH it draws nothing, leaves every sheet standing as it is,
writes a placeholder kb-root/claim-graph.svg only where none exists, and says so
on stderr and in its result: sheet is placeholder (nothing drawn, whatever
stands) and dot names the cause. It writes no
authored file but in two cases. A KB in an older metadata format is rewritten
whole in the current one, the format stamp standing at the old version until
the save's last write moves it to the current one, and the files the old
format alone used are removed. A KB whose load is degraded — `+degradedFaults+` —
has its records rewritten as the load resolved them, each record dropped, and
each edge demoted to break a cycle, listed under warnings. A missing record
file is refused, remedy "`+kb.RestoreFromGitRemedy+`"; one the load cannot parse
is refused, naming its file and line; refresh repairs only a degraded load. It
writes only files whose bytes change. Its result is one YAML document on
stdout.`, runRefresh)
}
