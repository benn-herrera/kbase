package markdown

import (
	"slices"
	"strings"
	"testing"

	"kbase/internal/survey"
)

// TestLinkClassification walks the destinations a real doc corpus contains
// and pins where each one is filed. The fixture links out of a subdirectory,
// so the relative cases exercise resolution against the LINKING file's
// directory rather than against the corpus root.
func TestLinkClassification(t *testing.T) {
	_, art := surveyFixtures(t, fixtures)
	f := fileOf(t, art, "guide/links.md")

	byTarget := make(map[string]survey.Link, len(f.Links))
	for _, l := range f.Links {
		if _, dup := byTarget[l.Target]; dup {
			t.Errorf("target %q recorded twice", l.Target)
		}
		byTarget[l.Target] = l
	}

	for _, tc := range []struct {
		name   string
		target string
		want   survey.Link
	}{
		{"sibling document", "./setup.md", survey.Link{Kind: survey.LinkInternal, Path: "guide/setup.md"}},
		{"parent directory", "../nested.md", survey.Link{Kind: survey.LinkInternal, Path: "nested.md"}},
		{"rooted at the corpus", "/flat.md", survey.Link{Kind: survey.LinkInternal, Path: "flat.md"}},
		{"extension left to the site generator", "setup", survey.Link{Kind: survey.LinkInternal, Path: "guide/setup.md"}},
		{"fragment on an internal target", "../nested.md#second", survey.Link{Kind: survey.LinkInternal, Path: "nested.md", Fragment: "second"}},
		{"same-file anchor", "#local", survey.Link{Kind: survey.LinkAnchor, Fragment: "local"}},
		{"absolute URL", "https://example.com/x", survey.Link{Kind: survey.LinkExternal}},
		{"autolink", "https://auto.example/", survey.Link{Kind: survey.LinkExternal}},
		{"mailto", "mailto:a@b.example", survey.Link{Kind: survey.LinkExternal}},
		{"target that does not exist", "./gone.md", survey.Link{Kind: survey.LinkUnresolved}},
		{"target outside the corpus root", "../../outside.md", survey.Link{Kind: survey.LinkUnresolved}},
		{"image, unresolved", "../img/logo.png", survey.Link{Kind: survey.LinkUnresolved, Image: true}},
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
	if !slices.IsSortedFunc(f.Links, func(a, b survey.Link) int { return strings.Compare(a.Target, b.Target) }) {
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

// TestLinkCaseFolding: the walk accepts `.MD` because casing conveys nothing
// about a document, so resolution has to reach it however the link spells it.
// Without the fold, every link into a Windows-authored file inflates the
// unresolved count the link stage has to adjudicate.
func TestLinkCaseFolding(t *testing.T) {
	_, art, lg := surveyFixturesLogged(t, map[string]string{
		"guide/Setup.MD": "# Setup\n",
		"index.md":       "[a](guide/Setup.MD) [b](guide/setup.md) [c](GUIDE/SETUP.MD) [d](guide/setup)\n",
	})

	for _, l := range fileOf(t, art, "index.md").Links {
		if l.Kind != survey.LinkInternal || l.Path != "guide/Setup.MD" {
			t.Errorf("link %q = %+v, want internal at the corpus's own byte-exact id", l.Target, l)
		}
	}
	if !lg.Has(t, "debug", "target", "guide/setup.md") {
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
	if len(links) != 1 || links[0].Kind != survey.LinkUnresolved {
		t.Fatalf("links = %+v, want the ambiguous target left unresolved", links)
	}
	if !lg.Has(t, "warn", "target", "TWIN.MD") {
		t.Error("an ambiguous fold must warn, naming the target it refused to guess at")
	}
}

// The two spellings of one document name, and the two of one accented
// character, built out of explicit code points: a combining mark pasted into
// a source literal is a fixture nobody can review.
const (
	combiningAcute = string(rune(0x0301))
	eAcute         = string(rune(0x00e9))

	nfdName = "guide/caf" + "e" + combiningAcute + ".md"
	nfcName = "guide/caf" + eAcute + ".md"
)

// TestLinkUnicodeSpelling: a reference reaches its document whichever of the
// two Unicode spellings each side was written in. Both sides are NFC by the
// time they meet — the id because ingest normalized it, the target because it
// came out of NFC custody bytes — so resolution stays plain byte equality and
// the four combinations collapse to one comparison.
func TestLinkUnicodeSpelling(t *testing.T) {
	for _, tc := range []struct {
		name       string
		corpusPath string // the spelling the corpus was ingested under
		target     string // the spelling the link was written in
	}{
		{"decomposed on disk, composed link", nfdName, nfcName},
		{"composed on disk, decomposed link", nfcName, nfdName},
		{"decomposed on both sides", nfdName, nfdName},
		{"composed on both sides", nfcName, nfcName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, art := surveyFixtures(t, map[string]string{
				tc.corpusPath: "# Cafe\n",
				"index.md":    "[cafe](" + tc.target + ")\n",
			})

			links := fileOf(t, art, "index.md").Links
			if len(links) != 1 {
				t.Fatalf("links = %+v, want the one reference", links)
			}
			// The resolved id is the corpus's own, which is NFC whatever the
			// corpus was handed; the target is preserved exactly as written.
			if links[0].Kind != survey.LinkInternal || links[0].Path != nfcName {
				t.Errorf("link = %+v, want internal at the NFC id %+q", links[0], nfcName)
			}
			// Target is the destination as it stands in CUSTODY, which is
			// the NFC spelling however the author typed it: the content
			// pre-pass reached a literal link destination before the parser
			// did. That is why resolution needs no normalization of its own.
			if links[0].Target != nfcName {
				t.Errorf("Target = %+q, want the custody spelling %+q", links[0].Target, nfcName)
			}
		})
	}
}

// TestLinkPercentDecodedSpelling: percent-encoding is the one way a target
// arrives in a spelling custody never saw. `%CC%81` is plain ASCII in the
// source bytes, so the NFC pre-pass has nothing to normalize there and the
// combining mark appears only after url.Parse decodes it — an editor that
// escapes a decomposed filename would otherwise produce a link that cannot
// resolve to a file that plainly exists.
func TestLinkPercentDecodedSpelling(t *testing.T) {
	for _, tc := range []struct{ name, target string }{
		{"decomposed", "guide/cafe%CC%81.md"},
		{"composed", "guide/caf%C3%A9.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, art := surveyFixtures(t, map[string]string{
				nfdName:    "# Cafe\n",
				"index.md": "[cafe](" + tc.target + ")\n",
			})

			links := fileOf(t, art, "index.md").Links
			if len(links) != 1 || links[0].Kind != survey.LinkInternal || links[0].Path != nfcName {
				t.Fatalf("links = %+v, want the escaped target resolved to the NFC id %+q", links, nfcName)
			}
		})
	}
}

