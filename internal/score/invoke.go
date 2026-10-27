package score

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	// Launch is the route the walk's calls take; a dry run does not use it.
	Launch Launch
}

// Report is what an invocation reports of its run.
type Report struct {
	// Outcome is the walk's: result.Done, result.Bounded, result.Cancelled or
	// result.Failed.
	Outcome string
	// Asked is whether the invocation reached the route its calls take: it was
	// not a dry run, so Launch was handed to the walk.
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
// walk, whose values land when it ends, in one landing through the write API
// under the KB write lock. An
// error is the invocation's stop; the report then carries Asked where the
// calls' route was reached, and Run and Ledger only where the error is the walk's — one that
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

	rep := Report{Asked: true}
	header := RunHeader{Run: inv.Run, KBRoot: inv.KBRoot, Pass: inv.Pass, BackEnd: inv.BackEnd,
		Provider: inv.Provider, Model: inv.Model, Started: Stamp()}
	// One write seam for all three: the values the walk lands, the values a
	// resume folds from a stopped invocation's results and the values it
	// re-prices under a changed table take the same landing, the KB's lock held
	// as every write holds it.
	land := func(values []Landing) []error { return landValues(inv.KBRoot, values, inv.Log) }
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

	res, err := (&Runner{Source: src, Index: ix, Items: items, Pass: inv.Pass, Launch: inv.Launch,
		Write: land, Skip: skip, Ledger: l, Model: inv.Model, Limit: inv.Limit, Log: inv.Log}).Run(ctx)
	if err != nil {
		return rep, err
	}
	rep.Outcome, rep.Counts = res.Outcome, res.Counts
	// A resume's fold landed what a stopped invocation answered, before the walk
	// landed what this one did.
	landed := slices.Concat(l.folded, res.Written)
	rep.Scored = make([]string, 0, len(landed))
	for _, it := range landed {
		rep.Scored = append(rep.Scored, ItemName(it))
	}
	rep.Unfinished = res.Counts.Failed > 0 || res.Counts.Remaining > 0
	// A resume that re-priced a value wrote as well, whether or not a value
	// landed.
	rep.Wrote = len(landed) > 0 || l.Repriced() > 0
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

// The ops a landing writes through: a node's rigor, and an edge's bearing.
const (
	opSetRigor   = "set-rigor"
	opSetBearing = "set-bearing"
)

// landValues is the landing: it lands values through the write API under one
// hold of the KB's write lock — the nodes' rigor as one set-rigor write batch,
// then the edges' bearings as one set-bearing write batch, each in the order
// given — and lands nothing else: the caller refreshes once the run's writes
// are in. It returns one error per value, nil where it landed. The lock is
// held as every writer of the KB holds it, and no operation lock is probed: the
// run holding that lock is the writer. A lock it cannot take fails every value,
// each error the held-lock item. A write batch is all-or-nothing: one the KB
// refuses is issued again without each entry its refusal names, until it lands
// or is empty; one that fails fails every value it carried. A value the KB
// already holds is no failure. The values are the run's own judgement, so they
// are written not manually scored.
func landValues(root string, values []Landing, lg log.Logger) []error {
	errs := make([]error, len(values))
	if len(values) == 0 {
		return errs
	}
	release, err := write.LockKB(root)
	if err != nil {
		for i, v := range values {
			if item, held := write.HeldLockItem(err, root, write.RetryRemedy(opOf(v.Item))); held {
				errs[i] = result.Items{item}
			} else {
				errs[i] = fmt.Errorf("the KB's write lock: %w", err)
			}
		}
		return errs
	}
	defer release()
	batches := map[string][]int{}
	for i, v := range values {
		op := opOf(v.Item)
		if op == "" {
			errs[i] = errors.New("the item names neither a node nor an edge")
			continue
		}
		batches[op] = append(batches[op], i)
	}
	for _, op := range []string{opSetRigor, opSetBearing} {
		landBatch(root, op, values, batches[op], errs, lg)
	}
	return errs
}

// landBatch issues op over the values batch names, as one write batch, and
// records each value's failure in errs. A refusal naming its entries fails
// those, and the rest are issued again. A refusal naming none cannot say which
// value it refuses, so a batch of more than one is then issued value by value.
func landBatch(root, op string, values []Landing, batch []int, errs []error, lg log.Logger) {
	for len(batch) > 0 {
		entries := make([]map[string]any, len(batch))
		for k, i := range batch {
			entries[k] = entryOf(values[i])
		}
		b, err := json.Marshal(map[string]any{"entry": entries})
		if err != nil {
			for _, i := range batch {
				errs[i] = err
			}
			return
		}
		res := write.Run(op, write.Options{KBRoot: root, Values: b, NoRefresh: true, Provenance: write.ProvenanceMachine, Logger: lg})
		switch res.Outcome {
		case result.Done, result.Unchanged:
			return
		case result.Refused:
			named := refusedEntries(res.Refusals, len(batch))
			if len(named) == 0 && len(batch) > 1 {
				for _, i := range batch {
					landBatch(root, op, values, []int{i}, errs, lg)
				}
				return
			}
			if len(named) == 0 {
				errs[batch[0]] = writeError(res)
				return
			}
			var rest []int
			for k, i := range batch {
				if details, ok := named[k+1]; ok {
					errs[i] = errors.New(strings.Join(details, "; "))
				} else {
					rest = append(rest, i)
				}
			}
			batch = rest
		default:
			err := writeError(res)
			for _, i := range batch {
				errs[i] = err
			}
			return
		}
	}
}

// refusedEntries is each entry of an n-entry batch the refusal names, with the
// details naming it, or nil where any of its items names no entry of the batch.
// It is empty, not nil, where there are no refusals: callers test len.
func refusedEntries(refusals []result.Item, n int) map[int][]string {
	named := map[int][]string{}
	for _, item := range refusals {
		if item.Entry < 1 || item.Entry > n {
			return nil
		}
		named[item.Entry] = append(named[item.Entry], item.Detail)
	}
	return named
}

// opOf is the op a value on it lands through, "" where it names neither a node
// nor an edge.
func opOf(it query.Item) string {
	switch {
	case it.Node != nil:
		return opSetRigor
	case it.Edge != nil:
		return opSetBearing
	}
	return ""
}

// entryOf is v as its op's values entry: a node's id and rigor, or an edge's
// source as id, its target and relation, and the bearing.
func entryOf(v Landing) map[string]any {
	if it := v.Item; it.Edge != nil {
		return map[string]any{"id": it.Edge.Source, "target": it.Edge.Target, "relation": it.Edge.Relation, "bearing": opValue(v.Value)}
	}
	return map[string]any{"id": v.Item.Node.ID, "rigor": opValue(v.Value)}
}

// writeError is what a write that did not land reports: each refusal's and
// failure's detail, or the outcome where it names none.
func writeError(res write.Result) error {
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
