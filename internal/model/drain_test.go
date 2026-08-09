package model

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// wantZeroResponse fails the test unless r is the zero Response — the
// "nothing partial escaped" check every error path here owes its caller.
func wantZeroResponse(t *testing.T, r Response) {
	t.Helper()
	if r != (Response{}) {
		t.Errorf("Response: got %+v, want zero", r)
	}
}

// TestConsultDrainedMatchesConsult: draining the stream must produce the
// same Response the blocking call would have — that equivalence is the
// whole premise of using the streaming transport for batch call sites.
func TestConsultDrainedMatchesConsult(t *testing.T) {
	want := Response{
		Content:      "the whole answer, delivered in pieces",
		FinishReason: "stop",
		Usage:        Usage{PromptTokens: 11, CompletionTokens: 7, TotalTokens: 18},
	}
	m := NewScriptedMock([]Response{want}, nil)
	m.SetMockChunks(5)

	got, err := ConsultDrained(context.Background(), m, Request{Model: "m"})
	if err != nil {
		t.Fatalf("ConsultDrained: %v", err)
	}
	if got.Content != want.Content {
		t.Errorf("Content: got %q, want %q", got.Content, want.Content)
	}
	if got.FinishReason != want.FinishReason {
		t.Errorf("FinishReason: got %q, want %q", got.FinishReason, want.FinishReason)
	}
	if got.Usage != want.Usage {
		t.Errorf("Usage: got %+v, want %+v", got.Usage, want.Usage)
	}

	blocking, err := m.Consult(context.Background(), Request{Model: "m"})
	if err != nil {
		t.Fatalf("Consult: %v", err)
	}
	if got.Content != blocking.Content {
		t.Errorf("drained %q != blocking %q", got.Content, blocking.Content)
	}
}

// TestConsultDrainedEmptyResponse: a generation that emitted nothing is a
// real shape (the whole budget spent on reasoning). It must come back as an
// empty Response, not an error.
func TestConsultDrainedEmptyResponse(t *testing.T) {
	m := NewScriptedMock([]Response{{Content: "", FinishReason: "stop"}}, nil)
	got, err := ConsultDrained(context.Background(), m, Request{Model: "m"})
	if err != nil {
		t.Fatalf("ConsultDrained: %v", err)
	}
	if got.Content != "" {
		t.Errorf("Content: got %q, want empty", got.Content)
	}
	if got.FinishReason != "stop" {
		t.Errorf("FinishReason: got %q, want stop", got.FinishReason)
	}
}

// TestConsultDrainedOpenError: a failure opening the stream propagates
// unchanged, so callers can still match their own sentinels.
func TestConsultDrainedOpenError(t *testing.T) {
	injected := errors.New("provider unavailable")
	m := NewScriptedMock([]Response{{Content: "unused"}}, nil)
	m.SetError(injected)

	got, err := ConsultDrained(context.Background(), m, Request{Model: "m"})
	if !errors.Is(err, injected) {
		t.Fatalf("error: got %v, want the injected error", err)
	}
	wantZeroResponse(t, got)
}

// TestConsultDrainedOverSSE: the real transport path — content accumulates
// across deltas and the include_usage final chunk lands in Usage.
func TestConsultDrainedOverSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"reasoning_content":"scratch"}}]}

data: {"choices":[{"delta":{"content":"Hel"}}]}

data: {"choices":[{"delta":{"content":"lo"},"finish_reason":"stop"}]}

data: {"choices":[],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}

data: [DONE]

`))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	got, err := ConsultDrained(context.Background(), c, DefaultRequest("m", []Message{{Role: "user", Content: "hi"}}))
	if err != nil {
		t.Fatalf("ConsultDrained: %v", err)
	}
	if got.Content != "Hello" {
		t.Errorf("Content: got %q, want %q (reasoning must not leak in)", got.Content, "Hello")
	}
	if got.FinishReason != "stop" {
		t.Errorf("FinishReason: got %q, want stop", got.FinishReason)
	}
	if got.Usage.TotalTokens != 6 {
		t.Errorf("Usage.TotalTokens: got %d, want 6", got.Usage.TotalTokens)
	}
}

// TestConsultDrainedContextCancellation: a context cancelled while the
// drain loop is blocked on a stalled provider surfaces as context.Canceled
// — the loop must not outlast its deadline waiting for the next delta.
func TestConsultDrainedContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Stall until the client's context fires.
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	got, err := ConsultDrained(ctx, c, DefaultRequest("m", []Message{{Role: "user", Content: "hi"}}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error: got %v, want context.Canceled", err)
	}
	wantZeroResponse(t, got)
}

// TestConsultDrainedPreCancelledContext: a context already cancelled at the
// call is refused at stream open, before any request is made.
func TestConsultDrainedPreCancelledContext(t *testing.T) {
	m := NewScriptedMock([]Response{{Content: "abcdefgh"}}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := ConsultDrained(ctx, m, Request{Model: "m"}); !errors.Is(err, context.Canceled) {
		t.Errorf("error: got %v, want context.Canceled", err)
	}
}
