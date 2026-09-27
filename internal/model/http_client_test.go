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
	"testing"
	"time"
)

const testAPIKey = "sk-TEST-MUST-NOT-LEAK-1234567890abcdef"

func newTestEndpoint(baseURL string) Endpoint {
	return Endpoint{
		Name:    "test",
		BaseURL: baseURL,
		APIKey:  testAPIKey,
	}
}

// runStream serves the given SSE body from a test server, drains the
// stream, and returns every chunk observed, the accumulated Final(), and
// the TERMINAL error from Next (io.EOF on a clean stream).
func runStream(t *testing.T, sse string) ([]Chunk, Response, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", mimeEventStream)
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	sr, err := c.ConsultStream(context.Background(), DefaultRequest("m", []Message{{Role: "user", Content: "go"}}, testEffort))
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	defer sr.Close()

	var chunks []Chunk
	var termErr error
	for {
		chunk, err := sr.Next()
		if err != nil {
			termErr = err
			break
		}
		chunks = append(chunks, chunk)
	}
	return chunks, sr.Final(), termErr
}

// TestHTTPClientHappyPath: verify the request body shape, the auth header,
// the User-Agent header, and the response decode round-trip.
func TestHTTPClientHappyPath(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotUA string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"index": 0,
				"finish_reason": "stop",
				"message": {"role": "assistant", "content": "hello world"}
			}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
		}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	resp, err := c.Consult(context.Background(), Request{
		Model: "test-model",
		Messages: []Message{
			{Role: "system", Content: "be brief"},
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Consult: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method: got %q, want POST", gotMethod)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path: got %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer "+testAPIKey {
		t.Errorf("auth header missing or wrong: got %q", gotAuth)
	}
	if gotUA != userAgent {
		t.Errorf("user-agent: got %q, want %q", gotUA, userAgent)
	}
	if gotBody["model"] != "test-model" {
		t.Errorf("model in body: got %v, want test-model", gotBody["model"])
	}
	msgs, _ := gotBody["messages"].([]any)
	if len(msgs) != 2 {
		t.Errorf("messages count: got %d, want 2", len(msgs))
	}

	if resp.Content != "hello world" {
		t.Errorf("content: got %q, want %q", resp.Content, "hello world")
	}
	if resp.FinishReason != "stop" {
		t.Errorf("finish_reason: got %q, want stop", resp.FinishReason)
	}
	if resp.Usage.TotalTokens != 7 {
		t.Errorf("total_tokens: got %d, want 7", resp.Usage.TotalTokens)
	}
}

// TestHTTPClientBaseURLTrailingSlash: a base URL ending in "/" must produce
// "<base>chat/completions", not "<base>/chat/completions".
func TestHTTPClientBaseURLTrailingSlash(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}],"usage":{}}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL + "/"))
	if _, err := c.Consult(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}}); err != nil {
		t.Fatalf("Consult: %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path with trailing-slash base: got %q, want /chat/completions", gotPath)
	}
}

// TestHTTPClientNoAPIKeyOmitsAuthHeader: an empty APIKey must not produce a
// "Bearer " header (some local servers reject empty bearer tokens).
func TestHTTPClientNoAPIKeyOmitsAuthHeader(t *testing.T) {
	var sawAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawAuth = r.Header["Authorization"]
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}],"usage":{}}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(Endpoint{Name: "nokey", BaseURL: srv.URL})
	if _, err := c.Consult(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}}); err != nil {
		t.Fatalf("Consult: %v", err)
	}
	if sawAuth {
		t.Errorf("Authorization header sent despite empty APIKey")
	}
}

// TestHTTPClientHTTPErrorWraps: a 5xx response is surfaced as a wrapped
// error, and the API key never appears in the error string even when the
// server echoes the Authorization header back.
func TestHTTPClientHTTPErrorWraps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo what we got, including the Authorization header (this is the
		// scenario the scrubber exists to defend against).
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error: auth was " + r.Header.Get("Authorization") + " but who cares"))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	_, err := c.Consult(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}})
	if err == nil {
		t.Fatal("expected error from 500, got nil")
	}
	if !strings.Contains(err.Error(), "http 500") {
		t.Errorf("error missing status code: %v", err)
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("API key leaked into 5xx error: %v", err)
	}
}

