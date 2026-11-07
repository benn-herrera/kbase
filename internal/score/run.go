package score

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"kbase/internal/asks"
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
type Ask func(ctx context.Context, prompt string) (Reply, error)

// Launch starts one item of a wave of calls: it waits until the item may make its
// call, adds it to wg, runs item with the Ask its call goes through, then runs
// the after item returns, where non-nil — the item's per-result writes, made
// once its call is done — and marks it done in wg. A ctx that ends before the
// item starts returns ctx.Err(), and nothing is started. The caller launches
// its items in set order from one goroutine and then waits on wg; each item
// writes only its own result cell.
type Launch func(ctx context.Context, wg *sync.WaitGroup, item func(Ask) (after func())) error

// Reply is what one call returned: the reply's text, and whether the endpoint
// cut it off at the completion cap. What a truncated reply is worth is the
// runner's to decide, not the seam's.
type Reply struct {
	Text      string
	Truncated bool
}

// RemedyError is an Ask's failure that names the command letting the call
// through, which the failed attempt's line records, with the note that goes
// with it where there is one. Its text is the call's error.
type RemedyError struct {
	Err    error
	Remedy string
	Note   string
}

func (e RemedyError) Error() string { return e.Err.Error() }

func (e RemedyError) Unwrap() error { return e.Err }

// errTruncatedReply fails an item whose reply reached the completion cap. The
// lines it did not reach are lost, and where the cut falls between lines the
// rest would parse as a clean under-charge: an incomplete answer is never a
// judgement.
var errTruncatedReply = errors.New("the reply reached the completion cap before it finished, so its charge list is incomplete")

// Landing is one value a landing writes: the value replaces the item's own.
// Judged is the judged hash, as hex, of the item's text as its call was
// composed: the value lands only where the KB's text still hashes to it.
type Landing struct {
	Item   query.Item
	Value  kb.Bearing
	Judged string
}

// Write lands a wave's values in one landing, and returns one error per
// value, in the order given: nil where it landed, and otherwise why it did not,
// that item then recorded failed and the run going on. Publishing what landed
// is the caller's refresh. A resume folds a stopped invocation's results and
// re-prices through this same seam, so every value reaches the KB by the path
// a landing takes.
type Write func(values []Landing) []error

// Skip is whether this invocation skips it: the walk asks it nothing and lands
// nothing on it, and a resume leaves its landed value unrepriced.
type Skip func(it query.Item) bool

// ManualSkip is the items a run over ix skips: each whose record carries the
// manually-scored mark, unless rescore — rescore-manual — says to re-judge
// them.
func ManualSkip(ix *query.Index, rescore bool) Skip {
	return func(it query.Item) bool { return !rescore && ix.ManuallyScored(it) }
}

// Counts is a run's items by the status the ledger last records for each, so a
// resumed run reports the whole run and not one invocation's attempts.
type Counts struct {
	Total   int
	Done    int
	Failed  int
	Skipped int
	// Remaining is what the ledger records no attempt for: the items the run
	// never reached.
	Remaining int
}

// Result is what a run reports of itself: how it stopped — one of result.Done,
// result.Cancelled and result.Failed — its items by status, and the items
// whose value landed, in the order it landed.
type Result struct {
	Outcome string
	Counts  Counts
	Written []query.Item
	// Walked is the set this invocation walked, which the caller's closing
	// record names items from.
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
	// Pass names the set the run walks: one pass, or query.PassAll for the
	// node set and then the edge set. It decides what an item's call asks and
	// what its charges are priced under, and a rigor item decides which of that
	// pass's two classes it takes: no item is asked or priced as another class's
	// subject.
	Pass string
	// Launch is the route each item's call takes.
	Launch Launch
	Write  Write
	// Skip is the items the walk records skipped rather than asks; nil skips
	// none.
	Skip   Skip
	Ledger *Ledger
	// Model is what the ledger records of each attempt's call.
	Model string
	Log   log.Logger
	// graph is the index as the passages read it, where the caller built it
	// already; nil builds it from Source and Index.
	graph *graph
}

