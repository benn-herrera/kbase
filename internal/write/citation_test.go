package write

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/query"
	"kbase/internal/result"
)

// citeLeaf hosts clm-aaaaaa under # A and clm-bbbbbb under ## B, inside A's
// section.
const citeLeaf = "[↑ Entry](entry-point.md)\n\n# A\n\nClaim A holds.<!-- kbase-id: clm-aaaaaa -->\n\n## B\n\n" +
	"Claim B is the bound.<!-- kbase-id: clm-bbbbbb -->\n"

// citeKB is a KB of two hosted claims in citeLeaf, a work and an axiom, its
// other.md holding body as its line 5.
func citeKB(t *testing.T, body string) string {
	t.Helper()
	root := recordKB(t, kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Claim A", Rationale: "Shown."},
			{ID: "clm-bbbbbb", Title: "Bound Claim", Rationale: "Shown by the bound argument."}},
		Works:  []kb.WorkNode{{ID: "wrk-s00000", Label: "Smith2020", Title: "Old Work", Rationale: "Cited."}},
		Axioms: []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "Closure"}},
	})
	for rel, text := range map[string]string{kb.EntryPointFile: "# Entry\n\n- [A](leaf.md)\n- [Other](other.md)\n",
		"leaf.md": citeLeaf, "other.md": "[↑ Entry](entry-point.md)\n\n# Other\n\n" + body + "\n",
		kb.AxiomsFile: "# Axioms\n\n" + kb.AxiomHeadingLine("S1", "Closure", "axm-s10000") + "\n\nIt closes.\n"} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// citeEntry is an add-citation batch of one entry on other.md's line 5.
func citeEntry(id, snippet string, quote ...string) string {
	entry := "entry:\n- document: other.md\n  line: 5\n  snippet: \"" + snippet + "\"\n  id: " + id + "\n"
	for _, q := range quote {
		entry += "  quote: \"" + q + "\"\n"
	}
	return entry
}

// otherLine is other.md's line 5.
func otherLine(t *testing.T, root string) string {
	t.Helper()
	return kb.SplitLines(recordFileText(t, root, "other.md"))[4]
}

// TestAddCitationWritesEachForm: the quote, paraphrase and naming forms over a
// hosted node, and the naming form over an axiom, each replace the snippet
// with the link to the node's target, reporting document, line, id and link;
// an axiom's quote or paraphrase is refused at snippet.
func TestAddCitationWritesEachForm(t *testing.T) {
	for _, tc := range []struct {
		name, body, id, snippet, link string
		quote                         []string
	}{
		{"quote", "As Claim B is the bound, we go on.", "clm-bbbbbb", "Claim B is the bound",
			`["Claim B is the bound"](leaf.md#b "clm-bbbbbb")`, nil},
		{"paraphrase", "As the limit is tight, we go on.", "clm-bbbbbb", "the limit is tight",
			`[the limit is tight](leaf.md#b "clm-bbbbbb")`, []string{"is the bound"}},
		{"naming a claim", "As clm-bbbbbb shows, we go on.", "clm-bbbbbb", "clm-bbbbbb",
			`[Bound Claim](leaf.md#b "clm-bbbbbb")`, nil},
		{"naming an axiom", "By axm-s10000 it closes.", "axm-s10000", "axm-s10000",
			`[Closure](axioms.md#axiom-s1-closure "axm-s10000")`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := citeKB(t, tc.body)
			e := onlyEntry(t, run(t, root, AddCitation, citeEntry(tc.id, tc.snippet, tc.quote...)))
			if e.Document != "other.md" || e.Line != 5 || e.ID != tc.id || e.Link != tc.link {
				t.Errorf("entry = %+v, want other.md line 5, %s and link %s", e, tc.id, tc.link)
			}
			want := strings.Replace(tc.body, tc.snippet, tc.link, 1)
			if got := otherLine(t, root); got != want {
				t.Errorf("other.md line 5 = %q, want %q", got, want)
			}
		})
	}
	for _, tc := range []struct{ name, values string }{
		{"quoting an axiom", citeEntry("axm-s10000", "it closes")},
		{"paraphrasing an axiom", citeEntry("axm-s10000", "it closes", "It closes")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := citeKB(t, "By axm-s10000 it closes.")
			res := run(t, root, AddCitation, tc.values)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "snippet" {
				t.Errorf("result = %+v, want refused at snippet", res)
			}
		})
	}
}