// TestHTTPClientContextCancellation: a cancelled context must propagate
// through Do() and be reachable via errors.Is(err, context.Canceled).
func TestHTTPClientContextCancellation(t *testing.T) {
	// Server holds the response until either the request context fires
	// or the test signals the stop channel. The stop channel is closed
	// before srv.Close so the handler returns and srv.Close's
	// WaitGroup.Wait doesn't block on an outstanding connection.
	stop := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-stop:
		}
	}))
	defer func() {
		close(stop)
		srv.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	_, err := c.Consult(ctx, Request{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}})
	if err == nil {
		t.Fatal("expected error from cancellation, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestHTTPClientErrorBodyIsBounded: a provider that answers a failed call
// with an enormous body (an HTML error page, a runaway log dump) must not
// turn one request into an unbounded error string.
func TestHTTPClientErrorBodyIsBounded(t *testing.T) {
	const bodyBytes = 4 * errorBodyLimit
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("x", bodyBytes)))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	_, err := c.Consult(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}})
	if err == nil {
		t.Fatal("expected error from 502, got nil")
	}
	if n := strings.Count(err.Error(), "x"); n != errorBodyLimit {
		t.Errorf("echoed body: got %d bytes, want the %d-byte cap (server sent %d)", n, errorBodyLimit, bodyBytes)
	}
}

// TestHTTPClientErrorScrubsAPIKey: smoke-test the scrubber against a
// pathological payload that intentionally embeds the key in the body.
func TestHTTPClientErrorScrubsAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		// Worst case: server reflects the key into the error body.
		_, _ = w.Write([]byte("bad request: token=" + testAPIKey))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	_, err := c.Consult(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("API key leaked into error string: %v", err)
	}
	if !strings.Contains(err.Error(), "<redacted>") {
		t.Errorf("expected <redacted> sentinel in scrubbed error, got: %v", err)
	}
}

// TestHTTPClientEmptyBaseURLErrors: an empty BaseURL must produce a clear
// error rather than an HTTP attempt against a malformed URL — on every
// method, since a misconfigured endpoint reaches all three.
func TestHTTPClientEmptyBaseURLErrors(t *testing.T) {
	c := NewHTTPClient(Endpoint{Name: "x"})
	ctx := context.Background()
	req := Request{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}}

	calls := map[string]func() error{
		"Consult":       func() error { _, err := c.Consult(ctx, req); return err },
		"ConsultStream": func() error { _, err := c.ConsultStream(ctx, req); return err },
		"ListModels":    func() error { _, err := c.ListModels(ctx); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			if err == nil {
				t.Fatal("expected error on empty BaseURL, got nil")
			}
			if !strings.Contains(err.Error(), "BaseURL") {
				t.Errorf("error doesn't mention BaseURL: %v", err)
			}
		})
	}
}

