package pipeline

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/pipeline/crashpoint"
	"kbase/internal/prompt"
	"kbase/internal/text"
)

// Retry policy (ARCHITECTURE.md §9). Two policies, deliberately separate,
// because the two failure classes say different things: a transport failure
// says nothing at all about the output (§12), while a verification failure
// says the model produced the wrong thing and a blind resend would only hope
// temperature fixes it.
//
// The wire policy is the runner's, and the constants below are it: network
// weather is the same weather for every ask, so nothing about a definition
// could inform it. The MODEL policy is not — how many semantic attempts an ask
// gets, how hard the retry asks and how much of the reason it carries are
// properties of the question, declared per definition (RetryPolicy). What is
// left here of that half is the two defaults a definition inherits by saying
// nothing.
const (
	// wireAttempts is how many times ONE call goes on the wire before
	// the runner concludes the network is not going to cooperate. Three is
	// the smallest count that survives a single blip plus its retry landing
	// in the same blip; more attempts on a batch job spend real minutes per
	// unit for a case a resume already handles.
	wireAttempts = 3

	// wireBackoffBase is the wait before the second attempt; each
	// further attempt doubles it (500ms, then 1s). Half a second is long
	// enough for a load balancer to pick a different backend and short
	// enough to be invisible against a call that takes tens of seconds.
	wireBackoffBase = 500 * time.Millisecond

	// defaultModelAttempts is the first attempt plus ONE informed retry — the
	// §9 "1 retry, then mechanical fallback" policy. The retry is worth making
	// only because it carries the mechanical failure reason; a second one
	// would be the blind resend that was ruled out.
	//
	// It is the value a definition that states no count gets (DeclareRetry),
	// not the value the runner applies: the policy is per-definition, and this
	// is what "unchanged" means for every definition that has not moved off it.
	defaultModelAttempts = 2

	// defaultRetryNoteWords bounds the machine-generated retry note. The
	// acceptance-criteria channel has a reserved word share (§9) that the
	// stage's own criteria are also spending, so the note takes a small,
	// fixed bite of it. A verifier's message is one sentence of mechanical
	// fact ("cuts do not tile: gap at 4120"); twelve words carries that and
	// truncates only prose nobody should be writing there.
	//
	// The same default-not-policy reading as defaultModelAttempts above.
	defaultRetryNoteWords = 12

	// promptHashChars is how much of a prompt's digest identifies it in the
	// provenance log: sixteen hex characters, eight bytes. Both attempts of a
	// call record theirs, so a rejected response and its informed retry can be
	// told apart — and shown to have been different requests — from the log
	// alone. This is an identity within one run's log, not a cryptographic
	// commitment, and a full digest in every record is fifty-odd characters of
	// noise around the few that distinguish anything.
	promptHashChars = 16

	// retryNotePrefix labels the note as machine-generated feedback
	// rather than a criterion the tree plan emitted. Kept to four words so
	// the note as a whole stays well inside the reserved share.
	retryNotePrefix = "- previous attempt rejected:"
)

// ErrVerifierDefect marks a verification failure that is KBASE's defect
// rather than the model's answer being wrong.
//
// The seam has two failure classes and they take opposite remedies. A
// response that failed a mechanical post-condition is a rejection: retry once
// with the reason, then keep the mechanical fallback (§3 monotone safety). A
// response that failed a check no legal answer could fail — ARCHITECTURE.md
// §5's whitespace-adjacency tripwire is the first of these, and the argument
// for why it cannot be the model's doing is in internal/dissect — indicts the
// derivation that produced BOTH the question and the fallback. Retrying it
// re-asks something that was never asked wrong, and falling back to the
// fallback trusts the same derivation, so the worker aborts instead.
//
// A verifier reports one by wrapping it (fmt.Errorf("%w: %w", …)), which
// keeps the classification with the package that can make it and out of the
// runner, which cannot.
var ErrVerifierDefect = errors.New("pipeline: the verifier reported a kbase defect, not a model failure")

// Crashpoints around a model call. pre-transport is the interesting one: it
// kills with the prompt built and nothing sent, which is the state a resume
// must treat as "this unit never happened". verified kills with a good
// artifact in hand and no write yet — the work is done and unrecorded, which
// is exactly the case that must cost tokens on resume rather than be guessed at.
var (
	cpCallPreTransport = crashpoint.Register("pipeline.runner.pre-transport")
	cpCallVerified     = crashpoint.Register("pipeline.runner.verified")
)

