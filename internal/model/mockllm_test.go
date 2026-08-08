package model

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// TestMockScriptedServesCurrentResponse: a single-slot scripted mock
// serves whatever response SetResponse last installed.
func TestMockScriptedServesCurrentResponse(t *testing.T) {
	scripted := []Response{
		{Content: "first", FinishReason: "stop"},
		{Content: "second", FinishReason: "stop"},
		{Content: "third", FinishReason: "stop"},
	}
	m := NewScriptedMock(nil, nil)

	for i, want := range scripted {
		m.SetResponse(want)
		got, err := m.Consult(context.Background(), Request{})
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if got.Content != want.Content {
			t.Errorf("step %d: got %q, want %q", i, got.Content, want.Content)
		}
	}
}

// TestMockScriptedReServesWithinStep: every consult after one SetResponse
// serves that one installed response — the retry case.
func TestMockScriptedReServesWithinStep(t *testing.T) {
	m := NewScriptedMock(nil, nil)
	m.SetResponse(Response{Content: "step1"})
	for c := 0; c < 3; c++ {
		got, err := m.Consult(context.Background(), Request{})
		if err != nil {
			t.Fatalf("consult %d: %v", c, err)
		}
		if got.Content != "step1" {
			t.Errorf("consult %d: got %q, want step1", c, got.Content)
		}
	}
}

// TestMockScriptedExhausted: a single-slot scripted mock with no
// response installed (neither via the constructor nor SetResponse)
// yields ErrMockExhausted on Consult.
func TestMockScriptedExhausted(t *testing.T) {
	m := NewScriptedMock(nil, nil)
	_, err := m.Consult(context.Background(), Request{})
	if !errors.Is(err, ErrMockExhausted) {
		t.Fatalf("expected ErrMockExhausted with no response installed, got %v", err)
	}
}

// TestMockPerConsultWalksQueue: the per-consult mode advances one entry per
// Consult and reports exhaustion past the end — the shape a
// reject-and-retry test needs (the retry must see a different response).
func TestMockPerConsultWalksQueue(t *testing.T) {
	m := NewScriptedMockPerConsult([]Response{{Content: "a"}, {Content: "b"}})
	for _, want := range []string{"a", "b"} {
		got, err := m.Consult(context.Background(), Request{})
		if err != nil {
			t.Fatalf("consult: %v", err)
		}
		if got.Content != want {
			t.Errorf("content: got %q, want %q", got.Content, want)
		}
	}
	if _, err := m.Consult(context.Background(), Request{}); !errors.Is(err, ErrMockExhausted) {
		t.Errorf("past queue end: got %v, want ErrMockExhausted", err)
	}
}

// TestMockErrorInjection: an installed error pre-empts the scripted
// response on every entry point, and clearing it restores service.
func TestMockErrorInjection(t *testing.T) {
	injected := errors.New("provider unavailable")
	m := NewScriptedMock([]Response{{Content: "ok"}}, []ModelInfo{{ID: "m"}})
	m.SetError(injected)

	ctx := context.Background()
	if _, err := m.Consult(ctx, Request{}); !errors.Is(err, injected) {
		t.Errorf("Consult: got %v, want the injected error", err)
	}
	if _, err := m.ConsultStream(ctx, Request{}); !errors.Is(err, injected) {
		t.Errorf("ConsultStream: got %v, want the injected error", err)
	}
	if _, err := m.ListModels(ctx); !errors.Is(err, injected) {
		t.Errorf("ListModels: got %v, want the injected error", err)
	}

	m.SetError(nil)
	got, err := m.Consult(ctx, Request{})
	if err != nil {
		t.Fatalf("after clearing: %v", err)
	}
	if got.Content != "ok" {
		t.Errorf("after clearing: got %q, want ok", got.Content)
	}
}

// TestMockCallsAccumulates: every Consult invocation is recorded,
// including a call past the queue end that yields ErrMockExhausted. A
// per-consult-queue mock has queue-exhaustion semantics, so it is the
// natural vehicle for this assertion.
func TestMockCallsAccumulates(t *testing.T) {
	m := NewScriptedMockPerConsult([]Response{
		{Content: "a"},
		{Content: "b"},
		{Content: "c"},
	})
	m.RecordCalls = true
	for i := 0; i < 4; i++ {
		_, _ = m.Consult(context.Background(), Request{Model: "test"})
	}
	calls := m.Calls()
	if len(calls) != 4 {
		t.Fatalf("Calls count: got %d, want 4 (3 successful + 1 exhausted)", len(calls))
	}
	if calls[0].Response.Content != "a" {
		t.Errorf("first call response: got %q, want a", calls[0].Response.Content)
	}
	if calls[0].Request.Model != "test" {
		t.Errorf("first call request model: got %q, want test", calls[0].Request.Model)
	}
	if !errors.Is(calls[3].Err, ErrMockExhausted) {
		t.Errorf("fourth call err: got %v, want ErrMockExhausted", calls[3].Err)
	}
}

