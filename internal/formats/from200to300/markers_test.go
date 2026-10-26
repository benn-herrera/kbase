package from200to300

import (
	"strings"
	"testing"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format300"
)

// leafListing is a 2.0.x leaf listing claims, its body after the frontmatter.
func leafListing(body string, claims ...string) string {
	return "---\nkind: leaf\nclaims: [" + strings.Join(claims, ", ") + "]\n---\n" + body
}

// leafUnlisting is a 2.0.x leaf listing no claim.
func leafUnlisting(body string) string { return "---\nkind: leaf\n---\n" + body }

var mark = format300.StatementMarker

// markerCase is a KB of the register entries and the documents by kb-root
// path; want is each named document's converted text and warnings the
// conversion's, each as path, the 2.0.x line holding a text, and a detail.
type markerCase struct {
	name     string
	entries  string
	docs     map[string]string
	want     map[string]string
	warnings []markerWarning
}

type markerWarning struct{ path, lineHolding, detail string }

func (tc markerCase) run(t *testing.T) {
	t.Helper()
	extra := common.Files{}
	for p, text := range tc.docs {
		extra["kb-root/"+p] = []byte(text)
	}
	entries := tc.entries
	if entries == "" {
		entries = entry("clm-aaaaaa", "Claim A", "- rigor: 0.5")
	}
	_, out, warnings := convert(t, kbFiles(entries, extra))
	for p, want := range tc.want {
		if got := string(out["kb-root/"+p]); got != want {
			t.Errorf("%s =\n%q\nwant\n%q", p, got, want)
		}
	}
	var want []common.Warning
	for _, w := range tc.warnings {
		want = append(want, common.Warning{Path: "kb-root/" + w.path, Line: lineOf(t, tc.docs[w.path], w.lineHolding), Detail: w.detail})
	}
	wantWarnings(t, warnings, want...)
}

const theorem = "> **Theorem**\n>\n> Claim A holds.\n"

// markedTheorem is theorem with clm-aaaaaa's marker on its first content line.
var markedTheorem = "> **Theorem**\n>\n> Claim A holds. " + mark("clm-aaaaaa") + "\n"

// TestMarkersFromListings: one case per 2.0.x state of a claim's listings and
// markers.
func TestMarkersFromListings(t *testing.T) {
	for _, tc := range []markerCase{
		{
			name: "listed in a leaf that marks it: that marker stands",
			docs: map[string]string{"a.md": leafListing("# A\n\nClaim A holds. "+mark("clm-aaaaaa")+"\n", "clm-aaaaaa")},
			want: map[string]string{"a.md": "# A\n\nClaim A holds. " + mark("clm-aaaaaa") + "\n"},
		},
		{
			name: "listed in two leaves, marked in none: a marker in the first, a mention in the other",
			docs: map[string]string{
				"a.md": leafListing("# A\n\n"+theorem, "clm-aaaaaa"),
				"b.md": leafListing("# B\n\nSee clm-aaaaaa.\n", "clm-aaaaaa"),
			},
			want: map[string]string{
				"a.md": "# A\n\n" + markedTheorem,
				"b.md": "# B\n\nSee clm-aaaaaa.\n",
			},
			warnings: []markerWarning{{"b.md", "claims:", "claims lists clm-aaaaaa, whose marker is in kb-root/a.md; this listing becomes a mention"}},
		},
		{
			name: "marked in a listing leaf and in another leaf: the listing leaf's kept",
			docs: map[string]string{
				"a.md": leafUnlisting("# A\n\n" + mark("clm-aaaaaa") + "\nClaim A, again.\n"),
				"b.md": leafListing("# B\n\nClaim A holds. "+mark("clm-aaaaaa")+"\n", "clm-aaaaaa"),
			},
			want: map[string]string{
				"a.md": "# A\n\nClaim A, again.\n",
				"b.md": "# B\n\nClaim A holds. " + mark("clm-aaaaaa") + "\n",
			},
			warnings: []markerWarning{{"a.md", mark("clm-aaaaaa"), "a marker for clm-aaaaaa is removed; its marker is in kb-root/b.md"}},
		},
		{
			name: "marked only in a leaf not listing it, listed elsewhere: inserted in the listing leaf",
			docs: map[string]string{
				"a.md": leafUnlisting("# A\n\nIgnored by 2.0.x. " + mark("clm-aaaaaa") + "\n"),
				"b.md": leafListing("# B\n\n"+theorem, "clm-aaaaaa"),
			},
			want: map[string]string{
				"a.md": "# A\n\nIgnored by 2.0.x.\n",
				"b.md": "# B\n\n" + markedTheorem,
			},
			warnings: []markerWarning{{"a.md", mark("clm-aaaaaa"), "a marker for clm-aaaaaa is removed; its marker is in kb-root/b.md"}},
		},
		{
			name: "marked only in leaves not listing it, listed nowhere: the first in path order kept",
			docs: map[string]string{
				"a.md": leafUnlisting("# A\n\nFirst. " + mark("clm-aaaaaa") + "\n"),
				"b.md": leafUnlisting("# B\n\nSecond. " + mark("clm-aaaaaa") + "\n"),
			},
			want: map[string]string{
				"a.md": "# A\n\nFirst. " + mark("clm-aaaaaa") + "\n",
				"b.md": "# B\n\nSecond.\n",
			},
			warnings: []markerWarning{{"b.md", mark("clm-aaaaaa"), "a marker for clm-aaaaaa is removed; its marker is in kb-root/a.md"}},
		},
		{
			name: "listed and marked nowhere: no marker, silently",
			docs: map[string]string{"a.md": leafUnlisting("# A\n\nClaim A holds.\n")},
			want: map[string]string{"a.md": "# A\n\nClaim A holds.\n"},
		},
	} {
		t.Run(tc.name, tc.run)
	}
}

