package buildstore

import (
	"slices"
	"strings"
	"testing"

	"kbase/internal/buildrecords"
)

// TestOwnedPathsHoldEveryBuildRecord: every build record lies under a path
// the build owns, so the ledger commits, restores and dirt-checks it with no
// list of its own.
func TestOwnedPathsHoldEveryBuildRecord(t *testing.T) {
	for _, p := range buildrecords.RecordFiles {
		if !slices.ContainsFunc(OwnedPaths, func(owned string) bool { return strings.HasPrefix(p, owned+"/") }) {
			t.Errorf("the build does not own %s", p)
		}
	}
}
