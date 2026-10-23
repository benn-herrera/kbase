package score

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/query"
	"kbase/internal/result"
)

// Ask is the runner's only view of a model: the prompt one item's call carries,
// handed over whole, and the reply the call returns. The seam is given the
// rendered prompt rather than the parts of one, so it has no room to change
// what is carried, and the digest the ledger records is of the bytes it was
// handed. An error is the call's failure: the item is recorded failed and the
// run goes on.
type Ask func(ctx context.Context, prompt string) (string, error)

// Write lands one answer on one item: the value replaces the item's own, and
// publishing it is the caller's refresh. An error is the write's refusal —
// nothing landed — and the item is recorded failed, the run going on. A resume
// re-prices through this same seam, so a value an update moved reaches the KB by
// the path a landing takes.
type Write func(it query.Item, value kb.Bearing) error

// Counts is a run's items by the status the ledger last records for each, so a
// resumed run reports the whole run and not one invocation's attempts.
type Counts struct {
	Total  int
	Done   int
	Failed int
	// Remaining is what the ledger records no attempt for: the items a limit
	// left, or the run never reached.
	Remaining int
}

// Result is what a run reports of itself: how it stopped — one of result.Done,
// result.Bounded, result.Cancelled and result.Failed — its items by status, and
// the items whose value landed, in the order it landed.
type Result struct {
	Outcome string
	Counts  Counts
	Written []query.Item
	// Walked is the slice of the set this invocation covered: the items before
	// the walk, cut at the runner's Limit — the same cut the walk made, so the
	// caller's closing record names items from this slice rather than cutting
	// the set a second time. A dry run walks none, and reports none.
	Walked []query.Item
}

// Runner is one score run: the set it walks, its pass, where its calls go,
// where an answer lands and where the attempt is recorded. It does no I/O of
// its own — the ledger writes the run's record, the write seam its values, and
// the caller refreshes what the two changed.
type Runner struct {
	Source *kb.Source
	Index  *query.Index
	Items  []query.Item
	// Pass names the set the run walks: one pass, or query.PassBoth for the
	// node set and then the edge set. It decides what an item's call asks and
	// what its charges are priced under, and a rigor item decides which of that
	// pass's two classes it takes: no item is asked or priced as another class's
	// subject.
	Pass   string
	Ask    Ask
	Write  Write
	Ledger *Ledger
	// Model is what the ledger records of each attempt's call.
	Model string
	// Limit bounds the items walked; 0 walks every one.
	Limit int
	// DryRun asks nothing, lands nothing and records nothing: the run reports
	// the set it would have walked. The caller branches on a dry run before the
	// walk; this is what the runner does when it is told.
	DryRun bool
	Log    log.Logger
}

// Run walks the set once: per item compose the closed text, ask, read the
// charges the reply records against that text, price them, and land the value. A
// reply the parse cannot read, a refused text and a failed call or write are
// recorded and the run goes on. An item the ledger already records as landed is
// not asked again — a fresh run's ledger records nothing, so a fresh run asks
// every item. A cancelled context stops before the next ask and records nothing
// for the item in flight.
func (r *Runner) Run(ctx context.Context) (Result, error) {
	lg := r.Log
	if lg == nil {
		lg = log.Discard()
	}
	if r.DryRun {
		return Result{Outcome: result.Done, Counts: Counts{Total: len(r.Items), Remaining: len(r.Items)}}, nil
	}
	// The pass is read once, before the walk: it is every item of the run's
	// classes, and an item only selects among them.
	facts, err := passFactsOf(r.Pass)
	if err != nil {
		return Result{Outcome: result.Failed}, err
	}
	// The graph is built once, before the walk: every item's passages read the
	// index the runner already holds, and no item reads an index file.
	g := graphOf(r.Index)
	items := r.Items
	bounded := r.Limit > 0 && r.Limit < len(items)
	if bounded {
		items = items[:r.Limit]
	}
	res := Result{Outcome: result.Done, Walked: items}
	for _, it := range items {
		if r.Ledger.Status(it) == StatusDone {
			continue
		}
		if ctx.Err() != nil {
			res.Outcome = result.Cancelled
			break
		}
		e, interrupted := r.attempt(ctx, it, facts, g)
		if interrupted {
			res.Outcome = result.Cancelled
			break
		}
		if err := r.Ledger.Append(it, e); err != nil {
			res.Outcome, res.Counts = result.Failed, r.counts()
			return res, fmt.Errorf("record %s: %w", ItemName(it), err)
		}
		if e.Status != StatusDone {
			lg.Warn("score: item not written", "item", label(it), "status", e.Status, "reason", ReplySummary(e.Reply))
			continue
		}
		res.Written = append(res.Written, it)
		lg.Debug("score: item written", "item", label(it), "value", e.Value)
	}
	res.Counts = r.counts()
	// An interruption outranks a bound and a failure: it says the run stopped
	// short, which is what the caller resumes from.
	if res.Outcome != result.Cancelled {
		switch {
		case bounded:
			res.Outcome = result.Bounded
		case res.Counts.Failed > 0:
			res.Outcome = result.Failed
		}
	}
	return res, nil
}