const equationTitle = "Equation (`eq:one`) — A"

// TestMarkerInsertionSite: an equation's marker goes on its display-maths
// fence's \label line, a lone block claim's on the first content line of the
// leaf's one labelled block, silently; any other claim's on the first heading
// with a warning, and a leaf with no heading gets none, with a warning.
func TestMarkerInsertionSite(t *testing.T) {
	claims := entry("clm-aaaaaa", "Claim A", "- rigor: 0.5") + entry("clm-bbbbbb", "Claim B", "- rigor: 0.5") +
		entry("clm-eeeeee", equationTitle, "- rigor: 0.5")
	maths := "```latex\n\\label{eq:one}\n```\n\n```math\nx = 1 \\label{eq:one}\n```\n"
	for _, tc := range []markerCase{
		{
			name: "an equation: the \\label line inside its maths fence",
			docs: map[string]string{"a.md": leafListing("# A\n\n"+maths, "clm-eeeeee")},
			want: map[string]string{"a.md": "# A\n\n```latex\n\\label{eq:one}\n```\n\n```math\nx = 1 \\label{eq:one} " + mark("clm-eeeeee") + "\n```\n"},
		},
		{
			name: "a lone block claim beside an equation: the block's first content line",
			docs: map[string]string{"a.md": leafListing("# A\n\n"+theorem+"\n```math\nx = 1 \\label{eq:one}\n```\n", "clm-aaaaaa", "clm-eeeeee")},
			want: map[string]string{"a.md": "# A\n\n" + markedTheorem + "\n```math\nx = 1 \\label{eq:one} " + mark("clm-eeeeee") + "\n```\n"},
		},
		{
			name:     "an equation whose label no maths fence carries: the first heading",
			docs:     map[string]string{"a.md": leafListing("# A\n\nNo maths here.\n", "clm-eeeeee")},
			want:     map[string]string{"a.md": "# A " + mark("clm-eeeeee") + "\n\nNo maths here.\n"},
			warnings: []markerWarning{{"a.md", "claims:", "clm-eeeeee's marker is appended to the first heading"}},
		},
		{
			name: "two non-equation claims: the first heading, a warning each",
			docs: map[string]string{"a.md": leafListing("Up.\n\n## A\n\n"+theorem, "clm-aaaaaa", "clm-bbbbbb")},
			want: map[string]string{"a.md": "Up.\n\n## A " + mark("clm-aaaaaa") + " " + mark("clm-bbbbbb") + "\n\n" + theorem},
			warnings: []markerWarning{
				{"a.md", "claims:", "clm-aaaaaa's marker is appended to the first heading"},
				{"a.md", "claims:", "clm-bbbbbb's marker is appended to the first heading"},
			},
		},
		{
			name:     "a block with no content line: the first heading",
			docs:     map[string]string{"a.md": leafListing("# A\n\n> **Theorem**\n>\n\nAfter.\n", "clm-aaaaaa")},
			want:     map[string]string{"a.md": "# A " + mark("clm-aaaaaa") + "\n\n> **Theorem**\n>\n\nAfter.\n"},
			warnings: []markerWarning{{"a.md", "claims:", "clm-aaaaaa's marker is appended to the first heading"}},
		},
		{
			name:     "two label lines: the first heading",
			docs:     map[string]string{"a.md": leafListing("# A\n\n"+theorem+"\n> **Lemma**\n>\n> Another.\n", "clm-aaaaaa")},
			want:     map[string]string{"a.md": "# A " + mark("clm-aaaaaa") + "\n\n" + theorem + "\n> **Lemma**\n>\n> Another.\n"},
			warnings: []markerWarning{{"a.md", "claims:", "clm-aaaaaa's marker is appended to the first heading"}},
		},
		{
			name:     "no heading: no marker",
			docs:     map[string]string{"a.md": leafListing("Claim A holds.\n", "clm-aaaaaa")},
			want:     map[string]string{"a.md": "Claim A holds.\n"},
			warnings: []markerWarning{{"a.md", "claims:", "clm-aaaaaa gets no marker: kb-root/a.md has no heading"}},
		},
	} {
		tc.entries = claims
		t.Run(tc.name, tc.run)
	}
}

