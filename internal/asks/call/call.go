// Package call puts a build's asks to a provider, each item of an ask batch
// on a slot of the build's queue: each composed prompt under the system
// fragment its kind names, as one chat call re-issued identically while it
// does not complete, every attempt captured, and the answer cached by what was
// asked so a resumed stage asks nothing it already has an answer for. An
// item's captures and answers are written once it has returned its token.
package call

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"kbase/internal/asks"
	"kbase/internal/atomicfile"
	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
)

var fileNameUnsafeRE = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

const (
	fileStemMaxChars = 150
	promptHashChars  = 12

	// overviewName is the overview call's name: its capture files' stem.
	overviewName = "overview"
)

// overviewBackoff is the pause before each re-issue of an overview call that
// did not complete; a multichoice ask is re-issued at once.
var overviewBackoff = []time.Duration{5 * time.Second, 30 * time.Second}

// Caller puts composed prompts to one provider. Safe for concurrent use over
// distinct prompts.
type Caller struct {
	client model.Client
	// provider is the pool entry client dials, which a remedy names.
	provider        string
	trail           func() string
	overviewBackoff []time.Duration
	lg              log.Logger
}

// New is a Caller over client, which dials the pool entry provider, keeping
// its answers and captures in the trail directory trail returns, asked at
// every call.
func New(client model.Client, provider string, trail func() string, lg log.Logger) *Caller {
	return &Caller{
		client:          client,
		provider:        provider,
		trail:           trail,
		overviewBackoff: overviewBackoff,
		lg:              lg,
	}
}

// keyRemedy is the remedy a failed call's err names: where the provider
// refused the call for its key, the command that stores one for it; else "".
func (c *Caller) keyRemedy(err error) string {
	var se model.StatusError
	if errors.As(err, &se) && se.RefusedKey() {
		return config.AuthAddRemedy(c.provider)
	}
	return ""
}

// incomplete is the call that never completed on err: detail as given, and
// where the provider refused the call for its key, the remedy and the note
// that ends the detail.
func (c *Caller) incomplete(subject, detail string, err error) asks.IncompleteError {
	remedy := c.keyRemedy(err)
	if remedy != "" {
		detail += "; " + config.RefusedKeyNote
	}
	return asks.IncompleteError{Subject: subject, Detail: detail, Remedy: remedy}
}

func (c *Caller) answers() string { return filepath.Join(c.trail(), asks.AnswersDir) }

func (c *Caller) captures() string { return filepath.Join(c.trail(), asks.CapturesDir) }

// Launch is the production asks.Launch: each item started on q, its reader
// putting each question to modelID as one Decision call through the item's
// slot — the composed state the request's state, the question serialized
// under its item's key — the answer mapped to a confidence keyed by the
// offered letters, and the mapped answer cached by the digest of the model,
// the question and the state.
func (c *Caller) Launch(q *model.Queue, modelID string) asks.Launch {
	return func(ctx context.Context, wg *sync.WaitGroup, item func(asks.MultichoiceReader) func(error)) error {
		return c.goHolding(ctx, q, wg, func(slot *model.Slot, held *heldFiles) func(error) {
			return item(func(ctx context.Context, lq asks.MultichoiceQuestion) (asks.Reply, error) {
				conf, err := c.decide(ctx, slot, held, modelID, lq)
				if err != nil {
					return asks.Reply{}, err
				}
				return asks.Reply{Confidence: conf}, nil
			})
		})
	}
}

// PassageLaunch starts the overview's item on q, its asker putting the
// overview passage to modelID from excerpts through the item's slot,
// rejected holding the lines a previous reply was refused for, attempt
// counting the asks from 1.
func (c *Caller) PassageLaunch(q *model.Queue, modelID string) asks.PassageLaunch {
	return func(ctx context.Context, wg *sync.WaitGroup, item func(func(context.Context, string, []string, int) (string, error)) func(error)) error {
		return c.goHolding(ctx, q, wg, func(slot *model.Slot, held *heldFiles) func(error) {
			return item(func(ctx context.Context, excerpts string, rejected []string, attempt int) (string, error) {
				prompt, err := asks.OverviewPrompt(excerpts, rejected)
				if err != nil {
					return "", err
				}
				reply, err := c.ask(ctx, slot, held, modelID, asks.OverviewSystem, prompt, attempt, c.overviewBackoff, overviewName,
					&overviewInputs{Excerpts: excerpts, Rejected: rejected})
				return reply.Text, err
			})
		})
	}
}

