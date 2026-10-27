package model

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// mlxUsageBlock is the `usage` payload a live streamed request to the
// MLX-backed provider returned, VERBATIM off the
// wire. It is kept byte-for-byte rather than hand-simplified so a field
// name that drifts — or one this decoder spells wrong — fails here instead
// of silently decoding to zero.
//
// Note what is genuinely ABSENT: `completion_tokens_details`. That is a
// provider gap (ReasoningTokens stays 0), not a parsing bug.
const mlxUsageBlock = `{"prompt_tokens":962,"completion_tokens":900,"total_tokens":1862,
"input_tokens":962,"output_tokens":900,
"prompt_tokens_details":{"cached_tokens":0},
"time_to_first_token":0.61,"total_time":6.89,
"prompt_eval_duration":0.61,"generation_duration":6.28,
"prompt_tokens_per_second":1567.45,"generation_tokens_per_second":143.35}`

// wantMLXUsage is mlxUsageBlock as the caller-facing type: token counts
// straight through, cached_tokens from the prompt-token details, and the
// four wire durations (FRACTIONAL SECONDS) converted to time.Duration.
var wantMLXUsage = Usage{
	PromptTokens:       962,
	CompletionTokens:   900,
	TotalTokens:        1862,
	ReasoningTokens:    0, // completion_tokens_details absent on this provider
	CachedPromptTokens: 0,
	Telemetry: InferenceTelemetry{
		TimeToFirstToken:          610 * time.Millisecond,
		PrefillDuration:           610 * time.Millisecond,
		GenerationDuration:        6280 * time.Millisecond,
		TotalDuration:             6890 * time.Millisecond,
		PrefillTokensPerSecond:    1567.45,
		GenerationTokensPerSecond: 143.35,
	},
}

// streamedUsage is the Usage a stream ending in the include_usage chunk
// carrying usage decodes to. SSE frames one event per line, so the block's
// line wrapping — and only that — is removed; field names and values stay
// verbatim.
func streamedUsage(t *testing.T, usage string) Usage {
	t.Helper()
	sse := `data: {"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}

data: {"choices":[],"usage":` + strings.ReplaceAll(usage, "\n", "") + `}

data: [DONE]

`
	_, final, err := runStream(t, sse)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("stream did not end cleanly: %v", err)
	}
	return final.Usage
}

// TestUsageTelemetryDecode_MLX pins the extended block's decode off the
// streamed include_usage chunk — an absent completion_tokens_details
// included, which decodes to zero reasoning tokens rather than an error.
func TestUsageTelemetryDecode_MLX(t *testing.T) {
	t.Run("streamed", func(t *testing.T) {
		if got := streamedUsage(t, mlxUsageBlock); got != wantMLXUsage {
			t.Errorf("streamed Usage =\n %+v\nwant\n %+v", got, wantMLXUsage)
		}
	})

	t.Run("split-is-the-signal", func(t *testing.T) {
		if !wantMLXUsage.HasTelemetry() {
			t.Fatal("HasTelemetry = false on a block that carries telemetry")
		}
		// The prefill/generation SPLIT is what the block is decoded for
		// (ARCHITECTURE §9 budget calibration): a slow call is diagnosable
		// only if the two halves arrive as distinct figures rather than
		// collapsing into the total. Assert the decoded relationship, not
		// a rendering of it.
		tm := wantMLXUsage.Telemetry
		if tm.PrefillDuration+tm.GenerationDuration > tm.TotalDuration {
			t.Errorf("prefill %v + generation %v exceeds total %v",
				tm.PrefillDuration, tm.GenerationDuration, tm.TotalDuration)
		}
		if tm.PrefillTokensPerSecond <= tm.GenerationTokensPerSecond {
			t.Errorf("prefill %.1f tok/s should outpace generation %.1f tok/s on this capture",
				tm.PrefillTokensPerSecond, tm.GenerationTokensPerSecond)
		}
	})
}

// TestUsageTelemetryAbsent: a standard OpenAI usage block carries none of
// the extension fields. That must decode with no error, leave every
// telemetry field zero, and report HasTelemetry false — absence is the
// normal case on most providers, never a fault.
func TestUsageTelemetryAbsent(t *testing.T) {
	const openAIUsage = `{"prompt_tokens":41,"completion_tokens":9,"total_tokens":50,
"completion_tokens_details":{"reasoning_tokens":4}}`

	usage := streamedUsage(t, openAIUsage)
	want := Usage{PromptTokens: 41, CompletionTokens: 9, TotalTokens: 50, ReasoningTokens: 4}
	if usage != want {
		t.Errorf("Usage = %+v; want %+v (every extension field zero)", usage, want)
	}
	if usage.HasTelemetry() {
		t.Errorf("HasTelemetry = true on a standard OpenAI usage block: %+v", usage.Telemetry)
	}
	if usage.Telemetry != (InferenceTelemetry{}) {
		t.Errorf("Telemetry = %+v; want zero", usage.Telemetry)
	}

	// A usage block with nothing in it at all stays fully zero, and IsZero
	// still reports "the provider supplied nothing".
	if empty := streamedUsage(t, `{}`); !empty.IsZero() || empty.HasTelemetry() {
		t.Errorf("empty usage block: IsZero=%v HasTelemetry=%v; want true/false",
			empty.IsZero(), empty.HasTelemetry())
	}
}

// TestUsageIsZeroWithTelemetryOnly: IsZero is the WHOLE-struct zero test —
// "did this block carry anything?" — so a block carrying telemetry but no
// token counts is NOT zero. That is what its callers assume: the stream
// reader latches the last non-empty usage block, and a telemetry-only block
// is a block worth latching.
//
// A token-budget check is unaffected either way: it branches on
// Usage.PromptTokens directly, never on IsZero, so a telemetry-only block
// still reads as `prompt_tokens=0` — unmeasured, not zero-cost. Telemetry
// is not a token count.
func TestUsageIsZeroWithTelemetryOnly(t *testing.T) {
	const telemetryOnly = `{"time_to_first_token":0.5,"total_time":2.0,
"prompt_eval_duration":0.5,"generation_duration":1.5,
"prompt_tokens_per_second":900.0,"generation_tokens_per_second":120.0}`

	usage := streamedUsage(t, telemetryOnly)
	if usage.IsZero() {
		t.Errorf("IsZero = true on a telemetry-bearing block: %+v", usage)
	}
	if !usage.HasTelemetry() {
		t.Errorf("HasTelemetry = false on a telemetry-bearing block: %+v", usage)
	}
	if usage.PromptTokens != 0 {
		t.Errorf("PromptTokens = %d; want 0 — a budget check must still see it as unmeasured",
			usage.PromptTokens)
	}
	if got := usage.Telemetry.GenerationDuration; got != 1500*time.Millisecond {
		t.Errorf("GenerationDuration = %v; want 1.5s", got)
	}
}
