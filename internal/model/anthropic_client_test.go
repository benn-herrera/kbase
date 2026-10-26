package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// newAnthropicTestEndpoint is the shared test endpoint, speaking the Messages
// protocol.
func newAnthropicTestEndpoint(baseURL string) Endpoint {
	ep := newTestEndpoint(baseURL)
	ep.Protocol = ProtocolAnthropic
	return ep
}

// anthropicMessagesBody is a blocking Messages response: one thinking block
// (scratch that must never join the reply), one text block, the stop reason and
// the usage block. cache_creation_input_tokens is sent because a real server
// does, and a client that decodes it into nowhere would be reading a field it
// has no home for.
func anthropicMessagesBody(text, stopReason string, input, output, cached int) string {
	return fmt.Sprintf(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-x",
		"content":[{"type":"thinking","thinking":"scratch"},{"type":"text","text":%q}],
		"stop_reason":%q,
		"usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":3}}`,
		text, stopReason, input, output, cached)
}

// anthropicStreamSSE is one complete Messages event sequence: usage on
// message_start, a ping, a thinking delta, two text deltas, the stop reason and
// output tokens on message_delta, and the closing message_stop.
const anthropicStreamSSE = `event: message_start
data: {"type":"message_start","message":{"id":"msg_1","role":"assistant","usage":{"input_tokens":25,"cache_read_input_tokens":7,"cache_creation_input_tokens":3}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}

event: ping
data: {"type":"ping"}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"pondering"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":9}}

event: message_stop
data: {"type":"message_stop"}

`

// drainAnthropicStream serves body as the Messages event sequence, iterates it
// to its end, and returns the chunks observed, the accumulated Final and the
// error that ended the iteration — nil when message_stop closed it. It also
// proves the request asked for a stream.
func drainAnthropicStream(t *testing.T, body string) ([]Chunk, Response, error) {
	t.Helper()
	var raw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", mimeEventStream)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewAnthropicClient(newAnthropicTestEndpoint(srv.URL))
	sr, err := c.ConsultStream(context.Background(), chatRequest("claude-x"))
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	defer sr.Close()

	var chunks []Chunk
	var termErr error
	for {
		chunk, err := sr.Next()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				termErr = err
			}
			break
		}
		chunks = append(chunks, chunk)
	}
	if !strings.Contains(string(raw), `"stream":true`) {
		t.Errorf("the request did not carry `\"stream\":true`: %s", raw)
	}
	return chunks, sr.Final(), termErr
}

// TestAnthropicConsultSendsTheMessagesRequest: the blocking call is one POST to
// <base>/messages with the protocol's headers, the required max_tokens
// estimated from the ask's characters, the system turn lifted to the top-level
// string, and none of the OpenAI surface's fields.
func TestAnthropicConsultSendsTheMessagesRequest(t *testing.T) {
	for _, tc := range []struct {
		name    string
		system  string
		prompt  string
		wantMax int
	}{
		{"a short ask takes the floor", "be brief", "hi", maxTokensFloor},
		{"exactly the floor's worth of characters", "", strings.Repeat("x", maxTokensFloor*4), maxTokensFloor},
		{"a long ask scales with its characters", "be brief", strings.Repeat("y", 4040), 1012},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			var headers http.Header
			var raw []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath, headers = r.URL.Path, r.Header.Clone()
				raw, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(anthropicMessagesBody("ok", "end_turn", 11, 2, 0)))
			}))
			defer srv.Close()

			c := NewAnthropicClient(newAnthropicTestEndpoint(srv.URL))
			_, err := c.Consult(context.Background(), Request{
				Model:              "claude-x",
				Messages:           []Message{{Role: "system", Content: tc.system}, {Role: "user", Content: tc.prompt}},
				Temperature:        0,
				ChatTemplateKwargs: map[string]any{"enable_thinking": false},
				Reasoning:          &Reasoning{Enabled: false},
				PromptCacheKey:     "run-abcdefgh",
			})
			if err != nil {
				t.Fatalf("Consult: %v", err)
			}

			if gotPath != "/messages" {
				t.Errorf("path: got %q, want /messages", gotPath)
			}
			for _, want := range []struct{ header, value string }{
				{"x-api-key", testAPIKey},
				{headerAnthropicVer, anthropicVersion},
				{"Content-Type", mimeJSON},
				{"User-Agent", userAgent},
			} {
				if got := headers.Get(want.header); got != want.value {
					t.Errorf("%s: got %q, want %q", want.header, got, want.value)
				}
			}
			if got := headers.Get("Authorization"); got != "" {
				t.Errorf("Authorization sent to a Messages endpoint: %q", got)
			}

			var wire map[string]json.RawMessage
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatalf("the request is no JSON body: %v\n%s", err, raw)
			}
			var maxTokens int
			if err := json.Unmarshal(wire["max_tokens"], &maxTokens); err != nil {
				t.Fatalf("max_tokens is absent or not a number: %s", raw)
			}
			if maxTokens != tc.wantMax {
				t.Errorf("max_tokens = %d, want %d (the ask is %d characters)", maxTokens, tc.wantMax, len(tc.system)+len(tc.prompt))
			}
			var system string
			_ = json.Unmarshal(wire["system"], &system)
			if system != tc.system {
				t.Errorf("system = %q, want the lifted system turn %q", system, tc.system)
			}
			if tc.system == "" {
				if _, ok := wire["system"]; ok {
					t.Errorf("an empty system text was sent as a field: %s", raw)
				}
			}
			var messages []wireMessage
			if err := json.Unmarshal(wire["messages"], &messages); err != nil {
				t.Fatalf("messages: %v", err)
			}
			if len(messages) != 1 || messages[0].Role != "user" || messages[0].Content != tc.prompt {
				t.Errorf("messages = %+v, want the user turn alone", messages)
			}
			// The OpenAI surface's fields have no Messages equivalent.
			for _, field := range []string{"prompt_cache_key", "chat_template_kwargs", "reasoning", "stream_options"} {
				if _, ok := wire[field]; ok {
					t.Errorf("%s sent to a Messages endpoint: %s", field, raw)
				}
			}
			if v, ok := wire["stream"]; ok && string(v) != "false" {
				t.Errorf("stream: got %s, want absent or false", v)
			}
			// Thinking is disabled explicitly: omitting the field leaves it
			// at the server's default.
			if v := wire["thinking"]; string(v) != `{"type":"disabled"}` {
				t.Errorf("thinking = %s, want {\"type\":\"disabled\"}", v)
			}
			// Temperature 0 is inside the protocol's range, so it is sent.
			if v, ok := wire["temperature"]; !ok || string(v) != "0" {
				t.Errorf("temperature = %s (present %t), want 0", v, ok)
			}
		})
	}
}