// attempt makes one item's pass: the item's class — the instruction its one call
// carries and the table its reply is priced under, derived once — the closed text
// the call is sent with, and either the charges the reply records or the reason it
// records none. interrupted reports a context cancelled during the call, which is
// the run's stop and not the item's outcome: the caller records nothing for it.
func (r *Runner) attempt(ctx context.Context, it query.Item, facts passFacts, g graph) (e Entry, interrupted bool) {
	e = Entry{Model: r.Model, Started: stamp()}
	defer func() { e.Ended = stamp() }()
	cl, err := facts.classOf(it)
	if err != nil {
		e.Status, e.Reply = StatusFailed, err.Error()
		return e, false
	}
	prompt, digest, closed, err := r.compose(it, cl, g)
	if err != nil {
		e.Status, e.Reply = StatusFailed, err.Error()
		return e, false
	}
	e.PromptSHA256 = digest
	reply, err := r.Ask(ctx, prompt)
	if err != nil {
		if ctx.Err() != nil {
			return Entry{}, true
		}
		e.Status, e.Reply = StatusFailed, err.Error()
		return e, false
	}
	answer, err := ParseAnswer(reply, closed, cl.table)
	if err != nil {
		// A reply that is no answer fails the attempt: no charges, no judgement,
		// and the record keeps the reason and not the text — a malformed reply is
		// blank, cut short or think-lock, and what matters is that it failed.
		e.Status, e.Reply = StatusFailed, err.Error()
		return e, false
	}
	e.Reply, e.Faults, e.Moot = reply, answer.Faults, answer.Moot
	if err := r.Write(it, answer.Value); err != nil {
		// The write's refusal is what the line records: no value landed, and the
		// charges were a judgement the KB did not take.
		e.Status, e.Reply, e.Faults, e.Moot = StatusFailed, err.Error(), nil, nil
		return e, false
	}
	e.Status, e.Value = StatusDone, kb.BearingText(answer.Value)
	return e, false
}

// compose is the prompt one item's call carries, the digest of exactly it, and
// the closed text the prompt is made of — whose numbered lines are what a charge
// cites. A text the guard refuses is composed of nothing and makes no call.
//
// The item's class has already fixed which pass's subject it is, and the text is
// the shape that class is judged on: a rigor class asks a node's text, a bearing
// class an edge's. The call carries the text numbered, so what it judges and the
// lines a charge names are one rendering.
func (r *Runner) compose(it query.Item, cl class, g graph) (prompt, digest string, closed closedText, err error) {
	if cl.edge {
		row, ok := r.Index.Edge(*it.Edge)
		if !ok {
			return "", "", closedText{}, fmt.Errorf("the index holds no %s edge %s → %s", it.Edge.Relation, it.Edge.Source, it.Edge.Target)
		}
		closed, err = bearingText(r.Source, g, row)
	} else {
		closed, err = rigorText(r.Source, g, it.Node.ID)
	}
	if err != nil {
		return "", "", closedText{}, err
	}
	if prompt, err = cl.ask(closed.numbered()); err != nil {
		return "", "", closedText{}, err
	}
	return prompt, digestOf(prompt), closed, nil
}

// graphOf is the loaded index as the run's passages read it: every node by id,
// and the citations in the order the index holds them.
func graphOf(ix *query.Index) graph {
	nodes := ix.Nodes()
	g := graph{nodes: make(map[string]kb.NodeRow, len(nodes)), cites: ix.Cites()}
	for _, n := range nodes {
		g.nodes[n.ID] = n
	}
	return g
}

// digestOf is the SHA-256 of a prompt as the run's records spell it.
func digestOf(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

// counts is the run's items by the status its ledger last records for each.
func (r *Runner) counts() Counts {
	c := Counts{Total: len(r.Items)}
	for _, it := range r.Items {
		switch r.Ledger.Status(it) {
		case StatusDone:
			c.Done++
		case StatusFailed:
			c.Failed++
		default:
			c.Remaining++
		}
	}
	return c
}

// ItemName is an item as every record names it: a node's id, or an edge's
// relation and its two ends — SPEC §8's form. The result's `scored`, its
// `cancelled` and `failures`, and a run directory's `repriced.yaml` all spell an
// item here, so a line of one joins to a line of another.
func ItemName(it query.Item) string {
	switch {
	case it.Node != nil:
		return it.Node.ID
	case it.Edge != nil:
		return it.Edge.Relation + " " + it.Edge.Source + " → " + it.Edge.Target
	}
	return ""
}

// label is an item as a log line names it: ItemName, with a node's kind before
// the id — what a person reading a line wants and what no record carries.
func label(it query.Item) string {
	if it.Node != nil {
		return it.Node.Kind + " " + it.Node.ID
	}
	return ItemName(it)
}

// replySummaryLimit bounds the reason a document or a log line carries of a
// failed attempt. The ledger's line holds the reply whole; none of that belongs
// in a result's detail or a terminal's log line.
const replySummaryLimit = 512

// ReplySummary is a failed attempt's reply as a document or a log line states
// it: bounded and stripped of control characters, with a line break read as a
// space. A reply is arbitrary bytes the endpoint's answer carried, so nothing it
// wrote reaches a terminal as an instruction. The ledger's line is
// the one place a reply lands whole, and every record and log line that reasons
// a failure reads it through here.
func ReplySummary(reply string) string {
	var b strings.Builder
	for _, r := range reply {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteByte(' ')
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
			// dropped: no terminal control, no bidirectional override
		default:
			b.WriteRune(r)
		}
	}
	s := strings.TrimSpace(b.String())
	if len(s) <= replySummaryLimit {
		return s
	}
	cut := replySummaryLimit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return strings.TrimSpace(s[:cut]) + "…"
}

// stamp is the time a ledger line records, RFC 3339 in UTC.
func stamp() string { return time.Now().UTC().Format(time.RFC3339) }
