package migrate

import (
	"maps"
	"slices"
	"testing"

	"kbase/internal/migrate/format_1_0_0"
	"kbase/internal/migrate/format_2_0_0"
)

// TestChainReachesAPatchBeyondTheLastConverter: a patch above the form the last
// converter produces converts nothing — a patch changes the bytes a writer
// produces, not a file's form — so the chain hands the files back as the last
// step left them rather than naming a gap.
func TestChainReachesAPatchBeyondTheLastConverter(t *testing.T) {
	files := Files{"kb-root/a.md": []byte("prose")}
	out, obsolete, err := Chain(format_1_0_0.Version100, "2.0.1", files)
	if err != nil {
		t.Fatalf("Chain(1.0.0, 2.0.1): %v", err)
	}
	if len(obsolete) != 0 {
		t.Errorf("the patch made %v obsolete, want nothing", obsolete)
	}
	if _, held := out["kb-root/a.md"]; !held {
		t.Error("the chain lost a file the patch does not touch")
	}
	// The same target one step short: the same files, so a patch above the form
	// changes no file's form.
	short, _, err := Chain(format_1_0_0.Version100, format_2_0_0.Version200, files)
	if err != nil {
		t.Fatalf("Chain(1.0.0, 2.0.0): %v", err)
	}
	if !maps.EqualFunc(out, short, func(a, b []byte) bool { return string(a) == string(b) }) {
		t.Errorf("the patch's chain = %v, want the form's own %v", out, short)
	}
}

// TestChainRefusesAGap: with converters to 1.1.0 and none beyond, a chain to
// 1.2.0 names the version it cannot leave.
func TestChainRefusesAGap(t *testing.T) {
	identity := func(f Files) (Files, []string, error) { return f, nil, nil }
	steps := append(slices.Clone(converters), converter{From: "1.0.0", To: "1.1.0", Convert: identity})
	_, _, err := chain(steps, format_1_0_0.Version100, "1.2.0", Files{})
	if err == nil || err.Error() != "migrate: no converter leads from 1.1.0 toward 1.2.0" {
		t.Errorf("err = %v, want the gap at 1.1.0 named", err)
	}
}
