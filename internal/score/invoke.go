package score

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/query"
	"kbase/internal/result"
	"kbase/internal/write"
)

// RunID is the run an invocation over the state directory dir runs under:
// resume where it names one, else an id minted past every run that stands —
// RunKindDry's for a dry run, so the runs kept of each kind are counted apart
// and a dry run never removes a real one. The runs directory is created either
// way. It is called before the run's locks are taken, because the holder
// record names the run.
func RunID(dir, resume string, dryRun bool) (string, error) {
	if err := os.MkdirAll(RunsDir(dir), 0o777); err != nil {
		return "", err
	}
	if resume != "" {
		return resume, nil
	}
	kind := RunKindReal
	if dryRun {
		kind = RunKindDry
	}
	return kb.MintID(kind, func(id string) bool {
		_, err := os.Stat(filepath.Join(RunsDir(dir), id))
		return err == nil
	}), nil
}

// Invocation is one invocation of a scoring run, from a KB its caller has
// opened, refreshed and locked.
type Invocation struct {
	// KBRoot is the KB's kb-root/, Source the KB as the caller opened it.
	KBRoot string
	Source *kb.Source
	// StateDir is the state directory the run's records stand under.
	StateDir string
	// Run is the id RunID gave; Resume says it names a run that stands.
	Run    string
	Resume bool
	// Pass is the set walked, Rescore the --rescore-manual pass or RescoreNone.
	Pass    string
	Rescore string
	// Limit bounds the items walked; 0 walks every one.
	Limit  int
	DryRun bool
	// BackEnd, Provider and Model are what the run's header records as
	// answering it; Model is what each line records too. A dry run records none.
	BackEnd, Provider, Model string
	Log                      log.Logger
	// NewAsk opens the Ask the walk calls. It is called once, on a run that is
	// not dry, after the set is read: a KB the load refuses is refused before
	// any endpoint is reached.
	NewAsk func() (Ask, error)
}

// Report is what an invocation reports of its run.
type Report struct {
	// Outcome is the walk's: result.Done, result.Bounded, result.Cancelled or
	// result.Failed.
	Outcome string
	// Asked is whether the invocation opened its Ask: NewAsk was called and
	// returned no error.
	Asked bool
	// Run and Ledger are the run's id and directory, set once its ledger stands
	// with its pid and its set beside it.
	Run, Ledger string
	Counts      Counts
	// Scored is each item whose value landed, as ItemName spells it, in the order
	// it landed.
	Scored []string
	// Failures is, on result.Failed, one item per item of the walk the ledger
	// records failed; Stopped is, on result.Cancelled, the walk's first item with
	// no attempt, empty where every item has one.
	Failures []result.Item
	Stopped  result.Record
	// Unfinished is whether an item failed or was not reached: a run a re-issue
	// continues. A skipped item is not one, and a dry run continues nothing.
	Unfinished bool
	// Wrote is whether the invocation landed or re-priced a value, which the
	// caller publishes with a refresh.
	Wrote bool
}

// Invoke runs one invocation: the set read from inv.Source, the run's ledger
// created or resumed, retention, the pid and the set beside the ledger, and the
// walk, each value landed through the write API under the KB write lock. An
// error is the invocation's stop; the report then carries Asked where the Ask
// was opened, and Run and Ledger only where the error is the walk's — one that
// stops the ledger's opening, retention, or the pid or the set beside it leaves
// them empty though the ledger stands. A refusal is a result.Refusal.
func Invoke(ctx context.Context, inv Invocation) (Report, error) {
	src := inv.Source
	// A value lands through a write that skips its refresh, which is refused
	// on a KB in an older format: a run walks only a KB its start refresh saved.
	if src.Migrated() {
		return Report{}, result.Refusal{{Check: index.CheckIndex, Path: src.KBPath(kb.EntryPointFile), Remedy: index.RefreshRemedy,
			Detail: fmt.Sprintf("the KB's metadata format is %s and a dry run saves no conversion; score walks only a KB saved in %s",
				src.Version(), kb.FormatVersion)}}
	}
	ix, err := query.Load(src)
	if err != nil {
		return Report{}, err
	}
	items, err := query.ScoringSet(ix, inv.Pass)
	if err != nil {
		return Report{}, err
	}

	if inv.DryRun {
		l, err := Create(inv.StateDir, RunHeader{Run: inv.Run, KBRoot: inv.KBRoot, Pass: inv.Pass, Started: Stamp()})
		if err != nil {
			return Report{}, err
		}
		defer l.Close()
		if err := opened(l, inv, items); err != nil {
			return Report{}, err
		}
		res, err := (&Runner{DryRun: true, Items: items}).Run(ctx)
		if err != nil {
			return Report{Run: inv.Run, Ledger: l.Dir()}, err
		}
		return Report{Outcome: res.Outcome, Run: inv.Run, Ledger: l.Dir(), Counts: res.Counts, Scored: []string{}}, nil
	}

	ask, err := inv.NewAsk()
	if err != nil {
		return Report{}, err
	}
	rep := Report{Asked: true}
	header := RunHeader{Run: inv.Run, KBRoot: inv.KBRoot, Pass: inv.Pass, BackEnd: inv.BackEnd,
		Provider: inv.Provider, Model: inv.Model, Started: Stamp()}
	// One write seam for both: the values the walk lands and the values a resume
	// re-prices under a changed table take the same path, the KB's lock held as
	// every write holds it.
	land := func(it query.Item, value kb.Bearing) error { return landValue(inv.KBRoot, it, value, inv.Log) }
	// The items this invocation skips, read off the KB as the run loaded it: the
	// walk asks none of them, and a resume re-prices none of their values.
	skip := ManualSkip(ix, inv.Rescore)
	var l *Ledger
	if inv.Resume {
		l, err = Resume(inv.StateDir, header, land, skip)
	} else {
		l, err = Create(inv.StateDir, header)
	}
	if err != nil {
		return rep, err
	}
	defer l.Close()
	if err := opened(l, inv, items); err != nil {
		return rep, err
	}
	rep.Run, rep.Ledger = inv.Run, l.Dir()

	res, err := (&Runner{Source: src, Index: ix, Items: items, Pass: inv.Pass, Ask: ask,
		Write: land, Skip: skip, Ledger: l, Model: inv.Model, Limit: inv.Limit, Log: inv.Log}).Run(ctx)
	if err != nil {
		return rep, err
	}
	rep.Outcome, rep.Counts = res.Outcome, res.Counts
	rep.Scored = make([]string, 0, len(res.Written))
	for _, it := range res.Written {
		rep.Scored = append(rep.Scored, ItemName(it))
	}
	rep.Unfinished = res.Counts.Failed > 0 || res.Counts.Remaining > 0
	// A resume that re-priced a value wrote as well, whether or not its walk
	// landed one.
	rep.Wrote = len(res.Written) > 0 || l.Repriced() > 0
	switch res.Outcome {
	case result.Failed:
		rep.Failures = failures(res.Walked, l)
	case result.Cancelled:
		rep.Stopped = stopped(res.Walked, l)
	}
	return rep, nil
}

