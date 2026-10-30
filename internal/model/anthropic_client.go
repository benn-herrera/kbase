package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// The Anthropic Messages wire surface: the path chat hangs off
// Endpoint.BaseURL at (the catalogue reuses pathModels), the version header the
// API requires, and the delta kinds this client reads.
const (
	pathMessages       = "messages"
	headerAnthropicVer = "anthropic-version"
	anthropicVersion   = "2023-06-01"

	// The delta kinds: the protocol mixes several kinds of block into one
	// response, and only these two carry anything a caller of this package
	// reads.
	deltaText     = "text_delta"
	deltaThinking = "thinking_delta"

	// The event names this client acts on. Events it does not name —
	// content_block_start, content_block_stop, ping, and anything the
	// protocol adds later — carry no text and no decision, so the reader
	// skips them.
	eventMessageStart      = "message_start"
	eventContentBlockDelta = "content_block_delta"
	eventMessageDelta      = "message_delta"
	eventMessageStop       = "message_stop"
	eventError             = "error"
)

// maxTokensFloor is the smallest completion cap a Messages request carries.
// The protocol REQUIRES max_tokens — no other request this package sends is
// capped — and kbase has no figure of its own to offer, so the cap is
// estimated from the ask itself (maxTokensFor). The floor keeps a one-word ask
// from being cut off by an estimate that rounds down to a handful of tokens.
const maxTokensFloor = 256

// The thinking budget a Messages request grants each level, in tokens, sent as
// `thinking.budget_tokens` and added to the answer's own allowance in
// `max_tokens`, which the budget is spent from.
const (
	thinkingBudgetLow    = 1024
	thinkingBudgetMedium = 4096
	thinkingBudgetHigh   = 16384
)

// AnthropicClient is a thin client for the Anthropic Messages protocol.
//
// Construction is intentionally cheap, as OpenAIClient's is: the underlying
// http.Client is reused across calls so connection pooling works.
type AnthropicClient struct {
	endpoint Endpoint
	http     *http.Client
}

var _ Client = (*AnthropicClient)(nil)

// NewAnthropicClient constructs an AnthropicClient for e. A zero Endpoint
// yields a client whose first call produces a clear error.
func NewAnthropicClient(e Endpoint) *AnthropicClient {
	e.Sampling = e.Sampling.Resolved()
	return &AnthropicClient{endpoint: e, http: newHTTPTransport(e.IdleConns)}
}

func (c *AnthropicClient) sampling() Sampling { return c.endpoint.Sampling }

// Close releases the client's idle connections, as OpenAIClient.Close does.
func (c *AnthropicClient) Close() error {
	return closeIdleConnections(c.http)
}

