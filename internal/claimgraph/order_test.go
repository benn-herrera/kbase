package claimgraph

import (
	"math"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
)

const orderLeaf = "v/leaf.md"

// orderFixture is one leaf under one volume: clm-b and clm-d share a
// paragraph, a non-claim paragraph follows, then clm-a, then the fence of
// equation node clm-e, then clm-c. Id order differs from line order.
func orderFixture(ids ...string) (*Tree, *AuthoredGraph, *Inventory) {
	m := kb.StatementMarker
	leaf := strings.Join([]string{
		"[up](index.md)",
		"# Leaf",
		"",
		"Beta holds. " + m("clm-b"),
		"Delta holds too. " + m("clm-d"),
		"",
		"A remark that states nothing.",
		"",
		"Alpha holds. " + m("clm-a"),
		"",
		"$$",
		"x = 1",
		"$$",
		"",
		"Gamma holds. " + m("clm-c"),
		"",
	}, "\n")
	t := &Tree{
		Documents: map[string]Document{
			kb.EntryPointFile: {Path: kb.EntryPointFile},
			"v/index.md":      {Path: "v/index.md"},
			orderLeaf:         {Path: orderLeaf, Text: leaf},
		},
		Paths:    []string{kb.EntryPointFile, "v/index.md", orderLeaf},
		Children: map[string][]string{kb.EntryPointFile: {"v/index.md"}, "v/index.md": {orderLeaf}},
	}
	all := map[string]ClaimNode{
		"clm-a": {ID: "clm-a", Document: orderLeaf, Title: "Alpha"},
		"clm-b": {ID: "clm-b", Document: orderLeaf, Title: "Beta"},
		"clm-c": {ID: "clm-c", Document: orderLeaf, Title: "Gamma"},
		"clm-d": {ID: "clm-d", Document: orderLeaf, Title: "Delta"},
		"clm-e": {ID: "clm-e", Document: orderLeaf, Title: "(eq:one)", Equation: "eq:one"},
		"clm-z": {ID: "clm-z", Document: orderLeaf, Title: "Unmarked"},
	}
	var nodes []ClaimNode
	for _, id := range ids {
		nodes = append(nodes, all[id])
	}
	inv := &Inventory{Fences: []MathFence{{Document: orderLeaf, Start: 10, End: 13, Labels: []string{"eq:one"}}}}
	return t, graphOf(nodes...), inv
}

func TestReadingOrderFollowsChildren(t *testing.T) {
	tree := &Tree{Children: map[string][]string{
		kb.EntryPointFile: {"z/index.md", "a/index.md"},
		"z/index.md":      {"z/b.md", "z/a.md"},
		"a/index.md":      {"a/x.md"},
	}}
	want := []string{kb.EntryPointFile, "z/index.md", "z/b.md", "z/a.md", "a/index.md", "a/x.md"}
	if got := readingOrder(tree); !slices.Equal(got, want) {
		t.Errorf("readingOrder = %q, want %q", got, want)
	}
}

func TestAdjacentLinks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ids    []string
		marked map[link]bool
		want   []link
	}{
		{"sharing a paragraph", []string{"clm-b", "clm-d"}, nil, []link{{"clm-b", "clm-d"}}},
		{"across a non-claim paragraph", []string{"clm-d", "clm-a"}, nil, []link{{"clm-a", "clm-d"}}},
		{"across an equation node", []string{"clm-a", "clm-e", "clm-c"}, nil, []link{{"clm-a", "clm-c"}}},
		{"a marked pair left out", []string{"clm-a", "clm-b", "clm-c", "clm-d"}, map[link]bool{linkOf("clm-d", "clm-a"): true},
			[]link{{"clm-a", "clm-c"}, {"clm-b", "clm-d"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, g, inv := orderFixture(tc.ids...)
			got := adjacentLinks(tree, g, inv, claimPositions(tree, g, inv), tc.marked)
			if !slices.Equal(got, tc.want) {
				t.Errorf("adjacentLinks = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClaimPositions(t *testing.T) {
	tree, g, inv := orderFixture("clm-e", "clm-z")
	pos := claimPositions(tree, g, inv)
	for _, tc := range []struct {
		name, id string
		want     position
	}{
		{"an equation node at its fence's start", "clm-e", position{2, 10}},
		{"a claim nothing reaches last in its document", "clm-z", position{2, math.MaxInt}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pos[tc.id]; got != tc.want {
				t.Errorf("position of %s = %+v, want %+v", tc.id, got, tc.want)
			}
		})
	}
}
