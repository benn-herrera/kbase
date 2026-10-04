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
// did not complete, as kb_tools' driver pauses its calls; a letter ask is
// re-issued at once, as kb_tools' claim graph re-issues it.
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

// Letters is the production LetterReader: each question put to modelID under
// the reader-system fragment. It returns no confidence.
func (c *Caller) Letters(modelID string) asks.LetterReader {
	return func(ctx context.Context, q asks.LetterQuestion) (asks.Reply, error) {
		return c.ask(ctx, modelID, asks.ReaderSystem, q.Prompt, q.Attempt, nil, fmt.Sprintf("%s-%s-%s", q.Kind, q.Group, q.Item))
	}
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
		return asks.Reply{}, asks.IncompleteError{Subject: name, Detail: fmt.Sprintf("%v (%s); it was re-issued identically to no effect, and no answer is assumed for a call that did not complete. Capture: %s", err, got.Error, capturePath)}
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