// consultStream performs one streaming Messages round-trip, returning a
// StreamReader that yields chunks as they arrive.
//
// The wire format is that protocol's named event sequence:
//
//	event: message_start
//	data: {"type":"message_start","message":{...,"usage":{...}}}
//
//	event: content_block_delta
//	data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}
//
//	event: message_delta
//	data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}
//
//	event: message_stop
//	data: {"type":"message_stop"}
//
// On non-2xx the body is read, the connection closed, and the same
// StatusError the OpenAI client returns, so a caller's 429/5xx classification
// holds on both protocols. On 2xx the response body is owned by the returned
// StreamReader; the caller MUST Close it.
func (c *AnthropicClient) consultStream(ctx context.Context, req Request) (StreamReader, error) {
	if c.endpoint.BaseURL == "" {
		return nil, fmt.Errorf("endpoint BaseURL is empty")
	}

	body, err := encodeAnthropicRequest(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	// The request rides a cancellable child of the caller's context so the
	// reader's idle watchdog has something to pull: cancelling this context
	// is what unblocks a read waiting on bytes that are never coming.
	streamCtx, cancel := context.WithCancelCause(ctx)
	resp, err := c.do(streamCtx, http.MethodPost, pathMessages, mimeEventStream, body)
	if err != nil {
		cancel(nil)
		return nil, err
	}
	// On 2xx the response body is owned by the StreamReader, which the
	// caller must Close; do leaves it unread for exactly this. The key goes
	// to the reader only so an error event's message can be scrubbed before
	// it becomes an error string.
	return newAnthropicStreamReader(streamCtx, cancel, resp.Body, streamIdleTimeout, c.endpoint.APIKey), nil
}

// ListModels performs a GET against <base>/models and returns the model list in
// the order the server provided.
func (c *AnthropicClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	if c.endpoint.BaseURL == "" {
		return nil, fmt.Errorf("endpoint BaseURL is empty")
	}

	resp, err := c.do(ctx, http.MethodGet, pathModels, mimeJSON, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var w wireAnthropicModelList
	if err := json.Unmarshal(respBody, &w); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}
	out := make([]ModelInfo, 0, len(w.Data))
	for _, m := range w.Data {
		out = append(out, ModelInfo{ID: m.ID, Created: m.createdUnix()})
	}
	return out, nil
}

// decision refuses: the Messages protocol has no decision surface. The registry
// gives the decision role an OpenAIClient whatever its entry's api, so nothing
// in kbase reaches this.
func (c *AnthropicClient) decision(ctx context.Context, req DecisionRequest) (DecisionResponse, error) {
	name := c.endpoint.Name
	if name == "" {
		name = c.endpoint.BaseURL
	}
	return DecisionResponse{}, fmt.Errorf("provider %q speaks the Anthropic Messages protocol, which has no decision surface",
		name)
}

// do builds a request to <BaseURL>/<path>, sets the protocol's headers and
// performs it. It is the Messages counterpart of OpenAIClient.doRequest: the
// authentication header is the one thing the two surfaces do not share — this
// one takes the key in `x-api-key` and names its protocol version — while the
// status handling below it is the same code, so a non-2xx is the same
// StatusError and a retryable status keeps its meaning.
//
// On a 2xx response the live *http.Response is returned with its Body unread;
// the caller owns reading and closing it.
func (c *AnthropicClient) do(ctx context.Context, method, path, accept string, body []byte) (*http.Response, error) {
	url := joinURL(c.endpoint.BaseURL, path)
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", mimeJSON)
	}
	httpReq.Header.Set("Accept", accept)
	httpReq.Header.Set("User-Agent", userAgent)
	httpReq.Header.Set(headerAnthropicVer, anthropicVersion)
	if c.endpoint.APIKey != "" {
		httpReq.Header.Set("x-api-key", c.endpoint.APIKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		// As doRequest: keep ctx errors reachable through errors.Is.
		return nil, fmt.Errorf("http: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return nil, statusError(resp, c.endpoint.APIKey)
	}
	return resp, nil
}

// --- wire format (Anthropic Messages) ---

type wireAnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// wireAnthropicRequest is the Messages request body. max_tokens is required by
// the protocol; system is the lifted system text and is omitted where there is
// none.
type wireAnthropicRequest struct {
	Model       string                 `json:"model"`
	MaxTokens   int                    `json:"max_tokens"`
	System      string                 `json:"system,omitempty"`
	Messages    []wireAnthropicMessage `json:"messages"`
	Temperature float64                `json:"temperature"`
	Stream      bool                   `json:"stream,omitempty"`
	Thinking    wireAnthropicThinking  `json:"thinking"`
}

// wireAnthropicThinking is the protocol's thinking control: type "disabled",
// or "enabled" with the tokens thinking may spend.
type wireAnthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// encodeAnthropicRequest serializes a Request to the Messages wire format.
//
// Three departures from the OpenAI body this package also speaks, each a fact
// about THIS protocol:
//
//   - `max_tokens` is required: the answer's allowance, estimated from the ask
//     (maxTokensFor), plus the thinking budget where thinking is on.
//   - A system message is not a turn: the protocol takes the system prompt as
//     the top-level `system` string, so system turns are lifted there in order
//     — joined as paragraphs — and every other turn keeps its role in
//     `messages`.
//   - Thinking is the protocol's own `thinking` field: `{"type":"disabled"}`
//     for off, since the field's absence would leave it at the server's
//     default, which is not off on every server behind this protocol; and
//     `{"type":"enabled","budget_tokens":B}` for a level.
//
// `temperature` is sent on every request, thinking or not. Anthropic's own
// endpoint rejects it beside a thinking level, and a server that does fails the
// call like any other request it refuses. Nothing else from the Request is
// sent: `prompt_cache_key`, `chat_template_kwargs`, `reasoning` and
// `reasoning_effort` are OpenAI-surface fields with no Messages equivalent.
func encodeAnthropicRequest(req Request) ([]byte, error) {
	wr := wireAnthropicRequest{
		Model:       req.Model,
		MaxTokens:   maxTokensFor(req),
		Temperature: *req.Sampling.Temperature,
		Stream:      true,
		Thinking:    wireAnthropicThinking{Type: "disabled"},
		// An empty array, never null: the protocol requires the list, and a
		// request carrying no turns at all should be refused by the server
		// as an empty conversation rather than as a malformed body.
		Messages: make([]wireAnthropicMessage, 0, len(req.Messages)),
	}
	if budget := thinkingBudget(*req.Sampling.Thinking); budget > 0 {
		wr.Thinking = wireAnthropicThinking{Type: "enabled", BudgetTokens: budget}
		wr.MaxTokens += budget
	}
	var system []string
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = append(system, m.Content)
			continue
		}
		wr.Messages = append(wr.Messages, wireAnthropicMessage{Role: m.Role, Content: m.Content})
	}
	wr.System = strings.Join(system, "\n\n")
	return json.Marshal(wr)
}

