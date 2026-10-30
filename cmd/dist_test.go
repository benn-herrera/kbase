//go:build !kbasedev

package main

import "testing"

// TestDistBuildHasNoReplay: the default build is the release shape, so it
// compiles out the development replays. The score and build commands it
// carries hold none of their flags and neither replay hook is set.
func TestDistBuildHasNoReplay(t *testing.T) {
	for _, name := range []string{"replay", "failed"} {
		if scoreCmd.Flags().Lookup(name) != nil {
			t.Errorf("score --%s is in the default build", name)
		}
	}
	if scoreReplay != nil {
		t.Error("the default build sets the score replay hook")
	}
	for _, name := range []string{"replay", "kind", "failed", "limit"} {
		if buildCmd.Flags().Lookup(name) != nil {
			t.Errorf("build --%s is in the default build", name)
		}
	}
	if buildReplay != nil {
		t.Error("the default build sets the build replay hook")
	}
}
