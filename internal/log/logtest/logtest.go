// Package logtest is a log.Logger that keeps what it was told, so a test can
// assert that a decision was RECORDED without asserting how it was worded.
//
// It exists because four packages had grown a near-identical private copy of
// it, and the third copy's own comment named the threshold: "at a fourth
// copy, revisit". This is that revisit. It is a test-support package, not a
// _test.go file, because Go cannot share an unexported test helper across
// packages — and it costs nothing in the shipped binary, since nothing
// outside a test imports it.
//
// Assertions go through Has and Count, which match on level and on one
// key/value pair. Matching on the message string would make every record a
// checksum of its own phrasing, which is the test that breaks on a rewording
// and holds on a logic error.
package logtest

import (
	"fmt"
	"sync"
	"testing"
)

// Record is one captured log call.
type Record struct {
	Level string
	Msg   string
	Args  []any
}

// Capture is the recording log.Logger. The zero value is ready to use.
//
// It is mutex-guarded because a worker pool logs from several goroutines at
// once, and a logger that raced under -race would fail the tests it is there
// to support rather than the code they cover.
type Capture struct {
	mu      sync.Mutex
	records []Record
}

func (c *Capture) Debug(msg string, args ...any) { c.add("debug", msg, args) }
func (c *Capture) Info(msg string, args ...any)  { c.add("info", msg, args) }
func (c *Capture) Warn(msg string, args ...any)  { c.add("warn", msg, args) }
func (c *Capture) Error(msg string, args ...any) { c.add("error", msg, args) }

func (c *Capture) add(level, msg string, args []any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, Record{Level: level, Msg: msg, Args: args})
}

// Snapshot copies the records out under the lock, so every reader walks a
// slice nothing is still appending to.
func (c *Capture) Snapshot() []Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Record(nil), c.records...)
}

// Reset drops everything recorded so far, so a test that exercised a
// component to set a scene can then assert about the next step's records
// alone.
func (c *Capture) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = nil
}

// Has reports whether any record at the given level carries key=value.
// Values are compared by their formatted form, so a test states the value it
// expects to see rather than reconstructing the logger's argument types.
func (c *Capture) Has(t *testing.T, level, key string, value any) bool {
	t.Helper()
	return c.Count(level, key, value) > 0
}

// Count returns how many records at the given level carry key=value.
func (c *Capture) Count(level, key string, value any) int {
	want := fmt.Sprint(value)
	n := 0
	for _, r := range c.Snapshot() {
		if r.Level != level {
			continue
		}
		for i := 0; i+1 < len(r.Args); i += 2 {
			if k, ok := r.Args[i].(string); ok && k == key && fmt.Sprint(r.Args[i+1]) == want {
				n++
				break
			}
		}
	}
	return n
}