// maxTokensFor is the completion cap a Messages request carries: four
// characters per token over every character the request sends — each message's
// content, the system text included — and never below maxTokensFloor.
//
// Four characters per token is the usual English-prose estimate, and prose is
// what kbase sends: no LaTeX or other dense notation reaches a chat ask. It is
// an estimate, not a budget — the OpenAI surface caps nothing because its
// servers carry their own default, and this protocol has no default to fall
// back on.
func maxTokensFor(req Request) int {
	chars := 0
	for _, m := range req.Messages {
		chars += utf8.RuneCountInString(m.Content)
	}
	if n := (chars + 3) / 4; n > maxTokensFloor {
		return n
	}
	return maxTokensFloor
}

// thinkingBudget is the tokens a Messages request grants thinking at level t:
// none where thinking is off.
func thinkingBudget(t Thinking) int {
	switch t {
	case ThinkingLow:
		return thinkingBudgetLow
	case ThinkingMedium:
		return thinkingBudgetMedium
	case ThinkingHigh:
		return thinkingBudgetHigh
	}
	return 0
}

// wireAnthropicUsage is the usage block a message_start event carries. A message_delta event carries output_tokens
// alone, so a streamed call fills this from two events.
type wireAnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	// CacheReadInputTokens is the tokens the provider read back from its
	// prompt cache. `cache_creation_input_tokens` — the tokens written to it
	// — is deliberately not decoded: Usage has no field for cache writes and
	// no caller reads one, so a decoded figure would have nowhere to go.
	CacheReadInputTokens int `json:"cache_read_input_tokens"`
}

// usage projects the block onto the caller-facing type. TotalTokens stays
// zero: this protocol reports no total, and summing the two halves here would
// be a second answer to a question the provider did not answer.
func (w wireAnthropicUsage) usage() Usage {
	return Usage{
		PromptTokens:       w.InputTokens,
		CompletionTokens:   w.OutputTokens,
		CachedPromptTokens: w.CacheReadInputTokens,
	}
}

// finishReason is a Messages stop_reason as the caller-facing FinishReason.
// Only `max_tokens` is translated: it is the one reason a caller acts on
// (Response.Truncated), and the protocol words it under a different name than
// FinishLength. Every other reason stays the provider's own — `end_turn` is a
// clean finish — because normalizing them would claim a precision the protocol
// does not have.
func finishReason(stopReason string) string {
	if stopReason == "max_tokens" {
		return FinishLength
	}
	return stopReason
}

// wireAnthropicModelList is the catalogue. `created_at` is RFC 3339 here, not
// the unix seconds the OpenAI-compatible catalogue sends; `display_name` and
// `type` have no caller-facing field, so neither is read.
type wireAnthropicModelList struct {
	Data []wireAnthropicModel `json:"data"`
}

type wireAnthropicModel struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
}

