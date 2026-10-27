package claimgraph

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
)

// readerLaunch is a Launch over read, which leaves no file to write, holding
// at most four items in flight.
func readerLaunch(read asks.MultichoiceReader) asks.Launch { return boundedLaunch(read, 4) }

// boundedLaunch is a Launch over read, which leaves no file to write, holding
// at most n items in flight.
func boundedLaunch(read asks.MultichoiceReader, n int) asks.Launch {
	tokens := make(chan struct{}, n)
	return func(ctx context.Context, wg *sync.WaitGroup, item func(asks.MultichoiceReader) func(error)) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		select {
		case tokens <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			after := func() func(error) {
				defer func() { <-tokens }()
				return item(read)
			}()
			if after != nil {
				after(nil)
			}
		}()
		return nil
	}
}

// launchEvent is one launch of a recordingLaunch, or one item's calls
// returning, by launch number.
type launchEvent struct {
	n        int
	returned bool
}

// recordingLaunch is a Launch that starts every item at once, with no bound,
// its calls answered by read after delay(n) for the n-th launch. It logs, in
// one order, each launch and each item's calls returning, and names each
// launch by the first question its item puts.
type recordingLaunch struct {
	read  asks.MultichoiceReader
	delay func(n int) time.Duration

	mu     sync.Mutex
	names  []string
	events []launchEvent
}

func (l *recordingLaunch) launch(_ context.Context, wg *sync.WaitGroup, item func(asks.MultichoiceReader) func(error)) error {
	l.mu.Lock()
	n := len(l.names)
	l.names = append(l.names, "")
	l.events = append(l.events, launchEvent{n: n})
	l.mu.Unlock()
	read := func(ctx context.Context, q asks.MultichoiceQuestion) (asks.Reply, error) {
		l.mu.Lock()
		if l.names[n] == "" {
			l.names[n] = q.Group + "->" + q.Item
		}
		l.mu.Unlock()
		time.Sleep(l.delay(n))
		return l.read(ctx, q)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		after := item(read)
		l.mu.Lock()
		l.events = append(l.events, launchEvent{n: n, returned: true})
		l.mu.Unlock()
		if after != nil {
			after(nil)
		}
	}()
	return nil
}

// at is where in the log event stands.
func (l *recordingLaunch) at(event launchEvent) int {
	return slices.Index(l.events, event)
}

// TestAStageLaunchesInSubmissionOrder: a stage launches its items in the
// order it submits them — groups by source in ascending id, each group's
// items in the order given — and each group's first item returns before the
// group's second is launched. A stage given only a Launch asks through it.
func TestAStageLaunchesInSubmissionOrder(t *testing.T) {
	node := func(id string) ClaimNode { return ClaimNode{ID: id, Document: "v/a.md", Title: "Claim " + id} }
	all := []Relation{SupportedBy, InSupportOf, MentionedBy}
	var candidates []candidate
	for _, p := range []pair{{"clm-c", "clm-d"}, {"clm-a", "clm-b"}, {"clm-a", "clm-c"}, {"clm-b", "clm-a"}, {"clm-a", "clm-d"}, {"clm-b", "clm-c"}} {
		candidates = append(candidates, candidate{source: node(p.source), target: node(p.target), offered: all, draft: MentionedBy, passages: []string{p.source + " names " + p.target}})
	}
	repo := t.TempDir()
	if err := WriteClassification(repo, buildrecords.ClassificationRecord{}); err != nil {
		t.Fatal(err)
	}
	l := &recordingLaunch{
		read: func(context.Context, asks.MultichoiceQuestion) (asks.Reply, error) {
			return asks.Reply{Text: asks.LetterMention}, nil
		},
		delay: func(int) time.Duration { return 10 * time.Millisecond },
	}
	if _, err := classify(context.Background(), repo, candidates, func(n ClaimNode) string { return n.Title }, Options{Launch: l.launch}); err != nil {
		t.Fatal(err)
	}
	want := []string{"clm-a->clm-b", "clm-a->clm-c", "clm-a->clm-d", "clm-b->clm-a", "clm-b->clm-c", "clm-c->clm-d"}
	if !slices.Equal(l.names, want) {
		t.Fatalf("launched %q, want %q", l.names, want)
	}
	source := func(n int) string { return strings.Split(want[n], "->")[0] }
	for n := 1; n < len(want); n++ {
		second := source(n) == source(n-1) && (n == 1 || source(n-2) != source(n-1))
		if second && l.at(launchEvent{n: n - 1, returned: true}) > l.at(launchEvent{n: n}) {
			t.Errorf("%s was launched before %s, its group's first, returned: %+v", want[n], want[n-1], l.events)
		}
	}
}

