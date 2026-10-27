package build

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/buildstore/storetest"
	"kbase/internal/kb"
	"kbase/internal/model"
	"kbase/internal/result"
)

// wireEvent is one decision request reaching the wire, or returning from it,
// named by its state and question key.
type wireEvent struct {
	ask      string
	returned bool
}

// wire is a provider answering each request from its content alone, whatever
// order requests arrive in and however many are in flight: each multichoice ask as
// answering answers it, counting re-asks per ask rather than per key, except
// that every co-linearity ask is answered that the first claim shown adds
// nothing; the overview by script. Each request is held for hold(its content)
// while counted in flight, and each decision request logged as it arrives and
// as it returns. A gated wire also holds its second request, in flight, until
// a later request has returned or gateTimeout passes.
type wire struct {
	hold  func(ask string) time.Duration
	later chan struct{} // gated only: closed as a request after the second returns
	once  sync.Once

	mu       sync.Mutex
	asks     map[string]*answering
	events   []wireEvent
	entered  int
	inFlight int
	peak     int
}

// gateTimeout bounds how long a gated wire holds its second request: past it,
// the request goes on, and the test reads what was in flight.
const gateTimeout = 5 * time.Second

func newWire(hold func(string) time.Duration) *wire {
	return &wire{hold: hold, asks: map[string]*answering{}}
}

// newGatedWire is a wire holding nothing but its second request, which returns
// after one that arrived later has. Not the first: the asked paper's first
// group is its introduction's three paragraphs, whose first ask goes alone, so
// nothing could join it; its second and third go out together.
func newGatedWire() *wire {
	w := newWire(func(string) time.Duration { return 0 })
	w.later = make(chan struct{})
	return w
}

// enter counts a request in flight, logging it where ask is not empty, holds
// it where it is a gated wire's second, and returns the leave that ends it.
func (w *wire) enter(ask string) (leave func()) {
	w.mu.Lock()
	w.entered++
	nth := w.entered
	w.inFlight++
	w.peak = max(w.peak, w.inFlight)
	if ask != "" {
		w.events = append(w.events, wireEvent{ask: ask})
	}
	w.mu.Unlock()
	if nth == 2 && w.later != nil {
		select {
		case <-w.later:
		case <-time.After(gateTimeout):
		}
	}
	return func() {
		w.mu.Lock()
		w.inFlight--
		if ask != "" {
			w.events = append(w.events, wireEvent{ask: ask, returned: true})
		}
		w.mu.Unlock()
		if nth > 2 && w.later != nil {
			w.once.Do(func() { close(w.later) })
		}
	}
}

// Client is the model.Client whose calls w answers.
func (w *wire) Client() model.Client {
	return &model.ClientFuncs{Stream: w.ConsultStream, Decide: w.Decision}
}

func (w *wire) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	prompt := req.Messages[len(req.Messages)-1].Content
	defer w.enter("")()
	time.Sleep(w.hold(prompt))
	return newAnswering(script).reply(ctx, req)
}

func (w *wire) Decision(ctx context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
	q := req.Questions[0]
	ask := askName(req.State, q.Key)
	w.mu.Lock()
	one := w.asks[ask]
	if one == nil {
		one = newAnswering(script)
		w.asks[ask] = one
	}
	w.mu.Unlock()
	defer w.enter(ask)()
	time.Sleep(w.hold(ask))
	resp, err := one.Decision(ctx, req)
	if err != nil || q.Instructions != asks.ColinearAsks([]asks.ColinearItem{{}})[0].Question.Instructions {
		return resp, err
	}
	a := &resp.Answers[0]
	a.Choice = asks.LetterFirstAddsNothing
	for l := range a.Probabilities {
		a.Probabilities[l] = 0.05
	}
	a.Probabilities[a.Choice] = 0.9
	return resp, nil
}

// askName names a multichoice ask by its state and its question's key, which
// together are one item's ask.
func askName(state, key string) string { return key + "\x00" + state }

// askingAt is the fixture's options asking client through a queue of n.
func (f fixture) askingAt(client model.Client, n int) Options {
	opts := f.asking(client)
	provider := opts.Provider
	opts.Provider = func() (Provider, error) {
		p, err := provider()
		p.Queue = model.NewQueue(n)
		return p, err
	}
	return opts
}

// builtAt is a fixture holding the asked paper built through a queue of n
// over w.
func builtAt(t *testing.T, w *wire, n int) fixture {
	t.Helper()
	f := newFixture(t)
	storetest.WriteFile(t, filepath.Join(f.repo, "paper.tex"), askedPaper)
	if outcome, fields := f.build(t, f.askingAt(w.Client(), n)); outcome != result.Done {
		t.Fatalf("the build at N=%d = %s %v", n, outcome, fields)
	}
	return f
}

