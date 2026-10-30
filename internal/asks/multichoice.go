package asks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
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

// Outcome is how an item's letter was reached; a defaulted item has none,
// and a failed one is an item whose call never completed.
type Outcome string

const (
	Answered  Outcome = "answered"
	Reasked   Outcome = "re-asked"
	Defaulted Outcome = "defaulted"
	Failed    Outcome = "failed"
)

// MultichoiceItem is one item of a multichoice-ask group: its name, its
// letters, its question, and the template inputs its state is composed from.
// The item's name and question key the answer cache and the records.
type MultichoiceItem struct {
	Item     string
	Offered  []string
	Question Question
	Inputs   map[string]string

	template string
	// loadErr is the error of loading the item's question file, deferred to
	// Compose so the ask fails at its call site.
	loadErr error
}

// Compose is the item's state: its kind's template rendered from its inputs.
func (m MultichoiceItem) Compose() (string, error) {
	if m.loadErr != nil {
		return "", m.loadErr
	}
	return render(load, m.template, m.Inputs, nil, nil)
}

// CallRecord is one call: what the answer decided and what it cost. A decision
// call carries no free-text reply — Reply is empty and the confidence map
// carries the answer — and the letter is nil where no offered letter was
// decided.
type CallRecord struct {
	Reply       string             `yaml:"reply"`
	Confidence  map[string]float64 `yaml:"confidence"`
	Choice      *string            `yaml:"choice"`
	WallSeconds float64            `yaml:"wall-seconds"`
	Stats       *CallStats         `yaml:"stats"`
}

// ItemRecord is one item's template inputs past its group's, the AskDigest
// of its state and question, its outcome and its completed calls. A failed
// item has no choice and no confidence.
type ItemRecord struct {
	Item         string             `yaml:"item"`
	Offered      []string           `yaml:"offered"`
	Inputs       map[string]string  `yaml:"inputs"`
	PromptSHA256 string             `yaml:"prompt-sha256"`
	Choice       *string            `yaml:"choice"`
	Outcome      Outcome            `yaml:"outcome"`
	Confidence   map[string]float64 `yaml:"confidence"`
	Calls        []CallRecord       `yaml:"calls"`
}

// GroupRecord is one group's asks: the template inputs every recorded item
// shares, once; and every recorded item in the order given.
type GroupRecord struct {
	Kind   Kind              `yaml:"kind"`
	Group  string            `yaml:"group"`
	Inputs map[string]string `yaml:"inputs"`
	Items  []ItemRecord      `yaml:"items"`
}

