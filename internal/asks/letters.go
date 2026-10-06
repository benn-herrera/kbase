package asks

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"

	"kbase/internal/atomicfile"
	"kbase/internal/kb"
)

// Letter asks: one decision per ask, answered by one letter from a closed set
// the build offers. The stage owns every decision; a reader owns only the
// call.

// Kind is which letter ask: a paragraph of the node pass, an edge candidate
// of classification, or a shortlisted pair of the unmarked-reference asks.
type Kind string

const (
	Paragraph Kind = "paragraph"
	Classify  Kind = "classify"
	Unmarked  Kind = "unmarked"
)

// DefaultReaderConcurrency is how many asks of one group are in flight once
// its first has returned and the server holds the group's shared prefix,
// where the configuration names no other number.
const DefaultReaderConcurrency = 4

// AskRecordsDir is where, under a build's scratch directory, each letter-ask
// group's record lands.
const AskRecordsDir = "asks"

// LetterQuestion is one ask as a reader receives it: Group and Item name it,
// Prompt is all it says, and Attempt counts the item's asks from 1.
type LetterQuestion struct {
	Kind        Kind
	Group, Item string
	Prompt      string
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

// LetterReader puts a question to a model. It returns an IncompleteError
// where the call never completed.
type LetterReader func(ctx context.Context, q LetterQuestion) (Reply, error)

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

// LetterItem is one item of a group: its name, its letters, and its prompt
// given what came back last time — nil on a first ask.
type LetterItem struct {
	Item    string
	Offered []string
	Compose func(returned *string) (string, error)
}

// CallRecord is one call: the prompt past the group's shared prefix, what
// came back, and what it cost.
type CallRecord struct {
	Tail        string             `yaml:"tail"`
	Reply       string             `yaml:"reply"`
	Confidence  map[string]float64 `yaml:"confidence"`
	Letter      *string            `yaml:"letter"`
	WallSeconds float64            `yaml:"wall-seconds"`
	Stats       *CallStats         `yaml:"stats"`
}

// ItemRecord is one item's outcome and its calls.
type ItemRecord struct {
	Item       string             `yaml:"item"`
	Offered    []string           `yaml:"offered"`
	Letter     *string            `yaml:"letter"`
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

type askCall struct {
	prompt string
	reply  Reply
	letter *string
	wall   time.Duration
}

func ask(ctx context.Context, reader LetterReader, q LetterQuestion) (askCall, error) {
	started := time.Now()
	reply, err := reader(ctx, q)
	if err != nil {
		return askCall{}, err
	}
	c := askCall{prompt: q.Prompt, reply: reply, wall: time.Since(started)}
	if l, ok := Decide(reply, q.Offered); ok {
		c.letter = &l
	}
	return c, nil
}

// resolve is an item's calls: the first ask, and the one re-ask where it
// carried no offered letter.
func resolve(ctx context.Context, reader LetterReader, kind Kind, group string, entry LetterItem) ([]askCall, error) {
	question := func(returned *string, attempt int) (LetterQuestion, error) {
		prompt, err := entry.Compose(returned)
		return LetterQuestion{kind, group, entry.Item, prompt, entry.Offered, attempt}, err
	}
	q, err := question(nil, 1)
	if err != nil {
		return nil, err
	}
	first, err := ask(ctx, reader, q)
	if err != nil || first.letter != nil {
		return []askCall{first}, err
	}
	if q, err = question(&first.reply.Text, 2); err != nil {
		return nil, err
	}
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

// AskGroup asks every item of one group and returns its record, items in the
// order given: the first alone, so the server prefills and caches the shared
// prefix once, then the rest with concurrency in flight —
// DefaultReaderConcurrency where it is 0. An error from any call — a call
// that never completed above all — stops the calls not yet made and returns
// once those in flight have, recording nothing. Where recordDir is set the
// record lands there once every item has its outcome.
func AskGroup(ctx context.Context, kind Kind, group string, items []LetterItem, reader LetterReader, concurrency int, recordDir string) (GroupRecord, error) {
	rec := GroupRecord{Kind: kind, Group: group}
	if len(items) == 0 {
		return rec, nil
	}
	if concurrency == 0 {
		concurrency = DefaultReaderConcurrency
	}
	resolved := make([][]askCall, len(items))
	errs := make([]error, len(items))
	resolved[0], errs[0] = resolve(ctx, reader, kind, group, items[0])
	if errs[0] != nil {
		return rec, errs[0]
	}
	rest, stop := context.WithCancel(ctx)
	defer stop()
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i := 1; i < len(items); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if rest.Err() != nil {
				errs[i] = rest.Err()
				return
			}
			if resolved[i], errs[i] = resolve(rest, reader, kind, group, items[i]); errs[i] != nil {
				stop()
			}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return rec, err
	}
	for _, err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			return rec, err
		}
	}

	var prompts []string
	for _, calls := range resolved {
		for _, c := range calls {
			prompts = append(prompts, c.prompt)
		}
	}
	rec.Prefix = sharedPrefix(prompts)
	for i, entry := range items {
		calls := resolved[i]
		last := calls[len(calls)-1]
		outcome := Answered
		switch {
		case last.letter == nil:
			outcome = Defaulted
		case len(calls) > 1:
			outcome = Reasked
		}
		item := ItemRecord{Item: entry.Item, Offered: entry.Offered, Letter: last.letter, Outcome: outcome, Confidence: last.reply.Confidence}
		for _, c := range calls {
			item.Calls = append(item.Calls, CallRecord{
				Tail: c.prompt[len(rec.Prefix):], Reply: c.reply.Text, Confidence: c.reply.Confidence, Letter: c.letter,
				WallSeconds: c.wall.Seconds(), Stats: c.reply.Stats,
			})
		}
		rec.Items = append(rec.Items, item)
	}
	if recordDir != "" {
		if err := writeGroupRecord(recordDir, rec); err != nil {
			return rec, err
		}
	}
	return rec, nil
}

// GroupRecordPath is where a group's record lands under dir.
func GroupRecordPath(dir string, kind Kind, group string) string {
	return filepath.Join(dir, string(kind)+"-"+strings.ReplaceAll(group, "/", "_")+".yaml")
}

func writeGroupRecord(dir string, rec GroupRecord) error {
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(rec); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return atomicfile.Write(GroupRecordPath(dir, rec.Kind, rec.Group), buf.Bytes(), nil)
}

// AskTotals is what a stage's letter asks cost and how they ended. The warm
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
