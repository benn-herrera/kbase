package model

import "context"

// ClientFuncs is a Client whose methods are its fields: how a test double
// outside this package becomes a Client. It opens no bypass, because a call
// still reaches Stream or Decide only through a Slot that Queue.Go issued.
type ClientFuncs struct {
	Stream func(ctx context.Context, req Request) (StreamReader, error)
	Decide func(ctx context.Context, req DecisionRequest) (DecisionResponse, error)
	List   func(ctx context.Context) ([]ModelInfo, error)
}

var _ Client = (*ClientFuncs)(nil)

func (f *ClientFuncs) consultStream(ctx context.Context, req Request) (StreamReader, error) {
	return f.Stream(ctx, req)
}

func (f *ClientFuncs) decision(ctx context.Context, req DecisionRequest) (DecisionResponse, error) {
	return f.Decide(ctx, req)
}

// ListModels calls List.
func (f *ClientFuncs) ListModels(ctx context.Context) ([]ModelInfo, error) { return f.List(ctx) }
