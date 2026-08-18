package distill

import (
	"path"
	"strings"
	"testing"

	"kbase/internal/treeplan"
)

func TestRelPath(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		want     string
	}{
		{"leaf to its sibling index", "guide/sync.md", "guide/index.md", "index.md"},
		{"index to its parent index", "guide/deep/index.md", "guide/index.md", "../index.md"},
		{"domain index to the entry-point", "guide/index.md", "entry-point.md", "../entry-point.md"},
		{"entry-point to a domain", "entry-point.md", "guide/index.md", "guide/index.md"},
		{"index to its own child", "guide/index.md", "guide/deep/index.md", "deep/index.md"},
		{"across two branches", "a/b/x.md", "c/d/y.md", "../../c/d/y.md"},
		{"root leaf to root leaf", "a.md", "b.md", "b.md"},
		{"deep leaf up two levels", "a/b/c/x.md", "a/index.md", "../../index.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RelPath(tt.from, tt.to); got != tt.want {
				t.Errorf("RelPath(%q, %q) = %q, want %q", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// A navigation link's two projections: the label is the destination's
// root-relative path, the target is the same node relative to the page, and one
// call renders both — so the pair a reader resolves is the pair the tree plan
// holds. The same rule on all three directions, because a continuation edge is
// the same projection of the same artifact as the up-link [MAD2: B-4].
func TestNavLinkProjections(t *testing.T) {
	tests := []struct {
		name       string
		render     func(string, string) string
		marker     string
		node, dest string
		want       string
	}{
		{"a leaf under a domain index", UpLink, UpMarker,
			"guide/sync.md", "guide/index.md", "[↑ guide/index.md](index.md)"},
		{"a subtopic index under its domain", UpLink, UpMarker,
			"guide/deep/index.md", "guide/index.md", "[↑ guide/index.md](../index.md)"},
		{"a domain index under the entry-point", UpLink, UpMarker,
			"guide/index.md", "entry-point.md", "[↑ entry-point.md](../entry-point.md)"},
		{"a depth-2 leaf", UpLink, UpMarker,
			"guide/deep/sync.md", "guide/deep/index.md", "[↑ guide/deep/index.md](index.md)"},
		{"a part's previous sibling", PrevLink, PrevMarker,
			"guide/sync-2.md", "guide/sync-1.md", "[← guide/sync-1.md](sync-1.md)"},
		{"a part's next sibling", NextLink, NextMarker,
			"guide/sync-2.md", "guide/sync-3.md", "[→ guide/sync-3.md](sync-3.md)"},
		{"a part at the tree root", NextLink, NextMarker,
			"sync-1.md", "sync-2.md", "[→ sync-2.md](sync-2.md)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.render(tt.node, tt.dest)
			if got != tt.want {
				t.Fatalf("the link from %q to %q is %q, want %q", tt.node, tt.dest, got, tt.want)
			}
			link, ok := ParseNavLink(got)
			if !ok {
				t.Fatalf("ParseNavLink does not read what the renderer wrote: %q", got)
			}
			if link.Marker != tt.marker {
				t.Errorf("the marker is %q, want %q", link.Marker, tt.marker)
			}
			if link.Label != tt.dest {
				t.Errorf("the label is %q, want the destination's root-relative path %q", link.Label, tt.dest)
			}
			if resolved := path.Clean(path.Join(path.Dir(tt.node), link.Target)); resolved != link.Label {
				t.Errorf("the target %q resolves to %q, which is not the label %q",
					link.Target, resolved, link.Label)
			}
		})
	}
	if _, ok := ParseNavLink("- [Sync](sync.md) — a down-link"); ok {
		t.Error("a down-link bullet parsed as a navigation link")
	}
}

// The navigation block is POSITIONAL: it is the run of links under the
// frontmatter and it stops at the first line that is not one, so a
// navigation-shaped line in a leaf's verbatim body is content and no gate
// reads it as page grammar [MAD2: B-11].
func TestNavBlockStopsAtTheBody(t *testing.T) {
	page := Frontmatter("guide/sync-2.md") + "\n\n" +
		UpLink("guide/sync-2.md", "guide/index.md") + "\n" +
		PrevLink("guide/sync-2.md", "guide/sync-1.md") + "\n" +
		NextLink("guide/sync-2.md", "guide/sync-3.md") + "\n\n" +
		"# Sync (2/3)\n\nThe source's own furniture:\n\n" +
		"[← Back to the manual](https://example.com/manual)\n"

	_, body, ok := ParseFrontmatter([]byte(page))
	if !ok {
		t.Fatal("the fixture page does not open with a frontmatter block")
	}
	block := NavBlock(body)
	if len(block) != 3 {
		t.Fatalf("the block holds %d links, want the three above the body: %+v", len(block), block)
	}
	for i, want := range []string{UpMarker, PrevMarker, NextMarker} {
		if block[i].Marker != want {
			t.Errorf("link %d is marked %q, want %q", i+1, block[i].Marker, want)
		}
	}
	if got := NavBlock([]string{"", "# Title", "", UpLink("a.md", "b.md")}); len(got) != 0 {
		t.Errorf("NavBlock read %d links off a page that opens with a heading", len(got))
	}
}

// The up-link COUNT guarantee 2 takes is the block's, so a body carrying
// up-link lines of its own — fenced or not — contributes none of them. A leaf's
// body is verbatim source and a corpus that documents this grammar (a KB of a
// KB) shows up-link lines both ways; counting either would refuse the whole
// delivery with no remedy, since §4.2 forbids editing a leaf body
// [MAD2: B-11, ARCH F3].
func TestNavBlockCountsOnlyItsOwnUpLink(t *testing.T) {
	page := Frontmatter("guide/sync.md") + "\n\n" +
		UpLink("guide/sync.md", "guide/index.md") + "\n\n" +
		"# Sync\n\nEvery page opens with its up-link:\n\n" +
		UpLink("a/b.md", "a/index.md") + "\n\n" +
		"```\n" + UpLink("c/d.md", "c/index.md") + "\n```\n"

	_, body, ok := ParseFrontmatter([]byte(page))
	if !ok {
		t.Fatal("the fixture page does not open with a frontmatter block")
	}
	var ups []NavLink
	for _, l := range NavBlock(body) {
		if l.Marker == UpMarker {
			ups = append(ups, l)
		}
	}
	if len(ups) != 1 {
		t.Fatalf("the block holds %d up-links, want the one above the body: %+v", len(ups), ups)
	}
	if ups[0].Label != "guide/index.md" {
		t.Errorf("the block's up-link names %q, want the page's own parent", ups[0].Label)
	}
	if got := NavBlock([]string{"", "# Sync", "", UpLink("a/b.md", "a/index.md")}); len(got) != 0 {
		t.Errorf("NavBlock read %d links off a page whose body opens with a heading", len(got))
	}
}

// The frontmatter block, and the ruling that makes its reader forgiving: a
// field this build does not know is skipped, the block's own shape is not.
func TestFrontmatterReader(t *testing.T) {
	tests := []struct {
		name     string
		page     string
		wantLoc  string
		wantOK   bool
		wantBody string
	}{
		{"what the writer writes", Frontmatter("guide/sync.md") + "\n\nbody\n", "guide/sync.md", true, "body"},
		{"an unknown field is skipped, not refused",
			"---\nSchema: kbase.page/2\nLocation: guide/sync.md\nTags: a, b\n---\nbody\n",
			"guide/sync.md", true, "body"},
		{"a block with no Location is a block", "---\nSchema: kbase.page/2\n---\nbody\n", "", true, "body"},
		{"no block at all", "# Title\n\nbody\n", "", false, "# Title"},
		{"an unterminated block is not a block", "---\nLocation: guide/sync.md\n", "", false, "---"},
		{"a rule below the first line is not a block", "# Title\n\n---\n\nbody\n", "", false, "# Title"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, body, ok := ParseFrontmatter([]byte(tt.page))
			if ok != tt.wantOK || loc != tt.wantLoc {
				t.Errorf("ParseFrontmatter = (%q, ok=%t), want (%q, ok=%t)", loc, ok, tt.wantLoc, tt.wantOK)
			}
			if got := FirstLine(body); got != tt.wantBody {
				t.Errorf("the body opens with %q, want %q", got, tt.wantBody)
			}
		})
	}
}

func TestProvenanceValidate(t *testing.T) {
	tests := []struct {
		name string
		p    Provenance
		ok   bool
	}{
		{"complete", Provenance{CorpusHash: "abc", BuildDate: "2026-08-13"}, true},
		{"no corpus", Provenance{BuildDate: "2026-08-13"}, false},
		{"no date", Provenance{CorpusHash: "abc"}, false},
		{"blank date", Provenance{CorpusHash: "abc", BuildDate: "   "}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.p.Validate(); (err == nil) != tt.ok {
				t.Errorf("Validate() error = %v, want ok=%t", err, tt.ok)
			}
		})
	}
}

