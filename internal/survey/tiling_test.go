package survey

import (
	"strings"
	"testing"
)

// TestVerifyTilingRejects exercises the guard directly. It is the production
// assertion that a heading tree covers its source exactly once, so each way
// of failing that gets its own case — a guard nobody has watched fail is a
// guard nobody knows still works.
func TestVerifyTilingRejects(t *testing.T) {
	const size = 100
	whole := File{Sections: []Section{{Level: 1, Title: "A", Start: 0, End: size}}}

	for _, tc := range []struct {
		name string
		file File
		size int
		want string // "" means the file must verify
	}{{
		name: "single section covering the file",
		file: whole,
		size: size,
	}, {
		name: "metadata block then preamble then sections",
		file: File{
			Metadata: &Range{Start: 0, End: 10},
			Preamble: &Section{Start: 10, End: 20},
			Sections: []Section{{Level: 1, Start: 20, End: size}},
		},
		size: size,
	}, {
		name: "nested children reaching the parent's end",
		file: File{Sections: []Section{{Level: 1, Start: 0, End: size, Children: []Section{
			{Level: 2, Start: 10, End: 40},
			{Level: 2, Start: 40, End: size},
		}}}},
		size: size,
	}, {
		name: "gap between siblings",
		file: File{Sections: []Section{{Level: 1, Start: 0, End: 40}, {Level: 1, Start: 50, End: size}}},
		size: size,
		want: "starts at 50, want 40",
	}, {
		name: "overlapping siblings",
		file: File{Sections: []Section{{Level: 1, Start: 0, End: 60}, {Level: 1, Start: 50, End: size}}},
		size: size,
		want: "starts at 50, want 60",
	}, {
		name: "children stop short of the parent",
		file: File{Sections: []Section{{Level: 1, Start: 0, End: size, Children: []Section{
			{Level: 2, Start: 10, End: 40},
		}}}},
		size: size,
		want: "children end at 40",
	}, {
		name: "coverage stops before the end of the file",
		file: File{Sections: []Section{{Level: 1, Start: 0, End: 40}}},
		size: size,
		want: "cover 40 bytes, file is 100",
	}, {
		name: "preamble does not follow the metadata block",
		file: File{Metadata: &Range{Start: 0, End: 10}, Preamble: &Section{Start: 12, End: size}},
		size: size,
		want: "preamble starts at 12",
	}, {
		name: "metadata block not at the start of the file",
		file: File{Metadata: &Range{Start: 5, End: 10}, Preamble: &Section{Start: 10, End: size}},
		size: size,
		want: "must start the file",
	}, {
		name: "inverted range",
		file: File{Sections: []Section{{Level: 1, Start: 0, End: -5}}},
		size: size,
		want: "is inverted",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyTiling(tc.file, tc.size)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("verifyTiling: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}
