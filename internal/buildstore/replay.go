//go:build kbasedev

package buildstore

import (
	"sync/atomic"
	"time"

	"kbase/internal/log"
)

// ReplayKey is the key a development replay's refusals name.
const ReplayKey = "--replay"

// ReplayProgressInterval is the most often a development replay's walk logs
// its progress.
const ReplayProgressInterval = 10 * time.Second

// ReplayProgress counts a replay walk's items as their calls return, and logs
// the counts at info level once every ReplayProgressInterval and once more at
// End. Its items only count; its one goroutine logs.
type ReplayProgress struct {
	lg           log.Logger
	total        int
	done, failed atomic.Int64
	stop, ended  chan struct{}
}

// StartReplayProgress starts logging the progress of a walk of total items
// to lg. Every ReplayProgress it returns is ended by its End.
func StartReplayProgress(lg log.Logger, total int) *ReplayProgress {
	p := &ReplayProgress{lg: lg, total: total, stop: make(chan struct{}), ended: make(chan struct{})}
	go p.tick()
	return p
}

// Done counts an item whose call returned an answer.
func (p *ReplayProgress) Done() { p.done.Add(1) }

// Failed counts an item whose call returned a failure.
func (p *ReplayProgress) Failed() { p.failed.Add(1) }

// End stops the ticking and logs the walk's last progress line. Called once
// the walk has drained, the line's counts are the walk's.
func (p *ReplayProgress) End() {
	close(p.stop)
	<-p.ended
	p.log()
}

func (p *ReplayProgress) tick() {
	defer close(p.ended)
	t := time.NewTicker(ReplayProgressInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			p.log()
		case <-p.stop:
			return
		}
	}
}

func (p *ReplayProgress) log() {
	done, failed := int(p.done.Load()), int(p.failed.Load())
	p.lg.Info("replay progress", "done", done, "failed", failed, "remaining", p.total-done-failed, "total", p.total)
}
