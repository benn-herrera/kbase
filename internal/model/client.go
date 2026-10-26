// Package model is the LLM client surface.
//
// Two implementations satisfy the same Client interface: OpenAIClient
// (openai_client.go), targeting any OpenAI-compatible chat-completions endpoint;
// and AnthropicClient (anthropic_client.go), targeting the Anthropic Messages
// protocol. NewClient is what picks between the two, from the protocol the
// caller resolved onto the Endpoint. Both consume the same Request/Response
// shape, so tests and the production pipeline exercise the same code paths. The
// test substrate lives in internal/model/modeltest, out of this package's API.
package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// Client is the abstraction the pipeline depends on for an LLM round-trip.
// Defined here so a test can substitute an implementation of its own without
// depending on the HTTP transport.
//
// Two consult flavors are provided:
//   - Consult: blocking, returns the full Response. Right for short calls.
//   - ConsultStream: returns a StreamReader; callers iterate chunks as they
//     arrive. Right for long generations, whether or not anything renders
//     the deltas — see ConsultDrained.
//
// Both methods consume the same Request and produce equivalent final
// content; ConsultStream additionally exposes per-chunk deltas.
type Client interface {
	Consult(ctx context.Context, req Request) (Response, error)
	ConsultStream(ctx context.Context, req Request) (StreamReader, error)
	Decision(ctx context.Context, req DecisionRequest) (DecisionResponse, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

// Protocol names the chat wire protocol a client speaks. The caller resolves
// the configured `api` value to one of these: this package knows the wire
// shapes, and the configuration owns the vocabulary that names them, so no
// third site compares the string.
type Protocol int

const (
	// ProtocolOpenAI is the OpenAI-compatible chat-completions surface — the
	// zero value, and what an endpoint naming no protocol gets.
	ProtocolOpenAI Protocol = iota
	// ProtocolAnthropic is the Anthropic Messages surface.
	ProtocolAnthropic
)

// Endpoint is what one client is built from: where to reach an inference API,
// which chat protocol it speaks, how to authenticate to it, and the
// prompt-cache key its OpenAI-surface requests carry. The provider fields are
// constructed from the canonical provider config at wiring time, which is what
// keeps this package independent of the config layer; the key names a run, not
// a provider.
//
// APIKey is secret-bearing. It is never logged, never placed in an error
// string by this package, and is scrubbed out of provider responses that
// echo it back (see scrubAuthorization).
type Endpoint struct {
	// Name is the pool entry's name, carried for diagnostics only.
	Name string
	// BaseURL is the API root; the protocol's paths ("chat/completions" and
	// "models", or "messages" and "models") hang off it. A trailing slash is
	// tolerated.
	BaseURL string
	// APIKey is the key. It rides the OpenAI surface as a bearer token and
	// the Messages surface as `x-api-key`. Empty means send no auth header —
	// several local servers reject an empty bearer value.
	APIKey string
	// Protocol is which chat wire protocol this endpoint speaks; the zero
	// value is ProtocolOpenAI. NewClient dispatches on it.
	Protocol Protocol
	// Decisions is whether this server speaks the OpenAI Decisions shape for
	// the letter asks: true for `POST <base-url>/decisions`, false for the
	// SystemOne shape kbase's asks have always used. Chat is unaffected: it
	// stays `chat/completions` under either surface, and Protocol is what
	// selects chat.
	//
	// The configured `decisionsFlavor` value is resolved to this by the caller:
	// this package knows the two wire shapes, and the configuration owns the
	// vocabulary that names them, so no third comparison reads the string.
	Decisions bool
	// PromptCacheKey is the prompt-cache key every OpenAI-surface request this
	// client sends carries, in the body's `prompt_cache_key`; empty sends none.
	// It names a run rather than a provider: every ask of one run carries the
	// same key, which is what holds that run's asks at one upstream and sharing
	// one cache entry. The caller sets it, from the run it is opening the
	// client for. The Messages protocol has no such field, so a client
	// speaking it sends none of this.
	PromptCacheKey string
}

// NewClient constructs the chat client the endpoint's protocol names: the
// OpenAI-compatible surface by default, and the Anthropic Messages surface
// where the caller resolved that. This is the one place the protocol value
// selects an implementation; each constructor it names always builds the
// protocol it is named for. The protocol value is the caller's — the
// configuration owns the vocabulary that names it — so nothing here reads a
// configuration string.
func NewClient(e Endpoint) Client {
	if e.Protocol == ProtocolAnthropic {
		return NewAnthropicClient(e)
	}
	return NewOpenAIClient(e)
}

// ConsultDrained performs a consult with blocking semantics over the
// STREAMING transport: it opens a stream, iterates to EOF discarding every
// delta, and returns the accumulated Final().
//
// Batch pipeline call sites use this instead of Consult. A blocking request
// puts nothing on the wire until the whole completion is ready, and a long
// generation therefore looks like a dead connection to every idle timeout in
// the path (proxy, gateway, http.Client). A streamed request keeps bytes
// flowing for the same work. Consult remains the right call for short
// round-trips and for callers that must not pay the SSE decode.
//
// A stream that fails mid-flight (transport error, decode error, idle
// watchdog) reports that error instead of io.EOF; ConsultDrained propagates
// it and returns no partial Response. Reasoning deltas are discarded along
// with the rest — Response has no field for them by design (see
// Chunk.Reasoning).
func ConsultDrained(ctx context.Context, c Client, req Request) (Response, error) {
	sr, err := c.ConsultStream(ctx, req)
	if err != nil {
		return Response{}, err
	}
	defer sr.Close()
	for {
		if _, err := sr.Next(); err != nil {
			if errors.Is(err, io.EOF) {
				return sr.Final(), nil
			}
			return Response{}, fmt.Errorf("drain stream: %w", err)
		}
	}
}

// ModelInfo describes a single model exposed by a provider, mapped from
// whichever catalogue shape the endpoint's protocol returns: the two protocols
// spell the listing differently and both land here.
type ModelInfo struct {
	ID      string // provider-specific model identifier
	Created int64  // unix seconds; 0 if provider didn't supply
	OwnedBy string // free-form ownership string; "" if absent
}

// Request is one chat invocation, as the pipeline's asks shape it. The two
// protocols read different halves of it: the OpenAI client sends no completion
// cap (the server's own applies) and reads ChatTemplateKwargs, Reasoning and
// PromptCacheKey; the Messages client derives its required cap from the text
// (maxTokensFor) and sends none of those three, which that protocol has no
// field for.
type Request struct {
	Model    string
	Messages []Message

	// Temperature is the sampling temperature, wired on the wire by the
	// OpenAI client always. 0 means deterministic (greedy decode); it is NOT
	// a "use server default" sentinel. The Messages client sends it only
	// inside that protocol's [0,1] range — see encodeAnthropicRequest.
	Temperature float64

	// ChatTemplateKwargs is the de-facto OpenAI-API extension for passing
	// chat-template-level kwargs through to the underlying tokenizer. It is
	// sent beside Reasoning, for servers that speak the tokenizer extension;
	// a provider that ignores it is not a reason to shape a request
	// differently, because Reasoning covers that case. Encoded as
	// `chat_template_kwargs` in the JSON wire format. Empty/nil → field
	// omitted entirely.
	ChatTemplateKwargs map[string]any

	// Reasoning is the provider's own reasoning control, encoded as the
	// `reasoning` object. It is what disables thinking on a provider that
	// ignores chat-template-level toggles — OpenRouter, for one — and a
	// server that speaks only the tokenizer extension ignores it. Every chat
	// request carries it; there is no per-provider selection. Nil → field
	// omitted entirely.
	Reasoning *Reasoning

	// PromptCacheKey is sent as `prompt_cache_key`, and omitted entirely
	// where empty. The client is what sets it, from the key it was built
	// with: one client serves one run, and every ask that run makes carries
	// the same key. A caller does not set it.
	PromptCacheKey string
}

// Reasoning is the `reasoning` request object: the provider's own control
// over thinking, as the OpenAI-compatible surfaces that support it take it.
// Only the one switch kbase sets is represented — kbase always asks for
// thinking off.
type Reasoning struct {
	Enabled bool `json:"enabled"`
}

// FinishLength is the finish reason that says a generation hit its completion
// cap instead of finishing — `length` on the OpenAI-compatible surface, and the
// Messages protocol's `max_tokens` translated to it at decode. It is the one
// finish reason that says the response is INCOMPLETE rather than wrong.
const FinishLength = "length"

// Message is one turn in the chat history. Role is one of "system",
// "user", or "assistant".
type Message struct {
	Role    string
	Content string
}

// Response is one chat-completions result.
type Response struct {
	Content      string
	Usage        Usage
	FinishReason string

	// StreamDone is whether a streamed response reached the provider's
	// closing event — `[DONE]` on the OpenAI surface, `message_stop` on the
	// Messages one: a stream whose body ended short of it did not complete,
	// whatever content arrived first. False on a blocking Consult.
	StreamDone bool
	// UsageReported is whether a streamed response carried a usage block at
	// all, which an all-zero Usage cannot say.
	UsageReported bool
}

// Truncated reports a response the provider cut off at the completion cap.
//
// It is the predicate rather than the string because of what a caller must do
// with it: a truncated response is INCOMPLETE, not wrong, and holding an
// incomplete answer to a content post-condition reports a rejection the model
// never earned. Reasoning makes this the common case rather than the exotic one
// — reasoning is spent from the same window (ThinkingMaxTokens) and Final
// accumulates the content channel alone, so a generation cut off mid-reasoning
// arrives as a well-formed response with an empty or one-byte Content.
func (r Response) Truncated() bool { return r.FinishReason == FinishLength }

// Chunk is one delta in a streamed chat-completion response. Most chunks
// carry a non-empty Content; the final chunk(s) typically carry empty
// Content but a non-empty FinishReason and Usage.
//
// Reasoning carries thinking-mode output, kept strictly separate from
// Content because they are different kinds of output: Content is the
// model's committed answer, Reasoning is scratch. Reasoning must never be
// concatenated into a response body — a verbatim-leaf discipline cannot
// survive scratch text leaking into distilled output. It is available
// per-chunk so a caller can display or discard it, and is NOT accumulated
// into Final() (see the note there). A reasoning-only chunk carries an
// empty Content — a stream can consist almost entirely of them.
type Chunk struct {
	Content      string
	Reasoning    string
	FinishReason string
	Usage        Usage

	// hasUsage is whether the payload carried a usage block, zero or not.
	hasUsage bool
}

// StreamReader iterates the chunks of a streamed response.
//
// Usage:
//
//	sr, err := client.ConsultStream(ctx, req)
//	if err != nil { ... }
//	defer sr.Close()
//	for {
//	    chunk, err := sr.Next()
//	    if errors.Is(err, io.EOF) { break }
//	    if err != nil { ... }
//	    // chunk.Content has the next delta
//	}
//	final := sr.Final()  // accumulated Response after iteration completes
//
// Calling Close before EOF aborts the stream cleanly. Close is
// idempotent; double-close is safe.
//
// CONCURRENCY: a StreamReader is single-goroutine. Next, Final, and Close
// must all be called from the goroutine that owns it; nothing here is
// guarded, and a second goroutine calling Close mid-Next is a data race.
// Cross-goroutine abort is the context's job — the ctx passed to
// ConsultStream is checked first on every Next, so cancelling it stops the
// iteration from anywhere. Close then releases the transport, from the
// owning goroutine, on the way out.
type StreamReader interface {
	Next() (Chunk, error)
	Final() Response
	Close() error
}

// Usage is the token-accounting block returned by the provider.
//
// ReasoningTokens is the thinking-mode share of CompletionTokens
// (`completion_tokens_details.reasoning_tokens` on the wire), 0 when the
// provider does not report it. It is the only measurement of what thinking
// mode costs — on a reasoning-heavy call it is most of CompletionTokens.
//
// CachedPromptTokens is the share of PromptTokens the provider served from
// its prompt cache (`prompt_tokens_details.cached_tokens`), 0 when not
// reported. It is standard OpenAI accounting — a token count, hence its
// place beside the others and NOT inside Telemetry.
//
// On a STREAMED response every figure here is zero unless the request carried
// `stream_options.include_usage` (the OpenAI client sends it on the streaming
// path); the Messages client's streams report usage on every stream, so this
// caveat is the OpenAI surface's alone. An all-zero Usage therefore means "not
// reported", not "no tokens", and anything gated on a token count must treat it
// as an unevaluable input rather than a passing one.
type Usage struct {
	PromptTokens       int
	CompletionTokens   int
	TotalTokens        int
	ReasoningTokens    int
	CachedPromptTokens int

	// Telemetry is the provider's non-standard inference-timing extension.
	// Zero on every provider that does not send it; test with
	// HasTelemetry rather than reading a field and hoping.
	Telemetry InferenceTelemetry
}

// InferenceTelemetry is the extended inference-timing block MLX-backed
// OpenAI-compatible servers attach to `usage`. It is a DE-FACTO extension,
// not part of the OpenAI schema: absence is normal, never an error and never
// a warning — every field is simply zero on a provider that does not send
// the block.
//
// The four durations arrive on the wire as FRACTIONAL SECONDS
// (`"total_time":6.89`) and are converted once, at decode, to
// time.Duration, so the unit travels with the value instead of living in a
// comment that the next caller may not read.
//
// The split matters more than the total: TimeToFirstToken / PrefillDuration
// against GenerationDuration separates a PREFILL-bound slow call (the
// assembled context is too large) from a GENERATION-bound one (the model is
// too slow). That is the primary signal for calibrating the per-call token
// budget (ARCHITECTURE.md §9).
type InferenceTelemetry struct {
	// TimeToFirstToken is `time_to_first_token`: request start to first
	// emitted token.
	TimeToFirstToken time.Duration
	// PrefillDuration is `prompt_eval_duration`: prompt evaluation
	// (prefill) alone.
	PrefillDuration time.Duration
	// GenerationDuration is `generation_duration`: token generation alone.
	GenerationDuration time.Duration
	// TotalDuration is `total_time`: the provider's own end-to-end figure.
	TotalDuration time.Duration

	// PrefillTokensPerSecond and GenerationTokensPerSecond are the
	// provider's own throughput measurements (`prompt_tokens_per_second`,
	// `generation_tokens_per_second`), in TOKENS PER SECOND. They stay
	// float64: a rate has no stdlib unit type, and the wire value is
	// already the provider's computed figure — re-deriving it from the
	// durations above would be a second, divergent answer.
	PrefillTokensPerSecond    float64
	GenerationTokensPerSecond float64
}

// IsZero reports whether the provider supplied no usage figures at all —
// no token counts AND no telemetry. It is the whole-struct zero test, and
// deliberately so: its callers ask "did this block carry anything?", and a
// telemetry-only block carries something.
func (u Usage) IsZero() bool { return u == Usage{} }

// HasTelemetry reports whether the provider supplied the non-standard
// inference-timing extension. This is the predicate for "did I get
// telemetry?" — false is the normal answer on most providers, and callers
// must treat it as absence, not as a fault.
func (u Usage) HasTelemetry() bool { return u.Telemetry != InferenceTelemetry{} }

// StatusError is a non-2xx HTTP response from the provider: the status code
// and the response body, with any Authorization material scrubbed.
//
// It is a typed error rather than a formatted string because the status is
// the thing callers act on — a 429 means "later" and is worth retrying, a 400
// means the request itself is wrong and will be just as wrong three times.
// Match with errors.As over a value target, as elsewhere in this codebase.
//
// Both the blocking and the streaming path return it, because both go through
// the one request helper: a caller's 429-awareness therefore holds on the
// streaming transport the long pipeline calls actually use.
type StatusError struct {
	Code int
	Body string
}

func (e StatusError) Error() string { return fmt.Sprintf("http %d: %s", e.Code, e.Body) }