// returnedOutOfOrder reports whether some ask in events returned while one
// that arrived before it was still in flight.
func returnedOutOfOrder(events []wireEvent) bool {
	var inFlight []string
	for _, e := range events {
		if !e.returned {
			inFlight = append(inFlight, e.ask)
			continue
		}
		i := slices.Index(inFlight, e.ask)
		if i > 0 {
			return true
		}
		inFlight = slices.Delete(inFlight, i, i+1)
	}
	return false
}

// TestABuildAtOneAndFourLandsTheSameKB: the asked paper built at N=1 and at
// N=4 against a provider answering by content — its answers coming back out
// of the order they were asked at N=4 — lands the same tracked records and
// kb-root/, modulo ids, the cull record and its one cull included.
func TestABuildAtOneAndFourLandsTheSameKB(t *testing.T) {
	scrambled := newGatedWire()
	one, four := builtAt(t, newWire(func(string) time.Duration { return 0 }), 1), builtAt(t, scrambled, 4)
	if !returnedOutOfOrder(scrambled.events) {
		t.Fatal("nothing arrived out of order")
	}
	cull, _, err := buildrecords.ReadCull(kb.OnDisk(filepath.Join(one.repo, kb.KBDir)))
	if err != nil || len(cull.Culls) != 1 {
		t.Fatalf("cull record %+v, %v; want the prose claim culled", cull, err)
	}
	sameModuloIDs(t, four.repo, one.repo)
}

// TestABuildHoldsNCallsInFlight: a build at N=2 whose stages ask more than
// two items at once has at most N multichoice asks in flight, and reaches N.
func TestABuildHoldsNCallsInFlight(t *testing.T) {
	w := newGatedWire()
	builtAt(t, w, 2)
	if w.peak != 2 {
		t.Errorf("%d calls in flight at most, want 2", w.peak)
	}
}

// groupRecords is every group record of kind the build in f wrote, in the
// order the stage launched the groups.
func groupRecords(t *testing.T, f fixture, kind asks.Kind) []asks.GroupRecord {
	t.Helper()
	file, err := os.Open(asks.GroupRecordsPath(filepath.Join(f.startTrailDir(t), asks.AskRecordsDir), kind))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var out []asks.GroupRecord
	for dec := yaml.NewDecoder(file); ; {
		var g asks.GroupRecord
		if err := dec.Decode(&g); errors.Is(err, io.EOF) {
			return out
		} else if err != nil {
			t.Fatal(err)
		}
		out = append(out, g)
	}
}

// itemAsk is the ask an item of g made.
func itemAsk(g asks.GroupRecord, item asks.ItemRecord) string {
	return askName(g.Prefix+item.Calls[0].Tail, item.Item)
}

// letterKinds is every multichoice-ask kind the asked paper's build asks.
var letterKinds = []asks.Kind{asks.Paragraph, asks.Unmarked, asks.Colinear, asks.Classify}

// TestABuildAsksInSubmissionOrder: at N=1 each stage's multichoice asks reach the
// wire in the order the stage launched them — its groups in order, each
// group's items in order, an item's re-ask right behind it.
func TestABuildAsksInSubmissionOrder(t *testing.T) {
	w := newWire(func(string) time.Duration { return 0 })
	f := builtAt(t, w, 1)
	for _, kind := range letterKinds {
		var launched []string
		for _, g := range groupRecords(t, f, kind) {
			for _, item := range g.Items {
				for range item.Calls {
					launched = append(launched, itemAsk(g, item))
				}
			}
		}
		var arrived []string
		for _, e := range w.events {
			if !e.returned && slices.Contains(launched, e.ask) {
				arrived = append(arrived, e.ask)
			}
		}
		if len(launched) == 0 || !slices.Equal(arrived, launched) {
			t.Errorf("%s: %d asks arrived in an order other than the %d launched", kind, len(arrived), len(launched))
		}
	}
}

// TestABuildAsksAGroupsFirstAlone: at N=4 each group's first ask returns
// before the group's second arrives.
func TestABuildAsksAGroupsFirstAlone(t *testing.T) {
	w := newWire(func(string) time.Duration { return 10 * time.Millisecond })
	f := builtAt(t, w, 4)
	groups := 0
	for _, kind := range letterKinds {
		for _, g := range groupRecords(t, f, kind) {
			if len(g.Items) < 2 {
				continue
			}
			groups++
			first, second := itemAsk(g, g.Items[0]), itemAsk(g, g.Items[1])
			// The first item's last return: a re-ask returns twice.
			returned := -1
			for i, e := range w.events {
				if e == (wireEvent{ask: first, returned: true}) {
					returned = i
				}
			}
			arrived := slices.Index(w.events, wireEvent{ask: second})
			if returned < 0 || arrived < 0 || returned > arrived {
				t.Errorf("%s group %s: its second ask arrived before its first returned", kind, g.Group)
			}
		}
	}
	if groups == 0 {
		t.Fatal("no group asked two items")
	}
}
