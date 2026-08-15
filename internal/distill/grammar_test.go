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

// The up-link's two projections: the label is the parent's root-relative path,
// the target is the same parent relative to the page, and one call renders
// both — so the pair a reader resolves is the pair the tree plan holds.
func TestUpLinkProjections(t *testing.T) {
	tests := []struct {
		name         string
		node, parent string
		want         string
	}{
		{"a leaf under a domain index", "guide/sync.md", "guide/index.md", "[↑ guide/index.md](index.md)"},
		{"a subtopic index under its domain", "guide/deep/index.md", "guide/index.md", "[↑ guide/index.md](../index.md)"},
		{"a domain index under the entry-point", "guide/index.md", "entry-point.md", "[↑ entry-point.md](../entry-point.md)"},
		{"a depth-2 leaf", "guide/deep/sync.md", "guide/deep/index.md", "[↑ guide/deep/index.md](index.md)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UpLink(tt.node, tt.parent)
			if got != tt.want {
				t.Fatalf("UpLink(%q, %q) = %q, want %q", tt.node, tt.parent, got, tt.want)
			}
			label, target, ok := ParseUpLink(got)
			if !ok {
				t.Fatalf("ParseUpLink does not read what UpLink wrote: %q", got)
			}
			if label != tt.parent {
				t.Errorf("the label is %q, want the parent's root-relative path %q", label, tt.parent)
			}
			if resolved := path.Clean(path.Join(path.Dir(tt.node), target)); resolved != label {
				t.Errorf("the target %q resolves to %q, which is not the label %q", target, resolved, label)
			}
		})
	}
	if _, _, ok := ParseUpLink("- [Sync](sync.md) — a down-link"); ok {
		t.Error("a down-link bullet parsed as an up-link")
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

// The leaf template, and the one provisional in it: a slice that opens with its
// own heading keeps it and gets no synthesised H1 (§4.1).
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
		{"a body with its own heading is not double-titled", "## Sync\n\ntext\n", "## Sync"},
		{"a headingless body gets the tree plan's title", "just text\n", "# Sync"},
		{"leading blank lines do not hide the heading", "\n\n# Sync\n\ntext\n", "# Sync"},
		{"a body with no trailing newline still ends cleanly", "text", "# Sync"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(leaf(node, []byte(tt.body), prov))
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
				if strings.HasPrefix(l, "#") && strings.HasSuffix(l, "Sync") {
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
