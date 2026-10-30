package model_test

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kbase/internal/model"
)

// peak counts what is alive now and the most ever alive at once.
type peak struct{ now, max atomic.Int64 }

func (p *peak) enter() {
	n := p.now.Add(1)
	for {
		m := p.max.Load()
		if n <= m || p.max.CompareAndSwap(m, n) {
			return
		}
	}
}

func (p *peak) exit() { p.now.Add(-1) }

// gateClient answers a decision with the model it was asked, and serves no
// chat stream. Each call counts toward inFlight while it runs, announces
// itself on entered where that is set, and, where release is set, waits for
// a value from it or for its ctx to end.
type gateClient struct {
	entered  chan struct{}
	release  chan struct{}
	calls    atomic.Int64
	inFlight peak
}

func (g *gateClient) wait(ctx context.Context) error {
	g.calls.Add(1)
	g.inFlight.enter()
	defer g.inFlight.exit()
	if g.entered != nil {
		g.entered <- struct{}{}
	}
	if g.release == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.release:
		return nil
	}
}

// client is the model.Client whose calls g answers.
func (g *gateClient) client() model.Client {
	return &model.ClientFuncs{
		Decide: func(ctx context.Context, req model.DecisionRequest) (model.DecisionResponse, error) {
			if err := g.wait(ctx); err != nil {
				return model.DecisionResponse{}, err
			}
			return model.DecisionResponse{Model: req.Model}, nil
		},
		Stream: func(ctx context.Context, _ model.Request) (model.StreamReader, error) {
			if err := g.wait(ctx); err != nil {
				return nil, err
			}
			return nil, errors.New("gateClient serves no stream")
		},
	}
}

// slotCalls are a slot's two calls, each made with a fixed request.
var slotCalls = []struct {
	name string
	call func(context.Context, *model.Slot, model.Client) error
}{{
	name: "chat",
	call: func(ctx context.Context, s *model.Slot, c model.Client) error {
		_, err := s.Chat(ctx, c, model.ChatRequest{Model: "m", Prompt: "p"})
		return err
	},
}, {
	name: "decision",
	call: func(ctx context.Context, s *model.Slot, c model.Client) error {
		_, err := s.Decision(ctx, c, model.DecisionRequest{Model: "m"})
		return err
	},
}}

// TestQueueBoundsItemsAlive: with N tokens and many items, never more than N
// items are alive at once, and N are once every token is taken.
func TestQueueBoundsItemsAlive(t *testing.T) {
	const n, items = 3, 20
	ctx := t.Context()
	q := model.NewQueue(n)
	g := &gateClient{entered: make(chan struct{}), release: make(chan struct{})}
	var alive peak
	var wg sync.WaitGroup
	launched := make(chan error, 1)
	go func() {
		for range items {
			err := q.Go(ctx, &wg, func(s *model.Slot) func() {
				alive.enter()
				defer alive.exit()
				if _, err := s.Decision(ctx, g.client(), model.DecisionRequest{Model: "m"}); err != nil {
					t.Error(err)
				}
				return nil
			})
			if err != nil {
				launched <- err
				return
			}
		}
		launched <- nil
	}()
	// Every call waits for a release; the first is sent once n calls are in,
	// and each later call frees the next.
	for i := range items {
		<-g.entered
		if i >= n-1 {
			g.release <- struct{}{}
		}
	}
	close(g.release)
	if err := <-launched; err != nil {
		t.Fatalf("Go: %v", err)
	}
	wg.Wait()
	if got := alive.max.Load(); got != n {
		t.Errorf("at most %d items were alive at once, want %d", got, n)
	}
}

// TestQueueTokensInLaunchOrder: items launched by one goroutine take their
// tokens in the order Go was called.
func TestQueueTokensInLaunchOrder(t *testing.T) {
	const items = 10
	ctx := t.Context()
	q := model.NewQueue(1)
	var mu sync.Mutex
	var order []int
	var wg sync.WaitGroup
	for i := range items {
		err := q.Go(ctx, &wg, func(*model.Slot) func() {
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
			return nil
		})
		if err != nil {
			t.Fatalf("Go: %v", err)
		}
	}
	wg.Wait()
	want := make([]int, items)
	for i := range want {
		want[i] = i
	}
	if !slices.Equal(order, want) {
		t.Errorf("items took their tokens in order %v, want %v", order, want)
	}
}