func TestFooterRoundTrip(t *testing.T) {
	p := Provenance{CorpusHash: "deadbeef", BuildDate: "2026-08-13"}
	page := []byte("# Title\n\nbody\n\n" + p.Footer() + "\n")
	if !HasFooter(page) {
		t.Fatalf("HasFooter is false for a page ending in %q", p.Footer())
	}
	if HasFooter([]byte("# Title\n\nbody\n")) {
		t.Fatal("HasFooter is true for a page with no receipt")
	}
	if !strings.Contains(p.Footer(), "deadbeef") || !strings.Contains(p.Footer(), "2026-08-13") {
		t.Fatalf("the receipt drops one of its own fields: %q", p.Footer())
	}
}

// The leaf template, and the one provisional in it: a slice that opens with a
// heading whose text IS the node's title keeps it and gets no synthesised H1
// (§4.2 item 3, narrowed [MAD2: B-8]).
func TestLeafTemplate(t *testing.T) {
	prov := Provenance{CorpusHash: "h", BuildDate: "2026-08-13"}
	node := treeplan.Node{Path: "guide/sync.md", Kind: treeplan.KindLeaf, Parent: "guide/index.md", Title: "Sync"}

	// wantHeading is the ONE heading line naming the node that the page must
	// carry: the body's own where it has one, the synthesised H1 where it does
	// not. Exactly one, in every case — that is the whole of the provisional.
	tests := []struct {
		name        string
		body        string
		wantHeading string
	}{
		{"a body opening on the node's title is not double-titled", "## Sync\n\ntext\n", "## Sync"},
		{"a headingless body gets the tree plan's title", "just text\n", "# Sync"},
		{"leading blank lines do not hide the heading", "\n\n# Sync\n\ntext\n", "# Sync"},
		{"a body with no trailing newline still ends cleanly", "text", "# Sync"},
		{"a closing marker run is not part of the heading's text", "## Sync ##\n\ntext\n", "## Sync ##"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(leaf(node, "", "", []byte(tt.body), prov))
			lines := strings.Split(got, "\n")
			loc, body, ok := ParseFrontmatter([]byte(got))
			if !ok || loc != node.Path {
				t.Errorf("the page does not open with its Location block (%q, ok=%t):\n%s", loc, ok, got)
			}
			if first := FirstLine(body); first != "[↑ guide/index.md](index.md)" {
				t.Errorf("the first line under the block is %q, want the up-link", first)
			}
			headings := 0
			for _, l := range lines {
				if strings.HasPrefix(l, "#") && strings.Contains(l, "Sync") {
					headings++
					if l != tt.wantHeading {
						t.Errorf("the page's heading is %q, want %q", l, tt.wantHeading)
					}
				}
			}
			if headings != 1 {
				t.Errorf("the page carries %d headings naming the node, want exactly 1:\n%s", headings, got)
			}
			if !strings.Contains(got, strings.TrimSpace(strings.TrimSuffix(tt.body, "\n"))) {
				t.Errorf("the body is not verbatim in the page:\n%s", got)
			}
			if !HasFooter([]byte(got)) {
				t.Errorf("the page does not end in its receipt:\n%s", got)
			}
			if !strings.HasSuffix(got, "\n") {
				t.Error("the page does not end in a newline")
			}
		})
	}
}

