package main

import (
	"testing"

	"kbase/internal/model"
	"kbase/internal/query"
)

// TestScoreKeepsAskConcurrencyIdleConnections: a score run at
// --ask-concurrency 4 builds its registry with 4, so the endpoint the seam is
// handed carries IdleConns 4.
func TestScoreKeepsAskConcurrencyIdleConnections(t *testing.T) {
	opts := scoreOptions{Pass: query.PassRigor, AskConcurrency: new(4), providerOptions: scoreProvider()}
	got := scoreIn(t, scoreFixture(t), t.TempDir(), opts, func() model.Client { return scoring("none", "none", "none", "none") })
	if len(got.endpoints) == 0 {
		t.Fatal("the run built no endpoint")
	}
	for _, e := range got.endpoints {
		if e.IdleConns != 4 {
			t.Errorf("endpoint %q IdleConns = %d, want 4", e.Name, e.IdleConns)
		}
	}
}