// createdUnix is the entry's creation as the unix seconds ModelInfo carries. An
// absent or unparsable stamp is 0, the documented "the provider didn't supply
// one": a catalogue's timestamp is decoration, and a malformed one is no reason
// to refuse a model list that is otherwise usable.
func (m wireAnthropicModel) createdUnix() int64 {
	t, err := time.Parse(time.RFC3339, m.CreatedAt)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// --- Messages event stream decoding ---

// wireAnthropicEvent is one `data:` payload of an event stream. One struct
// serves every event type: the fields are independently guarded and what an
// event IS is its own `type`.
type wireAnthropicEvent struct {
	Type    string                   `json:"type"`
	Message wireAnthropicMessageRoot `json:"message"`
	Delta   wireAnthropicDelta       `json:"delta"`
	Usage   wireAnthropicUsage       `json:"usage"`
	Error   wireAnthropicFault       `json:"error"`
}

// wireAnthropicMessageRoot carries the only part of a message_start's message
// this client reads.
type wireAnthropicMessageRoot struct {
	Usage wireAnthropicUsage `json:"usage"`
}

// wireAnthropicDelta is a content_block_delta's delta (kind plus its text) and
// a message_delta's delta (the stop reason) under the one struct the two
// events share.
type wireAnthropicDelta struct {
	Type       string `json:"type"`
	Text       string `json:"text"`
	Thinking   string `json:"thinking"`
	StopReason string `json:"stop_reason"`
}

type wireAnthropicFault struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// anthropicStreamReader is the Messages StreamReader: it decodes the event
// sequence onto the same Chunk stream the OpenAI reader produces, so chat.go
// reads one shape whichever protocol answered.
type anthropicStreamReader struct {
	sseScanner

	// apiKey is carried only to scrub an error event's message; this reader
	// sends nothing.
	apiKey string

	content      strings.Builder
	finishReason string
	usage        wireAnthropicUsage
	// usageReported is whether a usage block arrived at all: a streamed Usage
	// can be all zero, and only this flag says the provider reported one.
	usageReported bool
	// sawStop is set when message_stop arrived: it is this protocol's closing
	// event, the `[DONE]` of the Messages surface.
	sawStop bool
}

func newAnthropicStreamReader(ctx context.Context, cancel context.CancelCauseFunc, body io.ReadCloser, idle time.Duration, apiKey string) *anthropicStreamReader {
	return &anthropicStreamReader{sseScanner: newSSEScanner(ctx, cancel, body, idle), apiKey: apiKey}
}

// Next returns the next chunk in the stream, or io.EOF when the message is
// complete. A body that ends before message_stop is an error: the closing event
// is what says the message finished, so content arriving first says nothing
// about whether the rest did.
func (r *anthropicStreamReader) Next() (Chunk, error) {
	if r.done {
		return Chunk{}, io.EOF
	}
	for {
		payload, err := r.nextData()
		if err != nil {
			if errors.Is(err, io.EOF) {
				r.finish()
				return Chunk{}, fmt.Errorf("stream ended before message_stop")
			}
			return Chunk{}, err
		}
		var ev wireAnthropicEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			r.finish()
			// Through terminalErr as every other failure is: a payload that
			// will not decode is what a stall truncated mid-event looks like,
			// and the stall is the cause worth reporting.
			return Chunk{}, r.terminalErr(fmt.Errorf("decode stream event: %w", err))
		}
		switch ev.Type {
		case eventError:
			r.finish()
			return Chunk{}, r.terminalErr(fmt.Errorf("stream error %s: %s",
				ev.Error.Type, scrubAuthorization(ev.Error.Message, r.apiKey)))
		case eventMessageStart:
			r.usage = ev.Message.Usage
			r.usageReported = true
			return Chunk{Usage: r.usage.usage(), hasUsage: true}, nil
		case eventContentBlockDelta:
			switch ev.Delta.Type {
			case deltaText:
				r.content.WriteString(ev.Delta.Text)
				return Chunk{Content: ev.Delta.Text}, nil
			case deltaThinking:
				// Thinking never joins Content — see Chunk. It is delivered
				// so a caller can display or discard it, as on the OpenAI
				// surface's reasoning deltas.
				return Chunk{Reasoning: ev.Delta.Thinking}, nil
			}
			// A delta kind this client does not read (a citation, a tool
			// argument): nothing for the caller, so skip it.
		case eventMessageDelta:
			r.usage.OutputTokens = ev.Usage.OutputTokens
			r.usageReported = true
			r.finishReason = finishReason(ev.Delta.StopReason)
			return Chunk{FinishReason: r.finishReason, Usage: r.usage.usage(), hasUsage: true}, nil
		case eventMessageStop:
			r.sawStop = true
			r.finish()
			return Chunk{}, io.EOF
		}
		// content_block_start, content_block_stop, ping and anything the
		// protocol adds later carry no text and no decision: skip them.
	}
}

// Final returns the accumulated Response after iteration; Reasoning is
// delivered per-Chunk only, as on the OpenAI surface (see
// openaiStreamReader.Final).
func (r *anthropicStreamReader) Final() Response {
	return Response{
		Content:       r.content.String(),
		FinishReason:  r.finishReason,
		Usage:         r.usage.usage(),
		StreamDone:    r.sawStop,
		UsageReported: r.usageReported,
	}
}
