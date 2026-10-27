package buildstore

import (
	"slices"
	"strings"
	"testing"
)

// TestEveryModelCallingStepFollowsStart: a step that asks a model belongs to a
// stage after start, so the trail's start entry, which names the per-trail
// directory, exists before any ask reads it.
func TestEveryModelCallingStepFollowsStart(t *testing.T) {
	index := func(id string) int {
		return slices.IndexFunc(Stages, func(s Stage) bool { return s.ID == id })
	}
	start := index(StageStart)
	for _, s := range Steps {
		if !s.CallsAModel() {
			continue
		}
		if at := index(s.Stage); at <= start {
			t.Errorf("step %s calls a model in stage %s (position %d), not after %s (position %d)", s.ID, s.Stage, at, StageStart, start)
		}
	}
}

func TestResolveStage(t *testing.T) {
	for name, want := range map[string]string{
		"phase-3a": "phase-3a", "  Validation   Gate ": "phase-3a", "DEPENDS-ATTRIBUTED": "depends-attributed",
		"claim discovery":         "claims-discovered",
		"co-linear claims culled": "claims-culled",
	} {
		if s, ok := ResolveStage(name); !ok || s.ID != want {
			t.Errorf("ResolveStage(%q) = %v, %t; want %s", name, s, ok, want)
		}
	}
	if _, ok := ResolveStage("phase-5"); ok {
		t.Error("a name outside the vocabulary must resolve to no stage")
	}
}

func TestDroppedRowsFollowInference(t *testing.T) {
	var dropped []string
	for _, s := range Steps {
		if !s.applies(true) {
			dropped = append(dropped, s.ID)
		}
	}
	if strings.Join(dropped, ",") != "discover.build,unmarked.build,cull.build,ov.docs" {
		t.Errorf("rows dropped under --no-inference = %q, want the four that spend it", dropped)
	}
}
