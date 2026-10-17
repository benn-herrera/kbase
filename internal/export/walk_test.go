package export

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// writeKB authors a KB tree under a fresh temp dir: a map of kb-root-relative
// slash paths to their text, opened as the source under test. The tree is
// hand-authored so the walk's own contract — not one build's output — is what
// the test asserts.
func writeKB(t *testing.T, files map[string]string) *kb.Source {
	t.Helper()
	repo := t.TempDir()
	root := filepath.Join(repo, kb.KBDir)
	for rel, text := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return kb.OnDisk(root)
}

// smallKB is the fixture the walk and strip tests share: an entry point
// naming two volumes, each index carrying a spine, a nested index, a no-claim
// leaf, an overview.md, a references.md and a cross-linking leaf.
func smallKB() map[string]string {
	return map[string]string{
		"entry-point.md": `---
kind: entry-point
---
[↑ Entry Point](entry-point.md)

# A KB

A blurb about the KB.

## What this KB contains

Some authored prose.

## Key Results

Another authored section.

## Volumes

- [First](vol1/index.md) — the first volume
- [Second](vol2/index.md) — the second volume
`,
		"vol1/index.md": `---
kind: index
---
[↑ A KB](../entry-point.md)

# First Volume

## Key Results

Prose carrying a link to the first leaf ([§1](v1-leaf.md)) and more prose.

## Derivations

- [§1 The Leaf](v1-leaf.md) — a leaf
- [§2 The Nested Index](nested/index.md) — a nested index
- [Overview](overview.md) — the drafted overview
- [References](references.md) — the bibliography
`,
		"vol1/v1-leaf.md": `---
kind: leaf
no-claim: true
---
[↑ First Volume](index.md)

# §1 The Leaf

Body text with a cross-reference to [the other leaf](../vol2/v2-leaf.md).
`,
		"vol1/overview.md": `---
kind: leaf
---
[↑ First Volume](index.md)

# Overview

The drafted overview body.
`,
		"vol1/references.md": `---
kind: leaf
---
[↑ First Volume](index.md)

# References

- Entry one
`,
		"vol1/nested/index.md": `---
kind: index
---
[↑ First Volume](../index.md)

# Nested

- [Deep Leaf](deep.md) — one rank deeper
`,
		"vol1/nested/deep.md": `---
kind: leaf
---
[↑ Nested](index.md)

# Deep

Deeper body.
`,
		"vol2/index.md": `---
kind: index
---
[↑ A KB](../entry-point.md)

# Second Volume

- [The Second Leaf](v2-leaf.md) — a leaf
`,
		"vol2/v2-leaf.md": `---
kind: leaf
---
[↑ Second Volume](index.md)

# §1 The Second Leaf

Body.
`,
	}
}

// TestWalkSpine: the walk reads the entry point's volume list in written
// order, descends each index spine in the source's order one rank per nested
// index, skips references.md, and mints each document's label from its path
// below the volume.
func TestWalkSpine(t *testing.T) {
	src := writeKB(t, smallKB())
	rep := &Report{}
	tree, err := Walk(src, rep)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if tree.EntryPoint != "entry-point.md" {
		t.Errorf("EntryPoint = %q", tree.EntryPoint)
	}
	want := Tree{
		EntryPoint: "entry-point.md",
		Volumes: []Volume{
			{Dir: "vol1", Label: "vol1", Title: "First Volume", Documents: []Document{
				{Path: "vol1/v1-leaf.md", Rank: 1, Label: "d1-v1-leaf"},
				{Path: "vol1/nested/index.md", Rank: 1, Label: "d1-nested-index"},
				{Path: "vol1/nested/deep.md", Rank: 2, Label: "d1-nested-deep"},
				{Path: "vol1/overview.md", Rank: 1, Label: "d1-overview"},
			}},
			{Dir: "vol2", Label: "vol2", Title: "Second Volume", Documents: []Document{
				{Path: "vol2/v2-leaf.md", Rank: 1, Label: "d2-v2-leaf"},
			}},
		},
	}
	if !reflect.DeepEqual(tree, want) {
		t.Errorf("Walk = %+v\nwant       %+v", tree, want)
	}
	if !rep.Empty() {
		t.Errorf("Walk reported %q over a clean tree", rep.Findings())
	}
	targets := tree.Targets()
	for p, want := range map[string]Target{
		"vol1/nested/deep.md": {Label: "d1-nested-deep", Volume: 1},
		"vol2/v2-leaf.md":     {Label: "d2-v2-leaf", Volume: 2},
		"vol2/index.md":       {Label: "vol2", Volume: 2},
	} {
		if targets[p] != want {
			t.Errorf("Targets()[%s] = %+v; want %+v", p, targets[p], want)
		}
	}
}

