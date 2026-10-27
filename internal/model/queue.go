package model

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Queue bounds how many items make model calls at once: an item holds one of
// its tokens from the moment Go takes it until the item returns.
type Queue struct {
	tokens chan struct{}
}

// NewQueue is a queue of n tokens. n is user input, checked where it is read.
func NewQueue(n int) *Queue { return &Queue{tokens: make(chan struct{}, n)} }

// Go waits for a free token, then starts one goroutine that runs item with a
// Slot, through which the item makes every call it needs. The token is
// released the moment item returns, a panic included, and the slot is spent
// from then on; after, where item returns one, runs next, on no token, and
// wg.Done follows it. A ctx that ends before a token is free returns its error
// and starts nothing.
//
// One goroutine calling Go for a consumer's items in turn gives them their
// tokens in that order, since each Go holds its token before it returns.
func (q *Queue) Go(ctx context.Context, wg *sync.WaitGroup, item func(*Slot) (after func())) error {
	started := time.Now()
	// select picks at random among ready cases: a ctx already ended must win
	// over a free token.
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.tokens <- struct{}{}:
	}
	s := &Slot{live: true, waited: time.Since(started)}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if after := q.hold(s, item); after != nil {
			after()
		}
	}()
	return nil
}

// hold runs item on s, then spends s and releases its token as item returns.
// Spending waits out a call another goroutine is making through s, so no call
// outlives its token.
func (q *Queue) hold(s *Slot, item func(*Slot) func()) func() {
	defer func() {
		s.spend()
		<-q.tokens
	}()
	return item(s)
}

// Slot is an item's right to make model calls on the token Go took for it.
// It is good only until its item returns. It carries one call at a time: its
// methods hold its mutex across the call, so handing the slot to a second
// goroutine cannot put two calls on one token.
type Slot struct {
	mu     sync.Mutex
	live   bool // Go issued it and its item has not returned
	waited time.Duration
}

var errSlotNotLive = errors.New("model: the slot is spent or was not issued by Queue.Go")

func (s *Slot) spend() {
	s.mu.Lock()
	s.live = false
	s.mu.Unlock()
}

// refusal is why a call on s may not be made, nil where it may. The caller
// holds s.mu.
func (s *Slot) refusal() error {
	if !s.live {
		return errSlotNotLive
	}
	return nil
}

// Waited is how long Go blocked before it took this slot's token.
func (s *Slot) Waited() time.Duration { return s.waited }

// ChatRequest is one chat call: the model, the system prompt, the prompt, and
// the backoff the call pauses by between attempts.
type ChatRequest struct {
	Model   string
	System  string
	Prompt  string
	Backoff []time.Duration
}

// ChatResult is a chat call's attempts in the order they were made, the last
// being the call's outcome. An attempt the call's ctx ended is not among them.
type ChatResult struct {
	Attempts []ChatAttempt
	// Final is the attempt the call ended on. On a call that returned no
	// error it is the last of Attempts; where the context ended the call, it
	// is the interrupted attempt, which Attempts does not hold.
	Final ChatAttempt
}

// Chat makes req's chat call through c, as chat does, on s's token.
func (s *Slot) Chat(ctx context.Context, c Client, req ChatRequest) (ChatResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refusal(); err != nil {
		return ChatResult{}, err
	}
	var res ChatResult
	var err error
	res.Final, err = chat(ctx, c, req.Model, req.System, req.Prompt, req.Backoff, func(a ChatAttempt) {
		res.Attempts = append(res.Attempts, a)
	})
	return res, err
}

// Decision makes req's decision call through c on s's token.
func (s *Slot) Decision(ctx context.Context, c Client, req DecisionRequest) (DecisionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refusal(); err != nil {
		return DecisionResponse{}, err
	}
	return c.decision(ctx, req)
}
