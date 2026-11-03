package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/build"
	"kbase/internal/buildrecords"
	"kbase/internal/buildstore"
	"kbase/internal/gitledger"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbdocs"
	"kbase/internal/kbload"
	"kbase/internal/log"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
	"kbase/internal/write"
)

type maintenanceOptions struct {
	// WorkDir is where the search for the repository root starts.
	WorkDir string
	Commit  bool
	Stdout  io.Writer
	Stderr  io.Writer
	Logger  log.Logger
}

// The refusal classes of a verb that finds no KB to work on, and of
// update-format over a kb-root/ with uncommitted changes.
const (
	checkWorktree   = "worktree"
	checkKBRoot     = kb.CheckKBRoot
	checkLock       = kb.CheckLock
	checkDirtyPaths = "dirty-paths"
)

// kbRootFrom is the kb-root beside the .git at or above dir, or a refusal
// naming why there is none.
func kbRootFrom(dir string) (string, []toolresult.Item, error) {
	repo, err := kb.RepositoryRoot(dir)
	if err != nil {
		return "", nil, err
	}
	if repo == "" {
		return "", []toolresult.Item{worktreeRefusal(dir, fmt.Sprintf("%s is not inside a git worktree with %s/ beside its .git", dir, kb.KBDir))}, nil
	}
	root := filepath.Join(repo, kb.KBDir)
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", []toolresult.Item{{Check: checkKBRoot, Path: root,
			Detail: fmt.Sprintf("%s has no %s/ beside its .git", repo, kb.KBDir)}}, nil
	}
	return root, nil, nil
}

// worktreeRefusal is the refusal of a verb that finds no git worktree at
// dir, detail saying why.
func worktreeRefusal(dir, detail string) toolresult.Item {
	return toolresult.Item{Check: checkWorktree, Path: dir, Remedy: "git init", Detail: detail}
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

// loadErrorRefusedHelp is the help sentence of a verb refuseLoadError guards.
const loadErrorRefusedHelp = "A KB whose load fails is refused, naming each fault with its file and record."

// refuseLoadError is the answer of a verb whose KB load fails: the load's
// refusal, an item per fault, or a failure to read the KB. stop is false where
// the KB loads.
func refuseLoadError(fields []toolresult.Field, src *kb.Source) (outcome string, out []toolresult.Field, stop bool) {
	_, err := index.Derive(src)
	return refusalOrFailure(fields, err)
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
	fmt.Fprintf(stderr, "%s: the KB's format %s was converted to %s in memory: %s; kbase update-format saves the conversion\n",
		verb, m.From, m.To, m.Note)
	return migrationRecord(verb, stderr, m, m.Warnings)
}

// migrationRecord is m's record, listing warnings as its own, each written to
// stderr as a line of its own.
func migrationRecord(verb string, stderr io.Writer, m *kb.Migration, warnings []kb.MigrationWarning) toolresult.Record {
	items := []toolresult.Item{}
	for _, w := range warnings {
		at := w.Path
		if w.Line > 0 {
			at = fmt.Sprintf("%s:%d", w.Path, w.Line)
		}
		fmt.Fprintf(stderr, "%s: conversion warning: %s: %s\n", verb, at, w.Detail)
		items = append(items, toolresult.Item{Path: w.Path, Line: w.Line, Detail: w.Detail})
	}
	return toolresult.Record{{Key: "from", Value: m.From}, {Key: "to", Value: m.To}, {Key: "note", Value: m.Note},
		{Key: "warnings", Value: items}}
}

// emitResult writes a verb's result document and returns its exit code.
func emitResult(verb string, stdout io.Writer, outcome string, fields []toolresult.Field) (int, error) {
	if err := toolresult.Emit(documentOut(stdout), outcome, fields...); err != nil {
		return 0, fmt.Errorf("%s: writing the result: %w", verb, err)
	}
	return toolresult.ExitCode(outcome), nil
}

// sayNoDot tells the human on stderr that a verb's refresh drew no
// claim-graph sheet for want of Graphviz dot.
func sayNoDot(stderr io.Writer, verb string) {
	fmt.Fprintf(stderr, "%s: no Graphviz dot on PATH; the claim-graph sheets were not rendered\n", verb)
}

func runUpdateFormat(opts maintenanceOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "update-format"); err != nil {
		return 0, err
	}
	outcome, fields := updateFormat(opts)
	return emitResult("update-format", opts.Stdout, outcome, fields)
}

