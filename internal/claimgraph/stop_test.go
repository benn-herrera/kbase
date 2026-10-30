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

// TestAnInferenceStopCarriesItsRemedy: the remedy a call that never completed
// names rides its stop into the stage's finding and the failure item; a call
// naming none leaves both without one.
func TestAnInferenceStopCarriesItsRemedy(t *testing.T) {
	for _, remedy := range []string{"the command that lets the call through", ""} {
		_, _, opts := seedDiscoveryLeaves(t, []string{"a"}, "a")
		r, err := runStage(opts, false, func(*Tree, []records.Record, writer) ([]Finding, error) {
			return nil, askStop(asks.IncompleteError{Subject: "S1", Detail: "http 401", Remedy: remedy})
		})
		if err != nil {
			t.Fatal(err)
		}
		failures := r.Failures()
		if len(failures) != 1 || failures[0].Remedy != remedy || failures[0].Detail != "S1: http 401" {
			t.Errorf("failures = %+v, want the one stop with remedy %q", failures, remedy)
		}
		last := r.Findings[len(r.Findings)-1].Fields
		if got := slices.ContainsFunc(last, func(f result.Field) bool { return f.Key == "remedy" }); got != (remedy != "") {
			t.Errorf("the stop's finding %v carries a remedy: %v, want %v", last, got, remedy != "")
		}
	}
}
