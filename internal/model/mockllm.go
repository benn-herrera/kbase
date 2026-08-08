package model

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// MockClient implements Client without crossing the network. It is the test
// substrate for every stage that consults a model: scripted responses,
// injected failures, and a streaming simulation whose chunk boundaries the
// test chooses.
//
// All methods are safe for concurrent use; a fan-out stage under test may
// share one MockClient across worker goroutines.
type MockClient struct {
	mu sync.Mutex

	// Two scripted sub-modes, selected at construction.
	//
	// Single-slot mode (NewScriptedMock): the mock holds one current
	// Response that every Consult re-serves. A caller driving several
	// stages installs each stage's response with SetResponse before
	// driving it. If no response was ever installed, Consult returns
	// ErrMockExhausted.
	//
	// Per-consult queue mode (NewScriptedMockPerConsult): the mock walks
	// `queue` one entry per Consult — queue[0], queue[1], … This is for
	// tests that must observe two *distinct* consults, e.g. a
	// reject-and-retry path where the retry must see a different
	// response. A Consult past the queue end returns ErrMockExhausted.
	scripted   bool     // true once the mock is in scripted mode
	current    Response // single-slot mode: the response every Consult serves
	hasCurrent bool     // single-slot mode: SetResponse (or ctor) installed a response
	queue      []Response
	step       int
	perConsult bool

	// err is the injected failure installed by SetError; when non-nil it
	// pre-empts every mode.
	err error

	// Models is returned verbatim from ListModels. Settable at
	// construction or directly on the returned *MockClient. nil →
	// ListModels returns an empty slice.
	Models []ModelInfo

	calls []MockCall

	// RecordCalls — when true, every Consult call is appended to the
	// internal calls log (inspectable via Calls()). Default false —
	// recording is opt-in to prevent unbounded growth in long runs, where
	// each retained Request holds its full Messages slice and produces
	// O(N²) memory at scale. Tests that inspect Calls() set this true.
	RecordCalls bool

	// chunks is the per-response chunk count for ConsultStream. 0 →
	// DefaultMockChunks. Set via SetMockChunks.
	chunks int
}

// DefaultMockChunks is the per-response chunk count used by ConsultStream
// when SetMockChunks has not been called.
const DefaultMockChunks = 8

// MockCall records one Consult invocation for test assertions.
type MockCall struct {
	Request  Request
	Response Response
	Err      error
	At       time.Time
}

// NewScriptedMock returns a single-slot scripted MockClient. The mock
// holds one current Response that every Consult re-serves; it is changed
// with SetResponse.
//
// `responses` seeds the initial slot: if non-empty, responses[0] is
// installed as the current response, so a single-call test can pass a
// one-element slice and never call SetResponse. Any further elements
// are ignored — use NewScriptedMockPerConsult for a walked queue. A
// nil/empty `responses` leaves the slot empty; the first Consult before
// any SetResponse yields ErrMockExhausted.
//
// models is returned verbatim from ListModels; nil yields an empty list.
func NewScriptedMock(responses []Response, models []ModelInfo) *MockClient {
	m := &MockClient{scripted: true}
	if len(responses) > 0 {
		m.current = responses[0]
		m.hasCurrent = true
	}
	if models != nil {
		m.Models = make([]ModelInfo, len(models))
		copy(m.Models, models)
	}
	return m
}

// NewScriptedMockPerConsult returns a scripted MockClient that advances
// through the queue one entry per Consult: queue[0], queue[1], … This is
// for tests that must observe two distinct consults — a reject-and-retry
// path, where the retry must see a different response than the rejected
// first one. A Consult past the end yields ErrMockExhausted.
func NewScriptedMockPerConsult(responses []Response) *MockClient {
	q := make([]Response, len(responses))
	copy(q, responses)
	return &MockClient{scripted: true, queue: q, perConsult: true}
}

// Consult returns the next response from this mock, or the error installed
// by SetError.
func (m *MockClient) Consult(ctx context.Context, req Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var resp Response
	var err error
	switch {
	case m.err != nil:
		err = m.err
	case m.scripted && m.perConsult:
		if m.step >= 0 && m.step < len(m.queue) {
			resp = m.queue[m.step]
		} else {
			err = ErrMockExhausted
		}
		m.step++
	case m.scripted:
		if m.hasCurrent {
			resp = m.current
		} else {
			err = ErrMockExhausted
		}
	default:
		err = fmt.Errorf("model: MockClient constructed without a mode")
	}

	if m.RecordCalls {
		m.calls = append(m.calls, MockCall{
			Request:  req,
			Response: resp,
			Err:      err,
			At:       time.Now(),
		})
	}
	return resp, err
}

// ConsultStream produces the same Response Consult would have produced,
// then splits it across chunks for incremental delivery. The number of
// chunks is set by SetMockChunks (default DefaultMockChunks). An injected
// error surfaces here too, from the stream-open call rather than from
// Next — the shape a transport failure has.
//
// All non-content fields (FinishReason, Usage, ToolCalls) attach to the
// final chunk so iteration order matches a real provider.
func (m *MockClient) ConsultStream(ctx context.Context, req Request) (StreamReader, error) {
	resp, err := m.Consult(ctx, req)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	chunkCount := m.chunks
	m.mu.Unlock()
	if chunkCount <= 0 {
		chunkCount = DefaultMockChunks
	}
	return newMockStreamReader(ctx, resp, chunkCount), nil
}