// TestArtifactSpellingDeterminism: the artifact is the stage-3 input and its
// reproducibility is the corpus's ordering, so two corpora that differ only in
// how a filename was spelled must serialize to the same bytes — twice over, to
// catch a per-run source of order as well as a per-spelling one.
func TestArtifactSpellingDeterminism(t *testing.T) {
	render := func(corpusPath string) string {
		t.Helper()
		_, art := surveyFixtures(t, map[string]string{
			corpusPath:      "# Cafe\n\nBody.\n",
			"guide/cafz.md": "# Cafz\n\n[cafe](" + nfdName + ")\n",
			"index.md":      "# Index\n\n[cafe](/" + nfcName + ") [cafz](/guide/cafz.md)\n",
		})
		var b strings.Builder
		if err := art.WriteJSON(&b); err != nil {
			t.Fatalf("WriteJSON: %v", err)
		}
		return b.String()
	}

	want := render(nfdName)
	for _, got := range []string{render(nfdName), render(nfcName), render(nfcName)} {
		if got != want {
			t.Fatal("the artifact differs by the Unicode spelling of a filename, or between runs")
		}
	}
	// Guard against the whole comparison passing over an artifact that never
	// resolved anything: the sameness above is only worth something if the
	// links in it are edges.
	if !strings.Contains(want, `"kind": "internal"`) {
		t.Error("the fixture corpus must actually resolve its links")
	}
}

// TestLinkKindsExercised guards the corpus roll-up tested in internal/survey:
// that test sums hand-built files, so the fixture corpus is where every link
// kind has to actually occur.
func TestLinkKindsExercised(t *testing.T) {
	_, art := surveyFixtures(t, fixtures)

	var got survey.LinkTotals
	for _, f := range art.Files {
		for _, l := range f.Links {
			switch l.Kind {
			case survey.LinkInternal:
				got.Internal++
			case survey.LinkUnresolved:
				got.Unresolved++
			case survey.LinkExternal:
				got.External++
			case survey.LinkAnchor:
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
