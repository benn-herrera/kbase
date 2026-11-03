package kb

import (
	"maps"
	"reflect"
	"testing"
)

func TestLineMarkers(t *testing.T) {
	cases := []struct {
		name, line string
		want       []LineMarker
	}{
		{"none", "plain text", nil},
		{"hand-spaced", "a <!--kbase-id:  clm-aaaaaa--> b", []LineMarker{{"clm-aaaaaa", 2, 30}}},
		{"two markers abutting", "x<!-- kbase-id: clm-aaaaaa --><!-- kbase-id: ana-bbbbbb -->",
			[]LineMarker{{"clm-aaaaaa", 1, 30}, {"ana-bbbbbb", 30, 59}}},
		{"one marker, two ids", "a <!-- kbase-id: clm-aaaaaa ana-bbbbbb -->",
			[]LineMarker{{"clm-aaaaaa", 2, 42}, {"ana-bbbbbb", 2, 42}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := LineMarkers(c.line)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("LineMarkers(%q) = %v, want %v", c.line, got, c.want)
			}
		})
	}
}

func TestStripLineMarkers(t *testing.T) {
	cases := []struct{ name, line, want string }{
		{"mid-line", "a<!-- kbase-id: clm-aaaaaa --> b", "a b"},
		{"tab before", "a\t<!-- kbase-id: clm-aaaaaa -->", "a"},
		{"hand-spaced", "a <!--kbase-id:  clm-aaaaaa-->", "a"},
		{"several ids", "a <!-- kbase-id: clm-aaaaaa ana-bbbbbb -->", "a"},
		{"two markers abutting", "x<!-- kbase-id: clm-aaaaaa --><!-- kbase-id: ana-bbbbbb -->", "x"},
		{"a 3.0.0 marker is no marker", "x <!-- claim-rigor: clm-aaaaaa -->", "x <!-- claim-rigor: clm-aaaaaa -->"},
		{"none", "plain", "plain"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StripLineMarkers(c.line); got != c.want {
				t.Errorf("StripLineMarkers(%q) = %q, want %q", c.line, got, c.want)
			}
		})
	}
}

func TestHosts(t *testing.T) {
	g := Graph{
		Claims:      []ClaimNode{{ID: "clm-aaaaaa"}, {ID: "clm-bbbbbb"}},
		Analyses:    []AnalysisNode{{ID: "ana-cccccc"}},
		Experiments: []ExperimentNode{{ID: "exp-dddddd"}},
	}
	leaves := map[string]string{
		"v/z.md": "# Z\n<!-- kbase-id: ana-cccccc -->\nA claim.<!-- kbase-id: clm-aaaaaa -->\n",
		"v/a.md": "# A\n\n<!-- kbase-id: ana-cccccc -->\n<!-- kbase-id: clm-eeeeee -->\n",
		"v/m.md": "# M\nOne.<!-- kbase-id: clm-bbbbbb -->\nTwo.<!-- kbase-id: clm-bbbbbb -->\n",
	}
	hosts, faults := Hosts(g, leaves)
	wantHosts := map[string]string{"clm-aaaaaa": "v/z.md", "clm-bbbbbb": "v/m.md", "ana-cccccc": "v/a.md"}
	if !maps.Equal(hosts, wantHosts) {
		t.Errorf("hosts = %v, want %v", hosts, wantHosts)
	}
	wantFaults := []HostFault{
		{ID: "clm-bbbbbb", Markers: []MarkerSite{{"v/m.md", 1}, {"v/m.md", 2}}},
		{ID: "ana-cccccc", Markers: []MarkerSite{{"v/a.md", 2}, {"v/z.md", 1}}},
		{ID: "exp-dddddd"},
	}
	if !reflect.DeepEqual(faults, wantFaults) {
		t.Errorf("faults = %+v, want %+v", faults, wantFaults)
	}
}

// TestHeadingAnchors: each heading outside a fence anchors at its slug in
// document order, a repeat suffixed, a marker ending the heading no part of
// it; AnchorSection finds a repeated heading's section by its suffixed anchor.
func TestHeadingAnchors(t *testing.T) {
	text := "# Lemma<!-- kbase-id: clm-aaaaaa --><!-- kbase-id: ana-aaaaaa -->\nFirst.\n" +
		"```\n# Not a heading\n```\n" +
		"## A — B\nSecond.\n# Lemma\nThird.\n# Lemma <!-- a note -->\nFourth.\n"
	want := map[int]string{0: "lemma", 5: "a--b", 7: "lemma-1", 9: "lemma-2"}
	if got := HeadingAnchors(text); !maps.Equal(got, want) {
		t.Errorf("HeadingAnchors = %v, want %v", got, want)
	}
	for anchor, body := range map[string]string{"lemma": "First.\n\n\n\n## A — B\nSecond.", "lemma-1": "Third.", "lemma-2": "Fourth."} {
		if got, ok := AnchorSection(text, anchor); !ok || got != body {
			t.Errorf("AnchorSection(%q) = %q, %t; want %q", anchor, got, ok, body)
		}
	}
	if got, want := HeadingAnchors("## Title ##\n"), map[int]string{0: "title"}; !maps.Equal(got, want) {
		t.Errorf("HeadingAnchors of a closed heading = %v, want %v", got, want)
	}
}

func TestDocumentKind(t *testing.T) {
	for _, tc := range []struct {
		path        string
		hasChildren bool
		want        string
	}{
		{EntryPointFile, true, DocumentEntryPoint},
		{EntryPointFile, false, DocumentEntryPoint},
		{"v/index.md", true, DocumentIndex},
		{"v/leaf.md", true, DocumentIndex},
		{"v/index.md", false, DocumentLeaf},
		{"v/" + EntryPointFile, false, DocumentLeaf},
	} {
		if got := DocumentKind(tc.path, tc.hasChildren); got != tc.want {
			t.Errorf("DocumentKind(%q, %v) = %q, want %q", tc.path, tc.hasChildren, got, tc.want)
		}
	}
}

// TestAxiomHeadingLineParsesBack: a composed axiom heading reads back through
// AxiomHeadings as its axiom's id and its text, label and title, whatever the
// label holds, and anchors at its text's slug.
func TestAxiomHeadingLineParsesBack(t *testing.T) {
	for _, c := range []struct{ label, title string }{{"S1", "Closure"}, {"a.b-c_9", "Stock is conserved: always"},
		{`$\mathcal{A}$: the first`, "Stock"}} {
		text := AxiomHeadingLine(c.label, c.title, "axm-s00000") + "\n"
		got := AxiomHeadings(text)
		if want := []AxiomHeading{{ID: "axm-s00000", Text: AxiomHeadingText(c.label, c.title), Line: 1}}; !reflect.DeepEqual(got, want) {
			t.Errorf("AxiomHeadings(AxiomHeadingLine(%q, %q)) = %+v, want %+v", c.label, c.title, got, want)
		}
		if anchor, ok := AxiomAnchor(text, "axm-s00000"); !ok || anchor != Slugify(AxiomHeadingText(c.label, c.title)) {
			t.Errorf("AxiomAnchor = %q, %t, want the slug of its text", anchor, ok)
		}
	}
}