// Run walks the set once, as one wave of calls. Its launch loop decides each item's
// status and skip, composes its closed text and prompt, and launches the items
// it asks in set order; each launched item asks, reads the charges the reply
// records against that text, prices them into its own result cell, and writes
// its answer file once its call is done. An answer file that cannot be written
// is the run's failure, and stops the launches and the calls still out. When
// the walk has drained — done, cancelled or failed — the ledger's flush
// lands the cells' values, then appends their captures and their lines, in walk
// order, and removes the answer files; no common file is written before then. A
// reply the parse cannot read is asked once more with a correction, inside the
// item's own launch; a reply still refused, a refused text and a failed call or
// landing are recorded and the run goes on. An item the ledger already records
// as landed is not asked again — a fresh run's ledger records nothing, so a fresh run asks
// every item. An item Skip names is recorded skipped, once: a line already
// recording it so is not appended again. A cancelled context stops the walk and
// records nothing for an item without an answer.
func (r *Runner) Run(ctx context.Context) (Result, error) {
	lg := r.Log
	if lg == nil {
		lg = log.Discard()
	}
	// The pass is read once, before the walk: it is every item of the run's
	// classes, and an item only selects among them.
	facts, err := passFactsOf(r.Pass)
	if err != nil {
		return Result{Outcome: result.Failed}, err
	}
	// The graph is built once, before the walk: every item's passages read the
	// index the runner already holds, and no item reads an index file.
	if r.graph == nil {
		g, err := graphOf(r.Source, r.Index)
		if err != nil {
			return Result{Outcome: result.Failed}, err
		}
		r.graph = &g
	}
	g := *r.graph
	items := r.Items
	res := Result{Outcome: result.Done, Walked: items}
	// Each item's place in the walk is its result cell, so the walk's end
	// records in walk order whatever order the answers came in.
	cells := make([]resultCell, len(items))
	// cut is each launched item whose call the walk's stop interrupted: it
	// records nothing.
	cut := make([]bool, len(items))
	// walk ends with ctx, or at the first answer file an item cannot write,
	// which is then the run's error.
	walk, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg       sync.WaitGroup
		stopOnce sync.Once
		walkErr  error
		stopped  bool
	)
	for i, it := range items {
		status := r.Ledger.Status(it)
		if status == StatusDone {
			continue
		}
		if walk.Err() != nil {
			stopped = true
			break
		}
		// An item a resume walks whose node or edge was removed since the run
		// enumerated it is recorded skipped, once: nothing is asked of it.
		if !inKB(r.Index, it) {
			if status != StatusSkipped {
				rec := r.Ledger.line(it, Entry{Status: StatusSkipped, Reply: errItemGone.Error()})
				cells[i] = resultCell{item: it, line: &rec}
			}
			continue
		}
		if r.Skip != nil && r.Skip(it) {
			if status == StatusSkipped {
				continue
			}
			rec := r.Ledger.line(it, Entry{Status: StatusSkipped, Reply: ReasonManuallyScored})
			cells[i] = resultCell{item: it, line: &rec}
			lg.Debug("score: item skipped", "item", label(it), "reason", ReasonManuallyScored)
			continue
		}
		e, c := r.prepare(it, facts, g)
		if c == nil {
			rec := r.Ledger.line(it, e)
			cells[i] = resultCell{item: it, line: &rec}
			continue
		}
		err := r.Launch(walk, &wg, func(ask Ask) func() {
			calls, interrupted := c.attempt(walk, ask, e)
			if interrupted {
				cut[i] = true
				return nil
			}
			var file *answerFile
			for _, a := range calls {
				file = &answerFile{Line: r.Ledger.line(it, a.entry), Capture: r.Ledger.captureOf(it, a.entry, a.call), Prior: file}
			}
			cells[i] = resultCell{item: it, file: file, line: &file.Line}
			return func() {
				if err := r.Ledger.writeAnswerFile(i, *file); err != nil {
					stopOnce.Do(func() {
						walkErr = fmt.Errorf("write the result of %s: %w", ItemName(it), err)
						cancel()
					})
				}
			}
		})
		if err != nil {
			stopped = true
			break
		}
	}
	wg.Wait()
	if stopped || slices.Contains(cut, true) {
		res.Outcome = result.Cancelled
	}
	written, err := r.Ledger.flush(cells, r.Write)
	res.Written, res.Counts = written, r.counts()
	if err := errors.Join(walkErr, err); err != nil {
		res.Outcome = result.Failed
		return res, err
	}
	for _, c := range cells {
		switch {
		case c.line == nil || c.line.Status == StatusSkipped:
		case c.line.Status == StatusDone:
			lg.Debug("score: item written", "item", label(c.item), "value", c.line.Value)
		default:
			lg.Warn("score: item not written", "item", label(c.item), "status", c.line.Status, "reason", ReplySummary(c.line.Reply))
		}
	}
	// An interruption outranks a failure: it says the run stopped short, which
	// is what the caller resumes from.
	if res.Outcome != result.Cancelled && res.Counts.Failed > 0 {
		res.Outcome = result.Failed
	}
	return res, nil
}