// The narrowed suppression predicate, stated as the three cases that decide it
// [MAD2: B-8]. The old rule fired on ANY opening heading, which delivered a
// part ≥2 with no statement of its own identity and a page announcing an
// identity it was not routed by.
func TestSynthesisedTitleFollowsTheHeadingsText(t *testing.T) {
	prov := Provenance{CorpusHash: "h", BuildDate: "2026-08-13"}
	tests := []struct {
		name        string
		title       string
		body        string
		wantTitleH1 bool
	}{
		{"the opening heading is the node's title", "Sync", "# Sync\n\ntext\n", false},
		{"the opening heading says something else", "Technical Details", "# Sync Details\n\ntext\n", true},
		{"a part opening at an interior heading", "Sync (2/3)", "## Capability contract\n\ntext\n", true},
		{"a part opening at the source's own H1", "Sync (1/3)", "# Sync\n\ntext\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := treeplan.Node{Path: "guide/sync.md", Kind: treeplan.KindLeaf,
				Parent: "guide/index.md", Title: tt.title}
			got := string(leaf(node, "", "", []byte(tt.body), prov))
			// Synthesis is visible as a SECOND heading above the body's own,
			// which is what suppression exists to avoid; the body's opening
			// heading is identical to the synthesised one in the suppressed
			// case, so counting is what tells the two apart.
			var headings []string
			for _, l := range strings.Split(got, "\n") {
				if strings.HasPrefix(l, "#") {
					headings = append(headings, l)
				}
			}
			want := 1
			if tt.wantTitleH1 {
				want = 2
			}
			if len(headings) != want {
				t.Fatalf("the page carries the headings %q, want %d:\n%s", headings, want, got)
			}
			if tt.wantTitleH1 && headings[0] != "# "+tt.title {
				t.Errorf("the page's first heading is %q, want the node's own title", headings[0])
			}
			// The body is verbatim either way: suppression never edits it, and
			// neither does synthesis.
			if !strings.Contains(got, tt.body) {
				t.Errorf("the body is not verbatim in the page:\n%s", got)
			}
		})
	}
}