// CallRunner is the one path every model call takes (ARCHITECTURE.md §12,
// "Call runner protocol"). Concentrating build, assertion, transport,
// verification, retry and seam resolution here is what makes those policies
// facts of the system rather than habits each stage re-implements.
//
// It is stateless and safe for concurrent use: every worker of a stage shares
// one runner, and everything that varies per call arrives in the call.
type CallRunner struct {
	client model.Client
	cfg    config.Config
	lg     log.Logger

	// backoff is the first inter-attempt wait, wireBackoffBase in every
	// production path. It is a field only so the tests that count attempts
	// need not spend the real waits to do it; the one test that asserts the
	// wait actually happens uses the constant.
	backoff time.Duration
}

// NewCallRunner returns a runner over client, resolving tiers through cfg and
// logging through lg (log.Discard for a caller with nothing to hand it).
func NewCallRunner(client model.Client, cfg config.Config, lg log.Logger) *CallRunner {
	return &CallRunner{client: client, cfg: cfg, lg: lg, backoff: wireBackoffBase}
}

// call is one unit's model call.
type call struct {
	// Stage and ArtifactPath name the work, for the log and for a OwedArtifactFailure.
	Stage        string
	ArtifactPath string

	// BoundAsk is the stage's shared bound ask.
	BoundAsk *boundAsk

	// Input is the per-call half of the prompt.
	Input prompt.CallInput

	// Frontier is the stability claim this call makes — the caller's
	// LowestFrontier over the phases it traversed since its last build.
	// prompt.SlotTotal claims nothing, which is what a worker's first call
	// (and its first call after a failure) has to say.
	Frontier prompt.Slot

	// Prev is the hashes of the caller's previous built call, nil when there
	// is none. A non-zero Frontier with a nil Prev is refused by the
	// tripwire rather than passed.
	Prev map[prompt.Slot][32]byte

	// PreserveRejected keeps the raw bytes of a response that failed
	// mechanical verification and returns where it kept them, for the record
	// that reports the rejection. It is the caller's because the runner has no
	// store — the worker does, and it is the half of this seam that knows where
	// a unit lives.
	//
	// Why the bytes are kept at all: a verifier's rejection reason says what
	// RULE the response broke, and a post-mortem needs what the response
	// actually WAS. The 2026-08-18 shakedown could name a rejection class and
	// not one instance of it, which is the difference between "the answers
	// were malformed" and knowing how.
	//
	// Optional. A caller with nowhere to put them supplies nothing and the
	// rejection is reported without a location, which is what every test that
	// counts attempts wants.
	PreserveRejected func(attempt int, response string) string
}

// CallResult is what one unit's call produced.
type CallResult struct {
	// Artifact is the verified, typed artifact — or the mechanical fallback
	// when a fallback-backed seam fell back. Never raw model text.
	Artifact any

	// Usage is the sum over every attempt this call made, which is what the
	// unit actually cost.
	Usage model.Usage

	// FellBack reports a fallback-backed seam that kept its fallback: the unit is
	// correct and less good than it was meant to be.
	FellBack bool

	// Attempts is how many model attempts ran, up to the definition's declared
	// RetryPolicy.Attempts.
	Attempts int

	// Hashes is the last built call's per-slot hashes, which the caller
	// carries forward as the next call's Prev.
	Hashes map[prompt.Slot][32]byte
}

