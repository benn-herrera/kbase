package distill

import (
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
			got := string(leaf(node, "Guide", []byte(tt.body), prov))
			lines := strings.Split(got, "\n")
			if lines[0] != "[↑ Guide](index.md)" {
				t.Errorf("line 1 is %q, want the up-link", lines[0])
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