// TestHTTPClientListModels: the /models GET carries no body, and the
// listing comes back in the server's order.
func TestHTTPClientListModels(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_, _ = w.Write([]byte(`{"object":"list","data":[
			{"id":"gemma-4-31b-it","object":"model","created":123,"owned_by":"google"},
			{"id":"gemma-4-26b-a4b-it","object":"model"}
		]}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method: got %q, want GET", gotMethod)
	}
	if gotPath != "/models" {
		t.Errorf("path: got %q, want /models", gotPath)
	}
	want := []ModelInfo{
		{ID: "gemma-4-31b-it", Created: 123, OwnedBy: "google"},
		{ID: "gemma-4-26b-a4b-it"},
	}
	if len(models) != len(want) {
		t.Fatalf("models: got %+v, want %+v", models, want)
	}
	for i := range want {
		if models[i] != want[i] {
			t.Errorf("models[%d]: got %+v, want %+v", i, models[i], want[i])
		}
	}
}

// TestHTTPClientListModelsErrorScrubs: a failing /models call surfaces the
// status without leaking the key.
func TestHTTPClientListModelsErrorScrubs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("no: token=" + testAPIKey))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	_, err := c.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error from 401, got nil")
	}
	if !strings.Contains(err.Error(), "http 401") {
		t.Errorf("error missing status code: %v", err)
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("API key leaked into error: %v", err)
	}
}

// TestHTTPClientWiresSamplingDefaults: verify that DefaultRequest's values
// (Temperature: 0, MaxTokens: DefaultMaxTokens, chat_template_kwargs with
// thinking keys) round-trip onto the wire — `temperature: 0` must be
// PRESENT in the body, not omitted as it would be with `omitempty`.
func TestHTTPClientWiresSamplingDefaults(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}],"usage":{}}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	req := DefaultRequest("test-model", []Message{{Role: "user", Content: "hi"}}, DeclareEffort(RequestEffort{Thinking: true}))
	if _, err := c.Consult(context.Background(), req); err != nil {
		t.Fatalf("Consult: %v", err)
	}

	// Decode raw to assert presence (a map[string]any decode would lose the
	// "0 vs absent" distinction we care about; check the raw bytes too).
	var asMap map[string]any
	if err := json.Unmarshal(rawBody, &asMap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, ok := asMap["temperature"]; !ok {
		t.Errorf("temperature missing from body: %s", rawBody)
	} else if v != 0.0 {
		t.Errorf("temperature: got %v, want 0", v)
	}
	if v, ok := asMap["max_tokens"]; !ok {
		t.Errorf("max_tokens missing from body: %s", rawBody)
	} else if v != float64(DefaultMaxTokens) {
		t.Errorf("max_tokens: got %v, want %d", v, DefaultMaxTokens)
	}
	// Raw-bytes check that omitempty hasn't dropped temperature: 0.
	if !strings.Contains(string(rawBody), `"temperature":0`) {
		t.Errorf("expected literal `\"temperature\":0` in body: %s", rawBody)
	}
	// chat_template_kwargs presence + thinking keys.
	ctk, ok := asMap["chat_template_kwargs"].(map[string]any)
	if !ok {
		t.Fatalf("chat_template_kwargs missing or wrong type: %v", asMap["chat_template_kwargs"])
	}
	if ctk["thinking"] != true {
		t.Errorf("chat_template_kwargs.thinking: got %v, want true", ctk["thinking"])
	}
	if ctk["enable_thinking"] != true {
		t.Errorf("chat_template_kwargs.enable_thinking: got %v, want true", ctk["enable_thinking"])
	}
	// stream is false (or absent) for blocking Consult.
	if v, ok := asMap["stream"]; ok && v != false {
		t.Errorf("stream: got %v, want absent or false", v)
	}
}

