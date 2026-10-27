package modeltest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"kbase/internal/model"
)

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

// TestMockScriptedServesCurrentResponse: a single-slot scripted mock
// serves whatever response SetResponse last installed.
func TestMockScriptedServesCurrentResponse(t *testing.T) {
	scripted := []model.Response{
		{Content: "first", FinishReason: "stop"},
		{Content: "second", FinishReason: "stop"},
		{Content: "third", FinishReason: "stop"},
	}
	m := NewScriptedMock(nil, nil)

	for i, want := range scripted {
		m.SetResponse(want)
		got, err := m.consult(context.Background(), model.Request{})
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
	m.SetResponse(model.Response{Content: "step1"})
	for c := 0; c < 3; c++ {
		got, err := m.consult(context.Background(), model.Request{})
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
// yields ErrMockExhausted on consult.
func TestMockScriptedExhausted(t *testing.T) {
	m := NewScriptedMock(nil, nil)
	_, err := m.consult(context.Background(), model.Request{})
	if !errors.Is(err, ErrMockExhausted) {
		t.Fatalf("expected ErrMockExhausted with no response installed, got %v", err)
	}
}

// TestMockPerConsultWalksQueue: the per-consult mode advances one entry per
// consult and reports exhaustion past the end — the shape a
// reject-and-retry test needs (the retry must see a different response).
func TestMockPerConsultWalksQueue(t *testing.T) {
	m := NewScriptedMockPerConsult([]model.Response{{Content: "a"}, {Content: "b"}})
	for _, want := range []string{"a", "b"} {
		got, err := m.consult(context.Background(), model.Request{})
		if err != nil {
			t.Fatalf("consult: %v", err)
		}
		if got.Content != want {
			t.Errorf("content: got %q, want %q", got.Content, want)
		}
	}
	if _, err := m.consult(context.Background(), model.Request{}); !errors.Is(err, ErrMockExhausted) {
		t.Errorf("past queue end: got %v, want ErrMockExhausted", err)
	}
}

// TestMockErrorInjection: an installed error pre-empts the scripted
// response on every entry point, and clearing it restores service.
func TestMockErrorInjection(t *testing.T) {
	injected := errors.New("provider unavailable")
	m := NewScriptedMock([]model.Response{{Content: "ok"}}, []model.ModelInfo{{ID: "m"}})
	m.SetError(injected)

	ctx := context.Background()
	if _, err := m.consult(ctx, model.Request{}); !errors.Is(err, injected) {
		t.Errorf("consult: got %v, want the injected error", err)
	}
	if _, err := m.ConsultStream(ctx, model.Request{}); !errors.Is(err, injected) {
		t.Errorf("ConsultStream: got %v, want the injected error", err)
	}
	if _, err := m.ListModels(ctx); !errors.Is(err, injected) {
		t.Errorf("ListModels: got %v, want the injected error", err)
	}

	m.SetError(nil)
	got, err := m.consult(ctx, model.Request{})
	if err != nil {
		t.Fatalf("after clearing: %v", err)
	}
	if got.Content != "ok" {
		t.Errorf("after clearing: got %q, want ok", got.Content)
	}
}

// TestMockCallsAccumulates: every consult invocation is recorded,
// including a call past the queue end that yields ErrMockExhausted. A
// per-consult-queue mock has queue-exhaustion semantics, so it is the
// natural vehicle for this assertion.
func TestMockCallsAccumulates(t *testing.T) {
	m := NewScriptedMockPerConsult([]model.Response{
		{Content: "a"},
		{Content: "b"},
		{Content: "c"},
	})
	m.RecordCalls = true
	for i := 0; i < 4; i++ {
		_, _ = m.consult(context.Background(), model.Request{Model: "test"})
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

// TestMockContextCancellation: a cancelled ctx short-circuits consult.
func TestMockContextCancellation(t *testing.T) {
	m := NewScriptedMock([]model.Response{{Content: "ok"}}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := m.consult(ctx, model.Request{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestMockNoModeErrors: a zero MockClient (constructed without going
// through New*) errors out instead of silently returning empty responses.
func TestMockNoModeErrors(t *testing.T) {
	m := &MockClient{}
	_, err := m.consult(context.Background(), model.Request{})
	if err == nil {
		t.Fatal("expected error from mode-less mock, got nil")
	}
}