// TestAddCitationMatchesTheSnippetVerbatim: page add-citation says the snippet
// is "exactly the text on that line that becomes the link text", so one
// copying a double space from the line matches, and is not collapsed first.
func TestAddCitationMatchesTheSnippetVerbatim(t *testing.T) {
	root := citeKB(t, "As said, the bound  holds here.")
	res := run(t, root, AddCitation, citeEntry("clm-bbbbbb", "the bound  holds", "the bound"))
	if res.Outcome != result.Done {
		t.Fatalf("a snippet that is exactly a span of the line: %s %+v", res.Outcome, res.Refusals)
	}
	if got := otherLine(t, root); !strings.Contains(got, "[the bound  holds](") {
		t.Errorf("line = %q, want the double-spaced span as the link text", got)
	}
}

// TestASnippetTwiceOnTheLineIsRefused: a snippet occurring twice on the named
// line is refused, naming each column, and nothing is written.
func TestASnippetTwiceOnTheLineIsRefused(t *testing.T) {
	root := citeKB(t, "the bound and the bound")
	before := snapshot(t, root)
	res := run(t, root, AddCitation, citeEntry("clm-bbbbbb", "the bound"))
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "snippet" ||
		!strings.Contains(res.Refusals[0].Detail, "columns 1, 15") {
		t.Errorf("result = %+v, want refused at snippet naming columns 1, 15", res)
	}
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Error("a refused citation wrote to the KB")
	}
}

// TestAnUnbalancedBracketIsRefused: a snippet carrying an unbalanced [ or ]
// would write a link the link reader does not see as the citation, so it is
// refused at snippet, and nothing is written.
func TestAnUnbalancedBracketIsRefused(t *testing.T) {
	for _, snippet := range []string{"the [limit is tight", "the limit] is tight"} {
		t.Run(snippet, func(t *testing.T) {
			root := citeKB(t, "As "+snippet+", we go on.")
			before := snapshot(t, root)
			res := run(t, root, AddCitation, citeEntry("clm-bbbbbb", snippet, "is the bound"))
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "snippet" ||
				!strings.Contains(res.Refusals[0].Detail, "unbalanced") {
				t.Errorf("result = %+v, want refused at snippet for the unbalanced bracket", res)
			}
			if after := snapshot(t, root); !maps.Equal(before, after) {
				t.Error("a refused citation wrote to the KB")
			}
		})
	}
}

// TestACitationAnchorsAtTheNearestHeading: the anchor is the heading nearest
// above the cited node's marker, and the quote is read from that node's
// statement alone, never the whole enclosing section: B's text cited as
// clm-bbbbbb links #b, and the same text cited as clm-aaaaaa, whose # A
// section holds it, is refused as absent from its statement.
func TestACitationAnchorsAtTheNearestHeading(t *testing.T) {
	root := citeKB(t, "As Claim B is the bound, we go on.")
	if e := onlyEntry(t, run(t, root, AddCitation, citeEntry("clm-bbbbbb", "Claim B is the bound"))); !strings.Contains(e.Link, "(leaf.md#b ") {
		t.Errorf("link = %s, want it anchored #b", e.Link)
	}
	root = citeKB(t, "As Claim B is the bound, we go on.")
	res := run(t, root, AddCitation, citeEntry("clm-aaaaaa", "Claim B is the bound"))
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "snippet" ||
		!strings.Contains(res.Refusals[0].Detail, "does not appear in the statement of clm-aaaaaa") {
		t.Errorf("result = %+v, want refused as absent from clm-aaaaaa's statement", res)
	}
}