// SetMockChunks overrides the per-response chunk count for streamed mock
// responses. Subsequent ConsultStream calls use the new value. n ≤ 0 is
// equivalent to DefaultMockChunks.
func (m *MockClient) SetMockChunks(n int) {
	m.mu.Lock()
	m.chunks = n
	m.mu.Unlock()
}

// SetResponse installs the current response a single-slot scripted mock
// serves; every subsequent Consult returns it. Has no effect on a
// per-consult-queue mock.
func (m *MockClient) SetResponse(resp Response) {
	m.mu.Lock()
	if m.scripted && !m.perConsult {
		m.current = resp
		m.hasCurrent = true
	}
	m.mu.Unlock()
}

// SetError installs a sticky failure: every subsequent Consult — and
// therefore every ConsultStream — returns err instead of a response, until
// cleared with SetError(nil). It is the seam for exercising caller error
// paths (transport failure, provider rejection, retry-then-fall-back) with
// no network in the picture.
func (m *MockClient) SetError(err error) {
	m.mu.Lock()
	m.err = err
	m.mu.Unlock()
}

// ListModels returns a copy of the Models field, or an empty slice if
// none are configured. ctx is honored for cancellation.
func (m *MockClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	if m.Models == nil {
		return []ModelInfo{}, nil
	}
	out := make([]ModelInfo, len(m.Models))
	copy(out, m.Models)
	return out, nil
}

// Calls returns a snapshot of every Consult invocation observed so far.
// Returned slice is a defensive copy; safe to retain across further calls.
func (m *MockClient) Calls() []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]MockCall, len(m.calls))
	copy(out, m.calls)
	return out
}

// mockStreamReader splits a Response.Content into N near-equal chunks and
// hands them out one Next() call at a time. Tool-calls, finish_reason,
// and usage attach to the final chunk so iteration matches a real
// provider's emission order.
type mockStreamReader struct {
	ctx     context.Context
	pieces  []string
	idx     int
	full    Response
	closed  bool
	closeMu sync.Mutex
}

func newMockStreamReader(ctx context.Context, resp Response, chunks int) *mockStreamReader {
	if chunks < 1 {
		chunks = 1
	}
	return &mockStreamReader{
		ctx:    ctx,
		pieces: splitForChunks(resp.Content, chunks),
		full:   resp,
	}
}

func (r *mockStreamReader) Next() (Chunk, error) {
	if r.closed {
		return Chunk{}, io.EOF
	}
	if err := r.ctx.Err(); err != nil {
		return Chunk{}, err
	}
	if r.idx >= len(r.pieces) {
		return Chunk{}, io.EOF
	}
	chunk := Chunk{Content: r.pieces[r.idx]}
	r.idx++
	if r.idx == len(r.pieces) {
		// Final chunk carries the non-content fields.
		chunk.FinishReason = r.full.FinishReason
		chunk.Usage = r.full.Usage
		chunk.ToolCalls = r.full.ToolCalls
	}
	return chunk, nil
}

func (r *mockStreamReader) Final() Response {
	// The mock knows the full response from the start; expose only what
	// has been consumed. (Real providers compute Final() from observed
	// deltas; the mock cheats but the contract is the same.)
	if r.idx == 0 {
		return Response{}
	}
	consumed := strings.Join(r.pieces[:r.idx], "")
	out := Response{Content: consumed}
	if r.idx == len(r.pieces) {
		out.FinishReason = r.full.FinishReason
		out.Usage = r.full.Usage
		out.ToolCalls = r.full.ToolCalls
	}
	return out
}

func (r *mockStreamReader) Close() error {
	r.closeMu.Lock()
	defer r.closeMu.Unlock()
	r.closed = true
	return nil
}

// splitForChunks slices s into n near-equal pieces by rune count. The
// last piece absorbs any remainder so concat(pieces) == s exactly.
// If s is empty, the result is a single empty piece (so Next() still
// emits one chunk carrying the trailing finish_reason/usage).
func splitForChunks(s string, n int) []string {
	if n <= 1 {
		return []string{s}
	}
	if s == "" {
		return []string{""}
	}
	runes := []rune(s)
	total := len(runes)
	if total < n {
		// One rune per piece; trailing pieces empty.
		out := make([]string, 0, n)
		for i := 0; i < n; i++ {
			if i < total {
				out = append(out, string(runes[i:i+1]))
			} else {
				out = append(out, "")
			}
		}
		return out
	}
	per := total / n
	out := make([]string, 0, n)
	cursor := 0
	for i := 0; i < n; i++ {
		end := cursor + per
		if i == n-1 {
			end = total
		}
		out = append(out, string(runes[cursor:end]))
		cursor = end
	}
	return out
}
