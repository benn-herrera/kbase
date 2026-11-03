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

	"kbase/internal/argmark"
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
	// Run is the id RunID gave; Resume says it names a run that stands, which
	// the invocation continues over the item list it recorded.
	Run    string
	Resume bool
	// Pass is the set the run walks: query.PassRigor, query.PassBearing or
	// query.PassAll — a named node's rigor, a named edge's bearing.
	Pass string
	// Node and Edge name the one item a run walks, dirty or not; neither names
	// a full run over Pass, which walks each item that is dirty.
	Node string
	Edge *query.ScoringEdge
	// Rescore is rescore-manual: the run re-judges manually scored values — in
	// a full run every one in the pass, dirty or not — rather than skipping
	// them.
	Rescore bool
	DryRun  bool
	// BackEnd, Provider and Model are what the run's header records as
	// answering it; Model is what each line records too. A dry run records none.
	BackEnd, Provider, Model string
	Log                      log.Logger
	// Launch is the route the walk's calls take; a dry run does not use it.
	Launch Launch
	// Open opens the KB at a kb-root afresh, as the caller opened Source: a
	// landing reads the KB's text through it, under the write lock, to land
	// only values asked about the text that stands. A dry run does not use it.
	Open func(kbRoot string) (*kb.Source, error)
}

// Report is what an invocation reports of its run.
type Report struct {
	// Outcome is the walk's: result.Done, result.Cancelled or result.Failed.
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
	// Ask and Skip are, on a dry run, each item the run would ask and each dirty
	// manually scored item it would skip, as ItemName spells them, in walk
	// order.
	Ask, Skip []string
	// Failures is, on result.Failed, one item per item of the walk the ledger
	// records failed; Stopped is, on result.Cancelled, the walk's first item with
	// no attempt, empty where every item has one.
	Failures []result.Item
	Stopped  result.Record
	// Unfinished is whether an item failed or was not reached: a run a resume
	// continues. A skipped item is not one, and a dry run continues nothing.
	Unfinished bool
}

// Invoke runs one invocation: the set — the one item it names, else each item
// of its pass that is dirty, or a resumed run's own list — the run's ledger
// created or resumed, retention, the pid and the set beside the ledger, and the
// walk, whose values land when it ends, in one landing through the edit ops
// under the KB write lock, the derivation inside it. An error is the
// invocation's stop; the report then carries Asked where the calls' route was
// reached, and Run and Ledger only where the error is the walk's — one that
// stops the ledger's opening, retention, or the pid or the set beside it leaves
// them empty though the ledger stands. A refusal is a result.Refusal.
func Invoke(ctx context.Context, inv Invocation) (Report, error) {
	src := inv.Source
	ix, err := query.Load(src)
	if err != nil {
		return Report{}, err
	}
	g, err := graphOf(src, ix)
	if err != nil {
		return Report{}, err
	}
	var items []query.Item
	if inv.Resume {
		// A resume continues the scope the run recorded; a run.yaml that does
		// not read is Resume's refusal.
		runDir := filepath.Join(RunsDir(inv.StateDir), inv.Run)
		if h, err := readHeader(runDir); err == nil {
			inv.Pass, inv.Rescore = h.Pass, h.RescoreManual
		}
		items, err = readItems(runDir)
	} else {
		var recorded map[string]kb.JudgedHash
		if recorded, err = recordedHashes(src); err == nil {
			items, err = scopeOf(ix, g, recorded, inv)
		}
	}
	if err != nil && !(inv.Resume && errors.Is(err, os.ErrNotExist)) {
		return Report{}, err
	}
	// The items this invocation skips, read off the KB as the run loaded it: the
	// walk asks none of them, and a resume re-prices none of their values.
	skip := ManualSkip(ix, inv.Rescore)

	header := RunHeader{Run: inv.Run, KBRoot: inv.KBRoot, Pass: inv.Pass, RescoreManual: inv.Rescore, Started: Stamp()}
	if inv.DryRun {
		l, err := Create(inv.StateDir, header)
		if err != nil {
			return Report{}, err
		}
		defer l.Close()
		if err := opened(l, inv, items); err != nil {
			return Report{}, err
		}
		rep := Report{Outcome: result.Done, Run: inv.Run, Ledger: l.Dir(), Scored: []string{}, Ask: []string{}, Skip: []string{}}
		for _, it := range items {
			if skip(it) {
				rep.Skip = append(rep.Skip, ItemName(it))
			} else {
				rep.Ask = append(rep.Ask, ItemName(it))
			}
		}
		rep.Counts = Counts{Total: len(items), Skipped: len(rep.Skip), Remaining: len(rep.Ask)}
		return rep, nil
	}

	rep := Report{Asked: true}
	header.BackEnd, header.Provider, header.Model = inv.BackEnd, inv.Provider, inv.Model
	// One write seam for all three: the values the walk lands, the values a
	// resume folds from a stopped invocation's results and the values it
	// re-prices under a changed table take the same landing, the KB's lock held
	// as every write holds it.
	land := func(values []Landing) []error { return landValues(inv.Open, inv.KBRoot, values, inv.Log) }
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
		Write: land, Skip: skip, Ledger: l, Model: inv.Model, Log: inv.Log, graph: &g}).Run(ctx)
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
// ledger records failed, in the walk's order, each its line's failureItem.
func failures(walked []query.Item, l *Ledger) []result.Item {
	out := []result.Item{}
	for _, it := range walked {
		e, ok := l.Last(it)
		if !ok || e.Status != StatusFailed {
			continue
		}
		out = append(out, failureItem(ItemName(it), e))
	}
	return out
}