// goHolding starts item on q: every call it makes goes through its slot, and
// the files those calls leave are held in memory until it returns, then
// written in its after, on no token. The item's own after is handed the error
// of that write; one that returned none has already failed, and a write error
// beside its failure is logged.
func (c *Caller) goHolding(ctx context.Context, q *model.Queue, wg *sync.WaitGroup, item func(*model.Slot, *heldFiles) func(error)) error {
	return q.Go(ctx, wg, func(slot *model.Slot) func() {
		held := &heldFiles{}
		after := item(slot, held)
		return func() {
			err := held.write()
			if after != nil {
				after(err)
				return
			}
			if err != nil {
				c.lg.Warn("a failed item's captures and answers were not all written", "error", err)
			}
		}
	})
}

// heldFiles is the captures and answers one item's calls leave, in the order
// they were made, held until the item has returned its token.
type heldFiles struct {
	files []heldFile
}

type heldFile struct {
	dir, path string
	data      []byte
}

func (h *heldFiles) hold(dir, path string, data []byte) {
	h.files = append(h.files, heldFile{dir: dir, path: path, data: data})
}

// write writes every held file, each whatever another's write did, and
// returns their errors joined.
func (h *heldFiles) write() error {
	var errs []error
	for _, f := range h.files {
		if err := writeFileIn(f.dir, f.path, f.data); err != nil {
			errs = append(errs, fmt.Errorf("call: writing %s: %w", f.path, err))
		}
	}
	return errors.Join(errs...)
}

// decide is one question's Decision round-trip: the cached answer where one
// stands for these exact inputs, else the call through slot, its capture and
// its answer held in held.
//
// The mapping, per answer type, keyed by which criterion the letter names,
// never by offered order: a choice answer carries the probabilities whole;
// without them its own confidence sits under the chosen letter; with neither,
// the answer is unreadable and takes the re-ask/default path. A noul answer
// carries one figure read as P(true) — the OpenRouter example's reading,
// confirmed against the live provider before the hermetic suite treats it as
// fixed — so the true letter takes noul and the false letter the remainder.
func (c *Caller) decide(ctx context.Context, slot *model.Slot, held *heldFiles, modelID string, q asks.MultichoiceQuestion) (map[string]float64, error) {
	key := decisionDigest(modelID, q)
	answer := filepath.Join(c.answers(), key+".json")
	switch cached, err := os.ReadFile(answer); {
	case err == nil:
		var a model.DecisionAnswer
		if err := json.Unmarshal(cached, &a); err != nil {
			return nil, fmt.Errorf("call: cached answer %s: %w", answer, err)
		}
		return confidence(a, q), nil
	case !errors.Is(err, os.ErrNotExist):
		return nil, err
	}

	promptHash := key
	stem := fileNameUnsafeRE.ReplaceAllString(fmt.Sprintf("%s-%s-%s", q.Kind, q.Group, q.Item), "_")
	stem = stem[:min(len(stem), fileStemMaxChars)]
	captures := c.captures()
	capturePath := filepath.Join(captures, fmt.Sprintf("%s-%s-%d.capture.jsonl", stem, promptHash[:promptHashChars], q.Attempt))
	cl := newCaptureLog()
	stateHash := digestOfState(q.State)
	cl.record(decisionCaptureRequest{
		Type: "request", Model: modelID, StateSHA256: stateHash,
		Question: decisionQuestionCapture{
			Key: q.Question.Key, Type: q.Question.Type, TrueLetter: q.Question.TrueLetter,
			Instructions: q.Question.Instructions, Criteria: q.Question.Criteria,
		},
		Attempt:     q.Attempt,
		TokenWaitMS: slot.Waited().Milliseconds(),
	})
	started := time.Now()
	got, err := slot.Decision(ctx, c.client, model.DecisionRequest{
		Model: modelID,
		State: q.State,
		Questions: []model.DecisionQuestion{{
			Key:          q.Question.Key,
			Type:         q.Question.Type,
			Instructions: q.Question.Instructions,
			Criteria:     q.Question.Criteria,
		}},
	})
	wall := time.Since(started)
	a := oneAnswer(got, err, q)
	if a.Type != "" {
		cl.record(decisionCaptureAttempt{
			Type: "attempt", Attempt: q.Attempt, Outcome: "ok", DurationMS: wall.Milliseconds(),
			FinishReason: "stop", UsageReported: got.Usage.PromptTokens > 0,
			PromptTokens: got.Usage.PromptTokens, CompletionTokens: got.Usage.CompletionTokens,
		})
	} else {
		cl.record(decisionCaptureAttempt{
			Type: "attempt", Attempt: q.Attempt, Outcome: "failed", Error: fmt.Sprint(err), DurationMS: wall.Milliseconds(),
		})
	}
	if err := ctx.Err(); err != nil {
		// The call was stopped, not failed: its ctx ending is the cause, so a
		// sibling's stop is never read as this call's inference failure.
		return nil, err
	}
	captureErr := cl.holdIn(held, captures, capturePath)
	switch {
	case err != nil:
		// Decision has re-issued identically to its attempt count; a call
		// past it never completed, and no answer is assumed for one.
		return nil, c.incomplete(fmt.Sprintf("%s-%s-%s", q.Kind, q.Group, q.Item), fmt.Sprintf("%v; capture: %s", err, capturePath), err)
	case captureErr != nil:
		return nil, captureErr
	}
	held.hold(c.answers(), answer, mustMarshal(a))
	return confidence(a, q), nil
}