// TestBlockLabelLineSurvives: after a lone block claim's marker is inserted,
// its block's label line still reads as one, naming the block.
func TestBlockLabelLineSurvives(t *testing.T) {
	_, out, _ := convert(t, kbFiles(entry("clm-aaaaaa", "Claim A", "- rigor: 0.5"),
		common.Files{leafPath: []byte(leafListing("# A\n\n"+theorem, "clm-aaaaaa"))}))
	text := string(out[leafPath])
	if !strings.Contains(text, mark("clm-aaaaaa")) {
		t.Fatalf("no marker was inserted:\n%s", text)
	}
	var names []string
	for _, line := range strings.Split(text, "\n") {
		if name, ok := format300.LabelLine(line); ok {
			names = append(names, name)
		}
	}
	if len(names) != 1 || names[0] != "Theorem" {
		t.Errorf("label lines read %q, want [Theorem]:\n%s", names, text)
	}
}

// TestHostMarkers: an experiment's marker is written on a new line directly
// after its host's first heading; an analysis's marker that already stands
// stays where it is — beside the claim it argues for, which is the section its
// statement is read from — and no marker is written after the heading for it. A
// second marker for one node is dropped with a warning naming the one that
// stands.
func TestHostMarkers(t *testing.T) {
	host := "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n---\n\n" +
		"Up.\n# A Lemma\r\n\r\nClaim A holds. " + mark("clm-aaaaaa") + " " + mark("ana-aaaaaa") + "\r\n"
	markerCase{
		entries: entry("clm-aaaaaa", "Claim A", "- rigor: 0.5") + entry("ana-aaaaaa", "A Lemma", "- rigor: 0.8"),
		docs: map[string]string{
			"host.md":  host,
			"other.md": leafUnlisting("# Other\n\nStray. " + mark("ana-aaaaaa") + "\n"),
		},
		want: map[string]string{
			"host.md":  "\nUp.\n# A Lemma\r\n" + mark("exp-aaaaaa") + "\r\n\r\nClaim A holds. " + mark("clm-aaaaaa") + " " + mark("ana-aaaaaa") + "\r\n",
			"other.md": "# Other\n\nStray.\n",
		},
		warnings: []markerWarning{{"other.md", mark("ana-aaaaaa"), "a second marker for ana-aaaaaa is removed; its marker stands in kb-root/host.md"}},
	}.run(t)
}

// TestHostedMarkerOutsideItsDeclaringLeafWarns: an analysis declared in one
// leaf and marked in another is warned of at the declaration, naming both
// paths; the marker's leaf is the statement's home, as it is for a claim marked
// where no leaf lists it.
func TestHostedMarkerOutsideItsDeclaringLeafWarns(t *testing.T) {
	decl := "---\nkind: leaf\nanalysis-nodes:\n  - ana-id: ana-aaaaaa\n---\n\n# Declaring Leaf\n"
	marked := "---\nkind: leaf\n---\n\n# Marking Leaf\n\nStray. " + mark("ana-aaaaaa") + "\n"
	markerCase{
		entries: entry("clm-aaaaaa", "Claim A", "- rigor: 0.5") + entry("ana-aaaaaa", "A Lemma", "- rigor: 0.8"),
		docs: map[string]string{
			"decl.md":   decl,
			"marked.md": marked,
		},
		want: map[string]string{
			"decl.md":   "\n# Declaring Leaf\n",
			"marked.md": "\n# Marking Leaf\n\nStray. " + mark("ana-aaaaaa") + "\n",
		},
		warnings: []markerWarning{
			{"decl.md", "ana-id:", "ana-aaaaaa is declared in kb-root/decl.md, whose marker is in kb-root/marked.md; the marker's leaf is its statement's home"},
		},
	}.run(t)
}

// TestMarkersNamingNoNode: a listing of an id no claim carries writes no
// marker, a marker naming no node is removed, and a marker naming a node and
// an unknown id keeps the node's alone, each with a warning.
func TestMarkersNamingNoNode(t *testing.T) {
	both := "<!-- claim-rigor: clm-aaaaaa, clm-zzzzzz -->"
	markerCase{
		docs: map[string]string{
			"a.md": leafListing("# A\n\nClaim A holds. "+both+"\n", "clm-aaaaaa", "clm-yyyyyy"),
			"b.md": leafUnlisting("# B\n\n" + mark("clm-xxxxxx") + "\n\nText.\n"),
		},
		want: map[string]string{
			"a.md": "# A\n\nClaim A holds. " + mark("clm-aaaaaa") + "\n",
			"b.md": "# B\n\n\nText.\n",
		},
		warnings: []markerWarning{
			{"a.md", "claims:", "claims lists clm-yyyyyy, which names no claim"},
			{"a.md", both, "a marker for clm-zzzzzz, which names no node, is removed"},
			{"b.md", mark("clm-xxxxxx"), "a marker for clm-xxxxxx, which names no node, is removed"},
		},
	}.run(t)
}
