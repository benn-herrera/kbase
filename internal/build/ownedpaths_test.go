package build

import (
	"slices"
	"testing"

	"kbase/internal/buildrecords"
	"kbase/internal/migrate"
)

// TestOwnedPathsHoldEveryBuildRecord: the ledger commits, restores and
// dirt-checks every build record, current and superseded, with no list of
// its own.
func TestOwnedPathsHoldEveryBuildRecord(t *testing.T) {
	for _, p := range slices.Concat(buildrecords.RecordFiles, migrate.SupersededRecordPaths()) {
		if !slices.Contains(ownedPaths, p) {
			t.Errorf("the build does not own %s", p)
		}
	}
}
