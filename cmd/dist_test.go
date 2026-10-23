//go:build kbasedist

package main

import "testing"

// TestDistBuildHasNoReplay: the dist build compiles out the development replay,
// so the shipped score command carries neither of its flags.
func TestDistBuildHasNoReplay(t *testing.T) {
	for _, name := range []string{"replay", "failed"} {
		if scoreCmd.Flags().Lookup(name) != nil {
			t.Errorf("score --%s is in the dist build", name)
		}
	}
	if scoreReplay != nil {
		t.Error("the dist build sets the replay hook")
	}
}