// failureItem is the item a failed line reports for the item named name: the
// name and the reason the line recorded in the detail, and the remedy it
// recorded. The reason is the line's reply as a summary — bounded and
// stripped of control characters — where the line itself holds it whole. The
// line's note follows the summary, so the bound never cuts it.
func failureItem(name string, e Entry) result.Item {
	detail := name + ": " + ReplySummary(e.Reply)
	if e.Note != "" {
		detail += "; " + e.Note
	}
	return result.Item{Check: checkError, Remedy: e.Remedy, Detail: detail}
}

// errTextChanged fails a value whose item's text, as the KB holds it at the
// landing, no longer hashes to what its call was composed over: the value
// judged other text, and a resume asks the item again.
var errTextChanged = errors.New("its text was edited after it was asked; the value is discarded and a resume asks it again")

// errItemGone fails a value whose node or edge is no longer in the KB at the
// landing: there is nothing to land it on, and no resume walks it.
var errItemGone = errors.New("the node or edge is no longer in the KB; its value is discarded and a resume does not ask it again")

// discarded is whether err is a landing's refusal of a value the KB changed
// under since it was asked — its text, or its item removed — which rewrites
// that value's line and nothing else.
func discarded(err error) bool { return errors.Is(err, errTextChanged) || errors.Is(err, errItemGone) }

// inKB is whether ix holds it: its node, or its edge.
func inKB(ix *query.Index, it query.Item) bool {
	if it.Edge != nil {
		_, ok := ix.Edge(*it.Edge)
		return ok
	}
	_, ok := ix.Node(it.Node.ID)
	return ok
}

