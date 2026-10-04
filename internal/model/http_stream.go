package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// sseMaxLineBytes is the buffer cap for a single SSE line. The bufio.Scanner
// default of 64KB is too tight: nothing stops a provider from delivering a
// multi-KB content or reasoning delta as one event, and an over-long line
// would end the stream mid-generation. 1MB is generous for well-behaved
// providers and clamps a runaway server before OOM.
const sseMaxLineBytes = 1 << 20

// sseInitialBufferBytes is the scanner's starting buffer; it grows on demand
// up to sseMaxLineBytes.
const sseInitialBufferBytes = 64 << 10

// streamIdleTimeout bounds the GAP between reads on an open stream. It is
// the only bound a generation of unknown length can honor: a stream that
// keeps delivering is healthy however long it runs, and a stream that has
// gone silent is dead however recently it opened. A total-duration cap
// (http.Client.Timeout) answers the wrong question and kills the first
// kind — see NewHTTPClient.
//
// The clock is reset by ANY line off the wire, not only by a content chunk.
// SSE comment lines (`: ping`) are how a gateway signals liveness across a
// long prefill, and reading a keepalive as silence would abort exactly the
// slow-but-healthy call this bound exists to protect.
const streamIdleTimeout = 2 * time.Minute

// errStreamIdle is the cancellation cause the watchdog attaches, wrapped
// with the window that elapsed. The watchdog aborts a stalled read by
// cancelling the request, so without a cause the caller would be told
// "context canceled" — true, and the wrong answer to "why did my stream
// stop?". Match with errors.Is.
var errStreamIdle = errors.New("model: stream idle; provider stopped sending")

// httpStreamReader is the production StreamReader, backed by an HTTP
// response body delivering SSE-encoded chunks.
//
// Single-goroutine, per the StreamReader contract: no field here is
// guarded. The idle watchdog is the one thing that touches the reader from
// another goroutine, and it touches only context cancellation — which is
// exactly why cancellation, not a flag, is the abort mechanism.
type httpStreamReader struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	body    io.ReadCloser
	scanner *bufio.Scanner

	// idle is the watchdog timer, nil until the first Next arms it — see
	// armIdle. idleWindow is the window it is armed with.
	idle       *time.Timer
	idleWindow time.Duration

	// done is set once the stream has been fully drained (either [DONE]
	// observed, EOF on the body, or a hard error). Subsequent Next()
	// calls return io.EOF.
	done bool
	// closed makes Close idempotent.
	closed bool

	// content is the accumulator for the final Response. Each chunk's
	// Content is appended in order.
	content       strings.Builder
	finishReason  string
	usage         Usage
	usageReported bool
	// sawDone is set when the provider's closing [DONE] arrived.
	sawDone bool

	closeErr error
}

// newHTTPStreamReader takes ownership of body and of cancel. ctx MUST be the
// context the in-flight request was issued with: cancelling it is what
// interrupts a read blocked on bytes that are never coming, and that is the
// whole mechanism behind the idle watchdog.
//
// idle is the watchdog's window. Production passes streamIdleTimeout; it is
// a parameter so a test can trip the watchdog in milliseconds rather than
// pinning the suite to the real bound. The watchdog is not started here —
// the first Next arms it.
func newHTTPStreamReader(ctx context.Context, cancel context.CancelCauseFunc, body io.ReadCloser, idle time.Duration) *httpStreamReader {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, sseInitialBufferBytes), sseMaxLineBytes)
	return &httpStreamReader{ctx: ctx, cancel: cancel, body: body, scanner: sc, idleWindow: idle}
}

// armIdle starts the watchdog, or restarts it if it is already running.
//
// It is deliberately not called from the constructor. The window bounds the
// gap between a read starting and bytes arriving — provider silence — and a
// caller that does other work between ConsultStream and its first Next has
// kept nobody waiting. Arming at construction would charge that work to the
// provider and abort a stream that never misbehaved.
func (r *httpStreamReader) armIdle() {
	if r.idle == nil {
		r.idle = time.AfterFunc(r.idleWindow, func() {
			r.cancel(fmt.Errorf("%w after %v", errStreamIdle, r.idleWindow))
		})
		return
	}
	r.idle.Reset(r.idleWindow)
}

