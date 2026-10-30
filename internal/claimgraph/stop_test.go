package claimgraph

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/records"
	"kbase/internal/result"
)

// TestAStopWrappedWithAnUnwrittenRecordFails: a stop whose group records could
// not be written fails the stage on the write however the error is wrapped on
// its way out, and the report still ends on the stop's finding.
func TestAStopWrappedWithAnUnwrittenRecordFails(t *testing.T) {
	_, _, opts := seedDiscoveryLeaves(t, []string{"a"}, "a")
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	groups := []asks.GroupRecord{{Kind: asks.Paragraph, Group: "g", Items: []asks.ItemRecord{{Item: "S1", Outcome: asks.Failed}}}}
	stopErr := asks.IncompleteError{Subject: "S1", Detail: "no reply"}
	r, err := runStage(opts, false, func(*Tree, []records.Record, writer) ([]Finding, error) {
		return nil, fmt.Errorf("x: %w", askStopWriting(filepath.Join(file, "sub"), groups, stopErr))
	})
	if err == nil {
		t.Error("runStage returned nil, want the record write's failure")
	}
	inferenceStop := askStop(asks.IncompleteError{}).(*stop).check
	if !slices.ContainsFunc(r.Failures(), func(it result.Item) bool { return it.Check == inferenceStop }) {
		t.Errorf("the report holds %+v, want the inference stop", r.Failures())
	}
}
