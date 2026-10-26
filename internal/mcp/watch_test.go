package mcp

import (
	"testing"

	"kbase/internal/buildstore"
)

// TestBuildWatchAnnouncesEachRunOnce: the watcher announces a run's end once,
// keyed on the run's identity so a resumed build is a new run. A run already
// over at the subscription's first reading is announced there, but only for a
// run this server launched, or for any run where it launched none: until the
// child takes the store's lock the run before it reads interrupted, which is
// no new build's end — and which that guard consumes, so no later reading of it
// announces either.
func TestBuildWatchAnnouncesEachRunOnce(t *testing.T) {
	running := buildstore.State{State: buildstore.StateRunning, PID: 7, Started: "10:00"}
	finished := buildstore.State{State: buildstore.StateFinished, PID: 7, Started: "10:00"}
	resumed := buildstore.State{State: buildstore.StateFinished, PID: 9, Started: "10:30"}
	interrupted := buildstore.State{State: buildstore.StateInterrupted, PID: 5, Started: "09:00"}
	none := buildstore.State{State: buildstore.StateNone}
	for _, tc := range []struct {
		name     string
		launched int
		reads    []buildstore.State
		want     []bool
	}{
		{"a run ending while the subscription is held", 7,
			[]buildstore.State{running, running, finished, finished},
			[]bool{false, false, true, false}},
		{"a resumed build is a new run, announced once", 7,
			[]buildstore.State{finished, running, resumed, resumed},
			[]bool{true, false, true, false}},
		{"a store no build wrote", 7,
			[]buildstore.State{none, none},
			[]bool{false, false}},
		{"a run over before the subscription, this server's", 7,
			[]buildstore.State{finished},
			[]bool{true}},
		{"a run over before the subscription, one this server did not launch", 7,
			[]buildstore.State{interrupted, running, finished},
			[]bool{false, false, true}},
		{"a run this server did not launch, read twice, stays silent", 7,
			[]buildstore.State{interrupted, interrupted},
			[]bool{false, false}},
		{"a run over before the subscription, this server launched none", 0,
			[]buildstore.State{interrupted},
			[]bool{true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &buildWatch{atStart: true}
			for i, state := range tc.reads {
				if got := w.announce(state, tc.launched); got != tc.want[i] {
					t.Errorf("reading %d (%s, pid %d) announced %v, want %v", i+1, state.State, state.PID, got, tc.want[i])
				}
			}
		})
	}
}
