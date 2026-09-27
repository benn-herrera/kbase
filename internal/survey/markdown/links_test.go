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

// resolveOne surveys a synthetic corpus built for one resolution case — the
// documents the case needs plus a linking document holding exactly one
// reference — and returns the link the survey recorded. Everything the table
// below decides is visible in that one link, and building the corpus per case
// is what lets each row state its own two-address-space geometry: which
// directory the linking page sits in, and which documents exist above and
// beside it. Nothing here touches the filesystem or the network.
func resolveOne(t *testing.T, docs []string, from, target string) survey.Link {
	t.Helper()
	files := map[string]string{from: "# From\n\n[l](" + target + ")\n"}
	for _, d := range docs {
		files[d] = "# Doc\n\nBody.\n"
	}
	_, art := surveyFixtures(t, files)
	links := fileOf(t, art, from).Links
	if len(links) != 1 {
		t.Fatalf("links = %+v, want the one reference", links)
	}
	return links[0]
}

// TestLinkResolutionAddressSpaces is the hermetic statement of §4.5's
// resolution rules: two address spaces tried in order, each running the same
// lookup, an escape disqualifying one ATTEMPT rather than the destination.
//
// It exists because the pinned-corpus census next door is the only other place
// URL space is exercised, and that census SKIPS when the corpus is absent —
// so deleting the second attempt left `just test` green while every cross-file
// reference on a doc-site corpus went dead. The row that catches exactly that
// deletion is "URL space: a page is served as a directory"; the URL-space rows
// under it fail with it, and the file-space rows do not, which is the other
// half of the claim (URL space is a fallback, never an override).
func TestLinkResolutionAddressSpaces(t *testing.T) {
	for _, tc := range []struct {
		name string
		// docs is the corpus besides the linking document.
		docs   []string
		from   string
		target string
		// want is compared whole; Target is filled in from the row.
		want survey.Link
	}{{
		// File space: the destination joined against the linking file's own
		// directory. This is what the path means to anything reading the
		// corpus as files, and the spelling a corpus that never rendered uses.
		name:   "file space: a sibling named exactly",
		docs:   []string{"guide/setup.md"},
		from:   "guide/links.md",
		target: "./setup.md",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "guide/setup.md"},
	}, {
		name:   "file space: a document one directory up",
		docs:   []string{"nested.md"},
		from:   "guide/links.md",
		target: "../nested.md",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "nested.md"},
	}, {
		// The site generator supplies the extension, so resolution has to.
		name:   "file space: the extension left to the site generator",
		docs:   []string{"guide/setup.md"},
		from:   "guide/links.md",
		target: "setup",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "guide/setup.md"},
	}, {
		name:   "rooted: joined against the corpus root, not the linking file",
		docs:   []string{"flat.md"},
		from:   "guide/links.md",
		target: "/flat.md",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "flat.md"},
	}, {
		// Rooted means the same thing in both spaces, so it is resolved ONCE.
		// The document here is exactly what a second, URL-space attempt would
		// have joined its way to (`project-format` + `/properties`), and a
		// rooted destination must not reach it: the root said root.
		name:   "rooted: resolved once, never re-joined in URL space",
		docs:   []string{"project-format/properties.md"},
		from:   "project-format.md",
		target: "/properties",
		want:   survey.Link{Kind: survey.LinkUnresolved},
	}, {
		name:   "rooted: a destination that climbs above the root",
		docs:   []string{"flat.md"},
		from:   "guide/links.md",
		target: "/../outside.md",
		want:   survey.Link{Kind: survey.LinkUnresolved},
	}, {
		// THE URL-SPACE CASE (MAD #2's B-1). A doc-site generator serves
		// `project-format.md` at `project-format/`, so a destination written
		// inside that page is relative to that directory: `../properties`
		// names the SIBLING document. In file space it climbs above the root
		// and is discarded — which is precisely why the escape may not
		// disqualify the destination, only the attempt. Delete the URL-space
		// attempt and this row is the first to fail.
		name:   "URL space: a page is served as a directory",
		docs:   []string{"properties.md"},
		from:   "project-format.md",
		target: "../properties",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "properties.md"},
	}, {
		name:   "URL space: a fragment rides along",
		docs:   []string{"properties.md"},
		from:   "project-format.md",
		target: "../properties#bool",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "properties.md", Fragment: "bool"},
	}, {
		// Extensionless is a convention of doc sites, not a requirement of the
		// second space: a fully spelled destination resolves there too.
		name:   "URL space: the extension spelled out",
		docs:   []string{"properties.md"},
		from:   "project-format.md",
		target: "../properties.md",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "properties.md"},
	}, {
		// Neither space is a second resolver: the case fold applies to each,
		// and the id returned is the corpus's own byte-exact spelling.
		name:   "URL space: the case fold applies here too",
		docs:   []string{"Properties.MD"},
		from:   "project-format.md",
		target: "../PROPERTIES",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "Properties.MD"},
	}, {
		// So does the Unicode spelling agreement: the corpus was ingested
		// under a decomposed name, the link is written composed, and both
		// sides are NFC by the time the second attempt compares them.
		name:   "URL space: a decomposed id reached by a composed destination",
		docs:   []string{"caf" + "e" + combiningAcute + ".md"},
		from:   "project-format.md",
		target: "../caf" + eAcute,
		want:   survey.Link{Kind: survey.LinkInternal, Path: "caf" + eAcute + ".md"},
	}, {
		// And the percent decode: the destination is decoded before either
		// attempt, so the escape an editor wrote is not a second spelling the
		// second space has to know about.
		name:   "URL space: a percent-encoded destination",
		docs:   []string{"properties guide.md"},
		from:   "project-format.md",
		target: "../properties%20guide",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "properties guide.md"},
	}, {
		// Order matters, and the corpus here can answer in both spaces:
		// `../properties` from `guide/project-format.md` is `properties.md`
		// in file space and `guide/properties.md` in URL space. The first
		// attempt that names a document wins, so URL space may not re-point an
		// edge file space already answered.
		name:   "file space wins when both spaces name a document",
		docs:   []string{"properties.md", "guide/properties.md"},
		from:   "guide/project-format.md",
		target: "../properties",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "properties.md"},
	}, {
		// The refusal that survives both attempts. Two levels up from a root
		// document leaves the corpus in file space AND in URL space, and a
		// destination is unresolved only when every attempt has failed.
		name:   "a destination that escapes the root in both spaces",
		docs:   []string{"flat.md"},
		from:   "index.md",
		target: "../../outside.md",
		want:   survey.Link{Kind: survey.LinkUnresolved},
	}, {
		// A destination carrying an extension gets none appended, and no
		// extension is exchanged for another: a destination naming
		// `advanced.mdx` against a corpus holding only `advanced.md` stays
		// unresolved. `.mdx` is a document extension now (Extensions), which is
		// exactly why this still has to hold — the two spellings name two
		// documents, and swapping one for the other would hand stage 8 an edge
		// the corpus never had.
		name:   "an .mdx destination is not swapped onto a .md document",
		docs:   []string{"advanced.md"},
		from:   "index.md",
		target: "advanced.mdx",
		want:   survey.Link{Kind: survey.LinkUnresolved},
	}, {
		// The other half of the same rule: an EXTENSIONLESS destination is what
		// a site generator completes, so it is tried against every extension
		// the adapter reads — an `.mdx` sibling answers it exactly as a `.md`
		// one does.
		name:   "an extensionless destination resolves to an .mdx document",
		docs:   []string{"advanced.mdx"},
		from:   "index.md",
		target: "advanced",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "advanced.mdx"},
	}, {
		// And the order is the corpus's own site generator's: where both
		// spellings exist, the extensionless destination names the primary one.
		name:   "the primary extension wins an extensionless destination",
		docs:   []string{"advanced.md", "advanced.mdx"},
		from:   "index.md",
		target: "advanced",
		want:   survey.Link{Kind: survey.LinkInternal, Path: "advanced.md"},
	}, {
		// A fragment is recorded whatever the path part turned out to be: the
		// rebase stage needs it to place an exempted destination (§4.5 rule 3).
		name:   "a fragment on an unresolved destination is still recorded",
		docs:   []string{"flat.md"},
		from:   "index.md",
		target: "./gone.md#top",
		want:   survey.Link{Kind: survey.LinkUnresolved, Fragment: "top"},
	}, {
		// Fragment-only: the anchor branch decides before any resolution runs,
		// so neither address space ever sees it.
		name:   "a fragment-only destination never reaches the resolver",
		docs:   []string{"flat.md"},
		from:   "index.md",
		target: "#local",
		want:   survey.Link{Kind: survey.LinkAnchor, Fragment: "local"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveOne(t, tc.docs, tc.from, tc.target)
			want := tc.want
			want.Target = tc.target
			if got != want {
				t.Errorf("link = %+v, want %+v", got, want)
			}
		})
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
