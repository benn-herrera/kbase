package model

import (
	"math"
	"net/http"
	"testing"
	"time"
)

// quickRetry is DefaultRetry with pauses of a millisecond at most.
func quickRetry() Retry {
	r := DefaultRetry()
	r.Base = time.Millisecond
	return r
}

// TestAPauseIsDrawnWithinItsBound: the pause before attempt n is a uniform
// draw from [0, 1 s × 2ⁿ⁻²] — 1 s, 2 s and 4 s before the second, third and
// fourth attempts — whatever the draw, the bound itself included.
func TestAPauseIsDrawnWithinItsBound(t *testing.T) {
	failure := StatusError{Code: http.StatusServiceUnavailable}
	for _, tc := range []struct {
		attempt int
		bound   time.Duration
	}{{2, time.Second}, {3, 2 * time.Second}, {4, 4 * time.Second}} {
		r := DefaultRetry()
		var asked int64
		r.Draw = func(n int64) int64 { asked = n; return n - 1 }
		if got, fromServer := r.pause(tc.attempt, failure, time.Now()); got != tc.bound || fromServer || asked != int64(tc.bound)+1 {
			t.Errorf("attempt %d: the top draw is %v (server %v, drawn from [0, %d)), want %v drawn from [0, %v]", tc.attempt, got, fromServer, asked, tc.bound, tc.bound)
		}
		r = DefaultRetry()
		for range 1000 {
			if got, _ := r.pause(tc.attempt, failure, time.Now()); got < 0 || got > tc.bound {
				t.Fatalf("attempt %d: drew %v, outside [0, %v]", tc.attempt, got, tc.bound)
			}
		}
	}
}

// TestRetryAfterSetsThePauseOfA429OrA503: Retry-After, as delay-seconds or as
// an HTTP-date, is the pause after a 429 or a 503; any other status, and a
// value that is neither form, leave the drawn pause.
func TestRetryAfterSetsThePauseOfA429OrA503(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	r := DefaultRetry()
	r.Draw = func(int64) int64 { return int64(time.Millisecond) }
	for _, tc := range []struct {
		name       string
		code       int
		retryAfter string
		want       time.Duration
		fromServer bool
	}{
		{"seconds on 429", http.StatusTooManyRequests, "120", 2 * time.Minute, true},
		{"seconds on 503", http.StatusServiceUnavailable, " 7 ", 7 * time.Second, true},
		{"zero seconds", http.StatusTooManyRequests, "0", 0, true},
		{"HTTP-date ahead", http.StatusServiceUnavailable, now.Add(90 * time.Second).Format(http.TimeFormat), 90 * time.Second, true},
		{"HTTP-date passed", http.StatusTooManyRequests, now.Add(-time.Hour).Format(http.TimeFormat), 0, true},
		{"seconds past any duration", http.StatusTooManyRequests, "9999999999999", math.MaxInt64, true},
		{"on a 500", http.StatusInternalServerError, "120", time.Millisecond, false},
		{"absent", http.StatusTooManyRequests, "", time.Millisecond, false},
		{"neither form", http.StatusTooManyRequests, "soon", time.Millisecond, false},
		{"negative", http.StatusTooManyRequests, "-5", time.Millisecond, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, fromServer := r.pause(2, StatusError{Code: tc.code, RetryAfter: tc.retryAfter}, now)
			if got != tc.want || fromServer != tc.fromServer {
				t.Errorf("pause = %v (server %v), want %v (server %v)", got, fromServer, tc.want, tc.fromServer)
			}
		})
	}
}
