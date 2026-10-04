package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// ChatAttempts is how many times one chat call is put on the wire,
// identically, before a call that never completes is given up.
const ChatAttempts = 3

// How one request of a chat call ended.
const (
	// ChatOK is a stream that reached [DONE] — an empty reply and one cut off
	// at the completion cap included.
	ChatOK = "ok"
	// ChatTransportFailure is a stream that failed before, during or short of
	// [DONE].
	ChatTransportFailure = "transport-failure"
	// ChatServedNothing is a stream whose usage reports no token read and none
	// written: a server aborting a request answers so, its error message as the
	// reply, where a call that ran reports the prompt it read.
	ChatServedNothing = "served-nothing"
)

// ErrChatIncomplete is a chat call none of whose attempts completed.
var ErrChatIncomplete = errors.New("model: the chat call did not complete")

// ChatAttempt is one request of a chat call and how it ended.
type ChatAttempt struct {
	Attempt       int
	Outcome       string
	Error         string
	Reply         string
	FinishReason  string
	Usage         Usage
	UsageReported bool
	// Reasoned is whether the stream carried reasoning.
	Reasoned bool
	Duration time.Duration
}

// Chat puts prompt to modelID under system as one tool-less chat request —
// temperature 0, thinking disabled, the usage chunk requested — re-issued
// identically while it does not complete, up to ChatAttempts, pausing before
// each re-issue for the next of backoff, its last repeating; with no backoff
// it re-issues at once. observe, where set, is handed each attempt as it
// ends. It returns the completed attempt, or the last one with an error
// wrapping ErrChatIncomplete; a cancelled ctx returns its error at once.
func Chat(ctx context.Context, c Client, modelID, system, prompt string, backoff []time.Duration, observe func(ChatAttempt)) (ChatAttempt, error) {
	req := Request{
		Model:              modelID,
		Messages:           []Message{{Role: "system", Content: system}, {Role: "user", Content: prompt}},
		Temperature:        0,
		ChatTemplateKwargs: map[string]any{"enable_thinking": false},
	}
	var last ChatAttempt
	for attempt := 1; attempt <= ChatAttempts; attempt++ {
		if attempt > 1 && len(backoff) > 0 {
			pause := time.NewTimer(backoff[min(attempt-2, len(backoff)-1)])
			select {
			case <-ctx.Done():
				pause.Stop()
				return last, ctx.Err()
			case <-pause.C:
			}
		}
		last = chatOnce(ctx, c, req)
		last.Attempt = attempt
		if err := ctx.Err(); err != nil {
			return last, err
		}
		if observe != nil {
			observe(last)
		}
		if last.Outcome == ChatOK {
			return last, nil
		}
	}
	return last, fmt.Errorf("%w: %d attempts, the last ending %s", ErrChatIncomplete, ChatAttempts, last.Outcome)
}

func chatOnce(ctx context.Context, c Client, req Request) ChatAttempt {
	started := time.Now()
	var a ChatAttempt
	sr, err := c.ConsultStream(ctx, req)
	if err != nil {
		a.Outcome, a.Error = ChatTransportFailure, err.Error()
		a.Duration = time.Since(started)
		return a
	}
	defer sr.Close()
	for {
		chunk, err := sr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			a.Outcome, a.Error = ChatTransportFailure, err.Error()
			break
		}
		a.Reasoned = a.Reasoned || chunk.Reasoning != ""
	}
	resp := sr.Final()
	a.Reply, a.FinishReason, a.Usage, a.UsageReported = resp.Content, resp.FinishReason, resp.Usage, resp.UsageReported
	switch {
	case a.Outcome != "":
	case !resp.StreamDone:
		a.Outcome, a.Error = ChatTransportFailure, "the stream ended short of [DONE]"
	case resp.UsageReported && resp.Usage.PromptTokens == 0 && resp.Usage.CompletionTokens == 0:
		a.Outcome = ChatServedNothing
	default:
		a.Outcome = ChatOK
	}
	a.Duration = time.Since(started)
	return a
}