// formatResultList is one kind of conversion warning update-format's result
// lists under the key its list names, and the item it lists each as.
type formatResultList struct {
	list string
	item func(kb.MigrationWarning) toolresult.Record
}

// formatResultLists are the lists in result order; every other warning stays
// among migration's. An item's id and text are read from the KB, so each is a
// toolresult.Verbatim, written as it stands.
var formatResultLists = []formatResultList{
	{kbload.ListIDs, func(w kb.MigrationWarning) toolresult.Record {
		return toolresult.Record{{Key: "from", Value: toolresult.Verbatim(w.ID)}, {Key: "to", Value: w.Text}}
	}},
	{kbload.ListBareIDs, func(w kb.MigrationWarning) toolresult.Record {
		return toolresult.Record{{Key: "path", Value: w.Path}, {Key: "line", Value: w.Line}, {Key: "id", Value: toolresult.Verbatim(w.ID)}}
	}},
	{kbload.ListNoEdge, func(w kb.MigrationWarning) toolresult.Record {
		return toolresult.Record{{Key: "id", Value: toolresult.Verbatim(w.ID)}, {Key: "text", Value: toolresult.Verbatim(w.Text)}}
	}},
	{kbload.ListStrengthenBy, func(w kb.MigrationWarning) toolresult.Record {
		return toolresult.Record{{Key: "id", Value: toolresult.Verbatim(w.ID)}, {Key: "text", Value: toolresult.Verbatim(w.Text)}}
	}},
}

// listConversion is warnings split into those formatResultLists lists, each
// list a field keyed by its name in result order, and the rest.
func listConversion(warnings []kb.MigrationWarning) (own []kb.MigrationWarning, lists []toolresult.Field) {
	listed := map[string][]toolresult.Record{}
	for _, w := range warnings {
		if i := slices.IndexFunc(formatResultLists, func(l formatResultList) bool { return l.list == w.List }); i >= 0 {
			listed[w.List] = append(listed[w.List], formatResultLists[i].item(w))
			continue
		}
		own = append(own, w)
	}
	for _, l := range formatResultLists {
		lists = append(lists, toolresult.Field{Key: l.list, Value: append([]toolresult.Record{}, listed[l.list]...)})
	}
	return own, lists
}

// updateFormatRetryRemedy is the remedy of an update-format turned away by
// another writer's KB write lock.
var updateFormatRetryRemedy = "re-run " + index.UpdateFormatRemedy

