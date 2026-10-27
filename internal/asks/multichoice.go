package asks

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"

	"kbase/internal/atomicfile"
	"kbase/internal/kb"
)

// Multichoice asks: one decision per ask, answered by one letter from a closed set
// the build offers. The stage owns every decision; a reader owns only the
// call.

// Kind is which multichoice ask: a paragraph of the node pass, an edge candidate
// of classification, a shortlisted pair of the unmarked-reference asks, or a
// linked pair of the co-linearity cull.
type Kind string

const (
	Paragraph Kind = "paragraph"
	Classify  Kind = "classify"
	Unmarked  Kind = "unmarked"
	Colinear  Kind = "colinear"
)

// AskRecordsDir is where, under a build trail's directory, each stage's
// multichoice-ask group records land.
const AskRecordsDir = "asks"

// What a Caller keeps under a build trail's directory: each call's capture,
// and each answer under the digest of what was asked.
const (
	CapturesDir = "captures"
	AnswersDir  = "answers"
)

// MultichoiceQuestion is one ask as a reader receives it: Group and Item name it,
// State is the composed body, Question the one typed question over the
// offered letters, and Attempt counts the item's asks from 1. A decision
// model has no free-text reply to quote back, so a re-ask is the identical
// question again — the attempt ordinal is what distinguishes it.
type MultichoiceQuestion struct {
	Kind        Kind
	Group, Item string
	State       string
	Question    Question
	Offered     []string
	Attempt     int
}

// CallStats is what a reader measured of its own call.
type CallStats struct {
	APIDuration                              time.Duration
	PromptTokens, OutputTokens, CachedTokens int
	// ThinkingBlocks is how many of the call's attempts streamed reasoning.
	ThinkingBlocks int
}

// Reply is what came back. Confidence is a per-letter distribution, from a
// reader that has one; Stats is nil where the reader measured no call.
type Reply struct {
	Text       string
	Confidence map[string]float64
	Stats      *CallStats
}

// MultichoiceReader puts a question to a model. It returns an IncompleteError
// where the call never completed.
type MultichoiceReader func(ctx context.Context, q MultichoiceQuestion) (Reply, error)

// IncompleteError is a call that never completed: no answer arrived, so no
// re-ask can be spent on one. It is the one thing that stops a stage.
type IncompleteError struct{ Subject, Detail string }

func (e IncompleteError) Error() string { return e.Subject + ": " + e.Detail }

// Decide is the offered letter reply chose, or false where it chose none:
// the argmax over the offered letters where the reply carries a confidence,
// the first offered winning a tie; otherwise, after stripping whitespace,
// exactly one character, and an offered one.
func Decide(reply Reply, offered []string) (string, bool) {
	if reply.Confidence != nil {
		best, found := "", false
		for _, l := range offered {
			if p, ok := reply.Confidence[l]; ok && (!found || p > reply.Confidence[best]) {
				best, found = l, true
			}
		}
		return best, found
	}
	text := kb.Strip(reply.Text)
	for _, l := range offered {
		if text == l && len([]rune(text)) == 1 {
			return l, true
		}
	}
	return "", false
}

// Outcome is how an item's letter was reached; a defaulted item has none.
type Outcome string

const (
	Answered  Outcome = "answered"
	Reasked   Outcome = "re-asked"
	Defaulted Outcome = "defaulted"
)

// MultichoiceItem is one item of a multichoice-ask group: its name, its letters, its
// question, and the composed state. A group's constructor fills these; the
// item's name and question key the answer cache and the records.
type MultichoiceItem struct {
	Item     string
	Offered  []string
	Question Question
	Compose  func() (string, error)
}

// CallRecord is one call: the state past the group's shared prefix, what the
// answer decided, and what it cost. A decision call carries no free-text
// reply — Reply is empty and the confidence map carries the answer — and the
// letter is nil where no offered letter was decided.
type CallRecord struct {
	Tail        string             `yaml:"tail"`
	Reply       string             `yaml:"reply"`
	Confidence  map[string]float64 `yaml:"confidence"`
	Choice      *string            `yaml:"choice"`
	WallSeconds float64            `yaml:"wall-seconds"`
	Stats       *CallStats         `yaml:"stats"`
}

// ItemRecord is one item's outcome and its calls.
type ItemRecord struct {
	Item       string             `yaml:"item"`
	Offered    []string           `yaml:"offered"`
	Choice     *string            `yaml:"choice"`
	Outcome    Outcome            `yaml:"outcome"`
	Confidence map[string]float64 `yaml:"confidence"`
	Calls      []CallRecord       `yaml:"calls"`
}

// GroupRecord is one group's asks: the prefix every prompt in it opens with,
// once, and every item in the order given.
type GroupRecord struct {
	Kind   Kind         `yaml:"kind"`
	Group  string       `yaml:"group"`
	Prefix string       `yaml:"prefix"`
	Items  []ItemRecord `yaml:"items"`
}