// TestHTTPClientWiresDeclaredEffort: the declared effort reaches BOTH
// chat_template_kwargs keys, in both directions.
//
// The `false` row is the one that matters. Omitting the keys would leave the
// served chat template's own default deciding, which is indistinguishable on
// the wire from a request that never had an opinion — and an effort nobody can
// read back off the wire is not a declaration.
func TestHTTPClientWiresDeclaredEffort(t *testing.T) {
	for _, thinking := range []bool{true, false} {
		t.Run(fmt.Sprintf("thinking=%t", thinking), func(t *testing.T) {
			var rawBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rawBody, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}],"usage":{}}`))
			}))
			defer srv.Close()

			c := NewHTTPClient(newTestEndpoint(srv.URL))
			req := DefaultRequest("test-model", []Message{{Role: "user", Content: "hi"}},
				DeclareEffort(RequestEffort{Thinking: thinking}))
			if _, err := c.Consult(context.Background(), req); err != nil {
				t.Fatalf("Consult: %v", err)
			}

			var asMap map[string]any
			if err := json.Unmarshal(rawBody, &asMap); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			ctk, ok := asMap["chat_template_kwargs"].(map[string]any)
			if !ok {
				t.Fatalf("chat_template_kwargs missing or wrong type: %s", rawBody)
			}
			for _, key := range []string{"thinking", "enable_thinking"} {
				if got, ok := ctk[key]; !ok || got != thinking {
					t.Errorf("chat_template_kwargs.%s: got %v (present %t), want %t", key, got, ok, thinking)
				}
			}
		})
	}
}

// TestEffortDeclaration: a bare RequestEffort literal is NOT a declaration, and
// DeclareEffort is what makes one. The distinction is what lets the pipeline
// refuse an ask that never stated an effort instead of reading a forgotten
// field as a deliberate "no thinking".
func TestEffortDeclaration(t *testing.T) {
	if (RequestEffort{Thinking: true}).Declared() {
		t.Error("a composite literal must not count as a declared effort")
	}
	for _, want := range []bool{true, false} {
		got := DeclareEffort(RequestEffort{Thinking: want})
		if !got.Declared() || got.Thinking != want {
			t.Errorf("DeclareEffort(RequestEffort{Thinking: %t}) = %+v, want declared with that value", want, got)
		}
	}
}

// testEffort is the effort the transport-level tests declare for the requests
// they build. It is a fixture and nothing more: this package has no opinion
// about how hard anything is worth asking, and a package-level "utility"
// effort here would be an API with a test-only population — the declaration
// belongs at the site that registers a definition (ARCHITECTURE.md §9, §12).
var testEffort = DeclareEffort(RequestEffort{Thinking: false})

// TestHTTPClientChatTemplateKwargsOmittedWhenEmpty: an empty/nil map must
// not emit a `chat_template_kwargs: null` or `: {}` field on the wire —
// some providers reject the field's mere presence.
func TestHTTPClientChatTemplateKwargsOmittedWhenEmpty(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}],"usage":{}}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	// Build directly (not via DefaultRequest) so ChatTemplateKwargs stays nil.
	req := Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}}
	if _, err := c.Consult(context.Background(), req); err != nil {
		t.Fatalf("Consult: %v", err)
	}
	if strings.Contains(string(rawBody), "chat_template_kwargs") {
		t.Errorf("chat_template_kwargs should be omitted when empty; got body: %s", rawBody)
	}
}

// TestHTTPClientStreamHappyPath: SSE round-trip. Verify chunks come out in
// order, Final() concat matches the underlying body, finish_reason and
// usage populate, the wire body has stream: true, and the Accept header
// is text/event-stream.
func TestHTTPClientStreamHappyPath(t *testing.T) {
	var gotAccept string
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		rawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		// Three content deltas, then a final chunk with finish_reason
		// + usage, then [DONE].
		const sse = `data: {"choices":[{"delta":{"content":"Hello"}}]}

data: {"choices":[{"delta":{"content":" "}}]}

data: {"choices":[{"delta":{"content":"world"}}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}

data: [DONE]

`
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	req := DefaultRequest("test-model", []Message{{Role: "user", Content: "hi"}}, testEffort)
	sr, err := c.ConsultStream(context.Background(), req)
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
	if chunks < 3 {
		t.Errorf("expected at least 3 chunks; got %d", chunks)
	}
	if got.String() != "Hello world" {
		t.Errorf("concat content: got %q, want %q", got.String(), "Hello world")
	}
	final := sr.Final()
	if final.Content != "Hello world" {
		t.Errorf("Final.Content: got %q, want %q", final.Content, "Hello world")
	}
	if final.FinishReason != "stop" {
		t.Errorf("Final.FinishReason: got %q, want stop (lastChunk=%+v)", final.FinishReason, lastChunk)
	}
	if final.Usage.TotalTokens != 6 {
		t.Errorf("Final.Usage.TotalTokens: got %d, want 6", final.Usage.TotalTokens)
	}

	if gotAccept != "text/event-stream" {
		t.Errorf("Accept header: got %q, want text/event-stream", gotAccept)
	}
	if !strings.Contains(string(rawBody), `"stream":true`) {
		t.Errorf("expected `\"stream\":true` in body: %s", rawBody)
	}
}

// TestHTTPClientStreamErrorWraps: a 5xx during stream initiation is
// surfaced as a wrapped error, with the API key scrubbed.
func TestHTTPClientStreamErrorWraps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("bad: token=" + testAPIKey))
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	_, err := c.ConsultStream(context.Background(), DefaultRequest("m", []Message{{Role: "user", Content: "x"}}, testEffort))
	if err == nil {
		t.Fatal("expected error from 500, got nil")
	}
	if !strings.Contains(err.Error(), "http 500") {
		t.Errorf("error missing status code: %v", err)
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("API key leaked into 5xx error: %v", err)
	}
}

// TestHTTPClientStreamCloseBeforeEOF: closing the reader before exhausting
// it should release the body without leaking. Subsequent Next() returns
// io.EOF.
func TestHTTPClientStreamCloseBeforeEOF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"a"}}]}