// landValues is the landing: it lands values through the edit ops under one
// hold of the KB's write lock, as one mixed write batch in the order given — a
// node's rigor through its kind's edit op, an edge's bearing through edit-edge
// — the derivation inside the batch, so the derived files are current once it
// lands. It returns one error per value, nil where it landed. The lock is held
// as every writer of the KB holds it, and no operation lock is probed: the run
// holding that lock is the writer. A lock it cannot take fails every value,
// each error the held-lock item. A write batch is all-or-nothing: one the KB
// refuses is issued again without each entry its refusal names, until it lands
// or is empty; one that fails fails every value it carried. A value the KB
// already holds is no failure. The values are the run's own judgement, so they
// are written with the mark false, each recording the judged hash of what it
// was judged against.
//
// Before the batch, under the lock, the KB is opened afresh through open and
// each value's Judged is compared with the judged hash of its item's text as
// the KB now holds it: a value whose item is no longer in the KB fails with
// errItemGone, and
// one whose text changed since its call was composed — a hand edit made while
// no lock kept it out — fails with errTextChanged, so the hash the edit op
// records is always the hash of the text the value judged. A KB that does not
// open or load fails every value.
func landValues(open func(kbRoot string) (*kb.Source, error), root string, values []Landing, lg log.Logger) []error {
	errs := make([]error, len(values))
	if len(values) == 0 {
		return errs
	}
	release, err := write.LockKB(root)
	if err != nil {
		for i := range values {
			if item, held := write.HeldLockItem(err, root, write.RetryRemedy(write.Batch)); held {
				errs[i] = result.Items{item}
			} else {
				errs[i] = fmt.Errorf("the KB's write lock: %w", err)
			}
		}
		return errs
	}
	defer release()
	ix, g, err := graphAt(open, root)
	if err != nil {
		for i := range values {
			errs[i] = err
		}
		return errs
	}
	var batch []int
	for i, v := range values {
		if opOf(v.Item) == "" {
			errs[i] = errors.New("the item names neither a claim, an analysis or an experiment nor an edge")
			continue
		}
		if !inKB(ix, v.Item) {
			errs[i] = errItemGone
			continue
		}
		h, err := judgedHash(g, v.Item)
		if err != nil {
			errs[i] = err
			continue
		}
		if h.String() != v.Judged {
			errs[i] = errTextChanged
			continue
		}
		batch = append(batch, i)
	}
	landBatch(root, values, batch, errs, lg)
	return errs
}

// graphAt is the KB at root, opened afresh through open: its index, and the
// graph a run's passages read.
func graphAt(open func(kbRoot string) (*kb.Source, error), root string) (*query.Index, graph, error) {
	src, err := open(root)
	if err != nil {
		return nil, graph{}, err
	}
	ix, err := query.Load(src)
	if err != nil {
		return nil, graph{}, err
	}
	g, err := graphOf(src, ix)
	return ix, g, err
}

// landBatch issues the values batch names as one mixed write batch, and
// records each value's failure in errs. A refusal naming its entries fails
// those, and the rest are issued again. A refusal naming none cannot say which
// value it refuses, so a batch of more than one is then issued value by value.
func landBatch(root string, values []Landing, batch []int, errs []error, lg log.Logger) {
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
		res := write.Run(write.Batch, write.Options{KBRoot: root, Values: b, Provenance: write.ProvenanceMachine, Logger: lg})
		switch res.Outcome {
		case result.Done, result.Unchanged:
			return
		case result.Refused:
			named := refusedEntries(res.Refusals, len(batch))
			if len(named) == 0 && len(batch) > 1 {
				for _, i := range batch {
					landBatch(root, values, []int{i}, errs, lg)
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
					errs[i] = valueError(details)
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

// opOf is the edit op a value on it lands through: its kind's for a claim, an
// analysis or an experiment, edit-edge for an edge; "" for any other item.
func opOf(it query.Item) string {
	switch {
	case it.Edge != nil:
		return write.EditEdge
	case it.Node == nil:
		return ""
	}
	switch it.Node.Kind {
	case kb.NodeKindClaim:
		return write.EditClaim
	case kb.NodeKindAnalysis:
		return write.EditAnalysis
	case kb.NodeKindExperiment:
		return write.EditExperiment
	}
	return ""
}

// entryOf is v as its batch entry: the edit op, then a node's id and rigor, or
// an edge's source, target and relation, and the bearing.
func entryOf(v Landing) map[string]any {
	op := opOf(v.Item)
	if it := v.Item; it.Edge != nil {
		return map[string]any{"operation": op, "source": it.Edge.Source, "target": it.Edge.Target, "relation": it.Edge.Relation,
			"bearing": opValue(v.Value)}
	}
	return map[string]any{"operation": op, "id": v.Item.Node.ID, "rigor": opValue(v.Value)}
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
	return valueError(parts)
}

// valueError is a value's failure stated by the details naming it, joined, and
// each argument they mark spelled as the command line takes it: the text is
// recorded in a ledger line and logged, and neither carries a marker.
func valueError(details []string) error {
	return errors.New(argmark.Spelling{}.Spell(strings.Join(details, "; ")))
}

// opValue is a bearing as a write op reads it back: the number itself, or the
// pending literal. A number travels as one, never as its spelling.
func opValue(v kb.Bearing) any {
	if v.Pending {
		return kb.PendingLiteral
	}
	return v.Value
}
