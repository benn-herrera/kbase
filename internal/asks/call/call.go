// Package call puts a build's asks to a provider: each composed prompt under
// the system fragment its kind names, as one model.Chat call re-issued
// identically while it does not complete, every attempt captured, and the
// answer cached by what was asked so a resumed stage asks nothing it already
// has an answer for.
package call

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"kbase/internal/asks"
	"kbase/internal/atomicfile"
	"kbase/internal/log"
	"kbase/internal/model"
)

// What a Caller keeps under a build's scratch directory: each call's capture,
// and each answer under the digest of what was asked.
const (
	CapturesDir = "captures"
	AnswersDir  = "answers"
)

var fileNameUnsafeRE = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

const (
	fileStemMaxChars = 150
	promptHashChars  = 12
)

// overviewBackoff is the pause before each re-issue of an overview call that
// did not complete; a letter ask is re-issued at once.
var overviewBackoff = []time.Duration{5 * time.Second, 30 * time.Second}

// Caller puts composed prompts to one provider. Safe for concurrent use over
// distinct prompts.
type Caller struct {
	client          model.Client
	captures        string
	answers         string
	overviewBackoff []time.Duration
	lg              log.Logger
}

// New is a Caller over client keeping its evidence and answers under scratch.
func New(client model.Client, scratch string, lg log.Logger) *Caller {
	return &Caller{
		client:          client,
		captures:        filepath.Join(scratch, CapturesDir),
		answers:         filepath.Join(scratch, AnswersDir),
		overviewBackoff: overviewBackoff,
		lg:              lg,
	}
}

// Letters is the production LetterReader: each question put to modelID as
// one Decision call — the composed state the request's state, the question
// serialized under its item's key — the answer mapped to a confidence keyed
// by the offered letters, and the mapped answer cached by the digest of the
// model, the question and the state.
func (c *Caller) Letters(modelID string) asks.LetterReader {
	return func(ctx context.Context, q asks.LetterQuestion) (asks.Reply, error) {
		conf, err := c.decide(ctx, modelID, q)
		if err != nil {
			return asks.Reply{}, err
		}
		return asks.Reply{Confidence: conf}, nil
	}
}

