package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/buildstore"
	"kbase/internal/gitledger"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
	"kbase/internal/write"
)

type writeOpOptions struct {
	// Op is the write op whose batch this is, write.Batch for kbase write's
	// mixed batch.
	Op string
	// WorkDir is where the search for the repository root starts.
	WorkDir string
	// ValuesFile is the batch's path; "" reads Stdin.
	ValuesFile string
	Stdin      io.Reader
	DryRun     bool
	Commit     bool
	// ArgumentEntry is whether the values were composed from a tool call's
	// arguments rather than a batch the caller wrote, so a refusal naming
	// no path has a line and column that would point nowhere the caller can see.
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
		file := kb.ProjectPath(filepath.Dir(root), opts.ValuesFile)
		values, err = os.ReadFile(file)
	} else {
		values, err = io.ReadAll(opts.Stdin)
	}
	if err != nil {
		return failed(fields, fmt.Errorf("reading the batch: %w", err))
	}
	commits := &kbCommits{repo: filepath.Dir(root), lg: opts.Logger}
	run := write.Options{KBRoot: root, Values: values, DryRun: opts.DryRun, Commit: opts.Commit, Preflight: commits.preflight, Logger: opts.Logger}
	var res write.Result
	var uncommitted string
	release, outcome, items, err := holdKB(root, write.RetryRemedy(opts.Op), opts.Logger)
	if err != nil {
		return failed(fields, err)
	}
	if release == nil {
		res = write.Result{Outcome: outcome, DryRun: opts.DryRun || write.BatchDryRun(opts.Op, values), Refusals: items}
	} else {
		defer release()
		res = write.Run(opts.Op, run)
		if res.Commit && !res.DryRun && res.Outcome == toolresult.Done {
			uncommitted = commits.final(batchLabel(opts.Op, res.Entries), kb.KBDir)
		}
	}
	if res.Migration != nil {
		fields = append(fields, toolresult.Field{Key: "migration", Value: migrationNotice(opts.Op, opts.Stderr, res.Migration)})
	}
	fields = append(fields, batchFields(res, opts.Op, opts.Stderr)...)
	fields = append(fields, commits.fields(uncommitted)...)
	if opts.ArgumentEntry {
		for i := range res.Refusals {
			if res.Refusals[i].Path == "" {
				res.Refusals[i].Line, res.Refusals[i].Column = 0, 0
			}
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

// batchFields is a batch's result keys after kb-root: dry-run, each entry's
// report, the paths written and removed, and whether the sheets were drawn
// where the derivation ran.
func batchFields(res write.Result, verb string, stderr io.Writer) []toolresult.Field {
	entries := make([]toolresult.Record, 0, len(res.Entries))
	for _, e := range res.Entries {
		r := toolresult.Record{{Key: "entry", Value: e.Entry}, {Key: "operation", Value: e.Operation}, {Key: "outcome", Value: e.Outcome}}
		switch {
		case write.Adds(e.Operation) || write.Edits(e.Operation):
			r = append(r, toolresult.Field{Key: "id", Value: e.ID})
			if e.Operation == write.AddAxiom {
				r = append(r, toolresult.Field{Key: "heading", Value: e.Heading})
			}
			if e.Rewritten != nil {
				rows := make([]toolresult.Record, 0, len(e.Rewritten))
				for _, w := range e.Rewritten {
					rows = append(rows, toolresult.Record{{Key: "path", Value: w.Path}, {Key: "line", Value: w.Line}})
				}
				r = append(r, toolresult.Field{Key: "rewritten", Value: rows})
			}
		case write.Removes(e.Operation):
			rows := make([]toolresult.Record, 0, len(e.RemovedEdges))
			for _, row := range e.RemovedEdges {
				rows = append(rows, query.EdgeRowRecord(row))
			}
			r = append(r, toolresult.Field{Key: "id", Value: e.ID}, toolresult.Field{Key: "removed-edges", Value: rows})
		case e.Operation == write.AddCitation:
			r = append(r, toolresult.Field{Key: "document", Value: e.Document}, toolresult.Field{Key: "line", Value: e.Line},
				toolresult.Field{Key: "id", Value: e.ID}, toolresult.Field{Key: "link", Value: toolresult.Verbatim(e.Link)})
		case write.WritesEdge(e.Operation):
			r = append(r, toolresult.Field{Key: "source", Value: e.Source}, toolresult.Field{Key: "target", Value: e.Target},
				toolresult.Field{Key: "relation", Value: e.Relation})
			if e.Operation == write.RestoreDemoted {
				r = append(r, toolresult.Field{Key: "restored", Value: nonNil(e.Restored)})
			}
		}
		entries = append(entries, r)
	}
	fields := []toolresult.Field{{Key: "dry-run", Value: res.DryRun}, {Key: "entries", Value: entries},
		{Key: "written", Value: nonNil(res.Written)}, {Key: "removed", Value: nonNil(res.Removed)}}
	switch {
	case !res.Derived:
	case res.Drawn:
		fields = append(fields, toolresult.Field{Key: "sheet", Value: sheetDrawn})
	default:
		sayNoDot(stderr, verb)
		fields = append(fields, toolresult.Field{Key: "sheet", Value: sheetNotDrawn}, toolresult.Field{Key: "dot", Value: dotNotFound})
	}
	return fields
}

// The labels of a committing batch's pre-flight commits: where the changes
// lie, never who made them.
const (
	preflightOutsideIndexLabel = "kb-write: kb-root/ changes outside .index/ made before this batch"
	preflightRecordsLabel      = "kb-write: record changes made before this batch"
)

// kbCommits is the commits a committing write makes in the repository at
// repo, in the order made, each taking only the paths it is given.
type kbCommits struct {
	repo string
	lg   log.Logger
	made []toolresult.Record
}

// commit commits the uncommitted changes under owned, root-relative paths
// as buildstore.OpenRepo takes them, labelled label; where there are none it
// makes no commit.
func (c *kbCommits) commit(label string, owned ...string) error {
	repo, err := buildstore.OpenRepo(c.repo, owned, c.lg)
	if err != nil {
		return err
	}
	dirty, err := repo.Dirty()
	if err != nil || len(dirty) == 0 {
		return err
	}
	commit, err := repo.Commit(label, "")
	if err != nil {
		return err
	}
	paths, err := repo.Paths(commit)
	if err != nil {
		return err
	}
	c.made = append(c.made, toolresult.Record{{Key: "commit", Value: commit}, {Key: "label", Value: label}, {Key: "paths", Value: paths}})
	return nil
}

// preflight commits, before a committing batch writes, kb-root/'s uncommitted
// changes outside .index/, then those inside it, each alone, so the batch's
// own commit holds the batch and nothing else. On a dry run it opens the
// repository and commits nothing, so the dry run meets the real run's
// worktree check.
func (c *kbCommits) preflight(dryRun bool) error {
	if dryRun {
		_, err := buildstore.OpenRepo(c.repo, nil, c.lg)
		return err
	}
	index := kb.KBDir + "/" + kb.IndexDir
	for _, p := range []struct {
		label string
		owned []string
	}{
		{preflightOutsideIndexLabel, []string{kb.KBDir, gitledger.Exclude(index)}},
		{preflightRecordsLabel, []string{index}},
	} {
		if err := c.commit(p.label, p.owned...); err != nil {
			return fmt.Errorf("a pre-flight commit (%s) failed, so the batch wrote nothing: %w", p.label, err)
		}
	}
	return nil
}

// final commits what a write wrote, under owned, labelled label, and returns
// the uncommitted detail where that commit fails, "" where it is made.
func (c *kbCommits) final(label string, owned ...string) string {
	if err := c.commit(label, owned...); err != nil {
		return "the changes are written and stand, so do not re-run this call; they are not committed: " + err.Error()
	}
	return ""
}

// fields is a write's commits key, and uncommitted where it is not "".
func (c *kbCommits) fields(uncommitted string) []toolresult.Field {
	fields := []toolresult.Field{{Key: "commits", Value: append([]toolresult.Record{}, c.made...)}}
	if uncommitted != "" {
		fields = append(fields, toolresult.Field{Key: "uncommitted", Value: uncommitted})
	}
	return fields
}

// batchLabel is a committing batch's own commit label, kb-write: <ops> <ids>:
// the ops its entries ran, op where it has none, and the node ids they
// entered or name, each once, in entry order.
func batchLabel(op string, entries []write.EntryResult) string {
	var ops, ids []string
	for _, e := range entries {
		if !slices.Contains(ops, e.Operation) {
			ops = append(ops, e.Operation)
		}
		for _, id := range append([]string{e.ID}, e.Names...) {
			if id != "" && !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	if len(ops) == 0 {
		ops = []string{op}
	}
	return strings.TrimSpace("kb-write: " + strings.Join(ops, ",") + " " + strings.Join(ids, ","))
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// valuesHelp closes the root's help: how the CLI takes a batch,
// then the write batch's contract.
var valuesHelp = `kbase write and each write op read their batch as YAML (JSON accepted) on
stdin, or in the file --values names. A refusal names each offending key, its
position and every key the op takes. Each write op's own subcommand takes a
batch of that op alone, its entries naming no operation.

` + batchHelp

// batchHelp is the write batch's contract, which kbase write's help and the
// root's carry in full.
var batchHelp = `Every write is a batch: an ordered list of entries under "entry", each
naming its op under "operation" and carrying that op's keys; ` + argmark.Flag("dry-run") + ` and
` + argmark.Flag("commit") + ` are its options. A batch may mix ops. An empty batch writes no record
and runs the derivation, bringing the summary pages and sheets up to date after
leaf edits.

Entries apply in the order given, each checked against the records as the
earlier entries left them; kbase never reorders a batch. Every add mints its
node's id, known only from the result, so no entry names an id another entry
of the same batch mints: add those nodes in one batch, then their edges in a
second, from each add's "id" in the result's "entries".

A batch is all or nothing, its derivation included: the strengths, summary
pages and sheets are computed in memory with the records, and every changed
file is written once, after all of it is known. First every entry's shape is
checked — an unknown op, an unknown or missing key, a value's type, a value
outside a closed vocabulary — and every shape error is reported at once. Then
the entries apply in order, and the batch stops at the first refusal, which
names its entry; an entry that would leave the records invalid is refused even
where a later entry would repair it. A refused batch writes nothing.

` + argmark.Flag("dry-run") + ` does everything but write: its result is the real run's, with
"dry-run" true. A rigor or a bearing is a number in [0, 1] or "*pending*". A batch
holds the KB write lock from loading the KB to its result; a writer that cannot
take it within 30 s gets retry, and while a build or a score run holds the
operation lock a batch is refused at once.

` + argmark.Flag("commit") + ` makes git the batch's undo. It is never the default, and every
commit it makes covers the whole of kb-root/. Once the entries have applied in
memory, it commits kb-root/'s uncommitted changes outside .index/, then those
inside .index/, each alone and only where there are any. A pre-flight commit
that fails, for example on a hook, stops the batch with nothing written,
"outcome" failed. Then it writes the batch and commits it, so reverting that
commit undoes exactly the batch and never the author's work; a batch that
writes nothing makes no commit of its own. A batch commit that fails leaves the
batch written, "outcome" done, with "uncommitted" saying why. A committing batch
outside a git worktree is refused (check: worktree; remedy: git init). A batch
without ` + argmark.Flag("commit") + ` has no safety net: only commits of your own undo it.

The result: "outcome", "kb-root", "dry-run"; "entries", one per entry in
order, each with "entry", "operation" and "outcome" — done, or unchanged where
the KB already stood as the entry asks — and its op's own keys, which the op's
help names; "written" and "removed", as paths; "sheet", and "dot" where the
sheets were not drawn; "commits", each with "commit", "label" and "paths",
empty without ` + argmark.Flag("commit") + `; "uncommitted" where the batch commit failed; and
"refusals" where refused. A batch is unchanged where every entry is unchanged
and the derivation wrote nothing, and done otherwise.`

// writeOpMCPNote is a write op's MCP note.
const writeOpMCPNote = `Over MCP this op is an entry of the write tool's batch: "operation" names it,
beside its keys.`

// writeOpCLINote is a write op's CLI note.
const writeOpCLINote = `The batch arrives as YAML (JSON accepted) on stdin, or in the file --values
names: its "entry" key lists this op's entries, which name no "operation".
--dry-run does everything but write. --commit makes git the batch's undo,
committing the whole of kb-root/ before and after the write
(kbase write --help); a batch without it has no safety net.`

// writeOpCommand is one write-API op over the KB found from the working
// directory: a batch of that op alone.
func writeOpCommand(op, short, long string) *cobra.Command {
	return mcp.Notes(valuesCommand(op, short, long), writeOpMCPNote, writeOpCLINote)
}

// valuesCommand is a command reading a write batch for op from --values or
// stdin, taking --dry-run and --commit.
func valuesCommand(op, short, long string) *cobra.Command {
	var valuesFile string
	var dryRun, commit bool
	cmd := &cobra.Command{
		Use:   op,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := invocationDir(cmd)
			if err != nil {
				return err
			}
			code, err := runWriteOp(writeOpOptions{Op: op, WorkDir: wd, ValuesFile: valuesFile, Stdin: cmd.InOrStdin(),
				DryRun: dryRun, Commit: commit, Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(), Logger: processLog.logger})
			if err != nil {
				return err
			}
			if code != 0 {
				return exitCode(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&valuesFile, "values", "", "a file holding the batch (YAML or JSON); stdin when absent")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "do everything but write")
	cmd.Flags().BoolVar(&commit, "commit", false, "commit kb-root/'s prior changes, then the batch, so git can undo it")
	return cmd
}
