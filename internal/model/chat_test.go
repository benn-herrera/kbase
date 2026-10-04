package model

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// sseServer answers each request with the next body of bodies, the last one
// repeating, and keeps every request body it read.
func sseServer(t *testing.T, bodies ...string) (*httptest.Server, func() [][]byte) {
	t.Helper()
	var mu sync.Mutex
	var seen [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, b)
		body := bodies[min(len(seen), len(bodies))-1]
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, func() [][]byte {
		mu.Lock()
		defer mu.Unlock()
		return append([][]byte{}, seen...)
	}
}

const (
	sseAnswered = "data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":40,\"completion_tokens\":1,\"total_tokens\":41}}\n\n" +
		"data: [DONE]\n\n"
	sseShortOfDone   = "data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\n"
	sseServedNothing = "data: {\"choices\":[{\"delta\":{\"content\":\"out of memory\"}}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":0,\"completion_tokens\":0,\"total_tokens\":0}}\n\n" +
		"data: [DONE]\n\n"
)

func TestChatReissuesWhatDidNotComplete(t *testing.T) {
	for _, tc := range []struct {
		name     string
		bodies   []string
		requests int
		ok       bool
	}{
		{"answered at once", []string{sseAnswered}, 1, true},
		{"short of [DONE], then answered", []string{sseShortOfDone, sseAnswered}, 2, true},
		{"served nothing, then answered", []string{sseServedNothing, sseAnswered}, 2, true},
		{"never complete", []string{sseShortOfDone}, ChatAttempts, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, seen := sseServer(t, tc.bodies...)
			var observed []string
			got, err := Chat(context.Background(), NewHTTPClient(Endpoint{BaseURL: srv.URL}), "m", "the system", "the prompt", nil,
				func(a ChatAttempt) { observed = append(observed, a.Outcome) })
			if tc.ok != (err == nil) || (!tc.ok && !errors.Is(err, ErrChatIncomplete)) {
				t.Fatalf("Chat = %+v, %v; want ok %t", got, err, tc.ok)
			}
			if tc.ok && (got.Reply != "A" || got.Outcome != ChatOK || got.Usage.PromptTokens != 40) {
				t.Errorf("Chat = %+v", got)
			}
			requests := seen()
			if len(requests) != tc.requests || len(observed) != tc.requests {
				t.Fatalf("requests %d, observed %q; want %d", len(requests), observed, tc.requests)
			}
			for _, b := range requests[1:] {
				if string(b) != string(requests[0]) {
					t.Errorf("a re-issue differed from the first request:\n%s\n%s", b, requests[0])
				}
			}
		})
	}
}

func TestChatRequestShape(t *testing.T) {
	srv, seen := sseServer(t, sseAnswered)
	if _, err := Chat(context.Background(), NewHTTPClient(Endpoint{BaseURL: srv.URL}), "m", "the system", "the prompt", nil, nil); err != nil {
		t.Fatal(err)
	}
	var body struct {
		MaxTokens   *int             `json:"max_tokens"`
		Model       string           `json:"model"`
		Messages    []map[string]any `json:"messages"`
		Temperature *float64         `json:"temperature"`
		Kwargs      map[string]any   `json:"chat_template_kwargs"`
		Stream      bool             `json:"stream"`
		Options     map[string]any   `json:"stream_options"`
		Tools       any              `json:"tools"`
	}
	if err := json.Unmarshal(seen()[0], &body); err != nil {
		t.Fatal(err)
	}
	if body.Model != "m" || len(body.Messages) != 2 || body.Messages[0]["role"] != "system" || body.Messages[0]["content"] != "the system" ||
		body.Messages[1]["role"] != "user" || body.Messages[1]["content"] != "the prompt" {
		t.Errorf("messages = %+v", body.Messages)
	}
	if body.Temperature == nil || *body.Temperature != 0 || body.Kwargs["enable_thinking"] != false || len(body.Kwargs) != 1 {
		t.Errorf("temperature %v, chat_template_kwargs %v", body.Temperature, body.Kwargs)
	}
	if !body.Stream || body.Options["include_usage"] != true || body.Tools != nil {
		t.Errorf("stream %t, stream_options %v, tools %v", body.Stream, body.Options, body.Tools)
	}
	if body.MaxTokens != nil {
		t.Errorf("max_tokens %d sent; kb_tools sends none", *body.MaxTokens)
	}
}

func TestChatStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Chat(ctx, NewScriptedMock([]Response{{Content: "A"}}, nil), "m", "s", "p", nil, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Chat on a cancelled context = %v", err)
	}
}

// TestChatBacksOffBetweenReissues: each re-issue waits the next pause of the
// backoff, its last repeating, and a cancel during a pause ends the call.
func TestChatBacksOffBetweenReissues(t *testing.T) {
	srv, seen := sseServer(t, sseShortOfDone)
	started := time.Now()
	if _, err := Chat(context.Background(), NewHTTPClient(Endpoint{BaseURL: srv.URL}), "m", "s", "p",
		[]time.Duration{30 * time.Millisecond}, nil); !errors.Is(err, ErrChatIncomplete) {
		t.Fatalf("Chat = %v, want incomplete", err)
	}
	if took := time.Since(started); len(seen()) != ChatAttempts || took < time.Duration(ChatAttempts-1)*30*time.Millisecond {
		t.Errorf("%d requests over %v, want %d with a 30ms pause before each re-issue", len(seen()), took, ChatAttempts)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := Chat(ctx, NewHTTPClient(Endpoint{BaseURL: srv.URL}), "m", "s", "p", []time.Duration{time.Hour}, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Chat cancelled in its pause = %v", err)
	}
}
