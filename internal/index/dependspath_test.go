package index

import (
	"slices"
	"testing"
)

func TestDependsGraphPath(t *testing.T) {
	g := DependsGraph{"a": {"c", "b"}, "b": {"d"}, "c": {"d"}, "d": {"e"}}
	for _, tc := range []struct {
		name, from, to string
		want           []string
	}{
		{"shortest, successors in sorted order", "a", "e", []string{"a", "b", "d", "e"}},
		{"one edge", "b", "d", []string{"b", "d"}},
		{"a node to itself", "c", "c", []string{"c"}},
		{"against the edges", "e", "a", nil},
		{"a node with no edges", "x", "a", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.Path(tc.from, tc.to); !slices.Equal(got, tc.want) {
				t.Errorf("Path(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}