// TestAStageTakesItsAnswersByPositionNotArrival: a stage whose items return
// out of the order they were launched leaves the build records and kb-root/
// a sequential run of it leaves, modulo ids.
func TestAStageTakesItsAnswersByPositionNotArrival(t *testing.T) {
	body := "We show that every widget is a gadget for all inputs.\n\n" +
		"We prove that every gizmo is a doohickey for all inputs.\n\n" +
		"We show that every sprocket is a cog for all inputs.\n"
	leaves := []string{"a", "b", "c"}
	claimButS2 := func(_ context.Context, q asks.MultichoiceQuestion) (asks.Reply, error) {
		if q.Item == "S2" {
			return asks.Reply{Text: asks.LetterNotClaim}, nil
		}
		return asks.Reply{Text: asks.LetterClaim}, nil
	}

	_, sequential, opts := seedLeavesOf(t, body, leaves, leaves...)
	opts.Launch = boundedLaunch(claimButS2, 1)
	report, err := Discovered(context.Background(), opts)
	requireStagePassed(t, report, err)
	rec, err := requireNodePass(sequential)
	if err != nil {
		t.Fatal(err)
	}
	minted := 0
	for _, leaf := range rec.Leaves {
		minted += len(leaf.Claims)
	}
	if minted != 2*len(leaves) {
		t.Fatalf("the sequential run minted %d claims, want two of each leaf's three paragraphs", minted)
	}

	_, reversed, opts := seedLeavesOf(t, body, leaves, leaves...)
	l := &recordingLaunch{read: claimButS2, delay: func(n int) time.Duration { return time.Duration(max(0, 16-n)) * 10 * time.Millisecond }}
	opts.Launch = l.launch
	report, err = Discovered(context.Background(), opts)
	requireStagePassed(t, report, err)
	var returned []int
	for _, e := range l.events {
		if e.returned {
			returned = append(returned, e.n)
		}
	}
	if slices.IsSorted(returned) {
		t.Fatalf("the items returned in launch order %v, so nothing arrived out of order", returned)
	}

	checks, err := CompareModuloIDs(BuiltBy("sequential", sequential), BuiltBy("out of order", reversed))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range checks {
		for _, d := range c.Differences {
			t.Errorf("%s: %s", c.Name, d)
		}
	}
}

// TestAStageReportsItsBatchWallTimeNotItsCallsSum: a stage's ask totals carry
// stage-wall-seconds, the batch's elapsed time, beside wall-seconds, the sum
// of its calls' — so two calls held in flight together report a stage time
// near one call's and a sum near two.
func TestAStageReportsItsBatchWallTimeNotItsCallsSum(t *testing.T) {
	const hold = 300 * time.Millisecond
	node := func(id string) ClaimNode { return ClaimNode{ID: id, Document: "v/a.md", Title: "Claim " + id} }
	var candidates []candidate
	for _, p := range []pair{{"clm-a", "clm-b"}, {"clm-c", "clm-d"}} {
		candidates = append(candidates, candidate{source: node(p.source), target: node(p.target), offered: []Relation{SupportedBy, InSupportOf, MentionedBy}, draft: MentionedBy, passages: []string{p.source + " names " + p.target}})
	}
	repo := t.TempDir()
	if err := WriteClassification(repo, buildrecords.ClassificationRecord{}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	arrived, both := 0, make(chan struct{})
	read := func(context.Context, asks.MultichoiceQuestion) (asks.Reply, error) {
		mu.Lock()
		if arrived++; arrived == 2 {
			close(both)
		}
		mu.Unlock()
		select {
		case <-both:
		case <-time.After(2 * time.Second):
			return asks.Reply{}, asks.IncompleteError{Subject: "the double", Detail: "the two calls were never in flight together"}
		}
		time.Sleep(hold)
		return asks.Reply{Text: asks.LetterMention}, nil
	}
	c, err := classify(context.Background(), repo, candidates, func(n ClaimNode) string { return n.Title }, Options{Launch: readerLaunch(read)})
	if err != nil {
		t.Fatal(err)
	}
	seconds := map[string]float64{}
	for _, f := range classificationFindings(c, true) {
		if f.Check != "stage-D-asks" {
			continue
		}
		for _, fl := range f.Fields {
			if fl.Key == "wall-seconds" || fl.Key == "stage-wall-seconds" {
				v, err := strconv.ParseFloat(fl.Value.(string), 64)
				if err != nil {
					t.Fatal(err)
				}
				seconds[fl.Key] = v
			}
		}
	}
	stage, sum := seconds["stage-wall-seconds"], seconds["wall-seconds"]
	if len(seconds) != 2 || stage < hold.Seconds() || stage >= sum {
		t.Errorf("stage-D-asks reports %v; want stage-wall-seconds at least %.1f and below wall-seconds", seconds, hold.Seconds())
	}
}
