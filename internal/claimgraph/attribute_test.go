package claimgraph

import "testing"

// TestReferenceLineStaysBelowTheUpLink: a reference on a line contiguous with
// the up-link line is given its own words alone, and a reference on the
// up-link line is given none.
func TestReferenceLineStaysBelowTheUpLink(t *testing.T) {
	tree := &Tree{Documents: map[string]Document{
		"v/x.md": {Path: "v/x.md", Text: "[up](../index.md)\nWe use [the bound](y.md).\n\n# Heading\n"},
	}}
	for _, tc := range []struct {
		name string
		line int
		want string
	}{
		{"body line below the up-link", 1, "We use [the bound](y.md)."},
		{"up-link line", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := referenceLine(tree, "v/x.md", tc.line); got != tc.want {
				t.Errorf("referenceLine(line %d) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}