// Digest is the length-prefixed SHA-256 of parts, in hex: the answer cache's
// key and a record's prompt-sha256.
func Digest(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%d:%s", len(p), p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// AskDigest is the digest of a multichoice ask's question — its type,
// instructions, true letter and criteria in sorted key order — and its
// state. The question's key is not part of it.
func AskDigest(state string, q Question) string {
	return Digest(append(QuestionParts(q), state)...)
}

// QuestionParts is the question's serialization for digests: its type,
// instructions, true letter and each criterion's key and value in sorted key
// order. The question's key is not part of it.
func QuestionParts(q Question) []string {
	parts := []string{q.Type, q.Instructions, q.TrueLetter}
	for _, k := range slices.Sorted(maps.Keys(q.Criteria)) {
		parts = append(parts, k, q.Criteria[k])
	}
	return parts
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
	// carryOn is whether a call that never completes leaves its item failed
	// and the batch running, rather than stopping it.
	carryOn bool

	started time.Time
	elapsed time.Duration
}

// launchedGroup is one group of a batch and its items' result cells, each
// written by its own item alone and read once the batch has drained.
type launchedGroup struct {
	kind    Kind
	group   string
	items   []MultichoiceItem
	results []itemResult
}

// itemResult is what one launched item left: its composed state, the calls
// that completed, and whether one never did. An item with neither a call nor
// a failure was never asked.
type itemResult struct {
	state  string
	calls  []askCall
	failed bool
}

func (r itemResult) recorded() bool { return r.failed || len(r.calls) > 0 }

// NewBatch is an empty ask batch whose items launch would start under ctx. A
// call that never completes stops it. Every batch is ended by its Wait.
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
	g := &launchedGroup{kind: kind, group: group, items: items, results: make([]itemResult, len(items))}
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
			var incomplete IncompleteError
			switch {
			case errors.As(err, &incomplete):
				// The call that never completed is resolve's last, and left
				// no askCall to record.
				g.results[i] = itemResult{state: state, calls: calls[:len(calls)-1], failed: true}
				if b.carryOn {
					return b.written
				}
				b.stop(err)
				return nil
			case err != nil:
				b.stop(err)
				return nil
			}
			g.results[i] = itemResult{state: state, calls: calls}
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

// Wait waits for every item b launched, then returns the record of each group
// in launch order and, where b stopped, its first error by arrival. A finished
// batch records every group it was given, one launched with no items as a
// record with none. After a stop, a record holds the items that were asked: an
// item whose call never completed is recorded failed; one cancelled by a
// sibling's stop or never launched is absent, as is a group with no asked
// item. It writes no file: the stage writes the records through
// WriteGroupRecords.
func (b *Batch) Wait() ([]GroupRecord, error) {
	b.wg.Wait()
	b.elapsed = time.Since(b.started)
	defer b.stop(nil)
	var records []GroupRecord
	for _, g := range b.groups {
		if rec, ok := g.record(); ok || b.err() == nil {
			records = append(records, rec)
		}
	}
	return records, b.err()
}

// record is g's record, its asked items in the order given; false, with the
// group's kind and name only, where none was asked. The inputs every asked
// item shares go to the group, the rest to each item.
func (g *launchedGroup) record() (GroupRecord, bool) {
	var asked []int
	for i, r := range g.results {
		if r.recorded() {
			asked = append(asked, i)
		}
	}
	if len(asked) == 0 {
		return GroupRecord{Kind: g.kind, Group: g.group}, false
	}
	inputs := make([]map[string]string, len(asked))
	for j, i := range asked {
		inputs[j] = g.items[i].Inputs
	}
	shared, own := splitInputs(inputs)
	rec := GroupRecord{Kind: g.kind, Group: g.group, Inputs: shared}
	for j, i := range asked {
		entry, r := g.items[i], g.results[i]
		item := ItemRecord{Item: entry.Item, Offered: entry.Offered, Inputs: own[j], PromptSHA256: AskDigest(r.state, entry.Question), Outcome: Failed}
		if !r.failed {
			last := r.calls[len(r.calls)-1]
			item.Choice, item.Confidence, item.Outcome = last.letter, last.reply.Confidence, Answered
			switch {
			case last.letter == nil:
				item.Outcome = Defaulted
			case len(r.calls) > 1:
				item.Outcome = Reasked
			}
		}
		for _, c := range r.calls {
			item.Calls = append(item.Calls, CallRecord{
				Reply: "", Confidence: c.reply.Confidence, Choice: c.letter,
				WallSeconds: c.wall.Seconds(), Stats: c.reply.Stats,
			})
		}
		rec.Items = append(rec.Items, item)
	}
	return rec, true
}

// splitInputs is the entries every map of inputs holds with one value, and
// each map's remaining entries, in order.
func splitInputs(inputs []map[string]string) (map[string]string, []map[string]string) {
	shared := map[string]string{}
	for k, v := range inputs[0] {
		if !slices.ContainsFunc(inputs, func(m map[string]string) bool { w, ok := m[k]; return !ok || w != v }) {
			shared[k] = v
		}
	}
	own := make([]map[string]string, len(inputs))
	for j, m := range inputs {
		own[j] = map[string]string{}
		for k, v := range m {
			if _, ok := shared[k]; !ok {
				own[j][k] = v
			}
		}
	}
	return shared, own
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
	Failed                                int
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
			case Failed:
				t.Failed++
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
