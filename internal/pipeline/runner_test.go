package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"kbase/internal/log/logtest"
	"kbase/internal/model"
	"kbase/internal/prompt"
)

// stageConstantFrontier reads the derivation with its error checked.
func stageConstantFrontier(t *testing.T) prompt.Slot {
	t.Helper()
	f, err := StageConstantFrontier()
	if err != nil {
		t.Fatalf("StageConstantFrontier(): %v", err)
	}
	return f
}

// newRunnerCall builds a bound ask and a call over it, with the frontier a
// worker's first call declares (nothing claimed, nothing to compare against).
func newRunnerCall(t *testing.T, ask AskSpec) (*boundAsk, call) {
	t.Helper()
	bound, err := newBoundAsk(ask, synthSpec("Task: synthetic."), synthJobFrame(t))
	if err != nil {
		t.Fatalf("newBoundAsk: %v", err)
	}
	task := synthTask("survey/a.json", "all")
	return bound, call{
		Stage: "survey", ArtifactPath: task.Owed.Path, BoundAsk: bound,
		Input: task.Input(), Frontier: prompt.SlotTotal,
	}
}

// runnerFor builds a runner whose transport backoff is zero: these tests count
// attempts and classify errors, and paying the real wait to do it would add
// seconds to every suite run. TestRunnerTransportRetries keeps the real
// backoff, so the wait itself is still asserted somewhere.
func runnerFor(t *testing.T, client model.Client, lg *logtest.Capture) *CallRunner {
	t.Helper()
	r := NewCallRunner(client, synthConfig(), lg)
	r.backoff = 0
	return r
}

// TestRunnerVerifiedCall is the happy path: one attempt, a typed artifact, and
// the usage the provider reported.
func TestRunnerVerifiedCall(t *testing.T) {
	lg := &logtest.Capture{}
	client := echoStub()
	_, call := newRunnerCall(t, noFallbackAsk(t))

	res, err := runnerFor(t, client, lg).Run(context.Background(), call)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, ok := res.Artifact.(synthArtifact); !ok {
		t.Errorf("Artifact is %T, want a typed synthArtifact", res.Artifact)
	}
	if res.Attempts != 1 || res.FellBack {
		t.Errorf("Attempts = %d, FellBack = %v; want 1, false", res.Attempts, res.FellBack)
	}
	if res.Usage.PromptTokens != 100 || res.Usage.CachedPromptTokens != 40 {
		t.Errorf("Usage = %+v, want the provider's figures", res.Usage)
	}
	if len(res.Hashes) == 0 {
		t.Error("no per-slot hashes came back for the next call to compare against")
	}
	// cached_tokens is the wire-level ground truth that churn prevention is
	// holding, so it is recorded on every call regardless of configuration.
	if !lg.Has(t, "debug", "cached_tokens", 40) {
		t.Error("cached_tokens was not recorded")
	}
}

// TestRunnerSendsTheRolesDeclaredEffort: the effort on the wire is the ROLE's
// declaration, whatever it says.
//
// The runner has no view of what is being asked — it sees a built turn and a
// tier — so an effort it chose for itself would be the layer with the least
// information deciding how hard to think. Both rows are asserted because a
// runner that hardcoded either value would pass a single-row test.
func TestRunnerSendsTheRolesDeclaredEffort(t *testing.T) {
	for _, tc := range []struct {
		name string
		ask  AskSpec
		want bool
	}{
		{"thinking declared on", noFallbackAsk(t), true},
		{"thinking declared off", fallbackBackedAsk(t), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var kwargs map[string]any
			client := echoStub()
			inner := client.respond
			client.respond = func(n int, req model.Request) (model.Response, error) {
				kwargs = req.ChatTemplateKwargs
				return inner(n, req)
			}
			_, call := newRunnerCall(t, tc.ask)
			if _, err := runnerFor(t, client, &logtest.Capture{}).Run(context.Background(), call); err != nil {
				t.Fatalf("Run: %v", err)
			}
			for _, key := range []string{"thinking", "enable_thinking"} {
				if got, ok := kwargs[key]; !ok || got != tc.want {
					t.Errorf("%s = %v (present %t), want %t", key, got, ok, tc.want)
				}
			}
		})
	}
}

