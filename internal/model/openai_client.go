package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// The OpenAI-compatible chat path, hanging off Endpoint.BaseURL. The catalogue
// path and the media types both surfaces negotiate with are shared, and live in
// transport.go.
const pathChatCompletions = "chat/completions"

// OpenAIClient is a thin OpenAI-compatible chat-completions client.
//
// Construction is intentionally cheap: the underlying http.Client is
// reused across calls so connection pooling works.
type OpenAIClient struct {
	endpoint Endpoint
	http     *http.Client
	// promptCacheKey is the key every request this client sends carries,
	// stamped onto the Request in Consult and ConsultStream. It is the
	// client's alone: a request cannot name a key of its own.
	promptCacheKey string
}

// The constructor returns the concrete type (nil-interface traps stay out
// of the wiring), so the interface it is meant to satisfy is asserted here
// rather than discovered by a caller.
var _ Client = (*OpenAIClient)(nil)

// NewOpenAIClient constructs an OpenAIClient for the given endpoint, always
// speaking the OpenAI surface whatever protocol the endpoint names: tests
// construct the two wire clients directly, and NewClient is what dispatches. A
// zero Endpoint yields a client whose first call produces a clear error.
func NewOpenAIClient(e Endpoint) *OpenAIClient {
	return &OpenAIClient{
		endpoint:       e,
		http:           newHTTPTransport(),
		promptCacheKey: e.PromptCacheKey,
	}
}

// Close releases the client's idle connections. A verb that opened a client
// calls it before it returns, on every outcome.
func (c *OpenAIClient) Close() error {
	return closeIdleConnections(c.http)
}

