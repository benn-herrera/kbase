package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// openaiStreamReader is the OpenAI-compatible StreamReader, backed by an HTTP
// response body delivering SSE-encoded chunks.
type openaiStreamReader struct {
	sseScanner

	// content is the accumulator for the final Response. Each chunk's
	// Content is appended in order.
	content       strings.Builder
	finishReason  string
	usage         Usage
	usageReported bool
	// sawDone is set when the provider's closing [DONE] arrived.
	sawDone bool
}

// newOpenAIStreamReader takes ownership of body and of cancel, and reads it as
// the OpenAI-compatible event stream.
func newOpenAIStreamReader(ctx context.Context, cancel context.CancelCauseFunc, body io.ReadCloser, idle time.Duration) *openaiStreamReader {
	return &openaiStreamReader{sseScanner: newSSEScanner(ctx, cancel, body, idle)}
}

// Next returns the next chunk in the stream, or io.EOF when the stream has
// been drained. Context cancellation surfaces as the ctx error; a stream
// that goes quiet for streamIdleTimeout surfaces as errStreamIdle.
func (r *openaiStreamReader) Next() (Chunk, error) {
	if r.done {
		return Chunk{}, io.EOF
	}
	for {
		payload, err := r.nextData()
		if err != nil {
			return Chunk{}, err
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
func (r *openaiStreamReader) Final() Response {
	return Response{
		Content:       r.content.String(),
		FinishReason:  r.finishReason,
		Usage:         r.usage,
		StreamDone:    r.sawDone,
		UsageReported: r.usageReported,
	}
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