// TestAQuoteNotInTheStatementTakesItsKeysRemedy: a snippet given without quote
// that is not in the cited statement, and a quote that is not, are refused at
// their key, each with its own key's remedy.
func TestAQuoteNotInTheStatementTakesItsKeysRemedy(t *testing.T) {
	for _, tc := range []struct {
		key, entry string
	}{
		{"snippet", citeEntry("clm-aaaaaa", "Claim B is the bound")},
		{"quote", citeEntry("clm-aaaaaa", "the bound", "Claim B is the bound")},
	} {
		res := run(t, citeKB(t, "As Claim B is the bound, we go on."), AddCitation, tc.entry)
		if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != tc.key {
			t.Errorf("%s: result = %+v, want one refusal at %s", tc.key, res, tc.key)
			continue
		}
		if want := remedy(quoteRemedy(tc.key, "clm-aaaaaa", "leaf.md")); res.Refusals[0].Remedy != want {
			t.Errorf("%s: remedy = %q, want %q", tc.key, res.Refusals[0].Remedy, want)
		}
	}
	if quoteRemedy("snippet", "clm-aaaaaa", "leaf.md") == quoteRemedy("quote", "clm-aaaaaa", "leaf.md") {
		t.Error("the snippet's remedy is the quote's")
	}
}

// TestAHostedCitationTargetsItsHostNotTheSummaryPage: with the summary page
// written, its entry headings carrying the markers, a hosted node's citation
// still targets its host leaf and section, and a quote standing only in its
// summary entry, its rationale, is refused; a work's targets its root summary
// entry.
func TestAHostedCitationTargetsItsHostNotTheSummaryPage(t *testing.T) {
	root := citeKB(t, "As Claim B is the bound, and Shown by the bound argument., see wrk-s00000.")
	if res := run(t, root, AddClaim, "entry: []\n"); res.Outcome != result.Done {
		t.Fatalf("deriving: %+v", res)
	}
	if summary := recordFileText(t, root, kb.RegisterFile); !strings.Contains(summary, kb.StatementMarker("clm-bbbbbb")) ||
		!strings.Contains(summary, "Shown by the bound argument.") {
		t.Fatalf("%s carries no marker or rationale of clm-bbbbbb:\n%s", kb.RegisterFile, summary)
	}
	res := run(t, root, AddCitation, citeEntry("clm-bbbbbb", "Shown by the bound argument."))
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "snippet" {
		t.Errorf("citing the rationale: %+v, want refused at snippet", res)
	}
	if e := onlyEntry(t, run(t, root, AddCitation, citeEntry("clm-bbbbbb", "Claim B is the bound"))); !strings.HasSuffix(e.Link, `(leaf.md#b "clm-bbbbbb")`) {
		t.Errorf("link = %s, want leaf.md#b", e.Link)
	}
	if e := onlyEntry(t, run(t, root, AddCitation, citeEntry("wrk-s00000", "wrk-s00000"))); !strings.HasPrefix(e.Link, "[Old Work]("+kb.RegisterFile+"#") {
		t.Errorf("link = %s, want the root summary entry", e.Link)
	}
}

// TestCitationsOfFindsACitationAfterMoveNode: a citation of a node is found by
// the id in its link title once move-node has re-pointed it; one inside code
// is not read.
func TestCitationsOfFindsACitationAfterMoveNode(t *testing.T) {
	root := citeKB(t, "As [the bound](leaf.md#b \"clm-bbbbbb\") shows.\n\n```\n[x](leaf.md#b \"clm-bbbbbb\")\n```\n\nThe bound lives here now.")
	onlyEntry(t, run(t, root, MoveNode, "entry:\n- id: clm-bbbbbb\n  document: other.md\n  line: 11\n  snippet: lives here\n"))
	src, err := kbload.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := query.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	cites, err := ix.CitationsOf("clm-bbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if want := []query.Citation{{Path: "other.md", Line: 5, Text: "the bound"}}; len(cites) != 1 || cites[0] != want[0] {
		t.Errorf("citations-of = %+v, want %+v", cites, want)
	}
	if got := otherLine(t, root); !strings.Contains(got, `(other.md#other "clm-bbbbbb")`) {
		t.Errorf("other.md line 5 = %q, want the citation re-pointed to other.md#other", got)
	}
}