data: {"choices":[{"delta":{"content":"b"}}]}

`))
		// Hold the connection so the reader sees only what's been flushed.
		flusher, _ := w.(http.Flusher)
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	sr, err := c.ConsultStream(context.Background(), DefaultRequest("m", []Message{{Role: "user", Content: "x"}}, testEffort))
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	chunk, err := sr.Next()
	if err != nil {
		t.Fatalf("first Next: %v", err)
	}
	if chunk.Content != "a" {
		t.Errorf("first chunk content: got %q, want a", chunk.Content)
	}
	if err := sr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	// Idempotent.
	if err := sr.Close(); err != nil {
		t.Errorf("Close (second): %v", err)
	}
	// After Close, Next returns io.EOF.
	if _, err := sr.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("Next after Close: got %v, want io.EOF", err)
	}
}

// TestHTTPClientStreamCtxCancellation: a cancelled context mid-stream
// should surface via Next().
func TestHTTPClientStreamCtxCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n"))
		flusher, _ := w.(http.Flusher)
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewHTTPClient(newTestEndpoint(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	sr, err := c.ConsultStream(ctx, DefaultRequest("m", []Message{{Role: "user", Content: "x"}}, testEffort))
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	defer sr.Close()
	// Drain the first chunk.
	if _, err := sr.Next(); err != nil {
		t.Fatalf("first Next: %v", err)
	}
	cancel()
	// Subsequent Next must surface the ctx error (either via the ctx
	// check or via the underlying connection failing).
	_, err = sr.Next()
	if err == nil {
		t.Fatal("expected error after cancel, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// openStream opens an SSE stream against handler over the real transport
// and wraps the body in a reader whose idle watchdog is wound to `idle`
// rather than the production streamIdleTimeout — the only way to exercise
// a stall without pinning the suite to the real bound. The returned reader
// owns the body; the caller must Close it (which also releases the request
// context, letting a blocked handler return before the server shuts down).
func openStream(t *testing.T, idle time.Duration, handler http.HandlerFunc) *httpStreamReader {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancelCause(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		cancel(nil)
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel(nil)
		t.Fatalf("open stream: %v", err)
	}
	return newHTTPStreamReader(ctx, cancel, resp.Body, idle)
}

// sseContentDelta is one content-carrying SSE event, terminated by the
// blank separator line.
const sseContentDelta = "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n"

// TestStreamIdleWatchdogAbortsStall: a provider that opens a 2xx stream and
// then goes silent must be abandoned, not waited on. Without the watchdog
// this read blocks until the caller's context or the wall clock intervenes
// — and the caller of a long generation has no useful deadline to give.
func TestStreamIdleWatchdogAbortsStall(t *testing.T) {
	sr := openStream(t, 50*time.Millisecond, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", mimeEventStream)
		_, _ = w.Write([]byte(sseContentDelta))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Go silent. The watchdog's cancellation is the only way out.
		<-r.Context().Done()
	})
	defer sr.Close()

	chunk, err := sr.Next()
	if err != nil {
		t.Fatalf("first Next: %v", err)
	}
	if chunk.Content != "x" {
		t.Errorf("first chunk: got %q, want %q", chunk.Content, "x")
	}
	// The stall names itself rather than surfacing as a bare "context
	// canceled", which would read as the caller's doing.
	if _, err := sr.Next(); !errors.Is(err, errStreamIdle) {
		t.Fatalf("Next on a stalled stream: got %v, want errStreamIdle", err)
	}
	if _, err := sr.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("Next after the stall: got %v, want io.EOF", err)
	}
	if got := sr.Final().Content; got != "x" {
		t.Errorf("Final.Content: got %q, want the delta observed before the stall", got)
	}
}

// TestStreamIdleWatchdogToleratesSlowStream: the watchdog bounds the GAP
// between reads, not the call. A stream whose total duration is well past
// the idle window but which never goes quiet for one must run to [DONE].
// Keepalive comment lines count as liveness — that is how a gateway holds
// a connection open across a long prefill.
func TestStreamIdleWatchdogToleratesSlowStream(t *testing.T) {
	const (
		events      = 20                     // 400ms total: double the idle window
		gap         = 20 * time.Millisecond  // each gap well inside it
		idle        = 200 * time.Millisecond // the window under test
		contentEach = 5                      // every 5th event carries content
	)
	sr := openStream(t, idle, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", mimeEventStream)
		f, _ := w.(http.Flusher)
		for i := 1; i <= events; i++ {
			time.Sleep(gap)
			if i%contentEach == 0 {
				_, _ = w.Write([]byte(sseContentDelta))
			} else {
				_, _ = w.Write([]byte(": ping\n\n"))
			}
			if f != nil {
				f.Flush()
			}
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if f != nil {
			f.Flush()
		}
	})
	defer sr.Close()

	for {
		if _, err := sr.Next(); err != nil {
			if !errors.Is(err, io.EOF) {
				t.Fatalf("slow-but-flowing stream: got %v, want a clean io.EOF", err)
			}
			break
		}
	}
	if got, want := sr.Final().Content, strings.Repeat("x", events/contentEach); got != want {
		t.Errorf("Final.Content: got %q, want %q", got, want)
	}
}

// TestStreamIdleWatchdogArmsAtFirstRead: the window bounds provider silence,
// not caller latency. A caller that does its own work between ConsultStream
// and its first read — building the next call, writing out the last one —
// has kept nobody waiting, and must find the stream intact when it arrives.
func TestStreamIdleWatchdogArmsAtFirstRead(t *testing.T) {
	const idle = 50 * time.Millisecond
	sr := openStream(t, idle, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", mimeEventStream)
		_, _ = w.Write([]byte(sseContentDelta + "data: [DONE]\n\n"))
	})
	defer sr.Close()

	// Well past the window a watchdog armed at construction would have run.
	time.Sleep(4 * idle)

	chunk, err := sr.Next()
	if err != nil {
		t.Fatalf("first Next after a delay: got %v, want the buffered chunk", err)
	}
	if chunk.Content != "x" {
		t.Errorf("first chunk: got %q, want %q", chunk.Content, "x")
	}
	if _, err := sr.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("Next at [DONE]: got %v, want io.EOF", err)
	}
}

// TestStreamDecodeErrorNamesTheDecode: with no stall in play, an
// undecodable payload reports itself. This is the control for the test
// below — terminalErr must not rewrite every failure as a stall.
func TestStreamDecodeErrorNamesTheDecode(t *testing.T) {
	_, _, err := runStream(t, "data: {\"choices\":[\n\n")
	if err == nil || !strings.Contains(err.Error(), "decode stream chunk") {
		t.Fatalf("undecodable payload: got %v, want a decode error", err)
	}
	if errors.Is(err, errStreamIdle) {
		t.Error("a decode failure with no stall must not be reported as one")
	}
}

// truncatingBody trips a callback before handing over its payload — the
// decode-versus-watchdog race made deterministic. It stands in for the real
// sequence: the provider stalls mid-event, the watchdog cancels, and the
// half-event already on the wire is what the scanner hands the decoder.
type truncatingBody struct {
	payload *strings.Reader
	stall   func()
	stalled bool
}

func (b *truncatingBody) Read(p []byte) (int, error) {
	if !b.stalled {
		b.stalled = true
		b.stall()
	}
	return b.payload.Read(p)
}

func (b *truncatingBody) Close() error { return nil }

// TestStreamDecodeErrorUnderStallNamesTheStall: when the watchdog has
// already fired, the decode failure is a symptom of the stall, and the
// caller needs the cause. Reporting the JSON error would send a reader
// looking for a provider protocol bug that is not there.
func TestStreamDecodeErrorUnderStallNamesTheStall(t *testing.T) {
	const idle = time.Hour // long enough that only the scripted stall fires
	ctx, cancel := context.WithCancelCause(context.Background())
	body := &truncatingBody{
		payload: strings.NewReader("data: {\"choices\":[\n\n"),
		stall:   func() { cancel(fmt.Errorf("%w after %v", errStreamIdle, idle)) },
	}
	sr := newHTTPStreamReader(ctx, cancel, body, idle)
	defer sr.Close()

	if _, err := sr.Next(); !errors.Is(err, errStreamIdle) {
		t.Fatalf("decode failure under a fired watchdog: got %v, want errStreamIdle", err)
	}
}