func updateFormat(opts maintenanceOptions) (string, []toolresult.Field) {
	root, refusals, err := kbRootFrom(opts.WorkDir)
	if err != nil {
		return failed(nil, err)
	}
	if refusals != nil {
		return refused(nil, refusals...)
	}
	fields := []toolresult.Field{{Key: "kb-root", Value: root}}
	release, outcome, items, err := holdKB(root, updateFormatRetryRemedy)
	if err != nil {
		return failed(fields, err)
	}
	if release == nil {
		return outcome, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: items})
	}
	defer release()
	_, dirty, err := dirtyPaths(filepath.Dir(root), []string{kb.KBDir}, opts.Logger)
	if err != nil {
		return failed(fields, err)
	}
	if dirty != nil {
		return refused(fields, dirty...)
	}
	src, refusals, err := openKB(root)
	if err != nil {
		return failed(fields, err)
	}
	if refusals != nil {
		return refused(fields, refusals...)
	}
	moves, move, dirty, err := rootRecordMoves(src, opts.Logger)
	if err != nil {
		return failed(fields, err)
	}
	if dirty != nil {
		return refused(fields, dirty...)
	}
	m, migrated := src.Migration()
	if migrated {
		fmt.Fprintf(opts.Stderr, "update-format: converting the KB's format %s to %s: %s\n", m.From, m.To, m.Note)
	}
	wrote, gone, noDot, err := index.UpdateFormat(src, moves, move, opts.Logger)
	var unfenced []string
	if err == nil {
		var maintained []string
		maintained, unfenced, err = maintainBlocks(src)
		wrote = append(wrote, maintained...)
	}
	var lists []toolresult.Field
	if migrated {
		own := m.Warnings
		if err == nil {
			own, lists = listConversion(m.Warnings)
		}
		fields = append(fields, toolresult.Field{Key: "migration", Value: migrationRecord("update-format", opts.Stderr, &m, own)})
	}
	if noDot {
		sayNoDot(opts.Stderr, "update-format")
	}
	written := slices.Compact(slices.Sorted(slices.Values(wrote)))
	removed := slices.Sorted(slices.Values(gone))
	fields = append(fields, toolresult.Field{Key: "written", Value: append([]string{}, written...)},
		toolresult.Field{Key: "removed", Value: append([]string{}, removed...)})
	var refusal toolresult.Refusal
	switch {
	case errors.As(err, &refusal):
		return refused(fields, refusal...)
	case err != nil:
		return failed(fields, err)
	}
	fields = append(fields, toolresult.Field{Key: "unfenced", Value: append([]string{}, unfenced...)})
	if !noDot {
		fields = append(fields, toolresult.Field{Key: "sheet", Value: sheetDrawn})
	} else {
		fields = append(fields, toolresult.Field{Key: "sheet", Value: sheetNotDrawn}, toolresult.Field{Key: "dot", Value: dotNotFound})
	}
	commits := &kbCommits{repo: filepath.Dir(root), lg: opts.Logger}
	outcome, uncommitted := toolresult.Unchanged, ""
	if len(written) > 0 || len(removed) > 0 {
		outcome = toolresult.Done
	}
	if opts.Commit && outcome == toolresult.Done {
		from, to := kb.FormatVersion, kb.FormatVersion
		if migrated {
			from, to = m.From, m.To
		}
		owned := []string{kb.KBDir}
		for _, mv := range moves {
			owned = append(owned, mv.From)
		}
		uncommitted = commits.final("kb-write: update-format "+from+" → "+to, owned...)
	}
	return outcome, append(append(fields, commits.fields(uncommitted)...), lists...)
}

// dirtyPaths is the repository at dir over the repository-relative paths, and
// a dirty-paths refusal item per path under them that git reports changed,
// staged or untracked, none where all are clean: restoring kb-root/ and the
// moved build records from HEAD is what reverses a conversion. Where
// buildstore.OpenRepo refuses the repository, its items are that refusal's.
func dirtyPaths(dir string, paths []string, lg log.Logger) (*gitledger.Repo, []toolresult.Item, error) {
	repo, err := buildstore.OpenRepo(dir, paths, lg)
	var refusal toolresult.Refusal
	if errors.As(err, &refusal) {
		return nil, refusal, nil
	}
	if err != nil {
		return nil, nil, err
	}
	dirty, err := repo.Dirty()
	if err != nil {
		return nil, nil, err
	}
	var items []toolresult.Item
	for _, p := range dirty {
		item := toolresult.Item{Check: checkDirtyPaths, Path: p, Remedy: "commit or discard the changes under " + kb.KBDir + "/",
			Detail: "an uncommitted change under " + kb.KBDir + "/; update-format runs over a clean " + kb.KBDir + "/, so restoring it from HEAD reverses the conversion"}
		if !strings.HasPrefix(p, kb.KBDir+"/") {
			item.Remedy = "commit or discard the changes to " + p
			item.Detail = "an uncommitted build record; the conversion moves it into " + buildrecords.Dir + "/ with git mv, so restoring it from HEAD reverses the conversion"
		}
		items = append(items, item)
	}
	return repo, items, nil
}

// rootRecordMoves is each build record src's conversion moves from the
// repository root into kb-root/.build/, and the git mv that moves them, which
// index.UpdateFormat calls once the load has passed; or a dirty-paths refusal
// item per one of them with uncommitted changes.
func rootRecordMoves(src *kb.Source, lg log.Logger) ([]index.RecordMove, func(from, to string) error, []toolresult.Item, error) {
	var moves []index.RecordMove
	var from []string
	for _, rel := range buildrecords.RecordFiles {
		if root := buildrecords.RootRecordFile(rel); slices.Contains(src.Obsolete(), root) {
			moves = append(moves, index.RecordMove{From: root, To: rel})
			from = append(from, root)
		}
	}
	if len(moves) == 0 {
		return nil, nil, nil, nil
	}
	repo, dirty, err := dirtyPaths(src.Repo(), from, lg)
	if err != nil || dirty != nil {
		return nil, nil, dirty, err
	}
	return moves, repo.Move, nil, nil
}