// Next returns the next chunk in the stream, or io.EOF when the stream has
// been drained. Context cancellation surfaces as the ctx error; a stream
// that goes quiet for streamIdleTimeout surfaces as errStreamIdle.
func (r *httpStreamReader) Next() (Chunk, error) {
	if r.done {
		return Chunk{}, io.EOF
	}
	// The read starts here, so the window the provider has to answer in
	// starts here too — on the first Next and on every one after it.
	r.armIdle()
	for {
		// Honor context first so a slow server can't outlast a deadline.
		if err := r.ctx.Err(); err != nil {
			r.finish()
			return Chunk{}, r.terminalErr(err)
		}
		if !r.scanner.Scan() {
			r.finish()
			if err := r.scanner.Err(); err != nil {
				return Chunk{}, r.terminalErr(fmt.Errorf("stream read: %w", err))
			}
			return Chunk{}, io.EOF
		}
		// A line arrived, so the provider is alive — including the blank
		// separators and comment lines skipped just below.
		r.armIdle()
		line := r.scanner.Bytes()
		// SSE separators are blank lines; ignore them.
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		// Only `data:` lines carry payload; some servers also emit
		// comment lines starting with `:` and event/id headers — skip
		// anything that isn't a data line.
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(line[len("data:"):])
		if len(payload) == 0 {
			continue
		}
		if bytes.Equal(payload, []byte("[DONE]")) {
			r.sawDone = true
			r.finish()
			return Chunk{}, io.EOF
		}
		chunk, err := parseSSEChunk(payload)
		if err != nil {
			r.finish()
			// Through terminalErr like every other failure: a payload
			// that will not decode is what a stall truncated mid-event
			// looks like, and the stall is the cause worth reporting.
			return Chunk{}, r.terminalErr(fmt.Errorf("decode stream chunk: %w", err))
		}
		// Accumulate for Final().
		if chunk.Content != "" {
			r.content.WriteString(chunk.Content)
		}
		if chunk.FinishReason != "" {
			r.finishReason = chunk.FinishReason
		}
		// Last non-empty usage block wins. The include_usage final chunk is
		// choices-less, so it lands here with empty Content and updates
		// nothing but Usage.
		if !chunk.Usage.IsZero() {
			r.usage = chunk.Usage
		}
		r.usageReported = r.usageReported || chunk.hasUsage
		// Reasoning is deliberately NOT accumulated — see Final().
		return chunk, nil
	}
}

// finish marks the stream drained and retires the watchdog. Every terminal
// path runs through it, so no timer outlives the stream it was guarding — a
// stream closed before its first Next has no timer to retire.
func (r *httpStreamReader) finish() {
	r.done = true
	if r.idle != nil {
		r.idle.Stop()
	}
}

// terminalErr names the stall instead of the symptom when the watchdog is
// what ended the stream. The watchdog interrupts a blocked read by
// cancelling the request, so the error in hand at that point is a flavor of
// "context canceled" and points at the wrong cause.
func (r *httpStreamReader) terminalErr(err error) error {
	if cause := context.Cause(r.ctx); errors.Is(cause, errStreamIdle) {
		return cause
	}
	return err
}

// Final returns the accumulated Response after iteration. Safe to call
// before EOF, though the resulting Content/Usage/FinishReason will only
// reflect what has been observed so far.
//
// Reasoning is NOT part of the accumulated Response and Response has no
// field for it: reasoning is delivered per-Chunk only. Two reasons, in
// order of weight. (1) Reasoning is model scratch, not its committed
// answer — it must never reach a distilled leaf, a summary, or a replayed
// history, and the cheapest guarantee of that is a type in which it cannot
// be represented. (2) It is large: a reasoning-heavy generation is mostly
// reasoning bytes, so unconditional accumulation would buy every call a
// second full-size buffer for data nearly every caller discards. A caller
// that wants the text is already iterating chunks and can accumulate what
// it chooses to keep.
func (r *httpStreamReader) Final() Response {
	return Response{
		Content:       r.content.String(),
		FinishReason:  r.finishReason,
		Usage:         r.usage,
		StreamDone:    r.sawDone,
		UsageReported: r.usageReported,
	}
}

// Close releases the underlying body and retires the watchdog. Idempotent —
// and, like the rest of the reader, the owning goroutine's to call.
func (r *httpStreamReader) Close() error {
	if !r.closed {
		r.closed = true
		r.finish()
		r.closeErr = r.body.Close()
		// Release the request context last: the body is already closed, so
		// this only frees the transport's bookkeeping.
		r.cancel(nil)
	}
	return r.closeErr
}

// --- SSE wire decoding ---

type wireStreamChoice struct {
	Index        int             `json:"index"`
	Delta        wireStreamDelta `json:"delta"`
	FinishReason string          `json:"finish_reason"`
}

type wireStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`

	// ReasoningContent / Reasoning are the same field under two spellings.
	// Gateways normalize most vendors to `reasoning_content`; `reasoning`
	// appears in the wild (and is what some native endpoints emit). Both
	// are decoded and collapsed by parseSSEChunk; neither ever joins
	// Content — reasoning is model scratch, not its committed answer.
	ReasoningContent string `json:"reasoning_content,omitempty"`
	Reasoning        string `json:"reasoning,omitempty"`
}

type wireStreamResponse struct {
	Choices []wireStreamChoice `json:"choices"`
	Usage   *wireUsage         `json:"usage,omitempty"`
}

func parseSSEChunk(payload []byte) (Chunk, error) {
	var w wireStreamResponse
	if err := json.Unmarshal(payload, &w); err != nil {
		return Chunk{}, err
	}
	var out Chunk
	// A choices-less payload is not malformed: with
	// stream_options.include_usage the provider emits a final chunk whose
	// only content is `usage`. Everything below is independently guarded so
	// that chunk decodes to a Usage-only Chunk.
	if len(w.Choices) > 0 {
		c := w.Choices[0]
		out.Content = c.Delta.Content
		out.Reasoning = c.Delta.ReasoningContent
		if out.Reasoning == "" {
			out.Reasoning = c.Delta.Reasoning
		}
		out.FinishReason = c.FinishReason
	}
	if w.Usage != nil {
		out.Usage = w.Usage.usage()
		out.hasUsage = true
	}
	return out, nil
}