// TestAnthropicTemperatureOutsideTheRangeIsOmitted: the Messages protocol
// accepts a temperature only in [0,1], so a value outside it is left unsent
// rather than clamped — the second flavour's rule, Request.Temperature being
// always wired by the OpenAI client.
func TestAnthropicTemperatureOutsideTheRangeIsOmitted(t *testing.T) {
	for _, tc := range []struct {
		name        string
		temperature float64
		wantSent    bool
	}{
		{"zero", 0, true},
		{"one", 1, true},
		{"above one", 1.5, false},
		{"below zero", -0.5, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(anthropicMessagesBody("ok", "end_turn", 1, 1, 0)))
			}))
			defer srv.Close()

			c := NewAnthropicClient(newAnthropicTestEndpoint(srv.URL))
			req := chatRequest("claude-x")
			req.Temperature = tc.temperature
			if _, err := c.Consult(context.Background(), req); err != nil {
				t.Fatalf("Consult: %v", err)
			}
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if _, ok := wire["temperature"]; ok != tc.wantSent {
				t.Errorf("temperature %v sent = %t, want %t: %s", tc.temperature, ok, tc.wantSent, raw)
			}
		})
	}
}

// TestAnthropicConsultDecodesTheMessage: the text blocks are the reply, a
// thinking block never joins them, and the usage block lands on the caller's
// token counts.
func TestAnthropicConsultDecodesTheMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(anthropicMessagesBody("hello world", "end_turn", 11, 4, 3)))
	}))
	defer srv.Close()

	c := NewAnthropicClient(newAnthropicTestEndpoint(srv.URL))
	resp, err := c.Consult(context.Background(), chatRequest("claude-x"))
	if err != nil {
		t.Fatalf("Consult: %v", err)
	}
	if resp.Content != "hello world" {
		t.Errorf("content: got %q, want the text block alone", resp.Content)
	}
	if resp.FinishReason != "end_turn" {
		t.Errorf("finish reason: got %q, want the protocol's own end_turn", resp.FinishReason)
	}
	if want := (Usage{PromptTokens: 11, CompletionTokens: 4, CachedPromptTokens: 3}); resp.Usage != want {
		t.Errorf("usage = %+v, want %+v", resp.Usage, want)
	}
	if resp.Truncated() {
		t.Error("a call that ended on end_turn reports truncation")
	}
}

