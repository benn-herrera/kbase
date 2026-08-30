// Package model is the LLM client surface.
//
// Two implementations satisfy the same Client interface: HTTPClient
// (http_client.go), targeting any OpenAI-compatible chat-completions
// endpoint, and MockClient (mockllm.go), the test substrate. Both consume
// the same Request/Response shape, so tests and the production pipeline
// exercise the same code paths.
package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// Client is the abstraction the pipeline depends on for an LLM round-trip.
// Defined here so test code can swap in MockClient without depending on
// the HTTP transport.
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
	ListModels(ctx context.Context) ([]ModelInfo, error)
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

// ModelInfo describes a single model exposed by a provider, mirroring the
// shape OpenAI-compatible /models endpoints return.
type ModelInfo struct {
	ID      string // provider-specific model identifier
	Created int64  // unix seconds; 0 if provider didn't supply
	OwnedBy string // free-form ownership string; "" if absent
}

// Request is one chat-completions invocation.
//
// Sampling parameters are always wired on the wire — a zero value for
// Temperature or MaxTokens is a real value, not a "use server default"
// sentinel. Use DefaultRequest to construct a Request with the appliance
// defaults pre-filled; only override what you explicitly need.
type Request struct {
	Model    string
	Messages []Message

	// Temperature is the sampling temperature, always wired on the wire.
	// 0 means deterministic (greedy decode); it is NOT a "use server
	// default" sentinel.
	Temperature float64

	// MaxTokens is the response token cap, always wired on the wire. 0
	// is a real value (no tokens) — not a "use server default" sentinel.
	// Production callers should construct Requests via DefaultRequest.
	MaxTokens int

	// ChatTemplateKwargs is the de-facto OpenAI-API extension for passing
	// chat-template-level kwargs through to the underlying tokenizer.
	// Most commonly used for thinking-mode toggles on locally served
	// models. Encoded as `chat_template_kwargs` in the JSON wire format.
	// Empty/nil → field omitted entirely.
	ChatTemplateKwargs map[string]any
}

// DefaultMaxTokens is the completion cap an effort that states none gets.
// Provisional: the per-call budget is a calibration constant
// (ARCHITECTURE.md §9) and this value will be revisited there.
const DefaultMaxTokens = 16384

// ThinkingMaxTokens is the completion cap for an attempt that asks with
// THINKING ON (ARCHITECTURE.md §9).
//
// It exists because reasoning is spent out of the ANSWER's window: MaxTokens
// bounds completion tokens and ReasoningTokens is a share of them (see Usage),
// so an attempt that reasons under DefaultMaxTokens has strictly less room to
// answer than the attempt that did not — the escalation makes the ask harder
// and the budget smaller at the same time. A generation cut off while still in
// the reasoning channel returns a well-formed response whose Content is empty
// or one byte (Final accumulates content alone), which is the 2026-08-18
// one-byte escalated retry exactly.
//
// The value is twice DefaultMaxTokens, and the arithmetic is the one
// measurement this appliance owns: the 2026-08-12 boundary A/B spent 20,924
// completion tokens reasoning about an ask whose answer is a bare number —
// already over DefaultMaxTokens before a single byte of answer. Doubling clears
// that measured cost with margin and still leaves ~11.8K for an answer that is
// capped far lower at every seam that escalates. It remains a CAP: SD-4's
// 36-minute escalated retry is the runaway this row bounds, not a budget to
// grow until nothing complains.
const ThinkingMaxTokens = 32768

// FinishLength is the finish_reason an OpenAI-compatible provider reports when
// a generation hit its completion cap instead of finishing. It is the one
// finish reason that says the response is INCOMPLETE rather than wrong.
const FinishLength = "length"

