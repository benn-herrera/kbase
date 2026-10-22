package model

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
// kind — see newHTTPTransport.
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

// sseScanner is the machinery both protocols' stream readers share: the line
// scanner over the response body, the idle watchdog, the terminal paths and the
// close. What one EVENT means is each reader's own — the two protocols deliver
// different event sequences, and neither reader is the other's special case.
//
// Single-goroutine, per the StreamReader contract: no field here is guarded.
// The idle watchdog is the one thing that touches the scanner from another
// goroutine, and it touches only context cancellation — which is exactly why
// cancellation, not a flag, is the abort mechanism.
type sseScanner struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	body    io.ReadCloser
	scanner *bufio.Scanner

	// idle is the watchdog timer, nil until the first read arms it — see
	// armIdle. idleWindow is the window it is armed with.
	idle       *time.Timer
	idleWindow time.Duration

	// done is set once the stream has been fully drained (the protocol's
	// closing event, EOF on the body, or a hard error). Subsequent reads
	// return io.EOF.
	done bool
	// closed makes Close idempotent.
	closed bool

	closeErr error
}

// newSSEScanner takes ownership of body and of cancel. ctx MUST be the context
// the in-flight request was issued with: cancelling it is what interrupts a
// read blocked on bytes that are never coming, and that is the whole mechanism
// behind the idle watchdog.
//
// idle is the watchdog's window. Production passes streamIdleTimeout; it is
// a parameter so a test can trip the watchdog in milliseconds rather than
// pinning the suite to the real bound. The watchdog is not started here —
// the first read arms it.
func newSSEScanner(ctx context.Context, cancel context.CancelCauseFunc, body io.ReadCloser, idle time.Duration) sseScanner {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, sseInitialBufferBytes), sseMaxLineBytes)
	return sseScanner{ctx: ctx, cancel: cancel, body: body, scanner: sc, idleWindow: idle}
}

// armIdle starts the watchdog, or restarts it if it is already running.
//
// It is deliberately not called from the constructor. The window bounds the
// gap between a read starting and bytes arriving — provider silence — and a
// caller that does other work between ConsultStream and its first Next has
// kept nobody waiting. Arming at construction would charge that work to the
// provider and abort a stream that never misbehaved.
func (r *sseScanner) armIdle() {
	if r.idle == nil {
		r.idle = time.AfterFunc(r.idleWindow, func() {
			r.cancel(fmt.Errorf("%w after %v", errStreamIdle, r.idleWindow))
		})
		return
	}
	r.idle.Reset(r.idleWindow)
}

// nextData returns the payload of the next `data:` line, skipping the blank
// separators, comment lines and `event:`/`id:` headers an SSE body carries
// around them. The returned slice aliases the scanner's buffer and is valid
// only until the next call.
//
// io.EOF is the body's end and nothing more: whether that end COMPLETES the
// stream is the reader's question, because the two protocols say so
// differently — `[DONE]` on one, `message_stop` on the other.
func (r *sseScanner) nextData() ([]byte, error) {
	// The read starts here, so the window the provider has to answer in
	// starts here too — on the first read and on every one after it.
	r.armIdle()
	for {
		// Honor context first so a slow server can't outlast a deadline.
		if err := r.ctx.Err(); err != nil {
			r.finish()
			return nil, r.terminalErr(err)
		}
		if !r.scanner.Scan() {
			r.finish()
			if err := r.scanner.Err(); err != nil {
				return nil, r.terminalErr(fmt.Errorf("stream read: %w", err))
			}
			return nil, io.EOF
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
		return payload, nil
	}
}

// finish marks the stream drained and retires the watchdog. Every terminal
// path runs through it, so no timer outlives the stream it was guarding — a
// stream closed before its first read has no timer to retire.
func (r *sseScanner) finish() {
	r.done = true
	if r.idle != nil {
		r.idle.Stop()
	}
}

// terminalErr names the stall instead of the symptom when the watchdog is
// what ended the stream. The watchdog interrupts a blocked read by
// cancelling the request, so the error in hand at that point is a flavor of
// "context canceled" and points at the wrong cause.
func (r *sseScanner) terminalErr(err error) error {
	if cause := context.Cause(r.ctx); errors.Is(cause, errStreamIdle) {
		return cause
	}
	return err
}

// Close releases the underlying body and retires the watchdog. Idempotent —
// and, like the rest of the scanner, the owning goroutine's to call.
func (r *sseScanner) Close() error {
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