// TestAnthropicStopReasonMapsTheCap: `max_tokens` is the one stop reason
// translated — to FinishLength, so Response.Truncated() says the answer is
// incomplete — on the blocking and the streamed path alike; `end_turn` stays
// the protocol's own word and is a clean finish.
func TestAnthropicStopReasonMapsTheCap(t *testing.T) {
	for _, tc := range []struct {
		stopReason    string
		want          string
		wantTruncated bool
	}{
		{"max_tokens", FinishLength, true},
		{"end_turn", "end_turn", false},
	} {
		t.Run(tc.stopReason, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(anthropicMessagesBody("cut", tc.stopReason, 5, 5, 0)))
			}))
			defer srv.Close()

			c := NewAnthropicClient(newAnthropicTestEndpoint(srv.URL))
			resp, err := c.Consult(context.Background(), chatRequest("claude-x"))
			if err != nil {
				t.Fatalf("Consult: %v", err)
			}
			if resp.FinishReason != tc.want || resp.Truncated() != tc.wantTruncated {
				t.Errorf("blocking: finish reason %q, truncated %t; want %q, %t",
					resp.FinishReason, resp.Truncated(), tc.want, tc.wantTruncated)
			}

			streamed, final, err := drainAnthropicStream(t, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":5}}}\n\n"+
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\""+tc.stopReason+"\"},\"usage\":{\"output_tokens\":5}}\n\n"+
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
			if err != nil {
				t.Fatalf("stream: %v", err)
			}
			if len(streamed) != 2 || streamed[1].FinishReason != tc.want {
				t.Errorf("streamed chunks = %+v, want the stop reason on the message_delta chunk", streamed)
			}
			if final.FinishReason != tc.want || final.Truncated() != tc.wantTruncated {
				t.Errorf("streamed: finish reason %q, truncated %t; want %q, %t",
					final.FinishReason, final.Truncated(), tc.want, tc.wantTruncated)
			}
		})
	}
}

// TestAnthropicStreamDecodesTheEventSequence: the deltas arrive in order,
// thinking deltas land in Reasoning and never in Content, the closing
// message_stop sets StreamDone, and the usage carried by message_start and
// message_delta is reported as one block.
func TestAnthropicStreamDecodesTheEventSequence(t *testing.T) {
	chunks, final, err := drainAnthropicStream(t, anthropicStreamSSE)
	if err != nil {
		t.Fatalf("stream: %v, want a clean end at message_stop", err)
	}

	var content, reasoning []string
	for _, chunk := range chunks {
		if chunk.Content != "" {
			content = append(content, chunk.Content)
		}
		if chunk.Reasoning != "" {
			reasoning = append(reasoning, chunk.Reasoning)
		}
		if strings.Contains(chunk.Content, "pondering") {
			t.Errorf("thinking text reached Content: %+v", chunk)
		}
	}
	if want := []string{"Hello", " world"}; !slices.Equal(content, want) {
		t.Errorf("content deltas = %q, want %q", content, want)
	}
	if want := []string{"pondering"}; !slices.Equal(reasoning, want) {
		t.Errorf("reasoning deltas = %q, want %q", reasoning, want)
	}

	if final.Content != "Hello world" {
		t.Errorf("Final.Content: got %q, want the text deltas concatenated", final.Content)
	}
	if final.FinishReason != "end_turn" {
		t.Errorf("Final.FinishReason: got %q, want end_turn", final.FinishReason)
	}
	if !final.StreamDone {
		t.Error("message_stop did not mark the stream done")
	}
	if !final.UsageReported {
		t.Error("usage was not reported, though message_start and message_delta carried it")
	}
	if want := (Usage{PromptTokens: 25, CompletionTokens: 9, CachedPromptTokens: 7}); final.Usage != want {
		t.Errorf("Final.Usage = %+v, want %+v", final.Usage, want)
	}
}

// TestAnthropicStreamEndingBeforeMessageStopFails: message_stop is what says
// the message finished, so a body that ends without it did not complete — an
// error, not a short reply.
func TestAnthropicStreamEndingBeforeMessageStopFails(t *testing.T) {
	cut := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":9}}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"half\"}}\n\n"

	chunks, final, err := drainAnthropicStream(t, cut)
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("a stream cut before message_stop ended cleanly: %v", err)
	}
	if !strings.Contains(err.Error(), "message_stop") {
		t.Errorf("error = %q, want it to name the missing closing event", err)
	}
	if final.StreamDone {
		t.Error("Final.StreamDone is true for a stream that never reached message_stop")
	}
	if final.Content != "half" {
		t.Errorf("Final.Content = %q, want the delta that did arrive", final.Content)
	}
	if len(chunks) == 0 {
		t.Error("no chunk was delivered before the cut")
	}
}

