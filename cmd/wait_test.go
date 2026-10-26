package main

import (
	"testing"

	toolresult "kbase/internal/result"
)

// TestWaitRefusesAnAbsentOrNonPositivePid: --pid names the run to wait for;
// absent or not positive is a usage refusal, before any store is read.
func TestWaitRefusesAnAbsentOrNonPositivePid(t *testing.T) {
	for name, args := range map[string][]string{
		"absent":   nil,
		"zero":     {"--pid", "0"},
		"negative": {"--pid=-1"},
	} {
		t.Run(name, func(t *testing.T) {
			out, code := kbase(t, t.TempDir(), append([]string{"wait"}, args...)...)
			doc := checkDocument(t, "wait", out)
			items := doc.items(toolresult.RefusalsKey)
			if doc.Outcome != toolresult.Refused || code != toolresult.ExitCode(toolresult.Refused) || len(items) != 1 ||
				items[0]["check"] != checkUsage || items[0]["key"] != "--pid" {
				t.Errorf("wait %q = outcome %s, exit %d, %v\n%s", args, doc.Outcome, code, items, out)
			}
		})
	}
}