// TestRunnerBuildRefusalPropagates: refuse-and-split. An over-budget call is
// the tree plan's problem, and the runner must hand the error back untouched
// rather than retry it, wrap it, or fall back through it.
func TestRunnerBuildRefusalPropagates(t *testing.T) {
	client := echoStub()
	ask := fallbackBackedAsk(t) // a fallback exists, and must NOT be used here
	bound, err := newBoundAsk(ask, prompt.StageSpec{
		TaskDef: "Task: synthetic.",
		// A content budget no real span fits in. Budgeting the slot rather
		// than the whole call keeps the stage's own constant slots buildable,
		// so what fails here is this unit and not the stage.
		Budgets: prompt.Budgets{PerSlot: map[prompt.Slot]int{prompt.SlotContent: 1}},
	}, synthJobFrame(t))
	if err != nil {
		t.Fatalf("newBoundAsk: %v", err)
	}
	task := synthTask("survey/a.json", "all")

	res, err := runnerFor(t, client, &logtest.Capture{}).Run(context.Background(), call{
		Stage: "survey", ArtifactPath: task.Owed.Path, BoundAsk: bound,
		Input: task.Input(), Frontier: prompt.SlotTotal,
	})
	var target prompt.ErrOverBudget
	if !errors.As(err, &target) {
		t.Fatalf("err = %v, want prompt.ErrOverBudget", err)
	}
	if res.Artifact != nil {
		t.Error("a refused build must not produce an artifact, fallback or otherwise")
	}
	if client.callCount() != 0 {
		t.Errorf("%d calls went on the wire for a prompt that was never built", client.callCount())
	}
}

// TestRunnerFrozenPromptViolationAborts: the tripwire firing is a kbase
// defect. Never retried, never fallen back — even on a fallback-backed seam, where
// a fallback exists and would paper over it.
func TestRunnerFrozenPromptViolation(t *testing.T) {
	t.Run("per-worker churn", func(t *testing.T) {
		client := echoStub()
		_, call := newRunnerCall(t, fallbackBackedAsk(t))
		// A previous call from a DIFFERENT stage context, with a frontier
		// that claims the stage-constant slots held.
		otherAgent, err := newBoundAsk(fallbackBackedAsk(t), synthSpec("Task: something else."), synthJobFrame(t))
		if err != nil {
			t.Fatalf("newBoundAsk: %v", err)
		}
		prevBuilt, err := otherAgent.ctx.Build(call.Input)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		call.Prev = prevBuilt.Hashes
		call.Frontier = stageConstantFrontier(t)

		res, err := runnerFor(t, client, &logtest.Capture{}).Run(context.Background(), call)
		var target WorkerAbortError
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want WorkerAbortError", err)
		}
		var churn prompt.ErrSlotChurn
		if !errors.As(err, &churn) {
			t.Errorf("the abort does not carry the churn it saw: %v", err)
		}
		if res.Artifact != nil {
			t.Error("a defect must not fall back to the mechanical fallback")
		}
		if client.callCount() != 0 {
			t.Error("a prompt that failed its own stability contract went on the wire")
		}
	})

	t.Run("wrong stage context", func(t *testing.T) {
		client := echoStub()
		bound, call := newRunnerCall(t, fallbackBackedAsk(t))
		bound.canonicalHashes[prompt.SlotTaskDef] = [32]byte{0xBB}

		_, err := runnerFor(t, client, &logtest.Capture{}).Run(context.Background(), call)
		var target StageContextMismatchError
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want StageContextMismatchError", err)
		}
		if client.callCount() != 0 {
			t.Error("a call built against the wrong context went on the wire")
		}
	})
}