// Launch starts one item of an ask batch. It blocks until the item may start
// — where ctx ends first it returns ctx's error and starts nothing — then
// adds the item to wg and runs it on a goroutine of its own, handing it the
// reader its calls go through. The after the item returns, where not nil,
// runs once those calls are done, with the error of writing the files they
// left.
type Launch func(ctx context.Context, wg *sync.WaitGroup, item func(MultichoiceReader) (after func(writeErr error))) error

// PassageLaunch starts the overview's one item, as Launch starts a multichoice
// ask's: it blocks until the item may start — where ctx ends first it returns
// ctx's error and starts nothing — then adds the item to wg and runs it on a
// goroutine of its own, handing it the passage asker its calls go through.
// That asks for the overview passage from excerpts, rejected holding the lines
// a previous reply was refused for and attempt counting the asks from 1. The
// after the item returns, where not nil, runs once its calls are done, with
// the error of writing the files they left.
type PassageLaunch func(ctx context.Context, wg *sync.WaitGroup, item func(passage func(ctx context.Context, excerpts string, rejected []string, attempt int) (string, error)) (after func(writeErr error))) error

type askCall struct {
	state  string
	reply  Reply
	letter *string
	wall   time.Duration
}

func ask(ctx context.Context, reader MultichoiceReader, q MultichoiceQuestion) (askCall, error) {
	started := time.Now()
	reply, err := reader(ctx, q)
	if err != nil {
		return askCall{}, err
	}
	c := askCall{state: q.State, reply: reply, wall: time.Since(started)}
	if l, ok := Decide(reply, q.Offered); ok {
		c.letter = &l
	}
	return c, nil
}

// resolve is an item's calls: the first ask, q, and the one re-ask where it
// carried no offered letter. The re-ask is the identical question — a
// deterministic decision model answers it identically and defaults — the
// attempt ordinal keeping the asks apart in the answer cache.
func resolve(ctx context.Context, reader MultichoiceReader, q MultichoiceQuestion) ([]askCall, error) {
	first, err := ask(ctx, reader, q)
	if err != nil || first.letter != nil {
		return []askCall{first}, err
	}
	q.Attempt = 2
	second, err := ask(ctx, reader, q)
	return []askCall{first, second}, err
}

// sharedPrefix is the longest run of whole lines every prompt opens with.
func sharedPrefix(prompts []string) string {
	if len(prompts) == 0 {
		return ""
	}
	common := prompts[0]
	for _, p := range prompts[1:] {
		n := 0
		for n < len(common) && n < len(p) && common[n] == p[n] {
			n++
		}
		common = common[:n]
	}
	return common[:strings.LastIndex(common, "\n")+1]
}

// Batch is one ask batch: the groups one stage launches, in order and from
// the goroutine that runs the stage, before it waits for them all. Its first
// error by arrival stops it — no further item is launched, and the calls in
// flight see their context end — and is the error Wait reports, never a
// sibling's cancellation behind it.
type Batch struct {
	ctx    context.Context
	stop   context.CancelCauseFunc
	launch Launch
	wg     sync.WaitGroup
	groups []*launchedGroup

	started time.Time
	elapsed time.Duration
}

// launchedGroup is one group of a batch and its items' result cells, each
// written by its own item alone and read once the batch has drained.
type launchedGroup struct {
	kind  Kind
	group string
	items []MultichoiceItem
	calls [][]askCall
}

// NewBatch is an empty ask batch whose items launch would start under ctx.
// Every batch is ended by its Wait.
func NewBatch(ctx context.Context, launch Launch) *Batch {
	bctx, stop := context.WithCancelCause(ctx)
	return &Batch{ctx: bctx, stop: stop, launch: launch, started: time.Now()}
}

// Elapsed is b's time from NewBatch to its Wait's return, zero before Wait
// returns.
func (b *Batch) Elapsed() time.Duration { return b.elapsed }

// AskGroup launches every item of one group into b, in the order given. In a
// group of more than one, the first goes alone, waited for so the server
// prefills and caches the group's shared prefix once, then the rest, which it
// does not wait for; a group of one has no later item to reuse that prefix,
// so it waits for nothing. Each item's state is composed here, before the
// item launches; its re-ask, where it needs one, runs inside the same item.
// Where b has stopped it returns b's error, and the stage launches nothing
// more.
func (b *Batch) AskGroup(kind Kind, group string, items []MultichoiceItem) error {
	g := &launchedGroup{kind: kind, group: group, items: items, calls: make([][]askCall, len(items))}
	b.groups = append(b.groups, g)
	for i, entry := range items {
		state, err := entry.Compose()
		if err != nil {
			b.stop(err)
			return b.err()
		}
		q := entry.Question
		q.Key = entry.Item
		first := MultichoiceQuestion{Kind: kind, Group: group, Item: entry.Item, State: state, Question: q, Offered: entry.Offered, Attempt: 1}
		item := func(reader MultichoiceReader) func(error) {
			calls, err := resolve(b.ctx, reader, first)
			if err != nil {
				b.stop(err)
				return nil
			}
			g.calls[i] = calls
			return b.written
		}
		var alone sync.WaitGroup
		wg := &b.wg
		if i == 0 && len(items) > 1 {
			wg = &alone
		}
		if err := b.launch(b.ctx, wg, item); err != nil {
			b.stop(err)
			return b.err()
		}
		alone.Wait()
		if err := b.err(); err != nil {
			return err
		}
	}
	return nil
}