// TestAddCitationInAHeadingRepointsTheCitationsIntoIt: a naming-form citation
// written into a heading moves its anchor, and every citation of the leaf
// still resolves to the heading line it named before the entry.
func TestAddCitationInAHeadingRepointsTheCitationsIntoIt(t *testing.T) {
	root := seedKB(t)
	g := records(t, root)
	g.Claims = append(g.Claims, kb.ClaimNode{ID: "clm-bbbbbb", Title: "Second Result", Rigor: f64(0.5), Rationale: "Seeded."})
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(kb.GraphNodesFile)), kb.EncodeNodes(g), 0o644); err != nil {
		t.Fatal(err)
	}
	leaf := "[↑ Entry](entry-point.md)\n\n# Leaf\n\nThe first result holds here. <!-- kbase-id: " + seedClaim + " -->\n\n" +
		"## On " + seedClaim + "\n\nThe second result holds. <!-- kbase-id: clm-bbbbbb -->\n\n" +
		"See [the second](#on-" + seedClaim + " \"clm-bbbbbb\").\n"
	if err := os.WriteFile(filepath.Join(root, "leaf.md"), []byte(leaf), 0o644); err != nil {
		t.Fatal(err)
	}
	onlyEntry(t, run(t, root, AddCitation, "entry:\n- document: leaf.md\n  line: 7\n  snippet: "+seedClaim+"\n  id: "+seedClaim+"\n"))
	text := recordFileText(t, root, "leaf.md")
	lineOf := map[string]int{}
	for line, anchor := range kb.HeadingAnchors(text) {
		lineOf[anchor] = line
	}
	want := map[string]int{seedClaim: 2, "clm-bbbbbb": 6}
	for _, c := range kb.TextCitations(text) {
		_, anchor, _ := strings.Cut(c.Destination, "#")
		if line, ok := lineOf[anchor]; !ok || line != want[c.Title] {
			t.Errorf("the citation of %s points at #%s, want the heading on line %d (anchors %v)\n%s", c.Title, anchor, want[c.Title], lineOf, text)
		}
	}
}

// TestACitationIntoItsOwnSectionHeadingPointsAtItsNewAnchor: a naming-form
// citation written into the heading of the cited node's own section links,
// and reports, the anchor that heading has once it holds the citation.
func TestACitationIntoItsOwnSectionHeadingPointsAtItsNewAnchor(t *testing.T) {
	root := seedKB(t)
	leaf := "[↑ Entry](entry-point.md)\n\n## On " + seedClaim + "\n\nIt holds. <!-- kbase-id: " + seedClaim + " -->\n"
	if err := os.WriteFile(filepath.Join(root, "leaf.md"), []byte(leaf), 0o644); err != nil {
		t.Fatal(err)
	}
	e := onlyEntry(t, run(t, root, AddCitation, "entry:\n- document: leaf.md\n  line: 3\n  snippet: "+seedClaim+"\n  id: "+seedClaim+"\n"))
	link := `[Seed Result](leaf.md#on-seed-result "` + seedClaim + `")`
	if got := kb.SplitLines(recordFileText(t, root, "leaf.md"))[2]; got != "## On "+link || e.Link != link {
		t.Errorf("line 3 = %q, link = %q; want the link %s", got, e.Link, link)
	}
}

// TestAddCitationInAHeadingRepointsEveryLinkIntoIt: a naming-form citation
// written into a heading moves its anchor, and every link into that heading,
// in its own document or another, is re-pointed to the anchor it has after the
// entry, an ordinary link as a citation is.
func TestAddCitationInAHeadingRepointsEveryLinkIntoIt(t *testing.T) {
	root := seedKB(t)
	leaf := "[↑ Entry](entry-point.md)\n\n# Leaf\n\nThe first result holds here. <!-- kbase-id: " + seedClaim + " -->\n\n" +
		"## On " + seedClaim + "\n\nSee [that section](#on-" + seedClaim + ").\n"
	writeDocs(t, root, map[string]string{"leaf.md": leaf,
		kb.EntryPointFile: "# Entry\n\n- [Leaf](leaf.md)\n- [On the seed](leaf.md#on-" + seedClaim + ")\n"})
	onlyEntry(t, run(t, root, AddCitation, "entry:\n- document: leaf.md\n  line: 7\n  snippet: "+seedClaim+"\n  id: "+seedClaim+"\n"))
	text := recordFileText(t, root, "leaf.md")
	anchor := kb.HeadingAnchors(text)[6]
	if got, want := kb.SplitLines(text)[8], "See [that section](#"+anchor+")."; got != want {
		t.Errorf("leaf.md line 9 = %q, want %q", got, want)
	}
	if got, want := kb.SplitLines(recordFileText(t, root, kb.EntryPointFile))[3], "- [On the seed](leaf.md#"+anchor+")"; got != want {
		t.Errorf("entry-point.md line 4 = %q, want %q", got, want)
	}
}