// prepare is what the launch loop composes of one item before it launches it:
// the item's class — the instruction its one call carries and the table its
// reply is priced under, derived once — and the closed text and prompt the call
// is sent with, as the entry its line starts from and the call, the entry
// recording the judged hash of the item's text as g holds it. Where the class
// or the hash cannot be derived or the guard refuses the text, the entry is
// failed with the reason and there is no call.
func (r *Runner) prepare(it query.Item, facts passFacts, g graph) (Entry, *call) {
	e := Entry{Model: r.Model, Before: priorOf(r.Index, g, it)}
	refused := func(err error) (Entry, *call) {
		e.Status, e.Reply = StatusFailed, err.Error()
		e.Started, e.Ended = Stamp(), Stamp()
		return e, nil
	}
	cl, err := facts.classOf(it)
	if err != nil {
		return refused(err)
	}
	judged, err := judgedHash(g, it)
	if err != nil {
		return refused(err)
	}
	e.Judged = judged.String()
	prompt, digest, closed, err := r.compose(it, cl, g)
	if err != nil {
		return refused(err)
	}
	e.PromptSHA256 = digest
	return e, &call{class: cl, closed: closed, prompt: prompt}
}

// attempt makes the call through ask and reads its reply, each entry's bounds
// stamped around its call alone. A reply the parse refuses is asked once more,
// while ctx has not ended, with the correction it earns (correctionOf), up to
// asks.MaxAsks calls in all. It returns every call made, in call order: the
// last carries e with the charges the reply records and the value they price,
// which the walk's end lands, or failed with the reason it records none, and
// any before it is a refused one. interrupted reports ctx ended during a call,
// which is the walk's stop and not the item's outcome: the caller records
// nothing for the item, a refused call before it included.
func (c call) attempt(ctx context.Context, ask Ask, e Entry) (_ []sentCall, interrupted bool) {
	var out []sentCall
	for {
		try := e
		try.Started = Stamp()
		answer, interrupted, err := c.put(ctx, ask, &try)
		try.Ended = Stamp()
		if interrupted {
			return nil, true
		}
		if err == nil {
			try.Status, try.Value = StatusDone, kb.BearingText(answer.Value)
		}
		out = append(out, sentCall{entry: try, call: c})
		var refusal malformedError
		if !errors.As(err, &refusal) || len(out) == asks.MaxAsks || ctx.Err() != nil {
			return out, false
		}
		correction := correctionOf(refusal, c.reply)
		prompt, err := c.class.render(c.closed, &correction)
		if err != nil {
			// The item fails as one whose first ask cannot be composed does
			// (prepare): its reason is the render's error.
			out[len(out)-1].entry.Reply = err.Error()
			return out, false
		}
		c.correction, c.prompt, c.reply = &correction, prompt, ""
		e.PromptSHA256 = digestOf(prompt)
	}
}

// sentCall is one call an attempt made: the entry its line records, and the
// call as it was sent.
type sentCall struct {
	entry Entry
	call  call
}

// correctionOf is the correction a re-ask after refusal carries: the refusal's
// case and the values its fragment names, and each reply line the case names,
// its trailing whitespace trimmed and the rest sent as the reply wrote it. A
// quoted line is model output and not item text, so the input guard's check
// runs over each here, and a line it refuses drops the quotes; nothing is
// stripped or escaped. The values need no guard: each is a digit string or an
// id the parse read as `flt-[a-z0-9-]+`, which holds nothing the guard refuses.
func correctionOf(refusal malformedError, reply string) asks.Correction {
	c := asks.Correction{Case: refusal.refusal, Values: refusal.values}
	lines := strings.Split(reply, "\n")
	quoted := make([]string, 0, len(refusal.lines))
	for _, n := range refusal.lines {
		q := strings.TrimRightFunc(lines[n-1], unicode.IsSpace)
		if _, _, hit := refused(q); hit {
			return c
		}
		quoted = append(quoted, q)
	}
	c.Lines = quoted
	return c
}

// call is what one call carried and returned: the class it asked under, the
// closed text, the correction a re-ask carried (nil on a first ask), the prompt
// rendered from them, and the reply the Ask seam returned, "" where the call
// failed.
type call struct {
	class      class
	closed     closedText
	correction *asks.Correction
	prompt     string
	reply      string
}