// TestQueueGoEndedContext: a ctx already ended, with a token free, returns
// context.Canceled from Go, and the item never starts.
func TestQueueGoEndedContext(t *testing.T) {
	q := model.NewQueue(1)
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var started atomic.Bool
	err := q.Go(ctx, &wg, func(*model.Slot) func() { started.Store(true); return nil })
	wg.Wait()
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Go returned %v, want context.Canceled", err)
	}
	if started.Load() {
		t.Error("the item started on an ended ctx")
	}
}

// TestSlotCallEndsWithContext: a ctx that ends during a call ends the call.
func TestSlotCallEndsWithContext(t *testing.T) {
	for _, sc := range slotCalls {
		t.Run(sc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			q := model.NewQueue(1)
			g := &gateClient{entered: make(chan struct{}), release: make(chan struct{})}
			var callErr error
			var wg sync.WaitGroup
			err := q.Go(ctx, &wg, func(s *model.Slot) func() {
				callErr = sc.call(ctx, s, g.client())
				return nil
			})
			if err != nil {
				t.Fatalf("Go: %v", err)
			}
			<-g.entered
			cancel()
			wg.Wait()
			if !errors.Is(callErr, context.Canceled) {
				t.Errorf("the call returned %v, want context.Canceled", callErr)
			}
		})
	}
}

// TestQueueReleasesTokenBeforeAfter: with one token, the next item starts
// while the previous item's after is still running.
func TestQueueReleasesTokenBeforeAfter(t *testing.T) {
	ctx := t.Context()
	q := model.NewQueue(1)
	nextStarted := make(chan struct{})
	var wg sync.WaitGroup
	err := q.Go(ctx, &wg, func(*model.Slot) func() {
		return func() {
			select {
			case <-nextStarted:
			case <-time.After(5 * time.Second):
				t.Error("the next item did not start while this item's after ran: the token is held across after")
			}
		}
	})
	if err != nil {
		t.Fatalf("Go: %v", err)
	}
	if err := q.Go(ctx, &wg, func(*model.Slot) func() { close(nextStarted); return nil }); err != nil {
		t.Fatalf("Go: %v", err)
	}
	wg.Wait()
}

// TestSlotRefusesOutsideItsItem: a call on a spent slot or on a zero slot
// returns an error, does not panic, and reaches no client.
func TestSlotRefusesOutsideItsItem(t *testing.T) {
	for _, sc := range slotCalls {
		t.Run(sc.name+"/spent", func(t *testing.T) {
			ctx := t.Context()
			q := model.NewQueue(1)
			var spent *model.Slot
			var wg sync.WaitGroup
			if err := q.Go(ctx, &wg, func(s *model.Slot) func() { spent = s; return nil }); err != nil {
				t.Fatalf("Go: %v", err)
			}
			wg.Wait()
			g := &gateClient{}
			if err := sc.call(ctx, spent, g.client()); err == nil {
				t.Error("a spent slot made a call")
			}
			if n := g.calls.Load(); n != 0 {
				t.Errorf("a spent slot reached the client %d times", n)
			}
		})
		t.Run(sc.name+"/zero", func(t *testing.T) {
			g := &gateClient{}
			if err := sc.call(t.Context(), &model.Slot{}, g.client()); err == nil {
				t.Error("a zero slot made a call")
			}
			if n := g.calls.Load(); n != 0 {
				t.Errorf("a zero slot reached the client %d times", n)
			}
		})
	}
}