// written is an item's after: an error writing its files stops the batch.
func (b *Batch) written(err error) {
	if err != nil {
		b.stop(err)
	}
}

// err is b's first error by arrival, nil while it runs.
func (b *Batch) err() error {
	if b.ctx.Err() == nil {
		return nil
	}
	return context.Cause(b.ctx)
}

// Wait waits for every item b launched, then returns the record of each of
// its groups in the order they were launched — or, where b stopped, its first
// error by arrival and no record. It writes no file: the stage writes the
// records through WriteGroupRecords.
func (b *Batch) Wait() ([]GroupRecord, error) {
	b.wg.Wait()
	b.elapsed = time.Since(b.started)
	defer b.stop(nil)
	if err := b.err(); err != nil {
		return nil, err
	}
	records := make([]GroupRecord, len(b.groups))
	for i, g := range b.groups {
		records[i] = g.record()
	}
	return records, nil
}

// record is g's record, its items in the order given.
func (g *launchedGroup) record() GroupRecord {
	rec := GroupRecord{Kind: g.kind, Group: g.group}
	var states []string
	for _, calls := range g.calls {
		for _, c := range calls {
			states = append(states, c.state)
		}
	}
	rec.Prefix = sharedPrefix(states)
	for i, entry := range g.items {
		calls := g.calls[i]
		last := calls[len(calls)-1]
		outcome := Answered
		switch {
		case last.letter == nil:
			outcome = Defaulted
		case len(calls) > 1:
			outcome = Reasked
		}
		item := ItemRecord{Item: entry.Item, Offered: entry.Offered, Choice: last.letter, Outcome: outcome, Confidence: last.reply.Confidence}
		for _, c := range calls {
			item.Calls = append(item.Calls, CallRecord{
				Tail: c.state[len(rec.Prefix):], Reply: "", Confidence: c.reply.Confidence, Choice: c.letter,
				WallSeconds: c.wall.Seconds(), Stats: c.reply.Stats,
			})
		}
		rec.Items = append(rec.Items, item)
	}
	return rec
}

// GroupRecordsPath is the file under dir holding a stage's group records of
// kind. Each stage asks one kind, so the kind names the stage's file.
func GroupRecordsPath(dir string, kind Kind) string {
	return filepath.Join(dir, string(kind)+".yaml")
}

// WriteGroupRecords writes one stage's group records, all of one kind, as one
// file under dir: a YAML stream of one document per group, in the order
// given. It writes nothing where dir is "" or records is empty.
func WriteGroupRecords(dir string, records []GroupRecord) error {
	if dir == "" || len(records) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for _, rec := range records {
		if err := enc.Encode(rec); err != nil {
			return err
		}
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return atomicfile.Write(GroupRecordsPath(dir, records[0].Kind), buf.Bytes(), nil)
}

// AskTotals is what a stage's multichoice asks cost and how they ended. The warm
// figures cover every call but each group's first — the calls a cached prefix
// can serve. Unmeasured counts calls whose reader reported no stats, which
// every sum beside it leaves out.
type AskTotals struct {
	Items, Answered, Reasked, Defaulted   int
	Calls, Unmeasured                     int
	ThinkingBlocks, OutputTokens          int
	WallSeconds, APISeconds               float64
	WarmPromptTokens, WarmCacheReadTokens int
}

// Totals sums records.
func Totals(records []GroupRecord) AskTotals {
	var t AskTotals
	for _, r := range records {
		n := 0
		for _, item := range r.Items {
			t.Items++
			switch item.Outcome {
			case Answered:
				t.Answered++
			case Reasked:
				t.Reasked++
			case Defaulted:
				t.Defaulted++
			}
			for _, c := range item.Calls {
				t.Calls++
				t.WallSeconds += c.WallSeconds
				warm := n > 0
				n++
				if c.Stats == nil {
					t.Unmeasured++
					continue
				}
				t.ThinkingBlocks += c.Stats.ThinkingBlocks
				t.OutputTokens += c.Stats.OutputTokens
				t.APISeconds += c.Stats.APIDuration.Seconds()
				if warm {
					t.WarmPromptTokens += c.Stats.PromptTokens
					t.WarmCacheReadTokens += c.Stats.CachedTokens
				}
			}
		}
	}
	return t
}