// decide is one question's Decision round-trip: the cached answer where one
// stands for these exact inputs, else the call, its every attempt captured.
//
// The mapping, per answer type, keyed by which criterion the letter names,
// never by offered order: a choice answer carries the probabilities whole;
// without them its own confidence sits under the chosen letter; with neither,
// the answer is unreadable and takes the re-ask/default path. A noul answer
// carries one figure read as P(true) — the OpenRouter example's reading,
// confirmed against the live provider before the hermetic suite treats it as
// fixed — so the true letter takes noul and the false letter the remainder.
func (c *Caller) decide(ctx context.Context, modelID string, q asks.LetterQuestion) (map[string]float64, error) {
	key := decisionDigest(modelID, q)
	answer := filepath.Join(c.answers, key+".json")
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
	capturePath := filepath.Join(c.captures, fmt.Sprintf("%s-%s-%d.capture.jsonl", stem, promptHash[:promptHashChars], q.Attempt))
	if err := os.MkdirAll(c.captures, 0o777); err != nil {
		return nil, err
	}
	f, err := os.Create(capturePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	var writeErr error
	record := func(v any) {
		if writeErr == nil {
			writeErr = enc.Encode(v)
		}
	}
	stateHash := digestOfState(q.State)
	record(decisionCaptureRequest{
		Type: "request", Model: modelID, StateSHA256: stateHash,
		Question: decisionQuestionCapture{
			Key: q.Question.Key, Type: q.Question.Type,
			Instructions: q.Question.Instructions, Criteria: q.Question.Criteria,
		},
		Attempt: q.Attempt,
	})
	started := time.Now()
	got, err := c.client.Decision(ctx, model.DecisionRequest{
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
		record(decisionCaptureAttempt{
			Type: "attempt", Attempt: q.Attempt, Outcome: "ok", DurationMS: wall.Milliseconds(),
			FinishReason: "stop", UsageReported: got.Usage.PromptTokens > 0,
			PromptTokens: got.Usage.PromptTokens, CompletionTokens: got.Usage.CompletionTokens,
		})
	} else {
		record(decisionCaptureAttempt{
			Type: "attempt", Attempt: q.Attempt, Outcome: "failed", Error: fmt.Sprint(err), DurationMS: wall.Milliseconds(),
		})
	}
	if flushErr := w.Flush(); writeErr == nil {
		writeErr = flushErr
	}
	if err != nil {
		// Decision has re-issued identically to its attempt count; a call
		// past it never completed, and no answer is assumed for one.
		return nil, asks.IncompleteError{Subject: fmt.Sprintf("%s-%s-%s", q.Kind, q.Group, q.Item), Detail: fmt.Sprintf("%v; capture: %s", err, capturePath)}
	}
	if writeErr != nil {
		return nil, fmt.Errorf("call: writing the capture %s: %w", capturePath, writeErr)
	}
	if err := os.MkdirAll(c.answers, 0o777); err != nil {
		return nil, err
	}
	if err := atomicfile.Write(answer, mustMarshal(a), nil); err != nil {
		return nil, err
	}
	return confidence(a, q), nil
}

// oneAnswer is the response's answer under the question's key: an answer to
// a decision call arrives keyed like the questions object, and the lookup
// keys on it. A missing answer decodes to the zero answer — the seam's
// recoverable path, an unreadable answer here. The response is passed whole
// and the zero one returned where nothing decided, so the caller caches and
// maps the one answer it asked for.
func oneAnswer(resp model.DecisionResponse, err error, q asks.LetterQuestion) model.DecisionAnswer {
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
func confidence(a model.DecisionAnswer, q asks.LetterQuestion) map[string]float64 {
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
func otherLetter(q asks.LetterQuestion) string {
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
// capture carries the composed state as a digest, its bytes under
// scratch/asks/ as today.
func digestOfState(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// decisionCaptureRequest opens a decision call's capture: what was asked, by
// digest, never the endpoint or its key — the state as a digest, its bytes
// under scratch/asks/ as today, the question whole.
type decisionCaptureRequest struct {
	Type        string                  `json:"type"`
	Model       string                  `json:"model"`
	StateSHA256 string                  `json:"state-sha256"`
	Question    decisionQuestionCapture `json:"question"`
	Attempt     int                     `json:"ask-attempt"`
}

type decisionQuestionCapture struct {
	Key          string            `json:"key"`
	Type         string            `json:"type"`
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
func decisionDigest(modelID string, q asks.LetterQuestion) string {
	keys := make([]string, 0, len(q.Question.Criteria))
	for k := range q.Question.Criteria {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{modelID, q.Question.Key, q.Question.Type, q.Question.Instructions, q.Question.TrueLetter}
	for _, k := range keys {
		parts = append(parts, k, q.Question.Criteria[k])
	}
	return digest(append(parts, q.State, strconv.Itoa(q.Attempt))...)
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// Passage is the overview passage asked of modelID from excerpts, rejected
// holding the lines a previous reply was refused for, attempt counting the
// asks from 1.
func (c *Caller) Passage(ctx context.Context, modelID, excerpts string, rejected []string, attempt int) (string, error) {
	prompt, err := asks.OverviewPrompt(excerpts, rejected)
	if err != nil {
		return "", err
	}
	reply, err := c.ask(ctx, modelID, asks.OverviewSystem, prompt, attempt, c.overviewBackoff, "overview")
	return reply.Text, err
}

// captureRequest opens a capture: what was asked, by digest, never the
// endpoint or its key.
type captureRequest struct {
	Type           string  `json:"type"`
	Model          string  `json:"model"`
	System         string  `json:"system"`
	PromptSHA256   string  `json:"prompt-sha256"`
	Attempt        int     `json:"ask-attempt"`
	Temperature    float64 `json:"temperature"`
	EnableThinking bool    `json:"enable-thinking"`
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

func digest(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%d:%s", len(p), p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// answerPath is where the answer to an ask is cached: named by the digest of
// the model, the system prompt, the prompt and the ask's attempt, so the same
// ask finds its answer and a re-ask of an identical prompt does not.
func (c *Caller) answerPath(modelID, system, prompt string, attempt int) string {
	return filepath.Join(c.answers, digest(modelID, system, prompt, strconv.Itoa(attempt))+".txt")
}

// ask is prompt under the named system fragment: the answer cached for these
// exact inputs where one stands, else the call, its every attempt captured.
func (c *Caller) ask(ctx context.Context, modelID, systemName, prompt string, attempt int, backoff []time.Duration, name string) (asks.Reply, error) {
	system, err := asks.System(systemName)
	if err != nil {
		return asks.Reply{}, err
	}
	answer := c.answerPath(modelID, system, prompt, attempt)
	switch cached, err := os.ReadFile(answer); {
	case err == nil:
		return asks.Reply{Text: string(cached)}, nil
	case !errors.Is(err, os.ErrNotExist):
		return asks.Reply{}, err
	}

	promptHash := digest(prompt)
	stem := fileNameUnsafeRE.ReplaceAllString(name, "_")
	stem = stem[:min(len(stem), fileStemMaxChars)]
	capturePath := filepath.Join(c.captures, fmt.Sprintf("%s-%s-%d.capture.jsonl", stem, promptHash[:promptHashChars], attempt))
	if err := os.MkdirAll(c.captures, 0o777); err != nil {
		return asks.Reply{}, err
	}
	f, err := os.Create(capturePath)
	if err != nil {
		return asks.Reply{}, err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	var writeErr error
	record := func(v any) {
		if writeErr == nil {
			writeErr = enc.Encode(v)
		}
	}
	record(captureRequest{Type: "request", Model: modelID, System: systemName, PromptSHA256: promptHash, Attempt: attempt})
	thinking := 0
	got, err := model.Chat(ctx, c.client, modelID, system, prompt, backoff, func(a model.ChatAttempt) {
		if a.Reasoned {
			thinking++
		}
		record(captureAttempt{
			Type: "attempt", Attempt: a.Attempt, Outcome: a.Outcome, Error: a.Error, DurationMS: a.Duration.Milliseconds(),
			FinishReason: a.FinishReason, UsageReported: a.UsageReported, PromptTokens: a.Usage.PromptTokens,
			CompletionTokens: a.Usage.CompletionTokens, CachedTokens: a.Usage.CachedPromptTokens, Reasoned: a.Reasoned, Reply: a.Reply,
		})
	})
	if flushErr := w.Flush(); writeErr == nil {
		writeErr = flushErr
	}
	switch {
	case ctx.Err() != nil:
		return asks.Reply{}, ctx.Err()
	case errors.Is(err, model.ErrChatIncomplete):
		return asks.Reply{}, asks.IncompleteError{Subject: name, Detail: fmt.Sprintf("%v (%s); no answer is assumed for a call that did not complete. Capture: %s", err, got.Error, capturePath)}
	case err != nil:
		return asks.Reply{}, err
	case writeErr != nil:
		return asks.Reply{}, fmt.Errorf("call: writing the capture %s: %w", capturePath, writeErr)
	}
	if got.FinishReason == model.FinishLength {
		c.lg.Warn("a chat reply was cut off at the completion cap", "ask", name, "capture", capturePath)
	}
	if err := os.MkdirAll(c.answers, 0o777); err != nil {
		return asks.Reply{}, err
	}
	if err := atomicfile.Write(answer, []byte(got.Reply), nil); err != nil {
		return asks.Reply{}, err
	}
	return asks.Reply{Text: got.Reply, Stats: &asks.CallStats{
		APIDuration: got.Duration, PromptTokens: got.Usage.PromptTokens, OutputTokens: got.Usage.CompletionTokens,
		CachedTokens: got.Usage.CachedPromptTokens, ThinkingBlocks: thinking,
	}}, nil
}
