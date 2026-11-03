package main

import (
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/write"
)

// TestRelationLinesAreJoinsPairs: each relation line's sources × targets is
// exactly the kind pairs write.Joins joins for its relation, every relation
// Joins joins for some pair has a line, and add-edge's help holds each line.
func TestRelationLinesAreJoinsPairs(t *testing.T) {
	lines := map[string]relationLine{}
	for _, l := range relationLines() {
		lines[l.relation] = l
	}
	for _, r := range kb.Relations {
		l, listed := lines[r]
		for _, s := range kb.NodeKinds {
			for _, k := range kb.NodeKinds {
				joined := write.Joins(r, s, k)
				if joined && !listed {
					t.Errorf("Joins joins %s from %s into %s, and no line states %s", r, s, k, r)
				}
				if listed && joined != (slices.Contains(l.sources, s) && slices.Contains(l.targets, k)) {
					t.Errorf("%s's line: sources %q × targets %q, but Joins(%s, %s, %s) is %v", r, l.sources, l.targets, r, s, k, joined)
				}
			}
		}
	}
	long := addEdgeCommand().Long
	for _, l := range relationLines() {
		if !strings.Contains(long, l.help()) {
			t.Errorf("add-edge's help lacks its %s line %q", l.relation, l.help())
		}
	}
}