// TestRunnerTransportRetries: network weather is retried with backoff, and the
// retry is of the identical request — nothing about the output is in question.
func TestRunnerTransportRetries(t *testing.T) {
	client := &stubClient{respond: func(n int, req model.Request) (model.Response, error) {
		if n == 1 {
			return model.Response{}, model.StatusError{Code: 503, Body: "upstream connect error"}
		}
		return model.Response{Content: synthAccept + " recovered", FinishReason: "stop"}, nil
	}}
	_, call := newRunnerCall(t, noFallbackAsk(t))

	start := time.Now()
	res, err := NewCallRunner(client, synthConfig(), &logtest.Capture{}).Run(context.Background(), call)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if client.callCount() != 2 {
		t.Errorf("calls = %d, want 2 (one failure, one retry)", client.callCount())
	}
	if res.Attempts != 1 {
		t.Errorf("Attempts = %d; a wire retry is not a model attempt", res.Attempts)
	}
	if elapsed := time.Since(start); elapsed < wireBackoffBase {
		t.Errorf("retried after %v, want at least the %v backoff", elapsed, wireBackoffBase)
	}
	prompts := client.recorded()
	if len(prompts) == 2 && prompts[0] != prompts[1] {
		t.Error("the wire retry changed the prompt; it must resend the identical request")
	}
}

// TestRunnerTransportClassification is the 429-awareness: a rate limit is the
// one client error that means "later", so it alone among the 4xx is retried.
//
// The status arrives as model.StatusError, which is a compiler-checked
// contract with internal/model rather than a parse of its error text — the
// cases below cannot drift out of agreement with what the client returns
// without failing to build.
func TestRunnerTransportClassification(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		wantCalls int
	}{
		{"rate limit is retried", model.StatusError{Code: 429, Body: "slow down"}, wireAttempts},
		{"a bad request is not", model.StatusError{Code: 400, Body: "unknown model"}, 1},
		{"unauthorized is not", model.StatusError{Code: 401, Body: "bad key"}, 1},
		{"a server error is", model.StatusError{Code: 500, Body: "boom"}, wireAttempts},
		{"an error carrying no status is", errors.New("dial tcp: connection refused"), wireAttempts},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &stubClient{respond: func(int, model.Request) (model.Response, error) {
				return model.Response{}, tc.err
			}}
			// A fallback-backed seam, so the exhausted transport resolves rather
			// than failing the unit — what is under test is the call count.
			_, call := newRunnerCall(t, fallbackBackedAsk(t))

			res, err := runnerFor(t, client, &logtest.Capture{}).Run(context.Background(), call)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if !res.FellBack {
				t.Error("an exhausted transport must leave a fallback-backed seam degraded")
			}
			if client.callCount() != tc.wantCalls {
				t.Errorf("calls = %d, want %d", client.callCount(), tc.wantCalls)
			}
		})
	}
}

// TestRunnerContextCancellationIsNotRetried: a stopping job is not network
// weather. It returns the context's error, not a fallback and not a failure.
func TestRunnerContextCancellationIsNotRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &stubClient{respond: func(int, model.Request) (model.Response, error) {
		cancel()
		return model.Response{}, context.Canceled
	}}
	// Refinement seam again: cancellation must beat the fallback.
	_, call := newRunnerCall(t, fallbackBackedAsk(t))

	res, err := runnerFor(t, client, &logtest.Capture{}).Run(ctx, call)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if res.Artifact != nil {
		t.Error("a cancelled job must not produce an artifact")
	}
	if client.callCount() != 1 {
		t.Errorf("calls = %d, want 1; cancellation is not retryable", client.callCount())
	}
}

