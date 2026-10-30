package mcp

import (
	"context"
	"time"

	"kbase/internal/buildstore"
	"kbase/internal/log"
	"kbase/internal/mcp/protocol"
)

// buildWatchInterval is how often the watcher reads the store after the reading
// the subscription itself makes. ARCHITECTURE §12 carries it.
const buildWatchInterval = time.Second

// buildSubscription is the subscription to the build's state: the watcher that
// reads the store last names and announces each run that stops progressing. A
// subscription taken again after a drop starts a new watcher, whose first
// reading is the store as it stands then.
func buildSubscription(last *lastBuild, lg log.Logger) *protocol.Subscription {
	return &protocol.Subscription{URI: BuildURI, Watch: func(ctx context.Context, updated func()) {
		w := &buildWatch{last: last, lg: lg, atStart: true}
		w.watch(ctx, updated)
	}}
}

// buildWatch is one subscription's following of the build's state: the run it
// last read, and whether it has announced that run's end.
type buildWatch struct {
	last    *lastBuild
	lg      log.Logger
	atStart bool
	seen    runID
	done    bool
}

// runID is one run of a build: the pid that ran it and its start stamp, which
// together name no other run — a resumed build is a new pid, a new stamp.
type runID struct {
	pid     int
	started string
}

// watch reads the store at the subscription and then every
// buildWatchInterval, announcing each run that is no longer progressing, until
// the subscription ends.
func (w *buildWatch) watch(ctx context.Context, updated func()) {
	tick := time.NewTicker(buildWatchInterval)
	defer tick.Stop()
	for {
		w.read(updated)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// read is one reading of the store, announcing a run that ended since the last
// one announced.
func (w *buildWatch) read(updated func()) {
	_, dir, launched, _ := w.last.store()
	if dir == "" {
		w.atStart = false
		return
	}
	state, err := buildstore.ReadState(dir)
	if err != nil {
		w.lg.Error("mcp: reading the build's state failed", "state-dir", dir, "err", err)
		w.atStart = false
		return
	}
	if w.announce(state, launched) {
		updated()
	}
}

// announce is whether the reading is a run's end to tell the client about: a
// run that is over and has not been announced. A run already over at the
// subscription's first reading is announced there — the event is the only
// channel — but only for a run this server launched, or for any run where it
// launched none: a launch is not atomic, and until the child takes the store's
// lock the run before it reads interrupted, which is no new build's end.
func (w *buildWatch) announce(state buildstore.State, launched int) bool {
	atStart := w.atStart
	w.atStart = false
	id := runID{pid: state.PID, started: state.Started}
	if id != w.seen {
		w.seen, w.done = id, false
	}
	switch {
	case w.done, !buildstore.Ended(state.State):
		return false
	case atStart && launched != 0 && launched != state.PID:
		// Consume it: this reading is dropped, and a later one of the same run
		// must not take it for the new build's end.
		w.done = true
		return false
	}
	w.done = true
	return true
}