// TestMockListModels: the configured listing comes back by value, and an
// unconfigured mock reports an empty list rather than nil.
func TestMockListModels(t *testing.T) {
	want := []ModelInfo{{ID: "gemma-4-31b-it"}, {ID: "gemma-4-26b-a4b-it"}}
	m := NewScriptedMock(nil, want)
	got, err := m.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("models: got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("models[%d]: got %+v, want %+v", i, got[i], want[i])
		}
	}
	// Mutating the returned slice must not reach the mock's copy.
	got[0] = ModelInfo{ID: "tampered"}
	again, _ := m.ListModels(context.Background())
	if again[0] != want[0] {
		t.Errorf("ListModels returned an aliased slice: %+v", again[0])
	}

	empty, err := NewScriptedMock(nil, nil).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels (no models): %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("unconfigured ListModels: got %+v, want an empty slice", empty)
	}
}

// TestMockContextCancellation: a cancelled ctx short-circuits Consult.
func TestMockContextCancellation(t *testing.T) {
	m := NewScriptedMock([]Response{{Content: "ok"}}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := m.Consult(ctx, Request{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestMockNoModeErrors: a zero MockClient (constructed without going
// through New*) errors out instead of silently returning empty responses.
func TestMockNoModeErrors(t *testing.T) {
	m := &MockClient{}
	_, err := m.Consult(context.Background(), Request{})
	if err == nil {
		t.Fatal("expected error from mode-less mock, got nil")
	}
}

func TestMockScriptedStreamSplitsResponse(t *testing.T) {
	scripted := []Response{
		{Content: "abcdefghijklmnop", FinishReason: "stop", Usage: Usage{TotalTokens: 99}},
	}
	m := NewScriptedMock(scripted, nil)
	m.SetMockChunks(4)

	sr, err := m.ConsultStream(context.Background(), Request{})
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	defer sr.Close()

	var got strings.Builder
	chunks := 0
	var lastChunk Chunk
	for {
		chunk, err := sr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		got.WriteString(chunk.Content)
		chunks++
		lastChunk = chunk
	}
	if chunks != 4 {
		t.Errorf("chunks: got %d, want 4", chunks)
	}
	if got.String() != scripted[0].Content {
		t.Errorf("concat: got %q, want %q", got.String(), scripted[0].Content)
	}
	// Final chunk carries the metadata.
	if lastChunk.FinishReason != "stop" {
		t.Errorf("last chunk finish_reason: got %q, want stop", lastChunk.FinishReason)
	}
	if lastChunk.Usage.TotalTokens != 99 {
		t.Errorf("last chunk usage: got %d, want 99", lastChunk.Usage.TotalTokens)
	}

	final := sr.Final()
	if final.Content != scripted[0].Content {
		t.Errorf("Final.Content: got %q, want %q", final.Content, scripted[0].Content)
	}
	if final.FinishReason != "stop" {
		t.Errorf("Final.FinishReason: got %q, want stop", final.FinishReason)
	}
}

func TestMockStreamCloseIdempotent(t *testing.T) {
	m := NewScriptedMock([]Response{{Content: "x"}}, nil)
	sr, err := m.ConsultStream(context.Background(), Request{})
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	if err := sr.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := sr.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if _, err := sr.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("Next after Close: got %v, want io.EOF", err)
	}
}

func TestMockStreamCtxCancelBetweenChunks(t *testing.T) {
	m := NewScriptedMock([]Response{{Content: "abcdefgh"}}, nil)
	m.SetMockChunks(4)
	ctx, cancel := context.WithCancel(context.Background())
	sr, err := m.ConsultStream(ctx, Request{})
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	defer sr.Close()
	// One chunk delivered, then cancel; the next Next must report ctx err.
	if _, err := sr.Next(); err != nil {
		t.Fatalf("first Next: %v", err)
	}
	cancel()
	_, err = sr.Next()
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestMockStreamDefaultChunkCount(t *testing.T) {
	m := NewScriptedMock([]Response{{Content: strings.Repeat("a", 64)}}, nil)
	// No SetMockChunks call → DefaultMockChunks.
	sr, err := m.ConsultStream(context.Background(), Request{})
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	defer sr.Close()
	chunks := 0
	for {
		_, err := sr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		chunks++
	}
	if chunks != DefaultMockChunks {
		t.Errorf("chunk count: got %d, want %d", chunks, DefaultMockChunks)
	}
}

// TestMockStreamSplitPreservesContent: whatever the chunk count, the
// concatenated pieces reconstruct the response exactly — including the
// degenerate cases (empty body, more chunks than runes, multi-byte runes,
// n ≤ 0).
func TestMockStreamSplitPreservesContent(t *testing.T) {
	for _, content := range []string{"", "a", "abcdefghij", "héllo wörld ✓"} {
		for _, n := range []int{-1, 0, 1, 3, 8, 64} {
			pieces := splitForChunks(content, n)
			if got := strings.Join(pieces, ""); got != content {
				t.Errorf("splitForChunks(%q, %d) concat = %q", content, n, got)
			}
			if len(pieces) == 0 {
				t.Errorf("splitForChunks(%q, %d) produced no pieces", content, n)
			}
		}
	}
}