// TestRunnerSemanticRetryCarriesTheReason: ONE retry, and it carries the
// mechanical failure. A blind identical resend hopes temperature fixes it,
// which was ruled out — so the second prompt must differ from the first, and
// differ by the verifier's own words.
func TestRunnerSemanticRetryCarriesTheReason(t *testing.T) {
	lg := &logtest.Capture{}
	client := &stubClient{respond: func(n int, req model.Request) (model.Response, error) {
		if n == 1 {
			return model.Response{Content: "here is my best guess", FinishReason: "stop"}, nil
		}
		return model.Response{Content: synthAccept + " corrected", FinishReason: "stop"}, nil
	}}
	_, call := newRunnerCall(t, noFallbackAsk(t))

	res, err := runnerFor(t, client, lg).Run(context.Background(), call)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Attempts != modelAttempts {
		t.Errorf("Attempts = %d, want %d", res.Attempts, modelAttempts)
	}
	if res.FellBack {
		t.Error("a call the retry rescued is not degraded")
	}

	prompts := client.recorded()
	if len(prompts) != 2 {
		t.Fatalf("%d prompts recorded, want 2", len(prompts))
	}
	if prompts[0] == prompts[1] {
		t.Fatal("the semantic retry resent the identical prompt")
	}
	if !strings.Contains(prompts[1], retryNotePrefix) {
		t.Error("the retry does not carry the retry note")
	}
	if !strings.Contains(prompts[1], synthAccept) {
		t.Error("the retry note does not carry the verifier's reason")
	}
	// The note goes in the acceptance-criteria channel, which renders in the
	// trailer — the recency end, beside the other per-call binding facts.
	if trailer := prompts[1][strings.LastIndex(prompts[1], "REMINDER:"):]; !strings.Contains(trailer, retryNotePrefix) {
		t.Error("the retry note did not land in the trailer")
	}
	// Both attempts are on the record with their prompt identity, so a
	// rejected response and its retry can be told apart afterwards.
	if !lg.Has(t, "warn", "outcome", "rejected") || !lg.Has(t, "debug", "outcome", "verified") {
		t.Error("the two attempts' outcomes were not both logged")
	}
}

// TestRunnerRetryNoteIsBounded: the note takes a small fixed bite of the
// acceptance criteria's reserved share (§9), so a verbose verifier cannot
// squeeze out the criteria the tree plan emitted.
func TestRunnerRetryNoteIsBounded(t *testing.T) {
	long := errors.New(strings.TrimSpace(strings.Repeat("verbose ", 200)))
	got := withRetryNote(prompt.CallInput{AcceptanceCriteria: []string{"- keep me"}}, long)

	if len(got.AcceptanceCriteria) != 2 || got.AcceptanceCriteria[0] != "- keep me" {
		t.Fatalf("criteria = %q, want the caller's own kept and the note appended", got.AcceptanceCriteria)
	}
	note := got.AcceptanceCriteria[1]
	words := len(strings.Fields(note)) - len(strings.Fields(retryNotePrefix))
	if words > retryNoteWords+1 { // +1 for the truncation mark
		t.Errorf("note carries %d words of reason, cap is %d", words, retryNoteWords)
	}
}

// TestRunnerVerifierDefectAbortsInsteadOfFallingBack: a verifier that reports
// ErrVerifierDefect is saying the response could not be wrong in the way it
// failed, so the fault is ours. Neither remedy applies — a retry re-asks a
// question that was never asked wrong, and the fallback comes from the same
// derivation the verifier just indicted — so the worker aborts on the FIRST
// attempt, even on a fallback-backed seam that has a fallback to stand on.
func TestRunnerVerifierDefectAbortsInsteadOfFallingBack(t *testing.T) {
	lg := &logtest.Capture{}
	client := echoStub()
	ask := fallbackBackedAsk(t)
	ask.Verify = func(string, string) (any, error) {
		return nil, fmt.Errorf("%w: offsets do not match their bytes", ErrVerifierDefect)
	}
	_, call := newRunnerCall(t, ask)

	res, err := runnerFor(t, client, lg).Run(context.Background(), call)
	var abort WorkerAbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err = %v (%T), want a WorkerAbortError", err, err)
	}
	if !errors.Is(err, ErrVerifierDefect) {
		t.Error("the abort must carry the verifier's own diagnosis")
	}
	if res.Artifact != nil || res.FellBack {
		t.Errorf("result = %+v; a defect produces nothing, degraded or otherwise", res)
	}
	if client.callCount() != 1 {
		t.Errorf("calls = %d, want 1; a defect is never retried", client.callCount())
	}
}