// opened is what follows a run's ledger opening, before its walk: retention,
// under the store's holder lock, so no other run is writing a directory it
// could remove, and only once the ledger stands, so an invocation refused
// before it opens prunes nothing; then the result a run before this invocation
// left cleared, the pid, and the set the run enumerates beside the ledger, so
// status and a waiter count the whole run rather than the attempts made so far.
func opened(l *Ledger, inv Invocation, items []query.Item) error {
	if err := PruneRuns(inv.StateDir, inv.Run); err != nil {
		return fmt.Errorf("prune the score runs: %w", err)
	}
	if err := l.ClearResult(); err != nil {
		return err
	}
	if err := l.WritePID(os.Getpid()); err != nil {
		return err
	}
	return l.WriteItems(items)
}

// stopped is the record a cancelled run closes on: the walk's first item that
// carries no attempt — the one the run stopped before — named as the document
// names items, and an empty record where every item of the walk carries one.
func stopped(walked []query.Item, l *Ledger) result.Record {
	for _, it := range walked {
		if l.Status(it) == "" {
			return result.Record{{Key: "item", Value: ItemName(it)}}
		}
	}
	return result.Record{}
}

// failures is what a failed run closes on: one item per item of the walk the
// ledger records failed, its name and the reason its line recorded in the
// detail, in the walk's order. The reason is the line's reply as a summary —
// bounded and stripped of control characters — where the line itself holds it
// whole.
func failures(walked []query.Item, l *Ledger) []result.Item {
	out := []result.Item{}
	for _, it := range walked {
		e, ok := l.Last(it)
		if !ok || e.Status != StatusFailed {
			continue
		}
		out = append(out, result.Item{Check: checkError, Detail: ItemName(it) + ": " + ReplySummary(e.Reply)})
	}
	return out
}

// landValue lands one answer on one item through the write API — a node's rigor
// through set-rigor, an edge's bearing through set-bearing — and lands nothing
// else: the caller refreshes once the run's writes are in. The write holds the
// KB's write lock, as every writer of the KB does, and probes for no operation
// lock: the run holding that lock is the writer. A lock the write cannot take
// is the item's failure, its error the held-lock item. A value the KB already
// holds is no failure; a refusal or a failure is the item's, and the run goes
// on. The value is the run's own judgement, so it is written not manually
// scored.
func landValue(root string, it query.Item, value kb.Bearing, lg log.Logger) error {
	op, entry := "set-rigor", map[string]any{}
	switch {
	case it.Node != nil:
		entry["id"], entry["rigor"] = it.Node.ID, opValue(value)
	case it.Edge != nil:
		op = "set-bearing"
		entry["id"], entry["target"], entry["relation"] = it.Edge.Source, it.Edge.Target, it.Edge.Relation
		entry["bearing"] = opValue(value)
	default:
		return errors.New("the item names neither a node nor an edge")
	}
	values, err := json.Marshal(map[string]any{"entry": []map[string]any{entry}})
	if err != nil {
		return err
	}
	release, err := write.LockKB(root)
	if item, held := write.HeldLockItem(err, root, write.RetryRemedy(op)); held {
		return result.Items{item}
	}
	if err != nil {
		return fmt.Errorf("the KB's write lock: %w", err)
	}
	defer release()
	res := write.Run(op, write.Options{KBRoot: root, Values: values, NoRefresh: true, Provenance: write.ProvenanceMachine, Logger: lg})
	switch res.Outcome {
	case result.Done, result.Unchanged:
		return nil
	}
	parts := make([]string, 0, len(res.Refusals)+len(res.Failures))
	for _, item := range res.Refusals {
		parts = append(parts, item.Detail)
	}
	for _, item := range res.Failures {
		parts = append(parts, item.Detail)
	}
	if len(parts) == 0 {
		parts = append(parts, "the write ended "+res.Outcome)
	}
	return errors.New(strings.Join(parts, "; "))
}

// opValue is a bearing as a write op reads it back: the number itself, or the
// pending literal. A number travels as one, never as its spelling.
func opValue(v kb.Bearing) any {
	if v.Pending {
		return kb.PendingLiteral
	}
	return v.Value
}
