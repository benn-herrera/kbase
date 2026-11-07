package model

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Retry is how a queue's calls re-issue a request that did not get through:
// exponential backoff with full jitter, capped at Attempts.
type Retry struct {
	// Attempts is the most times one call puts its request on the wire.
	Attempts int
	// Base bounds the pause before the second attempt; the bound doubles
	// before each later one.
	Base time.Duration
	// Draw is the pauses' random source: a uniform draw from [0, n).
	Draw func(n int64) int64
}

// DefaultRetry is the rule NewQueue gives a queue: at most four attempts,
// the pause before attempt n drawn uniformly from [0, 1 s × 2ⁿ⁻²].
func DefaultRetry() Retry { return Retry{Attempts: 4, Base: time.Second, Draw: rand.Int64N} }

// pause is the wait before attempt, the second or a later one, of a call
// whose last attempt ended on failure: the delay a 429 or 503 asked for in
// its Retry-After, with asked true, else a draw from the attempt's bound.
func (r Retry) pause(attempt int, failure error, now time.Time) (pause time.Duration, asked bool) {
	var se StatusError
	if errors.As(failure, &se) && (se.Code == http.StatusTooManyRequests || se.Code == http.StatusServiceUnavailable) {
		if d, ok := retryAfter(se.RetryAfter, now); ok {
			return d, true
		}
	}
	bound := r.Base << (attempt - 2)
	return time.Duration(r.Draw(int64(bound) + 1)), false
}

// before waits out the pause before attempt. A pause the context's deadline
// cannot cover leaves no attempt to spend it on — the wait would end the call
// on the deadline rather than on its attempts — so it reports false at once,
// with beyond naming the delay the server asked for where its Retry-After set
// the pause, "" otherwise. A context with no deadline never refuses a pause.
// A ctx ended during the pause returns its error.
func (r Retry) before(ctx context.Context, attempt int, failure error) (fits bool, beyond string, err error) {
	now := time.Now()
	pause, asked := r.pause(attempt, failure, now)
	if deadline, ok := ctx.Deadline(); ok && pause >= deadline.Sub(now) {
		if asked {
			beyond = fmt.Sprintf(", the server asking a %v pause the deadline cannot cover", pause)
		}
		return false, beyond, nil
	}
	timer := time.NewTimer(pause)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false, "", ctx.Err()
	case <-timer.C:
		return true, "", nil
	}
}

// retryAfter is the delay a Retry-After value asks for from now: its
// delay-seconds, or the time to its HTTP-date, none where that has passed.
// ok is false for a value that is neither.
func retryAfter(value string, now time.Time) (d time.Duration, ok bool) {
	value = strings.TrimSpace(value)
	if secs, err := strconv.ParseUint(value, 10, 64); err == nil {
		if secs > math.MaxInt64/uint64(time.Second) {
			return math.MaxInt64, true
		}
		return time.Duration(secs) * time.Second, true
	}
	if at, err := http.ParseTime(value); err == nil {
		return max(at.Sub(now), 0), true
	}
	return 0, false
}