// TestRunnerSeamResolution is R-4 step 6, both branches side by side: the same
// unverifiable model output degrades a fallback-backed seam and fails an essential
// one. That difference IS why seams are classified.
func TestRunnerSeamResolution(t *testing.T) {
	rejectAlways := func() *stubClient {
		return &stubClient{respond: func(int, model.Request) (model.Response, error) {
			return model.Response{Content: "still not data", FinishReason: "stop"}, nil
		}}
	}

	t.Run("refinement keeps its fallback", func(t *testing.T) {
		lg := &logtest.Capture{}
		client := rejectAlways()
		_, call := newRunnerCall(t, fallbackBackedAsk(t))

		res, err := runnerFor(t, client, lg).Run(context.Background(), call)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if !res.FellBack {
			t.Error("FellBack = false; a fallback is a quality loss and must say so")
		}
		artifact, ok := res.Artifact.(synthArtifact)
		if !ok || artifact.Text != synthBaselineText {
			t.Errorf("Artifact = %v, want the mechanical fallback", res.Artifact)
		}
		if client.callCount() != modelAttempts {
			t.Errorf("calls = %d, want %d", client.callCount(), modelAttempts)
		}
		if !lg.Has(t, "warn", "kind", string(FailureVerification)) {
			t.Error("the fallback was not logged")
		}
	})

	t.Run("essential fails the unit", func(t *testing.T) {
		client := rejectAlways()
		_, call := newRunnerCall(t, noFallbackAsk(t))

		res, err := runnerFor(t, client, &logtest.Capture{}).Run(context.Background(), call)
		var target OwedArtifactFailure
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want OwedArtifactFailure", err)
		}
		if target.Kind != FailureVerification || target.ModelAttempts != modelAttempts {
			t.Errorf("failure = %+v, want %s after %d attempts", target, FailureVerification, modelAttempts)
		}
		if target.Path != call.ArtifactPath || target.Stage != call.Stage {
			t.Errorf("failure names %s/%s, want %s/%s", target.Stage, target.Path, call.Stage, call.ArtifactPath)
		}
		if res.Artifact != nil {
			t.Error("a no-fallback seam must produce NOTHING when it cannot verify")
		}
	})

	t.Run("transport exhaustion resolves the same way", func(t *testing.T) {
		client := &stubClient{respond: func(int, model.Request) (model.Response, error) {
			return model.Response{}, errors.New("http 502: bad gateway")
		}}
		_, call := newRunnerCall(t, noFallbackAsk(t))

		_, err := runnerFor(t, client, &logtest.Capture{}).Run(context.Background(), call)
		var target OwedArtifactFailure
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want OwedArtifactFailure", err)
		}
		if target.Kind != FailureTransport {
			t.Errorf("Kind = %s, want %s", target.Kind, FailureTransport)
		}
		// Nothing was verified, so there is nothing to correct: a semantic
		// retry after a dead wire is a second guess, not an informed one.
		if client.callCount() != wireAttempts {
			t.Errorf("calls = %d, want %d — no semantic retry after transport exhaustion",
				client.callCount(), wireAttempts)
		}
	})
}

// TestRunnerUnmappedTier: naming a tier config does not map is a configuration
// failure, and substituting the other tier's model would be a silent wrong
// answer.
func TestRunnerUnmappedTier(t *testing.T) {
	_, call := newRunnerCall(t, noFallbackAsk(t))
	runner := NewCallRunner(echoStub(), synthConfigWithout(t, call.BoundAsk.ask.Tier), &logtest.Capture{})

	if _, err := runner.Run(context.Background(), call); err == nil ||
		!strings.Contains(err.Error(), "no model is configured") {
		t.Fatalf("err = %v, want an unmapped-tier refusal", err)
	}
}