// TestWalkReportsLabelCollision: two reachable documents of one volume whose
// paths both slug away to nothing mint the same label — the path is exhausted —
// and the walk reports the collision rather than silently overwriting; the
// first path keeps the label, the finding naming it, and the second holds none.
func TestWalkReportsLabelCollision(t *testing.T) {
	files := smallKB()
	files["vol1/index.md"] = `---
kind: index
---
[↑ A KB](../entry-point.md)

# First Volume

- [One](§.md) — a name that slugs away
- [Two](日.md) — another
`
	files["vol1/§.md"] = "---\nkind: leaf\n---\n[↑ First Volume](index.md)\n\n# One\n\nBody.\n"
	files["vol1/日.md"] = "---\nkind: leaf\n---\n[↑ First Volume](index.md)\n\n# Two\n\nBody.\n"
	src := writeKB(t, files)
	rep := &Report{}
	tree, err := Walk(src, rep)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if rep.Empty() {
		t.Fatalf("no collision reported; findings %q", rep.Findings())
	}
	for _, f := range rep.Findings() {
		if f.Form != labelForm || !strings.Contains(f.Reason, "label d1- already names vol1/§.md") {
			t.Errorf("finding %+v is not the collision on the exhausted label", f)
		}
	}
	targets := tree.Targets()
	if got := targets["vol1/§.md"].Label; got != "d1-" {
		t.Errorf("the first path's label = %q; want d1-", got)
	}
	if got, ok := targets["vol1/日.md"]; ok {
		t.Errorf("the second path holds %+v; want no target", got)
	}
}

// TestWalkLabelsComplete: every walked path holds a target, and no two targets
// share a label. The two same-stem documents in different directories are
// labelled apart by the directories they sit under.
func TestWalkLabelsComplete(t *testing.T) {
	files := smallKB()
	files["vol1/index.md"] = `---
kind: index
---
[↑ A KB](../entry-point.md)

# First Volume

- [Introduction](introduction/index.md) — one branch
- [Appendix](appendix/index.md) — another branch
`
	files["vol1/introduction/index.md"] = `---
kind: index
---
[↑ First Volume](../index.md)

# Introduction

- [Overview](overview.md) — the introduction's overview
`
	files["vol1/introduction/overview.md"] = "---\nkind: leaf\n---\n[↑ Introduction](index.md)\n\n# Overview one\n\nBody.\n"
	files["vol1/appendix/index.md"] = `---
kind: index
---
[↑ First Volume](../index.md)

# Appendix

- [Overview](overview.md) — the appendix's overview
`
	files["vol1/appendix/overview.md"] = "---\nkind: leaf\n---\n[↑ Appendix](index.md)\n\n# Overview two\n\nBody.\n"
	src := writeKB(t, files)
	rep := &Report{}
	tree, err := Walk(src, rep)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if !rep.Empty() {
		t.Errorf("Walk reported %q; the paths are distinct", rep.Findings())
	}
	targets := tree.Targets()
	labelable := len(tree.Volumes)
	for _, v := range tree.Volumes {
		labelable += len(v.Documents)
	}
	if len(targets) != labelable {
		t.Errorf("Targets() holds %d paths for %d walked things: %v", len(targets), labelable, targets)
	}
	owner := map[string]string{}
	for p, target := range targets {
		if prev, taken := owner[target.Label]; taken {
			t.Errorf("label %s names both %s and %s", target.Label, prev, p)
		}
		owner[target.Label] = p
	}
	for _, want := range []string{"vol1/introduction/overview.md", "vol1/appendix/overview.md"} {
		if _, ok := targets[want]; !ok {
			t.Errorf("Targets() holds no %q: %v", want, targets)
		}
	}
}