// Consult performs one chat-completions round-trip.
//
// On any non-2xx response, the body is included in the returned error,
// scrubbed of any Authorization header value the server might have
// echoed back. The APIKey itself is never written into an error string
// constructed by this client.
func (c *OpenAIClient) Consult(ctx context.Context, req Request) (Response, error) {
	if c.endpoint.BaseURL == "" {
		return Response{}, fmt.Errorf("endpoint BaseURL is empty")
	}
	req.PromptCacheKey = c.promptCacheKey

	body, err := encodeRequest(req, false)
	if err != nil {
		return Response{}, fmt.Errorf("encode request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, pathChatCompletions, mimeJSON, body)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	out, err := decodeResponse(respBody)
	if err != nil {
		return Response{}, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}

// doRequest builds a request to <BaseURL>/<path> with the given method,
// Accept header, and optional JSON body (nil for a bodyless GET), sets
// the standard headers — Content-Type when a body is present, plus
// User-Agent and bearer Authorization when an API key is configured —
// and performs it against the shared http.Client.
//
// This is the collapsed shape shared by Consult, ConsultStream, and
// ListModels. Per-call differences (decode targets, and the streaming
// path's response ownership) stay at the call sites.
//
// On a non-2xx response the body is read, the connection closed, and a
// wrapped error returned with the API key scrubbed. On a 2xx response the
// live *http.Response is returned with its Body unread; the caller owns
// reading and closing it (the streaming caller hands the Body off to a
// StreamReader instead of reading it here).
func (c *OpenAIClient) doRequest(ctx context.Context, method, path, accept string, body []byte) (*http.Response, error) {
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
	if c.endpoint.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.endpoint.APIKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		// http.Client.Do already wraps ctx errors usefully; make sure the
		// caller can errors.Is(err, context.Canceled) without our wrapper
		// hiding it.
		return nil, fmt.Errorf("http: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return nil, statusError(resp, c.endpoint.APIKey)
	}
	return resp, nil
}

// ConsultStream performs one streaming chat-completions round-trip,
// returning a StreamReader that yields chunks as they arrive.
//
// Wire format is OpenAI-compatible Server-Sent Events:
//
//	data: {"choices":[{"delta":{"content":"Hello"}}]}
//
//	data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{...}}
//
//	data: [DONE]
//
// On non-2xx the body is read fully, the connection closed, and a wrapped
// error returned (matching Consult's shape, with the API key scrubbed).
// On 2xx the response body is owned by the returned StreamReader; the
// caller MUST Close it.
func (c *OpenAIClient) ConsultStream(ctx context.Context, req Request) (StreamReader, error) {
	if c.endpoint.BaseURL == "" {
		return nil, fmt.Errorf("endpoint BaseURL is empty")
	}
	req.PromptCacheKey = c.promptCacheKey

	body, err := encodeRequest(req, true)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	// The request rides a cancellable child of the caller's context so the
	// reader's idle watchdog has something to pull: cancelling this context
	// is what unblocks a read waiting on bytes that are never coming.
	streamCtx, cancel := context.WithCancelCause(ctx)
	resp, err := c.doRequest(streamCtx, http.MethodPost, pathChatCompletions, mimeEventStream, body)
	if err != nil {
		cancel(nil)
		return nil, err
	}
	// On 2xx the response body is owned by the StreamReader, which the
	// caller must Close; doRequest leaves it unread for exactly this.
	return newOpenAIStreamReader(streamCtx, cancel, resp.Body, streamIdleTimeout), nil
}

// ListModels performs a GET against <base>/models and returns the model
// list in the order the server provided. Errors are wrapped with status +
// body, with the API key scrubbed.
func (c *OpenAIClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	if c.endpoint.BaseURL == "" {
		return nil, fmt.Errorf("endpoint BaseURL is empty")
	}

	resp, err := c.doRequest(ctx, http.MethodGet, pathModels, mimeJSON, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var w wireModelList
	if err := json.Unmarshal(respBody, &w); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}
	out := make([]ModelInfo, 0, len(w.Data))
	for _, m := range w.Data {
		out = append(out, ModelInfo{
			ID:      m.ID,
			Created: m.Created,
			OwnedBy: m.OwnedBy,
		})
	}
	return out, nil
}

// joinURL appends path to base, handling the trailing-slash variation
// gracefully. path must not begin with "/".
func joinURL(base, path string) string {
	if strings.HasSuffix(base, "/") {
		return base + path
	}
	return base + "/" + path
}

// scrubAuthorization removes any occurrence of the bearer-token header
// value (or the bare key) from s. Empty key → no scrubbing.
func scrubAuthorization(s, apiKey string) string {
	const placeholder = "<redacted>"
	if apiKey != "" {
		s = strings.ReplaceAll(s, "Bearer "+apiKey, "Bearer "+placeholder)
		s = strings.ReplaceAll(s, apiKey, placeholder)
	}
	return s
}

// --- wire format (OpenAI chat-completions) ---

type wireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// wireRequest is the chat-completions request body. Temperature is always
// serialized — no `omitempty`: 0 is deterministic decode, not a "use server
// default" sentinel.
type wireRequest struct {
	Model              string             `json:"model"`
	Messages           []wireMessage      `json:"messages"`
	Temperature        float64            `json:"temperature"`
	ChatTemplateKwargs map[string]any     `json:"chat_template_kwargs,omitempty"`
	Reasoning          *Reasoning         `json:"reasoning,omitempty"`
	PromptCacheKey     string             `json:"prompt_cache_key,omitempty"`
	Stream             bool               `json:"stream,omitempty"`
	StreamOptions      *wireStreamOptions `json:"stream_options,omitempty"`
}

// wireStreamOptions is the `stream_options` request block. It is valid
// ONLY on a streaming request — several OpenAI-compatible providers reject
// its presence on a blocking one — so encodeRequest attaches it behind the
// stream flag and the pointer stays nil (field omitted) otherwise.
type wireStreamOptions struct {
	// IncludeUsage asks the provider to emit a final, CHOICES-LESS chunk
	// carrying only `usage`. Without it a streamed response carries NO
	// usage payload at all: every token count comes back zero, which is
	// indistinguishable from a real zero and silently disables anything
	// gated on one — including the token accounting the per-call budget
	// is calibrated from.
	IncludeUsage bool `json:"include_usage"`
}

type wireChoice struct {
	Index        int         `json:"index"`
	Message      wireMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// wireUsage is the `usage` block. The first fields are the OpenAI schema;
// everything below the marker is the de-facto MLX extension (see
// InferenceTelemetry) and is ABSENT on most providers — absence decodes to
// zero, which is the documented, non-erroneous outcome.
//
// `input_tokens` / `output_tokens` are deliberately NOT decoded. MLX-backed
// servers send them as exact duplicates of `prompt_tokens` /
// `completion_tokens`, so capturing them would create a second field per
// count that must always agree with the first — two projections of one
// shape, with no rule for a caller to choose between them if they ever
// disagreed. If a provider ever sends ONLY the input/output spelling, add
// them then, as a documented fallback inside usage().
type wireUsage struct {
	PromptTokens            int                        `json:"prompt_tokens"`
	CompletionTokens        int                        `json:"completion_tokens"`
	TotalTokens             int                        `json:"total_tokens"`
	CompletionTokensDetails *wireCompletionTokenDetail `json:"completion_tokens_details,omitempty"`
	PromptTokensDetails     *wirePromptTokenDetail     `json:"prompt_tokens_details,omitempty"`

	// --- non-standard inference-telemetry extension (MLX-backed servers) ---
	// Durations are FRACTIONAL SECONDS on the wire (0.61, 6.89); the
	// rates are tokens per second.
	TimeToFirstToken          float64 `json:"time_to_first_token"`
	PromptEvalDuration        float64 `json:"prompt_eval_duration"`
	GenerationDuration        float64 `json:"generation_duration"`
	TotalTime                 float64 `json:"total_time"`
	PromptTokensPerSecond     float64 `json:"prompt_tokens_per_second"`
	GenerationTokensPerSecond float64 `json:"generation_tokens_per_second"`
}

// wireCompletionTokenDetail is the OpenAI-compatible breakdown of the
// completion half of the usage block. Only reasoning_tokens is read: it is
// the sole measurement of what thinking mode actually costs, and on a
// reasoning-heavy call it is most of CompletionTokens.
type wireCompletionTokenDetail struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

// wirePromptTokenDetail is the prompt half's breakdown. Only cached_tokens
// is read — the provider's prompt-cache hit count.
type wirePromptTokenDetail struct {
	CachedTokens int `json:"cached_tokens"`
}

// usage projects the wire block onto the caller-facing type. One
// projection shared by the blocking decode and the SSE decode.
func (w wireUsage) usage() Usage {
	u := Usage{
		PromptTokens:     w.PromptTokens,
		CompletionTokens: w.CompletionTokens,
		TotalTokens:      w.TotalTokens,
		Telemetry: InferenceTelemetry{
			TimeToFirstToken:          secondsToDuration(w.TimeToFirstToken),
			PrefillDuration:           secondsToDuration(w.PromptEvalDuration),
			GenerationDuration:        secondsToDuration(w.GenerationDuration),
			TotalDuration:             secondsToDuration(w.TotalTime),
			PrefillTokensPerSecond:    w.PromptTokensPerSecond,
			GenerationTokensPerSecond: w.GenerationTokensPerSecond,
		},
	}
	if w.CompletionTokensDetails != nil {
		u.ReasoningTokens = w.CompletionTokensDetails.ReasoningTokens
	}
	if w.PromptTokensDetails != nil {
		u.CachedPromptTokens = w.PromptTokensDetails.CachedTokens
	}
	return u
}

// secondsToDuration converts a wire figure expressed in FRACTIONAL SECONDS
// to a time.Duration. Rounded, not truncated: 6.89 has no exact binary
// float representation, and truncation would render it 6.889999999s.
func secondsToDuration(sec float64) time.Duration {
	return time.Duration(math.Round(sec * float64(time.Second)))
}

type wireResponse struct {
	Choices []wireChoice `json:"choices"`
	Usage   wireUsage    `json:"usage"`
}

type wireModelList struct {
	Object string      `json:"object"`
	Data   []wireModel `json:"data"`
}

type wireModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// encodeRequest serializes a Request to the OpenAI chat-completions wire
// format. stream toggles the `stream` field AND attaches
// `stream_options.include_usage` (see wireStreamOptions — usage is not
// reported on a streamed response without it, and the field is a protocol
// error on a blocking one); the encoding is otherwise identical across the
// blocking and streaming paths. `prompt_cache_key` is written where the
// request carries one and omitted otherwise: a server that rejects the field
// must not receive an empty one from a caller sending no key.
func encodeRequest(req Request, stream bool) ([]byte, error) {
	wr := wireRequest{
		Model:              req.Model,
		Messages:           make([]wireMessage, 0, len(req.Messages)),
		Temperature:        req.Temperature,
		ChatTemplateKwargs: req.ChatTemplateKwargs,
		Reasoning:          req.Reasoning,
		PromptCacheKey:     req.PromptCacheKey,
		Stream:             stream,
	}
	if stream {
		wr.StreamOptions = &wireStreamOptions{IncludeUsage: true}
	}
	for _, m := range req.Messages {
		wr.Messages = append(wr.Messages, wireMessage{Role: m.Role, Content: m.Content})
	}
	return json.Marshal(wr)
}

func decodeResponse(body []byte) (Response, error) {
	var w wireResponse
	if err := json.Unmarshal(body, &w); err != nil {
		return Response{}, err
	}
	if len(w.Choices) == 0 {
		return Response{}, fmt.Errorf("response contains no choices")
	}
	choice := w.Choices[0]
	return Response{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Usage:        w.Usage.usage(),
	}, nil
}
