//go:build kbasedev

package buildstore

import (
	"slices"
	"testing"

	"kbase/internal/log/logtest"
)

// TestReplayProgressEndLogsTheCounts: a walk ended before the interval logs one
// progress record, End's, giving remaining as what neither counted.
func TestReplayProgressEndLogsTheCounts(t *testing.T) {
	for _, tc := range []struct {
		name                string
		total, done, failed int
	}{
		{"none counted", 3, 0, 0},
		{"some left", 5, 2, 1},
		{"every item counted", 3, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var lg logtest.Capture
			p := StartReplayProgress(&lg, tc.total)
			for range tc.done {
				p.Done()
			}
			for range tc.failed {
				p.Failed()
			}
			p.End()
			want := logtest.Record{Level: "info", Msg: "replay progress",
				Args: []any{"done", tc.done, "failed", tc.failed, "remaining", tc.total - tc.done - tc.failed, "total", tc.total}}
			got := lg.Snapshot()
			if len(got) != 1 || got[0].Level != want.Level || got[0].Msg != want.Msg || !slices.Equal(got[0].Args, want.Args) {
				t.Errorf("logged %+v, want only %+v", got, want)
			}
		})
	}
}
