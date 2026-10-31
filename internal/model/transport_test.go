package model

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestStatusErrorLeavesNoKeyPrefix: a key the server echoes across the
// errorBodyLimit cut leaves no prefix of itself in the returned body, alone
// and behind a whole key earlier in the body, whose scrub shortens the text
// ahead of the cut.
func TestStatusErrorLeavesNoKeyPrefix(t *testing.T) {
	straddle := strings.Repeat("x", errorBodyLimit-len(testAPIKey)/2) + testAPIKey + strings.Repeat("x", errorBodyLimit)
	for name, body := range map[string]string{
		"straddling key":            straddle,
		"whole key then straddling": "token=" + testAPIKey + " " + straddle[len("token=")+len(testAPIKey)+1:],
	} {
		t.Run(name, func(t *testing.T) {
			resp := &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(body))}
			var se StatusError
			if !errors.As(statusError(resp, testAPIKey), &se) {
				t.Fatal("statusError did not return a StatusError")
			}
			if n := longestKeyPrefix(se.Body, testAPIKey); n > 0 {
				t.Errorf("a %d-byte prefix of the %d-byte key survives in the body", n, len(testAPIKey))
			}
			if len(se.Body) > errorBodyLimit {
				t.Errorf("body is %d bytes, over the %d-byte cap", len(se.Body), errorBodyLimit)
			}
		})
	}
}

// longestKeyPrefix is the length of the longest prefix of key that s contains.
func longestKeyPrefix(s, key string) int {
	for n := len(key); n > 0; n-- {
		if strings.Contains(s, key[:n]) {
			return n
		}
	}
	return 0
}

// TestClientTransportKeepsIdleConnsPerHost: a client built from an endpoint
// with IdleConns n keeps max(n, 2) idle connections per host on either wire
// protocol, and the total never binds before the per-host count.
func TestClientTransportKeepsIdleConnsPerHost(t *testing.T) {
	for _, tc := range []struct {
		idle, perHost, total int
	}{{0, 2, 100}, {1, 2, 100}, {4, 4, 100}, {150, 150, 150}} {
		e := Endpoint{IdleConns: tc.idle}
		for name, h := range map[string]*http.Client{
			"openai":    NewOpenAIClient(e).http,
			"anthropic": NewAnthropicClient(e).http,
		} {
			tr := h.Transport.(*http.Transport)
			if tr.MaxIdleConnsPerHost != tc.perHost || tr.MaxIdleConns != tc.total {
				t.Errorf("%s IdleConns %d: per host %d, total %d; want %d, %d",
					name, tc.idle, tr.MaxIdleConnsPerHost, tr.MaxIdleConns, tc.perHost, tc.total)
			}
		}
	}
}
