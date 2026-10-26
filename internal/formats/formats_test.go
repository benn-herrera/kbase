package formats

import (
	"maps"
	"slices"
	"testing"

	"kbase/internal/formats/format100"
	"kbase/internal/formats/format200"
	"kbase/internal/formats/format300"
)

// TestChainReachesAPatchBeyondTheLastConverter: a patch above the form the last
// converter produces converts nothing — a patch changes the bytes a writer
// produces, not a file's form — so the chain hands the files back as the last
// step left them rather than naming a gap.
func TestChainReachesAPatchBeyondTheLastConverter(t *testing.T) {
	files := Files{"kb-root/a.md": []byte("prose")}
	out, obsolete, _, err := Chain(format100.Version100, "2.0.1", files)
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
	short, _, _, err := Chain(format100.Version100, format200.Version200, files)
	if err != nil {
		t.Fatalf("Chain(1.0.0, 2.0.0): %v", err)
	}
	if !maps.EqualFunc(out, short, func(a, b []byte) bool { return string(a) == string(b) }) {
		t.Errorf("the patch's chain = %v, want the form's own %v", out, short)
	}
}

// TestChainConvertsEveryPatchOfAMinor: one converter covers every patch of its
// minor, so a 2.0.1 KB converts through the 2.0.0 → 3.0.0 step as a 2.0.0 KB
// does.
func TestChainConvertsEveryPatchOfAMinor(t *testing.T) {
	files := Files{"kb-root/leaf.md": []byte("---\nkind: leaf\npath-stable: true\n---\n# A Leaf\n")}
	want, _, _, err := Chain(format200.Version200, format300.Version300, files)
	if err != nil {
		t.Fatalf("Chain(2.0.0, 3.0.0): %v", err)
	}
	got, _, _, err := Chain("2.0.1", format300.Version300, files)
	if err != nil {
		t.Fatalf("Chain(2.0.1, 3.0.0): %v", err)
	}
	if !maps.EqualFunc(got, want, func(a, b []byte) bool { return string(a) == string(b) }) {
		t.Errorf("2.0.1's chain = %v, want 2.0.0's %v", got, want)
	}
}

// TestCompareMinor: a patch does not order two versions.
func TestCompareMinor(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"2.0.1", "2.0.0", 0},
		{"2.0.9", "3.0.0", -1},
		{"3.1.0", "3.0.7", 1},
		{"4.0.0", "3.9.9", 1},
	}
	for _, tc := range tests {
		a, errA := ParseVersion(tc.a)
		b, errB := ParseVersion(tc.b)
		if errA != nil || errB != nil {
			t.Fatalf("ParseVersion(%s, %s): %v, %v", tc.a, tc.b, errA, errB)
		}
		if got := CompareMinor(a, b); max(-1, min(1, got)) != tc.want {
			t.Errorf("CompareMinor(%s, %s) = %d, want sign %d", tc.a, tc.b, got, tc.want)
		}
	}
	for _, bad := range []string{"3.0", "3.0.0.1", "3.01.0", "3.-1.0", "v3.0.0"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("ParseVersion(%q) accepted it", bad)
		}
	}
}

// TestChainCarriesItsStepsWarnings: a 1.0.0 KB runs both steps, and what the
// 2→3 step warns of reaches the chain's caller.
func TestChainCarriesItsStepsWarnings(t *testing.T) {
	files := Files{"kb-root/leaf.md": []byte("---\nkind: leaf\npath-stable: true\n---\n# A Leaf\n")}
	_, _, warnings, err := Chain(format100.Version100, format300.Version300, files)
	if err != nil {
		t.Fatalf("Chain(1.0.0, 3.0.0): %v", err)
	}
	if len(warnings) != 1 || warnings[0].Path != "kb-root/leaf.md" || warnings[0].Line != 3 {
		t.Errorf("warnings %+v, want the path-stable key at kb-root/leaf.md:3", warnings)
	}
}

// TestChainRefusesAGap: with converters to 1.1.0 and none beyond, a chain to
// 1.2.0 names the version it cannot leave.
func TestChainRefusesAGap(t *testing.T) {
	identity := func(f Files) (Files, []string, []Warning, error) { return f, nil, nil, nil }
	steps := append(slices.Clone(converters), converter{From: "1.0.0", To: "1.1.0", Convert: identity})
	_, _, _, err := chain(steps, format100.Version100, "1.2.0", Files{})
	if err == nil || err.Error() != "migrate: no converter leads from 1.1.0 toward 1.2.0" {
		t.Errorf("err = %v, want the gap at 1.1.0 named", err)
	}
}