// The continuation block on a three-part group: the first part has only a
// next, the last only a previous, and the middle one both — the asymmetry
// being the whole of what "walk the series in either direction" means.
func TestLeafContinuationLinks(t *testing.T) {
	prov := Provenance{CorpusHash: "h", BuildDate: "2026-08-13"}
	node := func(k int) treeplan.Node {
		return treeplan.Node{
			Path: "guide/sync-" + string(rune('0'+k)) + ".md", Kind: treeplan.KindLeaf,
			Parent: "guide/index.md", Title: "Sync", SplitGroup: "g0001", Part: k,
		}
	}
	tests := []struct {
		name       string
		part       int
		prev, next string
		want       []string
	}{
		{"the first part", 1, "", "guide/sync-2.md", []string{
			"[↑ guide/index.md](index.md)", "[→ guide/sync-2.md](sync-2.md)"}},
		{"a middle part", 2, "guide/sync-1.md", "guide/sync-3.md", []string{
			"[↑ guide/index.md](index.md)", "[← guide/sync-1.md](sync-1.md)", "[→ guide/sync-3.md](sync-3.md)"}},
		{"the last part", 3, "guide/sync-2.md", "", []string{
			"[↑ guide/index.md](index.md)", "[← guide/sync-2.md](sync-2.md)"}},
		{"an unsplit leaf", 1, "", "", []string{"[↑ guide/index.md](index.md)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := node(tt.part)
			page := leaf(n, tt.prev, tt.next, []byte("body\n"), prov)
			_, body, ok := ParseFrontmatter(page)
			if !ok {
				t.Fatalf("the page does not open with its frontmatter block:\n%s", page)
			}
			var got []string
			for _, l := range NavBlock(body) {
				got = append(got, navPrefix+l.Marker+" "+l.Label+"]("+l.Target+")")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("the navigation block is %q, want %q", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("navigation line %d is %q, want %q", i+1, got[i], tt.want[i])
				}
			}
			// One up-link, whatever the continuation edges are: guarantee 2's
			// count is about the up-link and nothing else.
			c := 0
			for _, l := range NavBlock(body) {
				if l.Marker == UpMarker {
					c++
				}
			}
			if c != 1 {
				t.Errorf("the navigation block holds %d up-links, want exactly one", c)
			}
		})
	}
}