// oneAnswer is the response's answer under the question's key: an answer to
// a decision call arrives keyed like the questions object, and the lookup
// keys on it. A missing answer decodes to the zero answer — the seam's
// recoverable path, an unreadable answer here. The response is passed whole
// and the zero one returned where nothing decided, so the caller caches and
// maps the one answer it asked for.
func oneAnswer(resp model.DecisionResponse, err error, q asks.MultichoiceQuestion) model.DecisionAnswer {
	if err != nil {
		return model.DecisionAnswer{}
	}
	for _, a := range resp.Answers {
		if a.Key == q.Question.Key {
			return a
		}
	}
	return model.DecisionAnswer{Key: q.Question.Key}
}

// confidence maps an answer to a per-letter confidence over the offered
// letters; nil where the answer decides nothing (an unreadable answer).
func confidence(a model.DecisionAnswer, q asks.MultichoiceQuestion) map[string]float64 {
	switch a.Type {
	case "choice":
		if a.Probabilities != nil {
			return a.Probabilities
		}
		if a.Choice != "" {
			return map[string]float64{a.Choice: a.Confidence}
		}
	case "noul":
		if a.Noul >= 0 && a.Noul <= 1 {
			out := map[string]float64{q.Question.TrueLetter: a.Noul}
			if other := otherLetter(q); other != "" {
				out[other] = 1 - a.Noul
			}
			return out
		}
	}
	return nil
}

// otherLetter is the offered letter whose criterion is not the true one, ""
// where the offered set is not two.
func otherLetter(q asks.MultichoiceQuestion) string {
	var rest []string
	for _, l := range q.Offered {
		if l != q.Question.TrueLetter {
			rest = append(rest, l)
		}
	}
	if len(rest) != 1 {
		return ""
	}
	return rest[0]
}