// RequestEffort is how hard the model is asked to work on ONE exact ask.
//
// It is per-DEFINITION: the value belongs to the question, not to the stage
// that asks it and not to the seam it crosses (ARCHITECTURE.md §12). A
// definition states its effort where it is registered, and every
// request-construction path takes one POSITIONALLY, so a call site cannot
// inherit an effort nobody stated.
//
// Temperature belongs here next, joining as a field the way MaxTokens did, so
// no call site that already states an effort has to change to accommodate it.
type RequestEffort struct {
	// Thinking asks the model to reason before answering. It is wired in
	// BOTH directions (see DefaultRequest) — false is SENT as false, never
	// omitted — because an omitted key leaves the chat template's own
	// default in charge, which is the silent inheritance this type exists
	// to prevent.
	Thinking bool

	// MaxTokens is the completion window this ask gets. It belongs to the
	// effort and not to the call site because it is the same dimension
	// Thinking is: how hard the model is asked to work is inseparable from how
	// much room it is given to do it in, and the provider spends both out of
	// one budget (ThinkingMaxTokens says why that matters).
	//
	// Zero is not a request for no tokens: DeclareEffort fills it with
	// DefaultMaxTokens, exactly as pipeline.DeclareRetry fills its two counts,
	// so a definition with nothing to say about its window says nothing and
	// gets the shipped default.
	MaxTokens int

	// declared separates a stated effort from a zero value. An effort makes
	// one hop through a struct field on its way to the runner
	// (pipeline.AskSpec), and a field is the one hop a positional parameter
	// cannot make mandatory; this mark is what lets that hop be checked
	// (pipeline.AskSpec.validate) rather than assumed. It is unexported so
	// DeclareEffort is the only way to obtain a declared value.
	declared bool
}

// DeclareEffort marks e as STATED by its caller and fills the completion
// window a definition left at zero with the shipped default. It is the only
// constructor: a bare composite literal is an undeclared value, and the
// pipeline refuses an ask spec carrying one.
//
// It takes the whole value rather than one parameter per dial so that adding
// Temperature is a new field at the call sites that want it and nothing at all
// at the ones that do not.
//
// Defaulting the window while requiring Thinking is the same honest split
// pipeline.DeclareRetry makes: an unstated window reads as "today's budget",
// which is a real statement, while an unstated Thinking would read as "no
// thinking", which is the silent inheritance this type exists to prevent.
func DeclareEffort(e RequestEffort) RequestEffort {
	if e.MaxTokens == 0 {
		e.MaxTokens = DefaultMaxTokens
	}
	e.declared = true
	return e
}

// Declared reports whether this effort was stated rather than left zero.
func (e RequestEffort) Declared() bool { return e.declared }

// DefaultRequest returns a Request prefilled with the appliance defaults.
// The caller fills in Model, Messages and the ask's declared RequestEffort;
// everything else is preset:
//
//   - Temperature: 0 (deterministic — reproducibility is a design property,
//     not a tuning preference)
//   - MaxTokens:   the effort's own window (DefaultMaxTokens unless the
//     definition declared otherwise), because the attempt asked to reason is
//     the attempt that needs room to reason AND answer
//   - ChatTemplateKwargs: {"thinking": E, "enable_thinking": E} for the
//     effort's Thinking value. Both keys are sent because the one a model
//     recognizes varies by model, and sending both is harmless to a model
//     that recognizes neither. Both are sent when E is FALSE as well: the
//     alternative is omitting the field and hoping the served template
//     defaults the way this ask wants, which is not a declaration.
//
// It is the only Request constructor, and RequestEffort is positional in it, so
// every call that goes on the wire states how hard it is asking.
func DefaultRequest(model string, messages []Message, effort RequestEffort) Request {
	return Request{
		Model:       model,
		Messages:    messages,
		Temperature: 0,
		MaxTokens:   effort.MaxTokens,
		ChatTemplateKwargs: map[string]any{
			"thinking":        effort.Thinking,
			"enable_thinking": effort.Thinking,
		},
	}
}

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
// On a STREAMED response every figure here is zero unless the request
// carried `stream_options.include_usage` (encodeRequest sends it on the
// streaming path). An all-zero Usage therefore means "not reported", not
// "no tokens", and anything gated on a token count must treat it as an
// unevaluable input rather than a passing one.
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

// ErrMockExhausted is returned by a scripted MockClient when its response
// queue has been drained. Tests check for this with errors.Is.
var ErrMockExhausted = errors.New("model: mock client exhausted")