// TestSlotCarriesOneCallAtATime: a slot handed to several goroutines puts
// one call on the wire at a time.
func TestSlotCarriesOneCallAtATime(t *testing.T) {
	const callers = 4
	ctx := t.Context()
	q := model.NewQueue(1)
	g := &gateClient{entered: make(chan struct{}), release: make(chan struct{})}
	var wg sync.WaitGroup
	err := q.Go(ctx, &wg, func(s *model.Slot) func() {
		var calls sync.WaitGroup
		for range callers {
			calls.Go(func() {
				if _, err := s.Decision(ctx, g.client(), model.DecisionRequest{Model: "m"}); err != nil {
					t.Error(err)
				}
			})
		}
		calls.Wait()
		return nil
	})
	if err != nil {
		t.Fatalf("Go: %v", err)
	}
	for range callers {
		<-g.entered
		g.release <- struct{}{}
	}
	wg.Wait()
	if got := g.inFlight.max.Load(); got != 1 {
		t.Errorf("one slot carried %d calls at once, want 1", got)
	}
}

// TestSlotWaitedReportsTheWait: a slot whose Go waited on a held token
// reports that wait, within the time Go blocked.
func TestSlotWaitedReportsTheWait(t *testing.T) {
	const hold = 50 * time.Millisecond
	ctx := t.Context()
	q := model.NewQueue(1)
	release := make(chan struct{})
	var wg sync.WaitGroup
	if err := q.Go(ctx, &wg, func(*model.Slot) func() { <-release; return nil }); err != nil {
		t.Fatalf("Go: %v", err)
	}
	var releasedAt time.Time
	time.AfterFunc(hold, func() { releasedAt = time.Now(); close(release) })
	var waited time.Duration
	before := time.Now()
	if err := q.Go(ctx, &wg, func(s *model.Slot) func() { waited = s.Waited(); return nil }); err != nil {
		t.Fatalf("Go: %v", err)
	}
	blocked := time.Since(before)
	wg.Wait()
	if floor := releasedAt.Sub(before) - 10*time.Millisecond; waited < floor || waited > blocked {
		t.Errorf("Waited is %v, want at least %v and at most the %v Go blocked", waited, floor, blocked)
	}
}

// TestOnlyASlotReachesTheWire: no exported name in package model reaches a
// client's inference but Slot's Chat and Decision. Client exports only
// ListModels and is sealed by its unexported methods; a type implementing it
// exports only ListModels and Close; and the exported functions that take a
// Client are the Slot's two calls, which reach the wire, and SamplingOf, which
// reads a client's effective sampling and makes no call.
func TestOnlyASlotReachesTheWire(t *testing.T) {
	takesClient := map[string]bool{"Slot.Chat": true, "Slot.Decision": true, "SamplingOf": true}
	clientMethods := map[string]bool{"ListModels": true, "Close": true}
	implementers := map[string]bool{"OpenAIClient": true, "AnthropicClient": true, "ClientFuncs": true}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	sealed := false
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || ts.Name.Name != "Client" {
						continue
					}
					for _, m := range ts.Type.(*ast.InterfaceType).Methods.List {
						for _, n := range m.Names {
							if !n.IsExported() {
								sealed = true
							} else if n.Name != "ListModels" {
								t.Errorf("%s: Client exports %s", fset.Position(n.Pos()), n.Name)
							}
						}
					}
				}
			case *ast.FuncDecl:
				if !d.Name.IsExported() {
					continue
				}
				name := d.Name.Name
				if d.Recv != nil {
					recv := d.Recv.List[0].Type
					if star, ok := recv.(*ast.StarExpr); ok {
						recv = star.X
					}
					if implementers[recv.(*ast.Ident).Name] && !clientMethods[name] {
						t.Errorf("%s: %s exports %s", fset.Position(d.Pos()), recv.(*ast.Ident).Name, name)
					}
					name = recv.(*ast.Ident).Name + "." + name
				}
				if mentionsClient(d.Type.Params) && !takesClient[name] {
					t.Errorf("%s: %s takes a Client outside a Slot", fset.Position(d.Pos()), name)
				}
			}
		}
	}
	if !sealed {
		t.Error("Client has no unexported method, so a type outside model can implement it")
	}
}

// mentionsClient reports whether a parameter list names the Client type.
func mentionsClient(params *ast.FieldList) bool {
	found := false
	ast.Inspect(params, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "Client" {
			found = true
		}
		return !found
	})
	return found
}
