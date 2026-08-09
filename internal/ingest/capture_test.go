package ingest

import (
	"fmt"
	"testing"
)

// capture is a log.Logger that keeps what it was told, so a test can assert
// that a decision was RECORDED without asserting how it was worded.
//
// Assertions go through has, which matches on level and on a key/value pair.
// Matching on the message string would make every record a checksum of its
// own phrasing: the next person to improve the wording would break tests that
// have nothing to do with the behavior they changed.
type capture struct {
	records []record
}

type record struct {
	level string
	msg   string
	args  []any
}

func (c *capture) Debug(msg string, args ...any) { c.add("debug", msg, args) }
func (c *capture) Info(msg string, args ...any)  { c.add("info", msg, args) }
func (c *capture) Warn(msg string, args ...any)  { c.add("warn", msg, args) }
func (c *capture) Error(msg string, args ...any) { c.add("error", msg, args) }

func (c *capture) add(level, msg string, args []any) {
	c.records = append(c.records, record{level: level, msg: msg, args: args})
}

// has reports whether any record at the given level carries key=value.
// Values are compared by their formatted form, so a test states the value it
// expects to see rather than reconstructing the logger's argument types.
func (c *capture) has(t *testing.T, level, key string, value any) bool {
	t.Helper()
	want := fmt.Sprint(value)
	for _, r := range c.records {
		if r.level != level {
			continue
		}
		for i := 0; i+1 < len(r.args); i += 2 {
			if k, ok := r.args[i].(string); ok && k == key && fmt.Sprint(r.args[i+1]) == want {
				return true
			}
		}
	}
	return false
}