// Run executes R-4's sequence for one call:
//
//	build → frozen-prompt assertion → transport (with backoff) → mechanical
//	verification → ONE informed retry → seam resolution
//
// Failure routing, by class:
//
//   - Build refusal (prompt.ErrOverBudget and friends) propagates UNTOUCHED on
//     the first attempt. It is refuse-and-split: the unit goes back to the
//     stage that can make it smaller, and nothing here can help.
//   - Frozen-prompt violation is a kbase defect: WorkerAbortError, never
//     retried, never fallen back.
//   - Context cancellation returns the context's error. It is not network
//     weather and not a model failure; the job is stopping.
//   - Transport exhaustion and verification exhaustion both mean "no verified
//     artifact", so both land in seam resolution: a fallback-backed seam keeps its
//     fallback and is marked as fallen back, a no-fallback seam returns OwedArtifactFailure
//     and produces nothing.
//
// How many attempts there are, how hard each one asks and how much of the
// rejection reason the retry carries are the DEFINITION's (AskSpec.Retry). The
// runner reads them; it decides none of them, for the same reason it has never
// decided the effort — it does not know what is being asked.
func (r *CallRunner) Run(ctx context.Context, c call) (CallResult, error) {
	modelID, ok := r.cfg.ModelFor(c.BoundAsk.ask.Tier)
	if !ok {
		return CallResult{}, fmt.Errorf("pipeline: %s: no model is configured for the %q tier", c.Stage, c.BoundAsk.ask.Tier)
	}

	var (
		res     CallResult
		lastErr error
		kind    FailureKind
		retry   = c.BoundAsk.ask.Retry
		in      = c.Input
	)
	for attempt := 1; attempt <= retry.Attempts; attempt++ {
		// The first attempt asks what the definition says the question costs;
		// every attempt after it asks what the definition says the question
		// costs once the previous answer has been named as wrong.
		effort := c.BoundAsk.ask.Effort
		if attempt > 1 {
			effort = retry.Effort
		}
		built, err := c.BoundAsk.ctx.Build(in)
		if err != nil {
			if attempt == 1 {
				return CallResult{}, err
			}
			// The informed retry could not be assembled — most plausibly
			// the retry note pushed the acceptance criteria over their
			// reserved share. The retry simply does not happen; monotone
			// safety still owes this unit its seam resolution, and skipping
			// that would turn a recoverable model failure into a hard one.
			r.lg.Warn("retry could not be built; resolving the seam without it",
				"stage", c.Stage, "unit", c.ArtifactPath, "error", err)
			break
		}
		res.Hashes = built.Hashes
		res.Attempts = attempt
		if err := r.assertFrozen(c, built); err != nil {
			return CallResult{}, err
		}
		if built.Warning != "" {
			r.lg.Warn("call is over the per-call target", "stage", c.Stage, "unit", c.ArtifactPath, "detail", built.Warning)
		}
		hash := HashBytes([]byte(built.UserTurn))[:promptHashChars]
		if attempt > 1 {
			// The retry, on the record: what it is asking at, and whether that
			// is an escalation over the first attempt. A definition that
			// escalates buys reasoning tokens on some fraction of its calls,
			// and this is the line that says which ones.
			//
			// At WARN, unlike the telemetry records, and the test is whether it
			// can fire on a healthy run: it cannot. It is emitted only after an
			// attempt was rejected, which is a warn already — so this costs the
			// default channel nothing and its absence would leave a reader
			// seeing the failure with no record of the remedy.
			r.lg.Warn("informed retry", "stage", c.Stage, "unit", c.ArtifactPath,
				"attempt", attempt, "of", retry.Attempts, "prompt", hash,
				"thinking", effort.Thinking,
				"escalated", effort.Thinking && !c.BoundAsk.ask.Effort.Thinking)
		}

		resp, err := r.transport(ctx, modelID, built.UserTurn, c, effort)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return CallResult{}, ctxErr
			}
			lastErr, kind = err, FailureTransport
			r.lg.Warn("call failed on the wire", "stage", c.Stage, "unit", c.ArtifactPath,
				"attempt", attempt, "prompt", hash, "outcome", "transport-exhausted", "error", err)
			// A retry with a retry note is a semantic remedy for a
			// semantic problem. Nothing was verified here, so there is
			// nothing to correct.
			break
		}
		res.Usage = addUsage(res.Usage, resp.Usage)
		r.account(c, resp.Usage)

		artifact, verr := c.BoundAsk.ask.Verify(c.ArtifactPath, resp.Content)
		if verr == nil {
			r.lg.Debug("call verified", "stage", c.Stage, "unit", c.ArtifactPath,
				"attempt", attempt, "prompt", hash, "outcome", "verified")
			crashpoint.At(cpCallVerified)
			res.Artifact = artifact
			return res, nil
		}
		if errors.Is(verr, ErrVerifierDefect) {
			// Not a rejection: the verifier is saying the response could not
			// be wrong in this way, so the fault is ours. Retrying re-asks a
			// question that was never the problem, and falling back would
			// trust the same broken derivation, so the worker stops.
			return CallResult{}, WorkerAbortError{Stage: c.Stage, ArtifactPath: c.ArtifactPath, Err: verr}
		}
		lastErr, kind = verr, FailureVerification
		rec := []any{"stage", c.Stage, "unit", c.ArtifactPath,
			"attempt", attempt, "prompt", hash, "outcome", "rejected", "reason", verr}
		if c.PreserveRejected != nil {
			if at := c.PreserveRejected(attempt, resp.Content); at != "" {
				rec = append(rec, "response", at)
			}
		}
		r.lg.Warn("response failed mechanical verification", rec...)
		in = withRetryNote(c.Input, verr, retry.NoteWords)
	}
	return r.resolveSeam(c, res, kind, lastErr)
}