// digestOfState is the state's digest, the prompt-sha256 precedent: the
// capture carries the composed state as a digest, its bytes in the group
// records under the trail's asks/.
func digestOfState(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// decisionCaptureRequest opens a decision call's capture: what was asked, by
// digest, never the endpoint or its key — the state as a digest, its bytes in
// the group records under the trail's asks/, the question whole — and how long the call's
// item waited for its token, time outside the attempt's duration-ms.
type decisionCaptureRequest struct {
	Type        string                  `json:"type"`
	Model       string                  `json:"model"`
	StateSHA256 string                  `json:"state-sha256"`
	Question    decisionQuestionCapture `json:"question"`
	Attempt     int                     `json:"ask-attempt"`
	TokenWaitMS int64                   `json:"token-wait-ms"`
}

type decisionQuestionCapture struct {
	Key          string            `json:"key"`
	Type         string            `json:"type"`
	TrueLetter   string            `json:"true-letter,omitempty"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

// decisionCaptureAttempt is the one request of the decision call, as it
// ended. Decision has no streaming and no finish reason; the usage figures
// are the API's own.
type decisionCaptureAttempt struct {
	Type             string `json:"type"`
	Attempt          int    `json:"attempt"`
	Outcome          string `json:"outcome"`
	Error            string `json:"error,omitempty"`
	DurationMS       int64  `json:"duration-ms"`
	FinishReason     string `json:"finish-reason"`
	UsageReported    bool   `json:"usage-reported"`
	PromptTokens     int    `json:"prompt-tokens"`
	CompletionTokens int    `json:"completion-tokens"`
	CachedTokens     int    `json:"cached-tokens"`
	Reasoned         bool   `json:"reasoned"`
}

// decisionDigest is the digest of the model, the question whole, the state
// and the ask's attempt, so the same ask finds its answer and a re-ask of
// the identical question does not — the attempt ordinal stays in the answer
// cache's key, the re-ask reaching the model. The criteria serialize in
// sorted key order, so the digest is deterministic across map iteration
// orders.
func decisionDigest(modelID string, q asks.MultichoiceQuestion) string {
	return asks.Digest(append(append([]string{modelID, q.Question.Key}, asks.QuestionParts(q.Question)...), q.State, strconv.Itoa(q.Attempt))...)
}

// captureLog composes a call's capture in memory, one JSON line per record;
// nothing reaches a file until write.
type captureLog struct {
	buf bytes.Buffer
	enc *json.Encoder
	err error
}

func newCaptureLog() *captureLog {
	l := &captureLog{}
	l.enc = json.NewEncoder(&l.buf)
	l.enc.SetEscapeHTML(false)
	return l
}

func (l *captureLog) record(v any) {
	if l.err == nil {
		l.err = l.enc.Encode(v)
	}
}

// holdIn holds the composed capture in held, for path under dir; where it
// could not be composed it holds nothing and says why.
func (l *captureLog) holdIn(held *heldFiles, dir, path string) error {
	if l.err != nil {
		return fmt.Errorf("call: composing the capture %s: %w", path, l.err)
	}
	held.hold(dir, path, l.buf.Bytes())
	return nil
}

// writeFileIn writes data atomically at path, creating dir first.
func writeFileIn(dir, path string, data []byte) error {
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}
	return atomicfile.Write(path, data, nil)
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// captureRequest opens a capture: what was asked, by digest, never the
// endpoint or its key, the effective temperature and thinking level, and how
// long the call's item waited for its token — time outside every attempt's
// duration-ms.
type captureRequest struct {
	Type         string   `json:"type"`
	Model        string   `json:"model"`
	System       string   `json:"system"`
	PromptSHA256 string   `json:"prompt-sha256"`
	Excerpts     *string  `json:"excerpts,omitempty"`
	Rejected     []string `json:"rejected-lines,omitempty"`
	Attempt      int      `json:"ask-attempt"`
	Temperature  float64  `json:"temperature"`
	Thinking     string   `json:"thinking"`
	TokenWaitMS  int64    `json:"token-wait-ms"`
}

// overviewInputs are the inputs asks.OverviewPrompt renders, recorded in the
// overview call's capture.
type overviewInputs struct {
	Excerpts string
	Rejected []string
}

// captureAttempt is one request of the call, as it ended.
type captureAttempt struct {
	Type             string `json:"type"`
	Attempt          int    `json:"attempt"`
	Outcome          string `json:"outcome"`
	Error            string `json:"error,omitempty"`
	DurationMS       int64  `json:"duration-ms"`
	FinishReason     string `json:"finish-reason"`
	UsageReported    bool   `json:"usage-reported"`
	PromptTokens     int    `json:"prompt-tokens"`
	CompletionTokens int    `json:"completion-tokens"`
	CachedTokens     int    `json:"cached-tokens"`
	Reasoned         bool   `json:"reasoned"`
	Reply            string `json:"reply"`
}

// answerPath is where the answer to an ask is cached: named by the digest of
// the model, the system prompt, the prompt, the ask's attempt and the client's
// resolved sampling, so the same ask finds its answer and a re-ask of an
// identical prompt does not.
func (c *Caller) answerPath(modelID, system, prompt string, attempt int, sampling model.Sampling) string {
	parts := append([]string{modelID, system, prompt, strconv.Itoa(attempt)}, samplingKey(sampling)...)
	return filepath.Join(c.answers(), asks.Digest(parts...)+".txt")
}

// samplingKey is the answer-cache key's parts for resolved sampling. kbase's
// default sampling contributes no key parts, so the key of a default-sampled
// ask is the digest of model, system, prompt and attempt alone; any other
// sampling contributes the temperature and the thinking level.
func samplingKey(sampling model.Sampling) []string {
	if *sampling.Temperature == model.DefaultTemperature && *sampling.Thinking == model.ThinkingOff {
		return nil
	}
	return []string{"temperature=" + strconv.FormatFloat(*sampling.Temperature, 'g', -1, 64), "thinking=" + sampling.Thinking.String()}
}

// ask is prompt under the named system fragment: the answer cached for these
// exact inputs where one stands, else the call through slot, its every
// attempt captured, the capture and the answer held in held.
func (c *Caller) ask(ctx context.Context, slot *model.Slot, held *heldFiles, modelID, systemName, prompt string, attempt int, backoff []time.Duration, name string, inputs *overviewInputs) (asks.Reply, error) {
	system, err := asks.System(systemName)
	if err != nil {
		return asks.Reply{}, err
	}
	sampling := model.SamplingOf(c.client)
	answer := c.answerPath(modelID, system, prompt, attempt, sampling)
	switch cached, err := os.ReadFile(answer); {
	case err == nil:
		return asks.Reply{Text: string(cached)}, nil
	case !errors.Is(err, os.ErrNotExist):
		return asks.Reply{}, err
	}

	promptHash := asks.Digest(prompt)
	stem := fileNameUnsafeRE.ReplaceAllString(name, "_")
	stem = stem[:min(len(stem), fileStemMaxChars)]
	captures := c.captures()
	capturePath := filepath.Join(captures, fmt.Sprintf("%s-%s-%d.capture.jsonl", stem, promptHash[:promptHashChars], attempt))
	cl := newCaptureLog()
	request := captureRequest{Type: "request", Model: modelID, System: systemName, PromptSHA256: promptHash, Attempt: attempt,
		Temperature: *sampling.Temperature, Thinking: sampling.Thinking.String(), TokenWaitMS: slot.Waited().Milliseconds()}
	if inputs != nil {
		request.Excerpts, request.Rejected = &inputs.Excerpts, inputs.Rejected
	}
	cl.record(request)
	res, err := slot.Chat(ctx, c.client, model.ChatRequest{Model: modelID, System: system, Prompt: prompt, Backoff: backoff})
	thinking := 0
	for _, a := range res.Attempts {
		if a.Reasoned {
			thinking++
		}
		cl.record(captureAttempt{
			Type: "attempt", Attempt: a.Attempt, Outcome: a.Outcome, Error: a.Error, DurationMS: a.Duration.Milliseconds(),
			FinishReason: a.FinishReason, UsageReported: a.UsageReported, PromptTokens: a.Usage.PromptTokens,
			CompletionTokens: a.Usage.CompletionTokens, CachedTokens: a.Usage.CachedPromptTokens, Reasoned: a.Reasoned, Reply: a.Reply,
		})
	}
	if err := ctx.Err(); err != nil {
		return asks.Reply{}, err
	}
	captureErr := cl.holdIn(held, captures, capturePath)
	switch {
	case errors.Is(err, model.ErrChatIncomplete):
		captured := "Capture: " + capturePath
		if captureErr != nil {
			captured = captureErr.Error()
		}
		return asks.Reply{}, c.incomplete(name, fmt.Sprintf("%v (%s); no answer is assumed for a call that did not complete. %s", err, res.Final.Error, captured), err)
	case err != nil:
		return asks.Reply{}, err
	case captureErr != nil:
		return asks.Reply{}, captureErr
	}
	final := res.Final
	if final.FinishReason == model.FinishLength {
		c.lg.Warn("a chat reply was cut off at the completion cap", "ask", name, "capture", capturePath)
	}
	held.hold(c.answers(), answer, []byte(final.Reply))
	return asks.Reply{Text: final.Reply, Stats: &asks.CallStats{
		APIDuration: final.Duration, PromptTokens: final.Usage.PromptTokens, OutputTokens: final.Usage.CompletionTokens,
		CachedTokens: final.Usage.CachedPromptTokens, ThinkingBlocks: thinking,
	}}, nil
}
