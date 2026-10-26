package buildstore

import (
	"strings"
	"testing"
)

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