// TestWalkMissingTargetsReported: a child-list target naming no file, a missing
// entry-point volume, and a volume the entry point names twice each raise a
// walk finding naming the document that named the target; none of them joins
// the spine.
func TestWalkMissingTargetsReported(t *testing.T) {
	files := smallKB()
	files["entry-point.md"] = `---
kind: entry-point
---

# A KB

## Volumes

- [First](vol1/index.md) — the first volume
- [First again](vol1/index.md) — named twice
- [Ghost](vol9/index.md) — a volume with no directory
`
	files["vol1/index.md"] = `---
kind: index
---
[↑ A KB](../entry-point.md)

# First Volume

- [Missing Leaf](no-such-leaf.md) — a child naming no file
`
	src := writeKB(t, files)
	rep := &Report{}
	tree, err := Walk(src, rep)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	reasons := reasonsOf(rep.Findings(), walkForm, "entry-point.md")
	for _, want := range []string{"vol9/index.md", "already walked"} {
		if !containsReason(reasons, want) {
			t.Errorf("no entry-point walk finding naming %q: %v", want, rep.Findings())
		}
	}
	leafReasons := reasonsOf(rep.Findings(), walkForm, "vol1/index.md")
	if !containsReason(leafReasons, "vol1/no-such-leaf.md") {
		t.Errorf("no finding names the missing leaf: %v", rep.Findings())
	}
	for _, d := range tree.Volumes[0].Documents {
		if d.Path == "vol1/no-such-leaf.md" {
			t.Errorf("the missing leaf joined the spine: %+v", d)
		}
	}
	if len(tree.Volumes) != 1 {
		t.Errorf("Walk = %+v; want the one volume that exists", tree)
	}
}

// TestWalkIndexCycle: an index naming itself or an ancestor raises a walk
// finding and does not descend, so a cycle is a finding rather than an
// unbounded recursion.
func TestWalkIndexCycle(t *testing.T) {
	for _, tc := range []struct {
		name  string
		index string
	}{
		{"self-naming index", "# Cycle\n\n- [Self](index.md) — the same index\n"},
		{"two-index cycle", "# Cycle\n\n- [Child](child/index.md) — the other\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := smallKB()
			files["vol1/index.md"] = "---\nkind: index\n---\n[↑ A KB](../entry-point.md)\n\n" + tc.index
			files["vol1/child/index.md"] = `---
kind: index
---
[↑ First Volume](../index.md)

# Child

- [Up](index.md) — names the ancestor
`
			src := writeKB(t, files)
			rep := &Report{}
			tree, err := Walk(src, rep)
			if err != nil {
				t.Fatalf("Walk: %v", err)
			}
			if !containsReason(reasonsOf(rep.Findings(), walkForm, ""), "already walked") {
				t.Errorf("no repeat finding raised: %v", rep.Findings())
			}
			for _, d := range tree.Volumes[0].Documents {
				if d.Path == "vol1/index.md" {
					t.Errorf("the cycle re-entered the ancestor: %+v", d)
				}
			}
			if n := len(tree.Volumes[0].Documents); n > 1 {
				t.Errorf("vol1 walked %d documents; want at most the one index below it: %+v", n, tree.Volumes[0].Documents)
			}
		})
	}
}

// TestWalkOutOfVolumeTarget: a child-list target resolving outside its volume's
// directory is reported and not walked, so another volume's subtree does not
// enter this volume's spine under this volume's number.
func TestWalkOutOfVolumeTarget(t *testing.T) {
	files := smallKB()
	files["vol1/index.md"] = `---
kind: index
---
[↑ A KB](../entry-point.md)

# First Volume

- [The second leaf](../vol2/v2-leaf.md) — another volume's content
`
	src := writeKB(t, files)
	rep := &Report{}
	tree, err := Walk(src, rep)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if !containsReason(reasonsOf(rep.Findings(), walkForm, "vol1/index.md"), "vol2/v2-leaf.md") {
		t.Errorf("no finding names the out-of-volume target: %v", rep.Findings())
	}
	for _, d := range tree.Volumes[0].Documents {
		if d.Path == "vol2/v2-leaf.md" {
			t.Errorf("the out-of-volume target joined vol1's spine: %+v", d)
		}
	}
}

// TestWalkTitleIsNotFenced: an index whose only heading-shaped line sits in a
// fenced block has no title, rather than taking the example's line as its
// \chapter title.
func TestWalkTitleIsNotFenced(t *testing.T) {
	files := smallKB()
	files["vol1/index.md"] = "---\nkind: index\n---\n[↑ A KB](../entry-point.md)\n\n" +
		"Write headings like this:\n\n```\n# Not the title\n```\n\nProse.\n"
	src := writeKB(t, files)
	tree, err := Walk(src, &Report{})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got := tree.Volumes[0].Title; got != "" {
		t.Errorf("Title = %q; want the fenced heading ignored", got)
	}
}