// assertFrozen is the frozen-prompt assertion: the per-worker churn tripwire
// plus the stage-level canonical-hash check. Both are OUR contract, so a violation
// is a defect in kbase and not a condition to recover from — the worker aborts
// and its siblings drain.
func (r *CallRunner) assertFrozen(c call, built prompt.BuiltCall) error {
	if err := prompt.CheckStability(c.Frontier, c.Prev, built.Hashes); err != nil {
		return WorkerAbortError{Stage: c.Stage, ArtifactPath: c.ArtifactPath, Err: err}
	}
	if err := c.BoundAsk.checkCanonicalHashes(built.Hashes); err != nil {
		return WorkerAbortError{Stage: c.Stage, ArtifactPath: c.ArtifactPath, Err: err}
	}
	return nil
}

// transport puts one built call on the wire, retrying network weather with
// bounded exponential backoff. ConsultDrained rather than Consult: a long
// generation on a blocking request looks like a dead connection to every idle
// timeout in the path.
//
// The effort is the ROLE's — the definition's declaration, threaded from the
// registration site through the bound ask to here. The runner picks nothing: it
// has no idea what is being asked, which is exactly why it is not the layer
// that gets to say how hard to ask it. It is a parameter rather than read off
// the ask because ONE ask has two of them: the first attempt's and the informed
// retry's (RetryPolicy), and which one this call is, is Run's to know.
func (r *CallRunner) transport(ctx context.Context, modelID, turn string, c call, effort model.RequestEffort) (model.Response, error) {
	req := model.DefaultRequest(modelID, []model.Message{{Role: "user", Content: turn}}, effort)
	var lastErr error
	for attempt := 1; attempt <= wireAttempts; attempt++ {
		if attempt > 1 {
			if err := sleep(ctx, r.backoff<<(attempt-2)); err != nil {
				return model.Response{}, err
			}
		}
		crashpoint.At(cpCallPreTransport)
		resp, err := model.ConsultDrained(ctx, r.client, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !retryableTransport(ctx, err) {
			break
		}
		r.lg.Warn("transport failure; retrying", "stage", c.Stage, "unit", c.ArtifactPath,
			"attempt", attempt, "of", wireAttempts, "error", err)
	}
	return model.Response{}, fmt.Errorf("transport: %w", lastErr)
}

// resolveSeam is R-4's step 6: what happens when no verified artifact came
// back. The branch is the whole point of classifying seams — a fallback-backed seam
// has somewhere valid to stand and an essential one does not, so pretending
// otherwise would emit unverified material.
func (r *CallRunner) resolveSeam(c call, res CallResult, kind FailureKind, cause error) (CallResult, error) {
	if c.BoundAsk.Seam() == FallbackBackedSeam {
		res.Artifact = c.BoundAsk.ask.Fallback(c.ArtifactPath)
		res.FellBack = true
		r.lg.Warn("keeping the mechanical fallback; the model's refinement did not verify",
			"stage", c.Stage, "unit", c.ArtifactPath, "attempts", res.Attempts, "kind", string(kind), "reason", cause)
		return res, nil
	}
	// The tokens a failed essential unit burned are its own: two semantic
	// attempts that produced nothing still cost what they cost, and the
	// aggregate that left them out was understating exactly the units that
	// cost the most and delivered least.
	return CallResult{}, OwedArtifactFailure{
		Stage:         c.Stage,
		Path:          c.ArtifactPath,
		Kind:          kind,
		ModelAttempts: res.Attempts,
		Usage:         res.Usage,
		Err:           cause,
	}
}

// account records what the call cost. Two records, because they answer to two
// different readers: cached_tokens is the wire-level ground truth that the §7
// churn prevention is holding against a real prefix cache, and it is logged on
// every call regardless of configuration. The telemetry block is a de-facto
// provider extension that is absent on most providers and useful only to
// someone diagnosing kbase, so it is gated on the [dev] switch AND on the
// block actually being present.
//
// The durations go in as time.Duration values rather than as the preformatted
// TelemetryLogDetail line: a structured record whose fields are one opaque
// string is a string, and the point of the logging seam is that a field is a
// field.
func (r *CallRunner) account(c call, u model.Usage) {
	r.lg.Debug("call usage", "stage", c.Stage, "unit", c.ArtifactPath,
		"prompt_tokens", u.PromptTokens, "cached_tokens", u.CachedPromptTokens,
		"completion_tokens", u.CompletionTokens, "reasoning_tokens", u.ReasoningTokens)

	if !r.cfg.Dev.Telemetry || !u.HasTelemetry() {
		return
	}
	t := u.Telemetry
	r.lg.Info("call telemetry", "stage", c.Stage, "unit", c.ArtifactPath,
		"time_to_first_token", t.TimeToFirstToken, "prefill", t.PrefillDuration,
		"generation", t.GenerationDuration, "total", t.TotalDuration,
		"prefill_tps", t.PrefillTokensPerSecond, "generation_tps", t.GenerationTokensPerSecond,
		"prompt_tokens", u.PromptTokens, "completion_tokens", u.CompletionTokens,
		"cached_tokens", u.CachedPromptTokens)
}

// withRetryNote returns a copy of in carrying the mechanical failure
// reason in the acceptance-criteria channel — the trailer, where the per-call
// binding facts already live and where the model is most likely to still be
// reading by the time it starts generating.
//
// It is built from the ORIGINAL input, not from the previous attempt's, so a
// second note could never stack on a first. words is the definition's declared
// budget for it (RetryPolicy.NoteWords).
func withRetryNote(in prompt.CallInput, cause error, words int) prompt.CallInput {
	out := in
	note := retryNotePrefix + " " + text.CapWords(cause.Error(), words)
	out.AcceptanceCriteria = slices.Concat(in.AcceptanceCriteria, []string{note})
	return out
}

// sleep waits d, or returns the context's error if the job stops first.
// time.NewTimer rather than time.After: a timer left to fire on its own is a
// leak per abandoned wait, and this one is abandoned exactly when a job is
// being cancelled.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// retryableTransport reports whether a transport error is worth another
// attempt.
//
// Cancellation never is: the job is stopping, and retrying is the one response
// that ignores it. A 4xx other than 429 never is either — the provider is
// saying the request itself is wrong, and the identical request will be just as
// wrong three times. 429 IS retryable despite being a 4xx, which is the whole
// of "429-aware": it is the one client error that means "later", and backing
// off is the correct response to it.
func retryableTransport(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var status model.StatusError
	if !errors.As(err, &status) {
		// Not a status at all — a dial failure, a dropped lane, a
		// timeout. Retrying is the right default for every one of them.
		return true
	}
	if status.Code == http.StatusTooManyRequests {
		return true
	}
	return status.Code < 400 || status.Code >= 500
}

// addUsage sums two usage blocks' token counts.
//
// Telemetry is deliberately not summed. The inference-timing block describes
// the shape of ONE call — time to first token, prefill against generation —
// and adding two of them produces numbers that describe no call that ever ran.
// Stage totals are token accounting; timing stays per-call, in the log.
func addUsage(a, b model.Usage) model.Usage {
	return model.Usage{
		PromptTokens:       a.PromptTokens + b.PromptTokens,
		CompletionTokens:   a.CompletionTokens + b.CompletionTokens,
		TotalTokens:        a.TotalTokens + b.TotalTokens,
		ReasoningTokens:    a.ReasoningTokens + b.ReasoningTokens,
		CachedPromptTokens: a.CachedPromptTokens + b.CachedPromptTokens,
	}
}

// FailureKind classifies a unit failure for the inventory a refused job
// reports. The classes exist because they take different remedies: a budget
// failure is re-split at the tree plan, a verification failure is a prompt or
// model problem, a transport failure is a resume once the provider is back,
// and a write failure is the filesystem.
type FailureKind string

const (
	// FailureVerification is a no-fallback seam whose response did not verify
	// on either attempt.
	FailureVerification FailureKind = "verification"
	// FailureTransport is a no-fallback seam whose call never completed.
	FailureTransport FailureKind = "transport"
	// FailureBudget is a build refusal — refuse-and-split (§3). The unit is
	// too big for one call and belongs back at the stage that sizes units.
	FailureBudget FailureKind = "budget"
	// FailureUpstream is a unit whose inputs could not be derived because
	// something it depends on was never produced. It costs no tokens: the
	// dependency is checked before the model is consulted, since a unit that
	// cannot be recorded is not worth generating. It is the cascade of some
	// other failure, and the remedy is that one.
	FailureUpstream FailureKind = "upstream"
	// FailureCascade is a unit that was never attempted because a unit it
	// consumes failed EARLIER IN THIS RUN (see markCascades). It is distinct
	// from FailureUpstream so the inventory separates root cause from
	// consequence: an upstream failure says the store cannot supply an input,
	// a cascade names the failure whose remedy is the remedy for both.
	FailureCascade FailureKind = "cascade"
	// FailureWrite is a verified artifact that could not be encoded or
	// stored.
	FailureWrite FailureKind = "write"
	// FailureProduce is a mechanical unit whose own derivation failed
	// (LaneTask.Produce). It costs no tokens and has no retry: the same inputs
	// derive the same failure, so the remedy is upstream data or a kbase
	// defect fix, never asking again.
	FailureProduce FailureKind = "produce"
)

// OwedArtifactFailure is one unit the job could not produce. It is an error so a
// caller can errors.As it out of a call, and a value in the job's inventory so
// the report lists every one rather than the first.
//
// A unit failure is NOT fatal to the job: siblings finish (graceful
// degradation), the inventory is reported, and emission is refused at
// assembly — "sorry, something rotted" beats "here is your invalid crap".
type OwedArtifactFailure struct {
	Stage string
	Path  string
	Kind  FailureKind

	// ModelAttempts is how many times the unit was ASKED — the first
	// attempt plus the definition's informed retries, never the wire attempts
	// underneath them. The distinction is the whole reason the two retry
	// policies are separate: a transport failure reports one model attempt
	// over three wire requests, and a reader deciding whether to blame the
	// provider needs the message to say the first rather than imply the
	// second.
	ModelAttempts int

	// Usage is what the unit burned before it failed. A failed essential
	// unit is not free, and an accounting that treated it as free would
	// understate precisely the units worth looking at.
	Usage model.Usage

	Err error
}

func (e OwedArtifactFailure) Error() string {
	attempts := "attempts"
	if e.ModelAttempts == 1 {
		attempts = "attempt"
	}
	return fmt.Sprintf("pipeline: %s: unit %s failed (%s, %d semantic %s): %v",
		e.Stage, e.Path, e.Kind, e.ModelAttempts, attempts, e.Err)
}

func (e OwedArtifactFailure) Unwrap() error { return e.Err }

// WorkerAbortError reports a kbase defect a worker cannot run through. It has
// two causes and they are the same failure seen from two places: the
// frozen-prompt assertion firing (the prompt bytes this run is sending are not
// the ones its own stability contract says it is sending), and a verifier
// declaring the failure OURS by wrapping ErrVerifierDefect (the response could
// not be wrong in the way the check found, so the derivation that produced
// both the question and the fallback is what is broken).
//
// It is never retried and never falls back, because neither would address it.
// The unit is not what failed — the orchestrator's model of its own state is,
// and every later call from that worker would be built on the same wrong
// belief. So the worker stops and the job reports a defect.
type WorkerAbortError struct {
	Stage        string
	ArtifactPath string
	Err          error
}

func (e WorkerAbortError) Error() string {
	return fmt.Sprintf("pipeline: %s: worker aborted at unit %s: %v", e.Stage, e.ArtifactPath, e.Err)
}

func (e WorkerAbortError) Unwrap() error { return e.Err }
