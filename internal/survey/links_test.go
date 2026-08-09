package survey

import (
	"slices"
	"strings"
	"testing"
)

// TestLinkClassification walks the destinations a real doc corpus contains
// and pins where each one is filed. The fixture links out of a subdirectory,
// so the relative cases exercise resolution against the LINKING file's
// directory rather than against the corpus root.
func TestLinkClassification(t *testing.T) {
	_, art := surveyFixtures(t, fixtures)
	f := fileOf(t, art, "guide/links.md")

	byTarget := make(map[string]Link, len(f.Links))
	for _, l := range f.Links {
		if _, dup := byTarget[l.Target]; dup {
			t.Errorf("target %q recorded twice", l.Target)
		}
		byTarget[l.Target] = l
	}

	for _, tc := range []struct {
		name   string
		target string
		want   Link
	}{
		{"sibling document", "./setup.md", Link{Kind: LinkInternal, Path: "guide/setup.md"}},
		{"parent directory", "../nested.md", Link{Kind: LinkInternal, Path: "nested.md"}},
		{"rooted at the corpus", "/flat.md", Link{Kind: LinkInternal, Path: "flat.md"}},
		{"extension left to the site generator", "setup", Link{Kind: LinkInternal, Path: "guide/setup.md"}},
		{"fragment on an internal target", "../nested.md#second", Link{Kind: LinkInternal, Path: "nested.md", Fragment: "second"}},
		{"same-file anchor", "#local", Link{Kind: LinkAnchor, Fragment: "local"}},
		{"absolute URL", "https://example.com/x", Link{Kind: LinkExternal}},
		{"autolink", "https://auto.example/", Link{Kind: LinkExternal}},
		{"mailto", "mailto:a@b.example", Link{Kind: LinkExternal}},
		{"target that does not exist", "./gone.md", Link{Kind: LinkUnresolved}},
		{"target outside the corpus root", "../../outside.md", Link{Kind: LinkUnresolved}},
		{"image, unresolved", "../img/logo.png", Link{Kind: LinkUnresolved, Image: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := byTarget[tc.target]
			if !ok {
				t.Fatalf("no link recorded for %q", tc.target)
			}
			want := tc.want
			want.Target = tc.target
			if got != want {
				t.Errorf("link = %+v, want %+v", got, want)
			}
		})
	}

	// An empty destination names nothing, so there is no edge to record.
	if _, ok := byTarget[""]; ok {
		t.Error("an empty destination must not produce a link")
	}
	if !slices.IsSortedFunc(f.Links, func(a, b Link) int { return strings.Compare(a.Target, b.Target) }) {
		t.Errorf("links must be sorted by target for a stable artifact; got %+v", f.Links)
	}
}

// TestLinkDeduplication: a document that links the same target from every
// section contributes one edge, not one per mention — but a link and an image
// to the same target are different references and both survive.
func TestLinkDeduplication(t *testing.T) {
	f := surveyOne(t, "repeat.md", `# Repeat

See [setup](setup.md), then [setup again](setup.md).

![setup](setup.md)
`)
	if len(f.Links) != 2 {
		t.Fatalf("links = %+v, want the link and the image only", f.Links)
	}
	if f.Links[0].Image || !f.Links[1].Image {
		t.Errorf("link must sort before image for the same target; got %+v", f.Links)
	}
}

// TestLinkCaseFolding: ingest accepts `.MD` because casing conveys nothing
// about a document, so resolution has to reach it however the link spells it.
// Without the fold, every link into a Windows-authored file inflates the
// unresolved count the link stage has to adjudicate.
func TestLinkCaseFolding(t *testing.T) {
	_, art, lg := surveyFixturesLogged(t, map[string]string{
		"guide/Setup.MD": "# Setup\n",
		"index.md":       "[a](guide/Setup.MD) [b](guide/setup.md) [c](GUIDE/SETUP.MD) [d](guide/setup)\n",
	})

	for _, l := range fileOf(t, art, "index.md").Links {
		if l.Kind != LinkInternal || l.Path != "guide/Setup.MD" {
			t.Errorf("link %q = %+v, want internal at the corpus's own byte-exact id", l.Target, l)
		}
	}
	if !lg.has(t, "debug", "target", "guide/setup.md") {
		t.Error("a link resolved by case fold must leave a debug record")
	}
}

// TestLinkAmbiguousFold: two documents that differ only by case fold to one
// key, and the corpus cannot say which one a link meant. Guessing would hand
// stage 8 a silently wrong edge, so the link is unresolved and loud.
func TestLinkAmbiguousFold(t *testing.T) {
	_, art, lg := surveyFixturesLogged(t, map[string]string{
		"Twin.md":  "# Upper\n",
		"twin.MD":  "# Lower\n",
		"index.md": "[ambiguous](TWIN.MD)\n",
	})

	links := fileOf(t, art, "index.md").Links
	if len(links) != 1 || links[0].Kind != LinkUnresolved {
		t.Fatalf("links = %+v, want the ambiguous target left unresolved", links)
	}
	if !lg.has(t, "warn", "target", "TWIN.MD") {
		t.Error("an ambiguous fold must warn, naming the target it refused to guess at")
	}
}

// TestLinkTotals: the corpus roll-up counts what the per-file entries hold —
// the unresolved figure in particular, which is the number the survey verb
// reports and the link stage will have to answer for.
func TestLinkTotals(t *testing.T) {
	_, art := surveyFixtures(t, fixtures)

	var got LinkTotals
	for _, f := range art.Files {
		for _, l := range f.Links {
			switch l.Kind {
			case LinkInternal:
				got.Internal++
			case LinkUnresolved:
				got.Unresolved++
			case LinkExternal:
				got.External++
			case LinkAnchor:
				got.Anchor++
			}
		}
	}
	if got != art.Corpus.Links {
		t.Errorf("link totals = %+v, want %+v", art.Corpus.Links, got)
	}
	if got.Unresolved == 0 || got.Internal == 0 || got.External == 0 || got.Anchor == 0 {
		t.Errorf("the fixture corpus should exercise every link kind; got %+v", got)
	}
}