// maintainBlocks rewrites the maintained blocks of src's documents, named for
// the repository its kb-root sits in, as init names them.
func maintainBlocks(src *kb.Source) (written, unfenced []string, err error) {
	rows, err := index.Derive(src)
	if err != nil {
		return nil, nil, err
	}
	return kbdocs.Maintain(src, filepath.Base(src.Repo()), kb.SheetDigestDrawn(rows.Nodes))
}

// Whether a verb's re-derivation drew the sheets through Graphviz dot or,
// with no dot on PATH, drew nothing.
const (
	sheetNotDrawn = "not-drawn"
	sheetDrawn    = "drawn"
	dotNotFound   = "not found on PATH"
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

func updateFormatCommand() *cobra.Command {
	var commit bool
	cmd := maintenanceCommand("update-format",
		"convert the KB to the current format, re-derive every derived file and rewrite the maintained blocks",
		`update-format converts the KB at kb-root/ in the git worktree root to the
current format, run alone, so that restoring kb-root/ and any moved build
record from HEAD reverses it. A KB in an older format is rewritten in the
current one, the format stamp standing at the old version until the save's last
write moves it, and the files the old format alone used are removed; once the
KB loads, each build record standing at the repository root is moved into
kb-root/.build/ with git mv before the save writes, its root path listed under
"removed" and its new path under "written". Every run, whether or not the
format changed, re-derives every derived file — every summary page the KB
calls for, generated and never read as a source, removing a page it no longer
calls for, and the claim-graph sheets, drawn through Graphviz dot — and
rewrites the block kbase maintains in AGENTS.md, CONVENTIONS.md and README.md,
between its vvv-kbase and ^^^-kbase marker lines, leaving the text outside it
as it stands. An absent AGENTS.md or CONVENTIONS.md is written whole; README.md
is written only by a build, never created here; a document without the one pair
of marker lines is left untouched and listed under "unfenced". It writes only
files whose bytes change. While a conversion is pending every other write is
refused (check: index; remedy: `+index.UpdateFormatRemedy+`).

update-format refuses a kb-root/, or a build record the conversion moves, with
uncommitted changes, naming each path (check: dirty-paths; remedy: commit or
discard them). A missing record file is refused (check: index; remedy:
`+kb.RestoreFromGitRemedy+`); any other load error is refused, naming each fault
with its file and record, and update-format repairs none. A KB stamped newer
than kbase is refused (check: kb-format; remedy: `+kbload.UpdateRemedy+`), and so
is a conversion that cannot carry a file's content, naming the file and line
(check: conversion). update-format waits up to 30 s for the KB write lock,
then answers retry (check: lock; remedy: `+updateFormatRetryRemedy+`); while a
build or a score run holds the operation lock it is refused at once (check:
lock).

With no dot on PATH it draws nothing, leaves every sheet standing as it is,
writes a placeholder kb-root/claim-graph.svg only where none exists, and says
so in its result: "sheet" is not-drawn and "dot" names the cause.

Where the format changed, "migration" carries the conversion; its "warnings"
include each citation link and axiom heading it could not resolve without
guessing. "ids" pairs each work's and axiom's 3.0.0 id ("from") with the id
minted in its place ("to"); "bare-ids" lists each bare id turned into a
citation ("path", "line", "id"); "no-edge" and "strengthen-by" list each dropped
field's node "id" and "text", for the author to recast. A refused or failed run
lists these four among migration's warnings instead.

With `+argmark.Flag("commit")+`, it commits the conversion as one commit of kb-root/ and the
build records it moved, labelled kb-write: update-format <from> → <to> and
listed under "commits"; its clean precondition makes that commit exactly the
conversion. A commit that fails, for example on a hook, leaves the conversion
written, outcome done, with "uncommitted" saying why. Committing is never the
default: without it update-format commits nothing and "commits" is empty.`,
		func(opts maintenanceOptions) (int, error) {
			opts.Commit = commit
			return runUpdateFormat(opts)
		})
	cmd.Flags().BoolVar(&commit, "commit", false, "commit the conversion as one commit")
	return mcp.Notes(cmd, "",
		"With no dot on PATH, stderr says so too.")
}
