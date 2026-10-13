package claimgraph

import "testing"

// TestReferenceLineStaysBelowTheFrontmatter: a reference on a line contiguous
// with the up-link line is given its own words alone, and a reference on the
// up-link line is given none.
func TestReferenceLineStaysBelowTheFrontmatter(t *testing.T) {
	tree := &Tree{Documents: map[string]Document{
		"v/x.md": {Path: "v/x.md", Text: "---\nkind: leaf\n---\n[up](../index.md)\nWe use [the bound](y.md).\n\n# Heading\n"},
	}}
	for _, tc := range []struct {
		name string
		line int
		want string
	}{
		{"body line below the up-link", 4, "We use [the bound](y.md)."},
		{"up-link line", 3, ""},
		{"frontmatter line", 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := referenceLine(tree, "v/x.md", tc.line); got != tc.want {
				t.Errorf("referenceLine(line %d) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}
