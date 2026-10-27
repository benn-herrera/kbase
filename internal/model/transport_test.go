package model

import (
	"net/http"
	"testing"
)

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