// TestRunnerDevTelemetry: the timing block is gated on the [dev] switch AND on
// the provider actually sending one, and it goes out as fields rather than as
// the preformatted line.
func TestRunnerDevTelemetry(t *testing.T) {
	withTelemetry := func() *stubClient {
		return &stubClient{respond: func(int, model.Request) (model.Response, error) {
			return model.Response{
				Content:      synthAccept + " fine",
				FinishReason: "stop",
				Usage: model.Usage{
					PromptTokens: 100,
					Telemetry: model.InferenceTelemetry{
						TimeToFirstToken:       250 * time.Millisecond,
						PrefillDuration:        200 * time.Millisecond,
						GenerationDuration:     6 * time.Second,
						TotalDuration:          6250 * time.Millisecond,
						PrefillTokensPerSecond: 500,
					},
				},
			}, nil
		}}
	}

	t.Run("emitted when switched on and present", func(t *testing.T) {
		lg := &logtest.Capture{}
		cfg := synthConfig()
		cfg.Dev.Telemetry = true
		_, call := newRunnerCall(t, noFallbackAsk(t))

		if _, err := NewCallRunner(withTelemetry(), cfg, lg).Run(context.Background(), call); err != nil {
			t.Fatalf("Run: %v", err)
		}
		if !lg.Has(t, "info", "generation", 6*time.Second) {
			t.Error("telemetry was not recorded as typed fields")
		}
	})

	t.Run("silent when switched off", func(t *testing.T) {
		lg := &logtest.Capture{}
		_, call := newRunnerCall(t, noFallbackAsk(t))

		if _, err := runnerFor(t, withTelemetry(), lg).Run(context.Background(), call); err != nil {
			t.Fatalf("Run: %v", err)
		}
		if lg.Count("info", "generation", 6*time.Second) != 0 {
			t.Error("telemetry leaked out with the [dev] switch off")
		}
	})

	t.Run("silent when the provider sends none", func(t *testing.T) {
		lg := &logtest.Capture{}
		cfg := synthConfig()
		cfg.Dev.Telemetry = true
		_, call := newRunnerCall(t, noFallbackAsk(t))

		if _, err := NewCallRunner(echoStub(), cfg, lg).Run(context.Background(), call); err != nil {
			t.Fatalf("Run: %v", err)
		}
		for _, r := range lg.Snapshot() {
			if r.Msg == "call telemetry" {
				t.Error("an all-zero telemetry line was emitted; absence is normal, not a fault")
			}
		}
	})
}

// TestRunnerFailedUnitReportsItsTokens: the tokens a failed essential unit
// burned are the ones an accounting most needs, and the failure used to
// discard them — the units that cost the most and delivered nothing were the
// ones missing from the total.
func TestRunnerFailedUnitReportsItsTokens(t *testing.T) {
	client := &stubClient{respond: func(int, model.Request) (model.Response, error) {
		return model.Response{
			Content: "still not data", FinishReason: "stop",
			Usage: model.Usage{PromptTokens: 100, CompletionTokens: 10, TotalTokens: 110},
		}, nil
	}}
	_, call := newRunnerCall(t, noFallbackAsk(t))

	_, err := runnerFor(t, client, &logtest.Capture{}).Run(context.Background(), call)
	var target OwedArtifactFailure
	if !errors.As(err, &target) {
		t.Fatalf("err = %v, want OwedArtifactFailure", err)
	}
	if want := modelAttempts * 100; target.Usage.PromptTokens != want {
		t.Errorf("Usage.PromptTokens = %d, want %d — both attempts cost real tokens",
			target.Usage.PromptTokens, want)
	}
}

func TestAddUsage(t *testing.T) {
	a := model.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12, CachedPromptTokens: 4,
		Telemetry: model.InferenceTelemetry{TotalDuration: time.Second}}
	b := model.Usage{PromptTokens: 5, CompletionTokens: 1, TotalTokens: 6, ReasoningTokens: 3,
		Telemetry: model.InferenceTelemetry{TotalDuration: time.Second}}

	got := addUsage(a, b)
	want := model.Usage{PromptTokens: 15, CompletionTokens: 3, TotalTokens: 18, ReasoningTokens: 3, CachedPromptTokens: 4}
	if got != want {
		t.Errorf("addUsage = %+v, want %+v", got, want)
	}
	// Summed durations describe no call that ever ran, so the aggregate
	// carries none.
	if got.HasTelemetry() {
		t.Error("telemetry was summed into the stage total")
	}
}
