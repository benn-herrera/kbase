package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"kbase/internal/model"
	"kbase/internal/model/modeltest"
)

// gateTimeout bounds how long a gated double holds its first call waiting for
// a second: past it, the call goes on, and the test reads what was in flight.
const gateTimeout = 5 * time.Second

// promptAnswering is a scoring provider answering each call from its prompt
// alone, whatever order calls arrive in and however many are in flight: one of
// a priced answer, a fault charge and a reply the parse refuses, chosen by the
// prompt. Its first call is held, counted in flight, until a second call has
// entered or gateTimeout passes; every later call passes straight through.
type promptAnswering struct {
	second chan struct{} // closed as the second call enters

	mu       sync.Mutex
	calls    int
	inFlight int
	peak     int
}

func newPromptAnswering() *promptAnswering {
	return &promptAnswering{second: make(chan struct{})}
}

// Client is the model.Client whose calls w answers.
func (w *promptAnswering) Client() model.Client { return &model.ClientFuncs{Stream: w.ConsultStream} }

func (w *promptAnswering) ConsultStream(ctx context.Context, req model.Request) (model.StreamReader, error) {
	prompt := req.Messages[len(req.Messages)-1].Content
	w.mu.Lock()
	w.calls++
	first := w.calls == 1
	if w.calls == 2 {
		close(w.second)
	}
	w.inFlight++
	w.peak = max(w.peak, w.inFlight)
	w.mu.Unlock()
	if first {
		select {
		case <-w.second:
		case <-time.After(gateTimeout):
		}
	}
	w.mu.Lock()
	w.inFlight--
	w.mu.Unlock()
	reply := []string{"none", "flt-underspecified: 2", "about half"}[len(prompt)%3]
	return modeltest.NewScriptedMock([]model.Response{{Content: reply}}, nil).ConsultStream(ctx, req)
}

// TestAScoreRunHoldsNCallsInFlight: a run at N=2 over more than two items has
// at most two calls in flight, and reaches two.
func TestAScoreRunHoldsNCallsInFlight(t *testing.T) {
	w := newPromptAnswering()
	scoreIn(t, scoreFixture(t), t.TempDir(), scoreOptions{AskConcurrency: new(2)}, func() model.Client { return w.Client() })
	if w.calls <= 2 || w.peak != 2 {
		t.Errorf("%d calls, %d in flight at most; want more than two calls, two at most in flight", w.calls, w.peak)
	}
}