// TestAnthropicStreamCarriesAProviderError: an `error` event is a failure
// carrying the event's type and message — and the key is scrubbed out of it
// even where a server echoes one back.
func TestAnthropicStreamCarriesAProviderError(t *testing.T) {
	sse := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":9}}}\n\n" +
		`event: error
data: {"type":"error","error":{"type":"overloaded_error","message":"try later, key was ` + testAPIKey + `"}}

`
	_, _, err := drainAnthropicStream(t, sse)
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("an error event ended the stream cleanly: %v", err)
	}
	for _, want := range []string{"overloaded_error", "try later"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to carry %q", err, want)
		}
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("API key leaked into a stream error: %v", err)
	}
}

// TestAnthropicStatusesClassifyThroughChat: a non-2xx response is the same
// StatusError the OpenAI client returns, so chat.go's handling is unchanged —
// a 4xx that is not 429 ends the call on the one attempt, and 429/5xx are
// re-issued up to ChatAttempts.
func TestAnthropicStatusesClassifyThroughChat(t *testing.T) {
	for _, tc := range []struct {
		code     int
		requests int
	}{
		{http.StatusBadRequest, 1},
		{http.StatusUnauthorized, 1},
		{http.StatusTooManyRequests, ChatAttempts},
		{http.StatusServiceUnavailable, ChatAttempts},
	} {
		t.Run(fmt.Sprintf("%d", tc.code), func(t *testing.T) {
			// The error type itself is what classification reads.
			typeSrv, _ := statusServer(t, tc.code)
			if _, err := NewClient(newAnthropicTestEndpoint(typeSrv.URL)).ConsultStream(context.Background(), chatRequest("claude-x")); !errors.As(err, new(StatusError)) {
				t.Fatalf("ConsultStream on %d = %v, want a StatusError", tc.code, err)
			}

			srv, requests := statusServer(t, tc.code)
			client := NewClient(newAnthropicTestEndpoint(srv.URL))

			got, err := Chat(context.Background(), client, "claude-x", "the system", "the prompt", nil, nil)
			if !errors.Is(err, ErrChatIncomplete) {
				t.Fatalf("Chat = %v, want an incomplete call", err)
			}
			if got.Outcome != ChatProviderRefused {
				t.Errorf("outcome = %q, want %q", got.Outcome, ChatProviderRefused)
			}
			if want := fmt.Sprintf("http %d", tc.code); !strings.Contains(got.Error, want) || !strings.Contains(err.Error(), want) {
				t.Errorf("reasons %q / %q, want both to name %q", got.Error, err, want)
			}
			if n := requests(); n != tc.requests {
				t.Errorf("%d requests, want %d", n, tc.requests)
			}
		})
	}
}

// TestAnthropicListModels: the catalogue is the same GET path under this
// protocol, and its `created_at` is RFC 3339 — not the unix seconds the
// OpenAI-compatible catalogue sends.
func TestAnthropicListModels(t *testing.T) {
	var gotPath string
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, headers = r.URL.Path, r.Header.Clone()
		_, _ = w.Write([]byte(`{"data":[
			{"id":"claude-a","type":"model","display_name":"A","created_at":"2025-02-19T00:00:00Z"},
			{"id":"claude-b","type":"model","created_at":"not a timestamp"}
		],"has_more":false}`))
	}))
	defer srv.Close()

	c := NewAnthropicClient(newAnthropicTestEndpoint(srv.URL))
	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if gotPath != "/models" {
		t.Errorf("path: got %q, want /models", gotPath)
	}
	if got := headers.Get("x-api-key"); got != testAPIKey {
		t.Errorf("x-api-key: got %q, want the configured key", got)
	}
	want := []ModelInfo{
		{ID: "claude-a", Created: time.Date(2025, 2, 19, 0, 0, 0, 0, time.UTC).Unix()},
		{ID: "claude-b"},
	}
	if !slices.Equal(models, want) {
		t.Errorf("models = %+v, want %+v", models, want)
	}
}

// TestAnthropicDecisionRefuses: the decision surfaces are OpenAI-shaped, so a
// Messages endpoint asked for a letter ask refuses — naming the provider and the
// role rather than answering in a shape this protocol never spoke.
func TestAnthropicDecisionRefuses(t *testing.T) {
	c := NewAnthropicClient(Endpoint{Name: "claude", BaseURL: "http://claude.example/v1", Protocol: ProtocolAnthropic})
	_, err := c.Decision(context.Background(), DecisionRequest{
		Model:     "claude-x",
		State:     "state",
		Questions: []DecisionQuestion{{Key: "q1", Type: "noul"}},
	})
	if err == nil {
		t.Fatal("a Messages endpoint answered a decision ask")
	}
	for _, want := range []string{`provider "claude"`, "decision role", "Messages"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal = %q, want it to name %q", err, want)
		}
	}
}
