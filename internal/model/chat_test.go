package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// statusServer answers every request with code and a provider-shaped error
// body, counting the requests it read.
func statusServer(t *testing.T, code int) (*httptest.Server, func() int) {
	t.Helper()
	var mu sync.Mutex
	var seen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen++
		mu.Unlock()
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"error":{"message":"unknown model: m"}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() int {
		mu.Lock()
		defer mu.Unlock()
		return seen
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
		name        string
		bodies      []string
		requests    int
		ok          bool
		wantOutcome string
	}{
		{"answered at once", []string{sseAnswered}, 1, true, ChatOK},
		{"short of [DONE], then answered", []string{sseShortOfDone, sseAnswered}, 2, true, ChatOK},
		{"served nothing, then answered", []string{sseServedNothing, sseAnswered}, 2, true, ChatOK},
		{"never complete", []string{sseShortOfDone}, ChatAttempts, false, ChatTransportFailure},
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
			if got.Outcome != tc.wantOutcome {
				t.Errorf("outcome = %q, want %q", got.Outcome, tc.wantOutcome)
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

// TestChatNamesAProviderRefusal: a provider that answers with a status that is
// not success read the request and declined it, which is not the transport
// failing — so the attempt says it refused, and the reason carries the status
// and the body's start rather than sending a reader to the path between kbase
// and a provider that is answering. A 400 will be as wrong on the third
// attempt as on the first, so the call is put on the wire once.
func TestChatNamesAProviderRefusal(t *testing.T) {
	srv, requests := statusServer(t, http.StatusBadRequest)

	var observed []string
	got, err := Chat(context.Background(), NewHTTPClient(Endpoint{BaseURL: srv.URL}), "m", "the system", "the prompt", nil,
		func(a ChatAttempt) { observed = append(observed, a.Outcome) })
	if !errors.Is(err, ErrChatIncomplete) {
		t.Fatalf("Chat = %v, want an incomplete call", err)
	}
	if got.Outcome != ChatProviderRefused {
		t.Errorf("outcome = %q, want %q", got.Outcome, ChatProviderRefused)
	}
	for _, reason := range []string{got.Error, err.Error()} {
		if !strings.Contains(reason, "http 400") || !strings.Contains(reason, "unknown model") {
			t.Errorf("reason = %q, want the status and the provider's own words", reason)
		}
		if strings.Contains(reason, ChatTransportFailure) {
			t.Errorf("reason = %q, want no transport failure where the provider answered", reason)
		}
	}
	if !strings.Contains(err.Error(), "1 attempt") {
		t.Errorf("reason = %q, want the one attempt it took", err)
	}
	if n := requests(); n != 1 || len(observed) != 1 {
		t.Fatalf("%d requests, %d attempts observed; want one: a refusal is not re-issued", n, len(observed))
	}
	for _, o := range observed {
		if o != ChatProviderRefused {
			t.Errorf("an attempt = %q, want %q", o, ChatProviderRefused)
		}
	}
}

// TestChatReissuesOnlyARetryableStatus: a status that may be the provider's
// moment — 429 and the 5xx class — is re-issued to the attempt count; every
// other answered status ends the call on the one attempt that met it.
func TestChatReissuesOnlyARetryableStatus(t *testing.T) {
	for _, tc := range []struct {
		code     int
		requests int
	}{
		{http.StatusBadRequest, 1},
		{http.StatusUnauthorized, 1},
		{http.StatusNotFound, 1},
		{http.StatusTooManyRequests, ChatAttempts},
		{http.StatusServiceUnavailable, ChatAttempts},
	} {
		t.Run(fmt.Sprintf("%d", tc.code), func(t *testing.T) {
			srv, requests := statusServer(t, tc.code)

			var observed []ChatAttempt
			got, err := Chat(context.Background(), NewHTTPClient(Endpoint{BaseURL: srv.URL}), "m", "the system", "the prompt", nil,
				func(a ChatAttempt) { observed = append(observed, a) })
			if !errors.Is(err, ErrChatIncomplete) {
				t.Fatalf("Chat = %v, want an incomplete call", err)
			}
			if got.Outcome != ChatProviderRefused {
				t.Errorf("outcome = %q, want %q", got.Outcome, ChatProviderRefused)
			}
			for _, reason := range []string{got.Error, err.Error()} {
				if want := fmt.Sprintf("http %d", tc.code); !strings.Contains(reason, want) {
					t.Errorf("reason = %q, want it to name %q", reason, want)
				}
			}
			if n := requests(); n != tc.requests || len(observed) != tc.requests {
				t.Fatalf("%d requests, %d attempts observed; want %d", n, len(observed), tc.requests)
			}
			if tc.requests == 1 {
				if got.Attempt != 1 || !strings.Contains(err.Error(), "1 attempt") {
					t.Errorf("attempt %d, reason %q; want the one attempt it took", got.Attempt, err)
				}
				return
			}
			if got.Attempt != ChatAttempts {
				t.Errorf("attempt = %d, want the last of %d", got.Attempt, ChatAttempts)
			}
		})
	}
}

// TestChatIncompleteReasonWithoutAttemptText: an attempt can end without an
// error text of its own — a server that served nothing says so through its
// usage alone — and the reason then ends on the outcome, with no separator
// left dangling after it.
func TestChatIncompleteReasonWithoutAttemptText(t *testing.T) {
	srv, _ := sseServer(t, sseServedNothing)

	got, err := Chat(context.Background(), NewHTTPClient(Endpoint{BaseURL: srv.URL}), "m", "the system", "the prompt", nil, nil)
	if !errors.Is(err, ErrChatIncomplete) {
		t.Fatalf("Chat = %v, want an incomplete call", err)
	}
	if got.Outcome != ChatServedNothing || got.Error != "" {
		t.Fatalf("last attempt = %+v, want an outcome of %q and no error text", got, ChatServedNothing)
	}
	if want := "the last ending " + got.Outcome; !strings.HasSuffix(err.Error(), want) {
		t.Errorf("reason = %q, want it to end %q", err, want)
	}
}

// TestChatNamesAnUnreachableEndpoint: nothing answering at all is the
// transport's failure — the case the refusal above must not be read as, and
// what a closed port looks like. No status refused it, so it is re-issued to
// the attempt count like an incomplete stream.
func TestChatNamesAnUnreachableEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	var observed []string
	got, err := Chat(context.Background(), NewHTTPClient(Endpoint{BaseURL: url}), "m", "the system", "the prompt", nil,
		func(a ChatAttempt) { observed = append(observed, a.Outcome) })
	if !errors.Is(err, ErrChatIncomplete) {
		t.Fatalf("Chat = %v, want an incomplete call", err)
	}
	if got.Outcome != ChatTransportFailure {
		t.Errorf("outcome = %q, want %q", got.Outcome, ChatTransportFailure)
	}
	if len(observed) != ChatAttempts {
		t.Errorf("%d attempts observed, want %d: a transport failure is re-issued", len(observed), ChatAttempts)
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
		Reasoning   map[string]any   `json:"reasoning"`
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
	if len(body.Reasoning) != 1 || body.Reasoning["enabled"] != false {
		t.Errorf("reasoning = %v, want the provider's control with enabled false", body.Reasoning)
	}
	if !body.Stream || body.Options["include_usage"] != true || body.Tools != nil {
		t.Errorf("stream %t, stream_options %v, tools %v", body.Stream, body.Options, body.Tools)
	}
	if body.MaxTokens != nil {
		t.Errorf("max_tokens %d sent; the request must carry none", *body.MaxTokens)
	}
}

// TestChatRecordsReasoned: a call's attempt says it reasoned from either
// source the wire offers — a reasoning delta on the stream, or a usage block
// reporting reasoning tokens spent. The second is what makes a call that
// thought silently visible: no delta carried the thought, and the token count
// is the only trace of it. A call with neither source did not reason.
func TestChatRecordsReasoned(t *testing.T) {
	const answered = "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n"
	usage := func(reasoningTokens int) string {
		return fmt.Sprintf("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":40,\"completion_tokens\":900,\"total_tokens\":940,\"completion_tokens_details\":{\"reasoning_tokens\":%d}}}\n\n", reasoningTokens)
	}
	const (
		thought = "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"weighing it\"}}]}\n\n"
		done    = "data: [DONE]\n\n"
	)
	for _, tc := range []struct {
		name string
		sse  string
		want bool
	}{
		{"usage reports reasoning tokens, no delta", answered + usage(880) + done, true},
		{"neither a delta nor reasoning tokens", answered + usage(0) + done, false},
		{"a reasoning delta, usage reports none", thought + answered + usage(0) + done, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, seen := sseServer(t, tc.sse)
			got, err := Chat(context.Background(), NewHTTPClient(Endpoint{BaseURL: srv.URL}), "m", "the system", "the prompt", nil, nil)
			if err != nil {
				t.Fatalf("Chat: %v", err)
			}
			if n := len(seen()); n != 1 {
				t.Fatalf("%d requests; want the one it took", n)
			}
			if got.Reasoned != tc.want {
				t.Errorf("Reasoned = %t, want %t (usage %+v)", got.Reasoned, tc.want, got.Usage)
			}
		})
	}
}

// TestChatCarriesAnEmptyMessageContent: a message's content field reaches
// the wire even when empty — `"content":""`, never an absent field. A
// provider reads `{"role":"system"}` as a message with no text part and
// declines the whole request, so this is the shape of kbase's empty system
// prompt being sent at all.
func TestChatCarriesAnEmptyMessageContent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		system string
		prompt string
		want   []string
	}{
		{"empty system", "", "the prompt", []string{
			`{"role":"system","content":""}`,
			`{"role":"user","content":"the prompt"}`,
		}},
		{"empty prompt", "the system", "", []string{
			`{"role":"system","content":"the system"}`,
			`{"role":"user","content":""}`,
		}},
		{"neither empty", "the system", "the prompt", []string{
			`{"role":"system","content":"the system"}`,
			`{"role":"user","content":"the prompt"}`,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, seen := sseServer(t, sseAnswered)
			if _, err := Chat(context.Background(), NewHTTPClient(Endpoint{BaseURL: srv.URL}), "m", tc.system, tc.prompt, nil, nil); err != nil {
				t.Fatal(err)
			}
			raw := string(seen()[0])
			for _, w := range tc.want {
				if !strings.Contains(raw, w) {
					t.Errorf("body = %s, want it to carry %s", raw, w)
				}
			}
		})
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