// TestWalkInlineMathSurvivesTitle: an index title carrying inline maths keeps
// it, the fence blanking not reaching into the heading line.
func TestWalkInlineMathSurvivesTitle(t *testing.T) {
	files := smallKB()
	files["vol1/index.md"] = "---\nkind: index\n---\n[↑ A KB](../entry-point.md)\n\n" +
		"# On the $`\\mathrm{L}^p`$-theory\n\nProse.\n"
	src := writeKB(t, files)
	tree, err := Walk(src, &Report{})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if want := "On the $`\\mathrm{L}^p`$-theory"; tree.Volumes[0].Title != want {
		t.Errorf("Title = %q; want %q", tree.Volumes[0].Title, want)
	}
}

// TestWalkNestedIndexDoubleSpine: a document reachable by two spines of one
// volume walks once, under its first walk's label, and the second naming raises
// a finding.
func TestWalkNestedIndexDoubleSpine(t *testing.T) {
	files := smallKB()
	files["vol1/index.md"] = `---
kind: index
---
[↑ A KB](../entry-point.md)

# First Volume

- [Nested](nested/index.md) — the nested index
- [Deep again](nested/deep.md) — names the same leaf a second time
`
	src := writeKB(t, files)
	rep := &Report{}
	tree, err := Walk(src, rep)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	seen := 0
	for _, d := range tree.Volumes[0].Documents {
		if d.Path == "vol1/nested/deep.md" {
			seen++
			if d.Label != "d1-nested-deep" {
				t.Errorf("double-spine label = %q; want the first walk's", d.Label)
			}
		}
	}
	if seen != 1 {
		t.Errorf("the leaf appears %d times in the spine: %+v", seen, tree.Volumes[0].Documents)
	}
	if !containsReason(reasonsOf(rep.Findings(), walkForm, ""), "vol1/nested/deep.md") {
		t.Errorf("no finding names the repeat: %v", rep.Findings())
	}
}

// TestWalkOnceAcrossVolumes: where one volume's directory nests inside
// another's and the outer volume's spine reaches the inner index, every
// document walks once across the export, under the first volume that reaches
// it, and the later volume's reach is a finding.
func TestWalkOnceAcrossVolumes(t *testing.T) {
	files := map[string]string{
		"entry-point.md": "---\nkind: entry-point\n---\n# A KB\n\n- [A](a/index.md) — outer\n- [B](a/b/index.md) — inner\n",
		"a/index.md":     "---\nkind: index\n---\n[↑ A KB](../entry-point.md)\n\n# A\n\n- [B](b/index.md) — nested\n",
		"a/b/index.md":   "---\nkind: index\n---\n[↑ A](../index.md)\n\n# B\n\n- [X](x.md) — a leaf\n",
		"a/b/x.md":       "---\nkind: leaf\n---\n[↑ B](index.md)\n\n# X\n\nBody.\n",
	}
	rep := &Report{}
	tree, err := Walk(writeKB(t, files), rep)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(tree.Volumes) != 1 {
		t.Fatalf("Walk = %+v; want the one volume that reached a/b first", tree)
	}
	if got := tree.Targets()["a/b/x.md"]; got != (Target{Label: "d1-b-x", Volume: 1}) {
		t.Errorf("Targets()[a/b/x.md] = %+v; want the first walk's", got)
	}
	if !containsReason(reasonsOf(rep.Findings(), walkForm, "entry-point.md"), "a/b/index.md was already walked") {
		t.Errorf("no finding names the second reach: %v", rep.Findings())
	}
}

// reasonsOf is the reasons of findings raising form against srcPath, every
// finding's when srcPath is "".
func reasonsOf(findings []Finding, form, srcPath string) []string {
	var out []string
	for _, f := range findings {
		if f.Form == form && (srcPath == "" || f.Path == srcPath) {
			out = append(out, f.Reason)
		}
	}
	return out
}

// containsReason is whether any reason carries sub.
func containsReason(reasons []string, sub string) bool {
	for _, r := range reasons {
		if strings.Contains(r, sub) {
			return true
		}
	}
	return false
}