// put sends the call's prompt through ask and reads the reply against its closed
// text under its class's table: the answer, e carrying the reply and its charges,
// or err, the attempt's failure, and e failed with its reason — a malformedError
// where the parse refused the reply. interrupted reports ctx cancelled during the
// call, which is the walk's stop: nothing of it is recorded.
func (c *call) put(ctx context.Context, ask Ask, e *Entry) (answer Answer, interrupted bool, _ error) {
	reply, err := ask(ctx, c.prompt)
	// A truncated reply is the call's failure, as an error from the seam is: the
	// capture keeps no text of it.
	if err == nil && reply.Truncated {
		err = errTruncatedReply
	}
	if err != nil {
		if ctx.Err() != nil {
			return Answer{}, true, nil
		}
		e.Status, e.Reply = StatusFailed, err.Error()
		var remedied RemedyError
		if errors.As(err, &remedied) {
			e.Remedy, e.Note = remedied.Remedy, remedied.Note
		}
		return Answer{}, false, err
	}
	c.reply = reply.Text
	answer, err = ParseAnswer(c.reply, c.closed, c.class.table)
	if err != nil {
		// A reply that is no answer fails the attempt: no charges, no judgement,
		// and the line keeps the reason and not the text — a malformed reply is
		// blank, cut short or think-lock, and what matters is that it failed. The
		// capture keeps the text.
		e.Status, e.Reply = StatusFailed, err.Error()
		return Answer{}, false, err
	}
	e.Reply, e.Faults, e.Moot = c.reply, answer.Faults, answer.Moot
	return answer, false, nil
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
	if prompt, err = cl.render(closed, nil); err != nil {
		return "", "", closedText{}, err
	}
	return prompt, digestOf(prompt), closed, nil
}

// render is the prompt cl's call carries over closed: its instruction around the
// text as the call numbers it, closed by the correction c a re-ask carries, nil
// on a first ask.
func (cl class) render(closed closedText, c *asks.Correction) (string, error) {
	return cl.ask(closed.numbered(), c)
}

// graphOf is the loaded index as the run's passages read it: every node by id,
// each hosted node's host, and each hosted node's statement, read through
// kb.Statement from its host leaf in src, each leaf read once, and the line it
// opens on.
func graphOf(src *kb.Source, ix *query.Index) (graph, error) {
	nodes := ix.Nodes()
	g := graph{nodes: make(map[string]kb.NodeRow, len(nodes)), hosts: ix.Hosts(), statements: map[string]string{}, lines: map[string]int{}}
	for _, n := range nodes {
		g.nodes[n.ID] = n
	}
	lifts := map[[2]string]bool{}
	for _, e := range ix.Edges() {
		if e.Relation == kb.RelationLifts {
			lifts[[2]string{e.Source, e.Target}] = true
		}
	}
	lifted := func(analysis, claim string) bool { return lifts[[2]string{analysis, claim}] }
	texts := map[string]string{}
	for id, host := range g.hosts {
		text, read := texts[host]
		if !read {
			var err error
			if text, err = src.ReadText(src.KBPath(host)); err != nil {
				return graph{}, err
			}
			texts[host] = text
		}
		g.statements[id] = kb.Statement(text, id, lifted)
		g.lines[id] = statementLine(text, id, g.statements[id])
	}
	return g, nil
}

// priorOf is the value the item carried in the index this run read, before the
// attempt's write: a node's rigor, or an edge's bearing, as a result spells it —
// a number, `*pending*`, or "" where the record carries none. It is what makes a
// run's line a before/after pair, so a replay's answers can be compared with the
// value the KB actually held rather than only with an earlier attempt's.
func priorOf(ix *query.Index, g graph, it query.Item) string {
	switch {
	case it.Node != nil:
		row, ok := g.nodes[it.Node.ID]
		if !ok || row.Rigor == nil {
			return ""
		}
		return kb.PyFloatRepr(*row.Rigor)
	case it.Edge != nil:
		row, ok := ix.Edge(*it.Edge)
		if !ok {
			return ""
		}
		switch v := row.Bearing.(type) {
		case float64:
			return kb.PyFloatRepr(v)
		case string:
			return v
		}
	}
	return ""
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
		case StatusSkipped:
			c.Skipped++
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

// replySummaryLimit bounds the reason a result or a log line carries of a
// failed attempt. The ledger's line holds the reply whole; none of that belongs
// in a result's detail or a terminal's log line.
const replySummaryLimit = 512

// ReplySummary is a failed attempt's reply as a result or a log line states
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

// Stamp is now as a run's records spell a time — a header's start and a ledger
// line's — RFC 3339 in UTC.
func Stamp() string { return time.Now().UTC().Format(time.RFC3339) }
