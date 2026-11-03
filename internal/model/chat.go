package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// ChatAttempts is how many times one chat call is put on the wire,
// identically, before a call that never completes is given up.
const ChatAttempts = 3

// How one request of a chat call ended.
const (
	// ChatOK is a stream that reached [DONE] — an empty reply, and one cut off
	// at the completion cap with answer text, included.
	ChatOK = "ok"
	// ChatProviderRefused is a request the provider answered with a status that
	// is not success: it read the request and declined it, so the path to it,
	// the name and the key all worked, and what ended the attempt is the answer
	// itself — its status and body.
	ChatProviderRefused = "provider-refused"
	// ChatTransportFailure is a call that reached no answer of the provider's —
	// the connection, the name or the certificate behind it failing — or a
	// stream that failed during or short of [DONE].
	ChatTransportFailure = "transport-failure"
	// ChatServedNothing is a stream whose usage reports no token read and none
	// written: a server aborting a request answers so, its error message as the
	// reply, where a call that ran reports the prompt it read.
	ChatServedNothing = "served-nothing"
	// ChatTruncatedEmpty is a stream that reached [DONE] with the completion
	// cap as its finish reason and no answer text: the request leaves the
	// answer no room, so an identical re-send is cut off the same way and the
	// call ends on this attempt.
	ChatTruncatedEmpty = "truncated-empty"
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
	// Reasoned is whether the call reasoned, from either source the wire
	// offers: a reasoning delta on the stream, or a usage block reporting
	// reasoning tokens spent. The second catches a provider that thinks
	// without streaming the thought — a call whose thinking is invisible in
	// the deltas but measured in the tokens.
	Reasoned bool
	Duration time.Duration
	// FirstToken is when the first non-empty content or reasoning delta
	// arrived, measured from the request's start; zero where the stream
	// carried neither — a failed call, an empty reply, an attempt that
	// reached no stream at all.
	FirstToken time.Duration
}

// chat puts prompt to modelID under system as one tool-less chat request,
// carrying the sampling c sends (Client.sampling) for c's protocol to map onto
// its own fields — re-issued identically while it does not complete, up to
// ChatAttempts, pausing before each re-issue for the next of backoff, its last
// repeating; with no backoff it re-issues at once, and a pause the context's
// deadline cannot cover ends the call at once with no further attempt. A provider that answered with a
// status that is not success is re-issued only where the status is retryable
// (retryableStatus): any other is the request itself being wrong, and the call
// ends on the one attempt that met it, as does an answer cut off at the
// completion cap before any answer text. observe, where set, is handed each
// attempt as it ends. It returns the completed attempt, or the last one with
// an error wrapping ErrChatIncomplete; a cancelled ctx returns its error at
// once.
func chat(ctx context.Context, c Client, modelID, system, prompt string, backoff []time.Duration, observe func(ChatAttempt)) (ChatAttempt, error) {
	req := Request{
		Model:    modelID,
		Messages: []Message{{Role: "system", Content: system}, {Role: "user", Content: prompt}},
		Sampling: c.sampling(),
	}
	var (
		last    ChatAttempt
		failure error
	)
	for attempt := 1; attempt <= ChatAttempts; attempt++ {
		if attempt > 1 && len(backoff) > 0 {
			pause := backoff[min(attempt-2, len(backoff)-1)]
			// A pause the deadline cannot cover leaves no attempt to spend it
			// on: the wait would end the call on the deadline rather than on
			// its attempts, so the call gives up now and says how many it
			// made. A context with no deadline never refuses a pause.
			if deadline, ok := ctx.Deadline(); ok && pause >= time.Until(deadline) {
				return last, gaveUp(attempt-1, last, failure)
			}
			timer := time.NewTimer(pause)
			select {
			case <-ctx.Done():
				timer.Stop()
				return last, ctx.Err()
			case <-timer.C:
			}
		}
		last, failure = chatOnce(ctx, c, req)
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
		// An answer cut off before any text is cut off the same way again.
		if last.Outcome == ChatTruncatedEmpty {
			return last, gaveUp(attempt, last, failure)
		}
		// A provider that read the request and declined it on a status that
		// will not change will decline it again.
		var se StatusError
		if errors.As(failure, &se) && !retryableStatus(se.Code) {
			return last, gaveUp(attempt, last, failure)
		}
	}
	return last, gaveUp(ChatAttempts, last, failure)
}

// gaveUp is a call none of whose attempts completed, said as one reason: how
// many attempts it took, and how the last one ended. It wraps cause, the error
// that ended the last attempt where one did, so a caller matches its
// StatusError with errors.As; its text is last.Error.
func gaveUp(attempts int, last ChatAttempt, cause error) error {
	count := fmt.Sprintf("%d attempts", attempts)
	if attempts == 1 {
		count = "1 attempt"
	}
	if cause != nil {
		return fmt.Errorf("%w: %s, the last ending %s: %w", ErrChatIncomplete, count, last.Outcome, cause)
	}
	if last.Error != "" {
		return fmt.Errorf("%w: %s, the last ending %s: %s", ErrChatIncomplete, count, last.Outcome, last.Error)
	}
	return fmt.Errorf("%w: %s, the last ending %s", ErrChatIncomplete, count, last.Outcome)
}

// chatOnce is one request of the call and how it ended, with the error that
// ended it where one did: the retry loop above decides which of them deserve
// a re-issue.
func chatOnce(ctx context.Context, c Client, req Request) (ChatAttempt, error) {
	started := time.Now()
	var a ChatAttempt
	var failure error
	sr, err := c.consultStream(ctx, req)
	if err != nil {
		a.Outcome, a.Error = chatFailure(err)
		a.Duration = time.Since(started)
		return a, err
	}
	defer sr.Close()
	for {
		chunk, err := sr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			a.Outcome, a.Error = chatFailure(err)
			failure = err
			break
		}
		a.Reasoned = a.Reasoned || chunk.Reasoning != ""
		if a.FirstToken == 0 && (chunk.Content != "" || chunk.Reasoning != "") {
			a.FirstToken = time.Since(started)
		}
	}
	resp := sr.Final()
	a.Reasoned = a.Reasoned || resp.Usage.ReasoningTokens > 0
	a.Reply, a.FinishReason, a.Usage, a.UsageReported = resp.Content, resp.FinishReason, resp.Usage, resp.UsageReported
	switch {
	case a.Outcome != "":
	case !resp.StreamDone:
		a.Outcome, a.Error = ChatTransportFailure, "the stream ended short of [DONE]"
	case resp.UsageReported && resp.Usage.PromptTokens == 0 && resp.Usage.CompletionTokens == 0:
		a.Outcome = ChatServedNothing
	case resp.Truncated() && strings.TrimSpace(resp.Content) == "":
		a.Outcome, a.Error = ChatTruncatedEmpty, "the answer reached the completion cap before any answer text"
	default:
		a.Outcome = ChatOK
	}
	a.Duration = time.Since(started)
	return a, failure
}

// chatFailure is the outcome of an attempt err ended, and the text a reason
// carries of it. A provider that answered with a status that is not success
// refused the request: the connection carried it and the answer says why, so
// reporting it as the transport's failure sends a reader to the path between
// kbase and a provider that is working.
func chatFailure(err error) (string, string) {
	outcome := ChatTransportFailure
	var se StatusError
	if errors.As(err, &se) {
		outcome = ChatProviderRefused
	}
	return outcome, err.Error()
}
