package model

import "context"

// ClientFuncs is a Client whose methods are its fields: how a test double
// outside this package becomes a Client. It opens no bypass, because a call
// still reaches Stream or Decide only through a Slot that Queue.Go issued.
// Sampling is the endpoint's, a nil field being kbase's default; the double
// is handed it on each Request with the defaults filled in, as a wire client
// is.
type ClientFuncs struct {
	Stream   func(ctx context.Context, req Request) (StreamReader, error)
	Decide   func(ctx context.Context, req DecisionRequest) (DecisionResponse, error)
	List     func(ctx context.Context) ([]ModelInfo, error)
	Sampling Sampling
}

var _ Client = (*ClientFuncs)(nil)

func (f *ClientFuncs) consultStream(ctx context.Context, req Request) (StreamReader, error) {
	return f.Stream(ctx, req)
}

func (f *ClientFuncs) decision(ctx context.Context, req DecisionRequest) (DecisionResponse, error) {
	return f.Decide(ctx, req)
}

func (f *ClientFuncs) sampling() Sampling { return f.Sampling.Resolved() }

// ListModels calls List.
func (f *ClientFuncs) ListModels(ctx context.Context) ([]ModelInfo, error) { return f.List(ctx) }
