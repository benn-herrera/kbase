package buildstore

import (
	"slices"
	"testing"

	"kbase/internal/buildrecords"
)

// TestOwnedPathsHoldEveryBuildRecord: the ledger commits, restores and
// dirt-checks every build record, with no list of its own.
func TestOwnedPathsHoldEveryBuildRecord(t *testing.T) {
	for _, p := range buildrecords.RecordFiles {
		if !slices.Contains(OwnedPaths, p) {
			t.Errorf("the build does not own %s", p)
		}
	}
}
